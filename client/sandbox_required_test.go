// INPUT: 新旧 runtime 的 initialize 确认
// OUTPUT: 必需沙箱能力确认前不发送首条任务
// POS: Bridge 启动协议的失败关闭回归
package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRequiredSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, supported := range []bool{false, true} {
		name := "unsupported"
		if supported {
			name = "supported"
		}
		t.Run(name, func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: &SandboxSettings{RequireSandbox: true, Filesystem: &SandboxFilesystemConfig{DenyWrite: []string{"/protected"}}}, Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer func() { _ = core.Disconnect(context.Background()) }()
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(context.Background(), "hello") }()
			req := receiveWrite(t, tr)
			assertInitializeRequest(t, req)
			if req["request"].(map[string]any)["required_sandbox"] != true {
				t.Fatal("missing trusted initialize requirement")
			}
			policy, ok := req["request"].(map[string]any)["sandbox_policy"].(map[string]any)
			if !ok {
				t.Fatal("host resource policy missing from initialize")
			}
			fs, ok := policy["filesystem"].(map[string]any)
			if !ok {
				t.Fatal("filesystem policy missing")
			}
			denied, ok := fs["denyWrite"].([]any)
			if !ok || len(denied) != 1 || denied[0] != "/protected" {
				t.Fatalf("host deny lost: %v", fs)
			}

			caps := []string{}
			if supported {
				caps = append(caps, requiredSandboxProtocolCapability)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "sandbox-session", "protocol_capabilities": caps}))
			if supported {
				user := receiveWrite(t, tr)
				if user["type"] != "user" {
					t.Fatalf("expected prompt: %v", user)
				}
			}
			err := receiveDone(t, done)
			if (err == nil) != supported {
				t.Fatalf("supported=%v error=%v", supported, err)
			}
			if !supported {
				select {
				case write := <-tr.writes:
					if write["type"] == "user" {
						t.Fatal("sent prompt before capability confirmation")
					}
				default:
				}
			}
			if !supported && core.isConnected() {
				t.Fatal("unsupported runtime remained connected")
			}
		})
	}
}

func TestRequiredSandboxRejectsClaude(t *testing.T) {
	core := newSessionCore(Options{Sandbox: &SandboxSettings{RequireSandbox: true}, Runtime: RuntimeOptions{Kind: RuntimeClaude}})
	if err := core.Connect(context.Background()); err == nil {
		t.Fatal("Claude accepted unsupported sandbox guarantee")
	}
}

// TestRequiredSandboxRealProcess 通过显式选择的 nxs 二进制核对真实 stdio 握手，不发送模型请求。
func TestRequiredSandboxRealProcess(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_SANDBOX_TEST_BINARY to a built nxs binary")
	}
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := NewSession(ctx, Options{
		CLIPath: binary, CWD: root,
		Env:     map[string]string{"NEXUS_CONFIG_DIR": filepath.Join(root, "config")},
		Sandbox: &SandboxSettings{RequireSandbox: true},
		Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: 10 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close(context.Background())
	if !session.Supports(CapabilityRequiredSandbox) {
		t.Fatal("real runtime did not acknowledge required sandbox")
	}
}

func TestRequiredSandboxConcurrentSendDuringInitialize(t *testing.T) {
	tr := newScriptedTransport()
	core := newSessionCoreWithTransport(Options{Transport: tr, Sandbox: &SandboxSettings{RequireSandbox: true}, Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
	defer func() { _ = core.Disconnect(context.Background()) }()
	done := make(chan error, 1)
	go func() { done <- core.Connect(context.Background()) }()
	assertInitializeRequest(t, receiveWrite(t, tr))
	// transport 已连接但运行时尚未确认能力；所有用户消息路径都必须拒绝。
	if err := core.Query(context.Background(), "too early"); err == nil {
		t.Fatal("Query entered unconfirmed runtime")
	}
	if err := core.SendRawMessage(context.Background(), map[string]any{"type": "user"}, ""); err == nil {
		t.Fatal("raw send bypassed negotiation")
	}
	if err := core.sendInternalRawMessage(map[string]any{"type": "user"}, ""); err == nil {
		t.Fatal("internal continuation bypassed negotiation")
	}
	select {
	case w := <-tr.writes:
		t.Fatalf("message sent before confirmation: %v", w)
	default:
	}
	tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "ready", "protocol_capabilities": []string{requiredSandboxProtocolCapability}}))
	if err := receiveDone(t, done); err != nil {
		t.Fatal(err)
	}
	if err := core.Query(context.Background(), "after confirmation"); err != nil {
		t.Fatal(err)
	}
	if w := receiveWrite(t, tr); w["type"] != "user" {
		t.Fatalf("missing admitted message: %v", w)
	}
}
