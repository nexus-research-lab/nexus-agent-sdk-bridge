// INPUT: 宿主普通配置要求、新旧 nxs 和其他后端的能力响应。
// OUTPUT: 独立普通配置能力在任何任务写入前确认，变化要求替换进程。
// POS: Bridge 普通配置执行合同的准入和兼容性回归。
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

// settingsFilesSandboxSettings 构造显式宿主要求，不通过普通 settings 透传。
func settingsFilesSandboxSettings() *SandboxSettings {
	return &SandboxSettings{RequireSandbox: true, RequireFileTools: true, RequireSettingsFiles: true}
}

// TestSettingsFilesSandboxHostRequirement 保留复制和重启边界，拒绝其他 runtime 冒用同名能力。
func TestSettingsFilesSandboxHostRequirement(t *testing.T) {
	settings := settingsFilesSandboxSettings()
	settings.Extra = map[string]any{"requireSettingsFiles": false}
	options := NewOptions().WithSandbox(*settings)
	if !options.Sandbox.RequireSettingsFiles {
		t.Fatal("settings files requirement lost in options copy")
	}
	if _, exists := sandboxSettingsMap(options.Sandbox)["requireSettingsFiles"]; exists {
		t.Fatal("host settings files requirement entered ordinary settings")
	}
	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.RequireSettingsFiles = false
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("missing process replacement: %s", reason)
	}
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxSettingsFiles) {
			t.Fatal("unnegotiated settings files capability advertised")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxSettingsFilesProtocolCapability}})
		if core.supports(CapabilitySandboxSettingsFiles) != (kind == RuntimeNXS) {
			t.Fatalf("wrong backend accepted: %s", kind)
		}
	}
}

// TestSettingsFilesSandboxNegotiationBeforePrompt 缺少独立确认不得发送首条任务。
func TestSettingsFilesSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities []string
		accepted     bool
	}{
		{"files_only", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability}, false},
		{"settings_only", []string{sandboxSettingsFilesProtocolCapability}, false},
		{"all", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxSettingsFilesProtocolCapability}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settingsFilesSandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "settings files task") }()
			request := receiveWrite(t, tr)["request"].(map[string]any)
			if request["required_sandbox_settings_files"] != true || request["required_sandbox_file_tools"] != true || request["required_sandbox"] != true {
				t.Fatalf("missing requirements: %v", request)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "context", "protocol_capabilities": tc.capabilities}))
			if tc.accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("accepted settings files task not sent")
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
				if tc.name == "files_only" && unsupported.Capability != CapabilitySandboxSettingsFiles {
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

// TestSettingsFilesSandboxRequiresBaseContracts 依赖缺失必须在启动 transport 前拒绝。
func TestSettingsFilesSandboxRequiresBaseContracts(t *testing.T) {
	for _, settings := range []*SandboxSettings{{RequireSettingsFiles: true}, {RequireSettingsFiles: true, RequireSandbox: true}} {
		tr := newScriptedTransport()
		core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settings}, tr)
		if err := core.Connect(context.Background()); err == nil || core.isConnected() {
			t.Fatalf("invalid settings files requirements accepted: %v", err)
		}
		select {
		case write := <-tr.writes:
			t.Fatalf("invalid requirement reached transport: %v", write)
		default:
		}
	}
}

// TestSettingsFilesSandboxRealProcess 使用只缺普通配置能力的旧二进制，不发送模型请求。
func TestSettingsFilesSandboxRealProcess(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		accepted       bool
	}{
		{"current", "NEXUS_SANDBOX_TEST_BINARY", runtime.GOOS == "darwin"},
		{"legacy", "NEXUS_SETTINGS_SANDBOX_LEGACY_TEST_BINARY", false},
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
				Sandbox: settingsFilesSandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second}})
			if err != nil {
				if tc.accepted {
					t.Fatal(err)
				}
				if tc.name == "legacy" {
					var unsupported *UnsupportedCapabilityError
					if !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxSettingsFiles {
						t.Fatalf("legacy did not reach settings files capability rejection: %v", err)
					}
				}
				return
			}
			if closeErr := session.Close(context.Background()); closeErr != nil {
				t.Fatal(closeErr)
			}
			if !tc.accepted || !session.Supports(CapabilitySandboxSettingsFiles) {
				t.Fatal("unexpected settings files capability result")
			}
		})
	}
}
