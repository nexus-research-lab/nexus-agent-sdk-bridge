// INPUT: 分项沙箱能力、可信宿主要求与新旧 nxs
// OUTPUT: 原生文件能力缺失时在任务发送前断开，普通旧宿主保持兼容
// POS: Bridge 文件工具覆盖的独立准入回归
package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

// TestFileSandboxCapabilityBelongsToNXS 不把其他 runtime 的同名字段视为已适配能力。
func TestFileSandboxCapabilityBelongsToNXS(t *testing.T) {
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxFileTools) {
			t.Fatal("unnegotiated file capability advertised")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxFileToolsProtocolCapability}})
		if core.supports(CapabilitySandboxFileTools) != (kind == RuntimeNXS) {
			t.Fatalf("wrong runtime admitted: %s", kind)
		}
	}
}

// TestFileSandboxRequirementIsHostOnly 保留宿主选项，同时防止 Extra 把它塞入普通配置。
func TestFileSandboxRequirementIsHostOnly(t *testing.T) {
	options := NewOptions().WithSandbox(SandboxSettings{RequireSandbox: true, RequireFileTools: true,
		Extra: map[string]any{"requireFileTools": false}})
	if !options.Sandbox.RequireFileTools {
		t.Fatal("host requirement lost when copying options")
	}
	if _, exists := sandboxSettingsMap(options.Sandbox)["requireFileTools"]; exists {
		t.Fatal("host requirement serialized as ordinary settings")
	}
	next := options
	next.Sandbox = &SandboxSettings{RequireSandbox: true}
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("file requirement changed without replacement: %s", reason)
	}
}

// TestFileSandboxNegotiationBeforePrompt 不把命令能力当作文件能力，也不接受缺少基本合同的确认。
func TestFileSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		caps     []string
		accepted bool
	}{
		{"missing", nil, false},
		{"command_only", []string{requiredSandboxProtocolCapability}, false},
		{"file_only", []string{sandboxFileToolsProtocolCapability}, false},
		{"both", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr,
				Sandbox: &SandboxSettings{RequireSandbox: true, RequireFileTools: true},
				Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "file task") }()
			req := receiveWrite(t, tr)
			assertInitializeRequest(t, req)
			body := req["request"].(map[string]any)
			if body["required_sandbox"] != true || body["required_sandbox_file_tools"] != true {
				t.Fatalf("missing independent requirements: %v", body)
			}
			policy := body["sandbox_policy"].(map[string]any)
			if _, ok := policy["requireFileTools"]; ok {
				t.Fatal("host requirement leaked into ordinary settings")
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "files", "protocol_capabilities": tc.caps}))
			if tc.accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("accepted request did not send prompt")
			}
			err := receiveDone(t, done)
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%v error=%v", tc.accepted, err)
			}
			if !tc.accepted {
				var unsupported *UnsupportedCapabilityError
				if !errors.As(err, &unsupported) || core.isConnected() {
					t.Fatalf("missing typed rejection or still connected: %v", err)
				}
				select {
				case write := <-tr.writes:
					t.Fatalf("write after rejection: %v", write)
				default:
				}
			}
		})
	}
}

// TestFileSandboxRequiresMandatoryPolicy 配置矛盾必须在启动 transport 前拒绝。
func TestFileSandboxRequiresMandatoryPolicy(t *testing.T) {
	tr := newScriptedTransport()
	core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: &SandboxSettings{RequireFileTools: true}}, tr)
	if err := core.Connect(context.Background()); err == nil || core.isConnected() {
		t.Fatalf("invalid requirements accepted: %v", err)
	}
	select {
	case write := <-tr.writes:
		t.Fatalf("invalid requirements reached transport: %v", write)
	default:
	}
}

// TestFileSandboxRealProcess 确认真实新旧二进制的握手差异，不发送模型请求。
func TestFileSandboxRealProcess(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		accepted       bool
	}{
		{"current", "NEXUS_SANDBOX_TEST_BINARY", runtime.GOOS == "darwin"},
		{"legacy", "NEXUS_SANDBOX_LEGACY_TEST_BINARY", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := os.Getenv(tc.variable)
			if binary == "" {
				t.Skip("set " + tc.variable + " to an explicit nxs binary")
			}
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			session, err := NewSession(ctx, Options{CLIPath: binary, CWD: root,
				Env:     map[string]string{"NEXUS_CONFIG_DIR": filepath.Join(root, "config")},
				Sandbox: &SandboxSettings{RequireSandbox: true, RequireFileTools: true},
				Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second}})
			if err != nil {
				if tc.accepted {
					t.Fatal(err)
				}
				if tc.name == "legacy" {
					var unsupported *UnsupportedCapabilityError
					if !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxFileTools {
						t.Fatalf("legacy process did not reach file capability rejection: %v", err)
					}
				} else if !strings.Contains(err.Error(), "sandbox_file_tools_v1 is unavailable") {
					t.Fatalf("unsupported platform did not reach file capability rejection: %v", err)
				}
				return
			}
			defer session.Close(context.Background())
			if !tc.accepted || !session.Supports(CapabilitySandboxFileTools) {
				t.Fatal("unexpected file capability result")
			}
		})
	}
}
