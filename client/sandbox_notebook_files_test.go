// INPUT: 宿主 Notebook 文件要求、新旧 nxs 和其他后端的能力响应。
// OUTPUT: 独立 Notebook 文件能力在任何任务写入前确认，变化要求替换进程。
// POS: Bridge Notebook 文件执行合同的准入和兼容性回归。
package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

func notebookFileSandboxSettings() *SandboxSettings {
	return &SandboxSettings{RequireSandbox: true, RequireFileTools: true, RequireNotebookFiles: true}
}

func TestNotebookFileSandboxHostRequirement(t *testing.T) {
	settings := notebookFileSandboxSettings()
	settings.Extra = map[string]any{"requireNotebookFiles": false}
	options := NewOptions().WithSandbox(*settings)
	if !options.Sandbox.RequireNotebookFiles {
		t.Fatal("notebook requirement lost in options copy")
	}
	if _, exists := sandboxSettingsMap(options.Sandbox)["requireNotebookFiles"]; exists {
		t.Fatal("host notebook requirement entered ordinary settings")
	}
	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.RequireNotebookFiles = false
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("missing process replacement: %s", reason)
	}
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxNotebookFiles) {
			t.Fatal("unnegotiated notebook capability advertised")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxNotebookFilesProtocolCapability}})
		if core.supports(CapabilitySandboxNotebookFiles) != (kind == RuntimeNXS) {
			t.Fatalf("wrong backend accepted: %s", kind)
		}
	}
}

func TestNotebookFileSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities []string
		accepted     bool
	}{
		{"files_only", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability}, false},
		{"notebook_only", []string{sandboxNotebookFilesProtocolCapability}, false},
		{"all", []string{requiredSandboxProtocolCapability, sandboxFileToolsProtocolCapability, sandboxNotebookFilesProtocolCapability}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: notebookFileSandboxSettings(), Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "notebook task") }()
			request := receiveWrite(t, tr)["request"].(map[string]any)
			if request["required_sandbox_notebook_files"] != true || request["required_sandbox_file_tools"] != true || request["required_sandbox"] != true {
				t.Fatalf("missing requirements: %v", request)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "notebook", "protocol_capabilities": tc.capabilities}))
			if tc.accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("accepted notebook task not sent")
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
				if tc.name == "files_only" && unsupported.Capability != CapabilitySandboxNotebookFiles {
					t.Fatalf("wrong rejection: %v", err)
				}
			}
		})
	}
}

func TestNotebookFileSandboxRequiresBaseContracts(t *testing.T) {
	for _, settings := range []*SandboxSettings{{RequireNotebookFiles: true}, {RequireNotebookFiles: true, RequireSandbox: true}} {
		tr := newScriptedTransport()
		core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: settings}, tr)
		if err := core.Connect(context.Background()); err == nil || core.isConnected() {
			t.Fatalf("invalid notebook requirements accepted: %v", err)
		}
		select {
		case write := <-tr.writes:
			t.Fatalf("invalid requirement reached transport: %v", write)
		default:
		}
	}
}
