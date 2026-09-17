// INPUT: 宿主普通配置写入要求、新旧 nxs 和其他后端的能力响应。
// OUTPUT: 独立写入能力在任何任务写入前确认，变化要求替换进程并改变指纹。
// POS: Bridge 普通配置受控写入合同的准入和兼容性回归。
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

// settingsWritesSandboxSettings 构造完整宿主要求，不通过普通 settings 透传。
func settingsWritesSandboxSettings() *SandboxSettings {
	return &SandboxSettings{
		RequireSandbox:        true,
		RequireFileTools:      true,
		RequireSettingsFiles:  true,
		RequireSettingsWrites: true,
	}
}

// TestSettingsWritesSandboxHostRequirement 保留复制、重启和指纹边界，拒绝其他 runtime 冒用能力。
func TestSettingsWritesSandboxHostRequirement(t *testing.T) {
	settings := settingsWritesSandboxSettings()
	settings.Extra = map[string]any{"requireSettingsWrites": false}
	options := NewOptions().WithCLIPath("nxs").WithSandbox(*settings)
	if !options.Sandbox.RequireSettingsWrites {
		t.Fatal("settings writes requirement lost in options copy")
	}
	if _, exists := sandboxSettingsMap(options.Sandbox)["requireSettingsWrites"]; exists {
		t.Fatal("host settings writes requirement entered ordinary settings")
	}

	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.RequireSettingsWrites = false
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("missing process replacement: %s", reason)
	}
	currentFingerprint, err := options.OptionsFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	nextFingerprint, err := next.OptionsFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if currentFingerprint.SandboxContract == nextFingerprint.SandboxContract ||
		currentFingerprint.RestartSensitive == nextFingerprint.RestartSensitive ||
		currentFingerprint.Full == nextFingerprint.Full {
		t.Fatalf("settings writes requirement missing from fingerprint: current=%+v next=%+v", currentFingerprint, nextFingerprint)
	}

	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxSettingsWrites) {
			t.Fatal("unnegotiated settings writes capability advertised")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxSettingsWritesProtocolCapability}})
		if core.supports(CapabilitySandboxSettingsWrites) != (kind == RuntimeNXS) {
			t.Fatalf("wrong backend accepted: %s", kind)
		}
	}
}

// TestSettingsWritesSandboxNegotiationBeforePrompt 要求链缺少任一确认都不得发送首条任务或暴露 Session。
func TestSettingsWritesSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name           string
		capabilities   []string
		accepted       bool
		wantCapability Capability
	}{
		{"base_only", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability}, false, CapabilitySandboxSettingsFiles},
		{"read_only", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxSettingsFilesProtocolCapability}, false, CapabilitySandboxSettingsWrites},
		{"writes_only", []string{sandboxSettingsWritesProtocolCapability}, false, CapabilityRequiredSandbox},
		{"all", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxSettingsFilesProtocolCapability, sandboxSettingsWritesProtocolCapability}, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{
				Transport: tr,
				Sandbox:   settingsWritesSandboxSettings(),
				Runtime:   RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second},
			}, tr)
			defer core.Disconnect(context.Background())

			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "settings writes task") }()
			request := receiveWrite(t, tr)["request"].(map[string]any)
			if request["required_sandbox"] != true ||
				request["required_sandbox_file_tools"] != true ||
				request["required_sandbox_settings_files"] != true ||
				request["required_sandbox_settings_writes"] != true {
				t.Fatalf("missing requirements: %v", request)
			}
			policy, ok := request["sandbox_policy"].(map[string]any)
			if !ok {
				t.Fatalf("missing sandbox policy: %v", request)
			}
			if _, exists := policy["requireSettingsWrites"]; exists {
				t.Fatal("host settings writes requirement entered ordinary settings")
			}

			tr.pushRead(successfulInitializeResponse(map[string]any{
				"session_id":            "must-not-leak-before-admission",
				"protocol_capabilities": tc.capabilities,
			}))
			if tc.accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("accepted settings writes task not sent")
			}
			err := receiveDone(t, done)
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%v err=%v", tc.accepted, err)
			}
			if tc.accepted {
				if core.SessionID() == "" || !core.supports(CapabilitySandboxSettingsWrites) {
					t.Fatal("accepted runtime did not expose negotiated session")
				}
				return
			}

			var unsupported *UnsupportedCapabilityError
			if !errors.As(err, &unsupported) || unsupported.Capability != tc.wantCapability || core.isConnected() {
				t.Fatalf("missing typed rejection or still connected: %v", err)
			}
			if core.SessionID() != "" {
				t.Fatalf("rejected runtime polluted session identity: %q", core.SessionID())
			}
			select {
			case write := <-tr.writes:
				t.Fatalf("write after rejection: %v", write)
			default:
			}
		})
	}
}

// TestSettingsWritesSandboxRequiresBaseContracts 依赖缺失必须在启动 transport 前拒绝。
func TestSettingsWritesSandboxRequiresBaseContracts(t *testing.T) {
	for _, settings := range []*SandboxSettings{
		{RequireSettingsWrites: true},
		{RequireSandbox: true, RequireSettingsWrites: true},
		{RequireSandbox: true, RequireFileTools: true, RequireSettingsWrites: true},
	} {
		tr := newScriptedTransport()
		core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settings}, tr)
		if err := core.Connect(context.Background()); err == nil || core.isConnected() {
			t.Fatalf("invalid settings writes requirements accepted: %v", err)
		}
		select {
		case write := <-tr.writes:
			t.Fatalf("invalid requirement reached transport: %v", write)
		default:
		}
	}
}

// TestSettingsWritesSandboxRejectsClaudeBeforeTransport 不把 Claude 原生沙箱当作 nxs 配置写合同。
func TestSettingsWritesSandboxRejectsClaudeBeforeTransport(t *testing.T) {
	tr := newScriptedTransport()
	core := newSessionCoreWithTransport(Options{
		Transport: tr,
		Sandbox:   settingsWritesSandboxSettings(),
		Runtime:   RuntimeOptions{Kind: RuntimeClaude},
	}, tr)
	if err := core.Connect(context.Background()); err == nil || core.isConnected() {
		t.Fatalf("Claude accepted nxs settings writes requirement: %v", err)
	}
	select {
	case write := <-tr.writes:
		t.Fatalf("Claude requirement reached transport: %v", write)
	default:
	}
}

// TestSettingsWritesSandboxGuardsAllMessagePaths 独立确认缺失时，普通、原始和内部消息均不得写入。
func TestSettingsWritesSandboxGuardsAllMessagePaths(t *testing.T) {
	tr := newScriptedTransport()
	core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settingsWritesSandboxSettings()}, tr)
	core.lifecycle.setConnected(true)
	core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{
		requiredSandboxProtocolCapability,
		sandboxFileToolsProtocolCapability,
		sandboxSettingsFilesProtocolCapability,
	}})
	for name, send := range map[string]func() error{
		"prompt":   func() error { return core.Query(context.Background(), "blocked") },
		"raw":      func() error { return core.SendRawMessage(context.Background(), map[string]any{"type": "user"}, "") },
		"internal": func() error { return core.sendInternalRawMessage(map[string]any{"type": "user"}, "") },
	} {
		t.Run(name, func(t *testing.T) {
			var unsupported *UnsupportedCapabilityError
			if err := send(); !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxSettingsWrites {
				t.Fatalf("message path bypassed writes capability: %v", err)
			}
		})
	}
	select {
	case write := <-tr.writes:
		t.Fatalf("message written without settings writes capability: %v", write)
	default:
	}
}

// TestSettingsWritesSandboxRealProcess 使用当前和只缺写能力的旧二进制核对握手，不发送模型请求。
func TestSettingsWritesSandboxRealProcess(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		accepted       bool
	}{
		{"current", "NEXUS_SANDBOX_TEST_BINARY", runtime.GOOS == "darwin"},
		{"legacy", "NEXUS_SETTINGS_WRITES_SANDBOX_LEGACY_TEST_BINARY", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := os.Getenv(tc.variable)
			if binary == "" {
				t.Skip("set " + tc.variable + " to an explicit nxs binary")
			}
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			session, err := NewSession(ctx, Options{
				CLIPath: binary,
				CWD:     root,
				Env:     map[string]string{"NEXUS_CONFIG_DIR": filepath.Join(root, "config")},
				Sandbox: settingsWritesSandboxSettings(),
				Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second},
			})
			if err != nil {
				if tc.accepted {
					t.Fatal(err)
				}
				if tc.name == "legacy" {
					var unsupported *UnsupportedCapabilityError
					if !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxSettingsWrites {
						t.Fatalf("legacy did not reach settings writes capability rejection: %v", err)
					}
				}
				return
			}
			if closeErr := session.Close(context.Background()); closeErr != nil {
				t.Fatal(closeErr)
			}
			if !tc.accepted || !session.Supports(CapabilitySandboxSettingsWrites) {
				t.Fatal("unexpected settings writes capability result")
			}
		})
	}
}
