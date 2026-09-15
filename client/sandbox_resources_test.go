// INPUT: 宿主资源选项、分项能力与真实新旧 nxs。
// OUTPUT: 资源合同独立发送；旧版本在用户任务之前拒绝且无静默直跑。
// POS: Bridge 资源准入与版本兼容回归。
package client

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

// resourceSandboxSettings 创建独立资源请求，scratch 的生命周期仍由测试宿主拥有。
func resourceSandboxSettings(t *testing.T) *SandboxSettings {
	t.Helper()
	return &SandboxSettings{RequireSandbox: true, RequireFileTools: true,
		Resources: &SandboxResourcePolicy{Version: 1, WriteScope: SandboxWriteScopeReadOnly, ScratchRoot: t.TempDir()},
	}
}

// TestSandboxResourcesHostAuthority 校验复制、普通 settings 和进程替换边界。
func TestSandboxResourcesHostAuthority(t *testing.T) {
	settings := resourceSandboxSettings(t)
	settings.Extra = map[string]any{"resources": map[string]any{"write_scope": "all"}}
	options := NewOptions().WithSandbox(*settings)
	settings.Resources.WriteScope = SandboxWriteScopeWorkspaceWrite
	if options.Sandbox.Resources.WriteScope != SandboxWriteScopeReadOnly {
		t.Fatal("source mutation changed resource policy")
	}
	for _, encode := range []func() ([]byte, error){
		func() ([]byte, error) { return json.Marshal(options.Sandbox) },
		func() ([]byte, error) { return json.Marshal(sandboxSettingsMap(options.Sandbox)) },
	} {
		encoded, err := encode()
		if err != nil {
			t.Fatal(err)
		}
		var ordinary map[string]any
		if err := json.Unmarshal(encoded, &ordinary); err != nil {
			t.Fatal(err)
		}
		if _, exists := ordinary["resources"]; exists {
			t.Fatal("host resources leaked into ordinary settings")
		}
	}
	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.Resources.WriteScope = SandboxWriteScopeWorkspaceWrite
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatal("resource change did not require replacement")
	}
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxResourcesProtocolCapability}})
		if core.supports(CapabilitySandboxResources) != (kind == RuntimeNXS) {
			t.Fatal("resource capability attributed to wrong runtime")
		}
	}
}

// TestSandboxResourcesRejectBeforeTransport 阻止无效合同启动传输或进入用户任务。
func TestSandboxResourcesRejectBeforeTransport(t *testing.T) {
	for _, name := range []string{"base", "files", "version", "scope", "scratch", "escape", "write_grant", "claude"} {
		t.Run(name, func(t *testing.T) {
			settings := resourceSandboxSettings(t)
			kind := RuntimeNXS
			switch name {
			case "base":
				settings.RequireSandbox = false
			case "files":
				settings.RequireFileTools = false
			case "version":
				settings.Resources.Version = 2
			case "scope":
				settings.Resources.WriteScope = "full-access"
			case "scratch":
				settings.Resources.ScratchRoot = "relative"
			case "escape":
				yes := true
				settings.AllowUnsandboxedCommands = &yes
			case "write_grant":
				settings.Filesystem = &SandboxFilesystemConfig{AllowWrite: []string{t.TempDir()}}
			case "claude":
				kind = RuntimeClaude
			}
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settings, Runtime: RuntimeOptions{Kind: kind}}, tr)
			if err := core.ConnectWithPrompt(context.Background(), "must not send"); err == nil || core.isConnected() {
				t.Fatal("invalid resource policy admitted")
			}
			select {
			case write := <-tr.writes:
				t.Fatalf("invalid policy reached transport: %v", write)
			default:
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := core.Disconnect(ctx); err != nil {
				t.Fatalf("rejected configuration did not finish cleanup: %v", err)
			}
		})
	}
}

// TestSandboxResourcesNegotiationBeforePrompt 资源能力不能替代命令和文件确认。
func TestSandboxResourcesNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		caps     []string
		accepted bool
	}{
		{"missing", nil, false},
		{"command_and_file", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability}, false},
		{"resources_only", []string{sandboxResourcesProtocolCapability}, false},
		{"all", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxResourcesProtocolCapability}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: resourceSandboxSettings(t), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "resource task") }()
			request := receiveWrite(t, tr)
			assertInitializeRequest(t, request)
			body := request["request"].(map[string]any)
			resources, ok := body["required_sandbox_resources"].(map[string]any)
			if !ok || resources["write_scope"] != "read-only" || resources["scratch_root"] != core.options.Sandbox.Resources.ScratchRoot {
				t.Fatalf("missing independent resources: %v", body)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "resources", "protocol_capabilities": tc.caps}))
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
					t.Fatalf("task write after rejection: %v", write)
				default:
				}
			}
		})
	}
}

// TestSandboxResourcesRealProcess 真实旧版本只确认命令和文件时必须在发送任务前拒绝。
func TestSandboxResourcesRealProcess(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		accepted       bool
	}{
		{"current", "NEXUS_SANDBOX_TEST_BINARY", runtime.GOOS == "darwin"},
		{"legacy", "NEXUS_SANDBOX_RESOURCES_LEGACY_BINARY", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := os.Getenv(tc.variable)
			if binary == "" {
				t.Skip("set " + tc.variable + " to an explicit nxs binary")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			session, err := NewSession(ctx, Options{CLIPath: binary, CWD: t.TempDir(), Sandbox: resourceSandboxSettings(t),
				Env:     map[string]string{"NEXUS_CONFIG_DIR": filepath.Join(t.TempDir(), "config")},
				Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second},
			})
			if err != nil {
				if tc.accepted {
					t.Fatal(err)
				}
				if tc.name == "legacy" {
					var unsupported *UnsupportedCapabilityError
					if !errors.As(err, &unsupported) || unsupported.Capability != CapabilitySandboxResources {
						t.Fatalf("legacy did not reach resource rejection: %v", err)
					}
				}
				return
			}
			defer session.Close(context.Background())
			if !tc.accepted || !session.Supports(CapabilitySandboxResources) {
				t.Fatal("incorrect resource admission")
			}
		})
	}
}
