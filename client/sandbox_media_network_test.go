// INPUT: 宿主媒体网络要求、新旧 nxs 和其他后端的能力响应。
// OUTPUT: 独立媒体网络能力在任何任务写入前确认，变化要求替换进程。
// POS: Bridge 媒体网络执行合同的准入和兼容性回归。
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

// mediaNetworkSandboxSettings 构造显式宿主要求，不通过普通 settings 透传。
func mediaNetworkSandboxSettings() *SandboxSettings {
	return &SandboxSettings{RequireSandbox: true, RequireFileTools: true, RequireMediaFiles: true, RequireMediaNetwork: true}
}

// TestMediaNetworkSandboxHostRequirement 保留复制和重启边界，拒绝其他 runtime 冒用同名能力。
func TestMediaNetworkSandboxHostRequirement(t *testing.T) {
	settings := mediaNetworkSandboxSettings()
	settings.Extra = map[string]any{"requireMediaNetwork": false}
	options := NewOptions().WithSandbox(*settings)
	if !options.Sandbox.RequireMediaNetwork {
		t.Fatal("media requirement lost in options copy")
	}
	if _, exists := sandboxSettingsMap(options.Sandbox)["requireMediaNetwork"]; exists {
		t.Fatal("host media requirement entered ordinary settings")
	}
	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.RequireMediaNetwork = false
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("missing process replacement: %s", reason)
	}
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxMediaNetwork) {
			t.Fatal("unnegotiated media capability advertised")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxMediaNetworkProtocolCapability}})
		if core.supports(CapabilitySandboxMediaNetwork) != (kind == RuntimeNXS) {
			t.Fatalf("wrong backend accepted: %s", kind)
		}
	}
}

// TestMediaNetworkSandboxNegotiationBeforePrompt 缺少独立确认不得发送首条任务。
func TestMediaNetworkSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities []string
		accepted     bool
	}{
		{"files_only", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxMediaFilesProtocolCapability}, false},
		{"media_only", []string{sandboxMediaNetworkProtocolCapability}, false},
		{"all", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxMediaFilesProtocolCapability, sandboxMediaNetworkProtocolCapability}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: mediaNetworkSandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "media task") }()
			request := receiveWrite(t, tr)["request"].(map[string]any)
			if request["required_sandbox_media_network"] != true || request["required_sandbox_media_files"] != true || request["required_sandbox_file_tools"] != true || request["required_sandbox"] != true {
				t.Fatalf("missing requirements: %v", request)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "media", "protocol_capabilities": tc.capabilities}))
			if tc.accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("accepted media task not sent")
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
				if tc.name == "files_only" && unsupported.Capability != CapabilitySandboxMediaNetwork {
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

// TestMediaNetworkSandboxRequiresBaseContracts 依赖缺失必须在启动 transport 前拒绝。
func TestMediaNetworkSandboxRequiresBaseContracts(t *testing.T) {
	for _, settings := range []*SandboxSettings{{RequireMediaNetwork: true}, {RequireMediaNetwork: true, RequireSandbox: true}, {RequireMediaNetwork: true, RequireSandbox: true, RequireFileTools: true}} {
		tr := newScriptedTransport()
		core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settings}, tr)
		if err := core.Connect(context.Background()); err == nil || core.isConnected() {
			t.Fatalf("invalid media requirements accepted: %v", err)
		}
		select {
		case write := <-tr.writes:
			t.Fatalf("invalid requirement reached transport: %v", write)
		default:
		}
	}
}

// TestMediaNetworkSandboxRealProcess 使用只缺媒体网络能力的旧二进制，不发送模型请求。
func TestMediaNetworkSandboxRealProcess(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		accepted       bool
	}{
		{"current", "NEXUS_SANDBOX_TEST_BINARY", runtime.GOOS == "darwin"},
		{"legacy", "NEXUS_MEDIA_NETWORK_SANDBOX_LEGACY_TEST_BINARY", false},
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
				Sandbox: mediaNetworkSandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second}})
			if err != nil {
				if tc.accepted {
					t.Fatal(err)
				}
				if tc.name == "legacy" {
					var unsupported *UnsupportedCapabilityError
					if !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxMediaNetwork {
						t.Fatalf("legacy did not reach media capability rejection: %v", err)
					}
				}
				return
			}
			if closeErr := session.Close(context.Background()); closeErr != nil {
				t.Fatal(closeErr)
			}
			if !tc.accepted || !session.Supports(CapabilitySandboxMediaNetwork) {
				t.Fatal("unexpected media capability result")
			}
		})
	}
}
