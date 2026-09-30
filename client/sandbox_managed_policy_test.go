// INPUT: 宿主托管策略要求、新旧 nxs 和其他后端的能力响应。
// OUTPUT: 独立托管策略能力在任何任务写入前确认，变化要求替换进程。
// POS: Bridge 托管策略执行合同的准入和兼容性回归。
package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

// managedPolicySandboxSettings 构造显式宿主要求，不通过普通 settings 透传。
func managedPolicySandboxSettings() *SandboxSettings {
	return &SandboxSettings{RequireSandbox: true, RequireFileTools: true, RequireManagedPolicy: true}
}

// TestManagedPolicySandboxHostRequirement 保留复制和重启边界，拒绝其他 runtime 冒用同名能力。
func TestManagedPolicySandboxHostRequirement(t *testing.T) {
	settings := managedPolicySandboxSettings()
	settings.Extra = map[string]any{"requireManagedPolicy": false}
	options := NewOptions().WithSandbox(*settings)
	if !options.Sandbox.RequireManagedPolicy {
		t.Fatal("managed policy requirement lost in options copy")
	}
	if _, exists := sandboxSettingsMap(options.Sandbox)["requireManagedPolicy"]; exists {
		t.Fatal("host managed policy requirement entered ordinary settings")
	}
	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.RequireManagedPolicy = false
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("missing process replacement: %s", reason)
	}
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxManagedPolicy) {
			t.Fatal("unnegotiated managed policy capability advertised")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxManagedPolicyProtocolCapability}})
		if core.supports(CapabilitySandboxManagedPolicy) != (kind == RuntimeNXS) {
			t.Fatalf("wrong backend accepted: %s", kind)
		}
	}
}

// TestManagedPolicySandboxNegotiationBeforePrompt 缺少独立确认不得发送首条任务。
func TestManagedPolicySandboxNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities []string
		accepted     bool
	}{
		{"files_only", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability}, false},
		{"managed_only", []string{sandboxManagedPolicyProtocolCapability}, false},
		{"all", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxManagedPolicyProtocolCapability}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: managedPolicySandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "managed policy task") }()
			request := receiveWrite(t, tr)["request"].(map[string]any)
			if request["required_sandbox_managed_policy"] != true || request["required_sandbox_file_tools"] != true || request["required_sandbox"] != true {
				t.Fatalf("missing requirements: %v", request)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "context", "protocol_capabilities": tc.capabilities}))
			if tc.accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("accepted managed policy task not sent")
			}
			err := receiveDone(t, done)
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%v err=%v", tc.accepted, err)
			}
			if !tc.accepted {
				var unsupported *UnsupportedCapabilityError
				if !errors.As(err, &unsupported) || core.isConnected() {
					t.Fatalf("missing typed rejection or still connected: %v", err)
				}
				if tc.name == "files_only" && unsupported.Capability != CapabilitySandboxManagedPolicy {
					t.Fatalf("wrong rejection: %v", err)
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

// TestManagedPolicySandboxRequiresBaseContracts 依赖缺失必须在启动 transport 前拒绝。
func TestManagedPolicySandboxRequiresBaseContracts(t *testing.T) {
	for _, settings := range []*SandboxSettings{{RequireManagedPolicy: true}, {RequireManagedPolicy: true, RequireSandbox: true}} {
		tr := newScriptedTransport()
		core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settings}, tr)
		if err := core.Connect(context.Background()); err == nil || core.isConnected() {
			t.Fatalf("invalid managed policy requirements accepted: %v", err)
		}
		select {
		case write := <-tr.writes:
			t.Fatalf("invalid requirement reached transport: %v", write)
		default:
		}
	}
}

// TestManagedPolicySandboxRealProcess 使用只缺托管策略能力的旧二进制，不发送模型请求。
func TestManagedPolicySandboxRealProcess(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		accepted       bool
	}{
		{"current", "NEXUS_SANDBOX_TEST_BINARY", runtime.GOOS == "darwin"},
		{"legacy", "NEXUS_MANAGED_SANDBOX_LEGACY_TEST_BINARY", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := os.Getenv(tc.variable)
			if binary == "" {
				t.Skip("set " + tc.variable + " to an explicit nxs binary")
			}
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			session, err := NewSession(ctx, Options{CLIPath: binary, CWD: root, Env: map[string]string{"NEXUS_CONFIG_DIR": filepath.Join(root, "config")},
				Sandbox: managedPolicySandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second}})
			if err != nil {
				if tc.accepted {
					t.Fatal(err)
				}
				if tc.name == "legacy" {
					var unsupported *UnsupportedCapabilityError
					if !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxManagedPolicy {
						t.Fatalf("legacy did not reach managed policy capability rejection: %v", err)
					}
				}
				return
			}
			if closeErr := session.Close(context.Background()); closeErr != nil {
				t.Fatal(closeErr)
			}
			if !tc.accepted || !session.Supports(CapabilitySandboxManagedPolicy) {
				t.Fatal("unexpected managed policy capability result")
			}
		})
	}
}
