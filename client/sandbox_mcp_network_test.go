// INPUT: MCP 专用网络要求、初始化协商与进程复用。
// OUTPUT: 缺少能力/显式来源时在 prompt 前拒绝，配置变更要求替换进程。
// POS: Bridge 只声明 nxs HTTP/SSE 合同，不借用 Claude 或 helper 能力。
package client

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

func TestMCPNetworkSandboxContract(t *testing.T) {
	options := Options{Runtime: RuntimeOptions{Kind: RuntimeNXS}, MCP: MCPOptions{StrictConfig: true}, Sandbox: &SandboxSettings{RequireSandbox: true, RequireMCPNetwork: true}}
	options.Sandbox.Extra = map[string]any{"requireMCPNetwork": false}
	if _, ok := sandboxSettingsMap(options.Sandbox)["requireMCPNetwork"]; ok {
		t.Fatal("host requirement entered task settings")
	}
	next := options
	next.Sandbox = cloneSandboxSettings(options.Sandbox)
	next.Sandbox.RequireMCPNetwork = false
	if reason, required := restartReasonForReconfigure(options, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("requirement reuse: %s", reason)
	}
	for _, kind := range []RuntimeKind{RuntimeNXS, RuntimeClaude} {
		core := newSessionCore(Options{Runtime: RuntimeOptions{Kind: kind}})
		if core.supports(CapabilitySandboxMCPNetwork) {
			t.Fatal("unnegotiated capability")
		}
		core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{sandboxMCPNetworkProtocolCapability}})
		if core.supports(CapabilitySandboxMCPNetwork) != (kind == RuntimeNXS) {
			t.Fatal("wrong backend capability")
		}
	}
	for _, missing := range []string{"base", "strict"} {
		invalid := options
		invalid.Sandbox = cloneSandboxSettings(options.Sandbox)
		if missing == "base" {
			invalid.Sandbox.RequireSandbox = false
		} else {
			invalid.MCP.StrictConfig = false
		}
		if err := newSessionCore(invalid).validateSandboxRequirements(); err == nil {
			t.Fatalf("accepted missing %s", missing)
		}
	}
}

func TestMCPNetworkSandboxNegotiationBeforePrompt(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{true: "supported", false: "missing_capability"}[accepted], func(t *testing.T) {
			tr := newScriptedTransport()
			core := newSessionCoreWithTransport(Options{Transport: tr, MCP: MCPOptions{StrictConfig: true}, Sandbox: &SandboxSettings{RequireSandbox: true, RequireMCPNetwork: true}, Runtime: RuntimeOptions{Kind: RuntimeNXS, InitializeTimeout: time.Second}}, tr)
			defer core.Disconnect(context.Background())
			done := make(chan error, 1)
			go func() { done <- core.ConnectWithPrompt(t.Context(), "fixture") }()
			request := receiveWrite(t, tr)["request"].(map[string]any)
			if request["required_sandbox_mcp_network"] != true || request["required_sandbox"] != true {
				t.Fatalf("request: %v", request)
			}
			capabilities := []string{requiredSandboxProtocolCapability}
			if accepted {
				capabilities = append(capabilities, sandboxMCPNetworkProtocolCapability)
			}
			tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "mcp-fixture", "protocol_capabilities": capabilities}))
			if accepted && receiveWrite(t, tr)["type"] != "user" {
				t.Fatal("missing prompt")
			}
			if err := receiveDone(t, done); (err == nil) != accepted {
				t.Fatalf("accepted=%v err=%v", accepted, err)
			}
			if !accepted {
				select {
				case item := <-tr.writes:
					t.Fatalf("unexpected prompt: %v", item)
				default:
				}
			}
		})
	}
}
