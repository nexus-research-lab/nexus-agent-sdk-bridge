// INPUT: 有无协商自动审核能力的 runtime initialize 响应。
// OUTPUT: 自动审核模式不会落到不支持能力的运行时。
// POS: bridge 能力协商与结构化审核投影回归。
package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestAutoReviewRequiresNegotiatedCapability(t *testing.T) {
	core := newSessionCore(Options{})
	if core.supports(CapabilityAutoReview) {
		t.Fatal("unnegotiated runtime supports auto review")
	}
	core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{autoReviewProtocolCapability}})
	if !core.supports(CapabilityAutoReview) {
		t.Fatal("negotiated capability missing")
	}
}

func TestAutoReviewPermissionRequestPreservesHumanReason(t *testing.T) {
	called := false
	core := newSessionCore(Options{Callbacks: CallbackOptions{PermissionHandler: func(_ context.Context, request permission.Request) (permission.Decision, error) {
		called = true
		if !request.RequiresHuman || request.Review == nil || request.Review.Rationale != "需要确认" || request.DecisionReason != "自动审核：需要确认" {
			t.Fatalf("request=%+v", request)
		}
		return permission.Allow(nil, nil), nil
	}}})
	result := core.resolvePermissionRequest(context.Background(), map[string]any{"tool_name": "Write", "requires_human": true, "decision_reason": "自动审核：需要确认", "review": map[string]any{"status": "needs_approval", "risk": "high", "authorization": "unknown", "rationale": "需要确认"}})
	if !called || result["behavior"] != "allow" {
		t.Fatalf("result=%v", result)
	}
}

func TestAutoReviewModeRejectsUnsupportedRuntime(t *testing.T) {
	core := newSessionCore(Options{})
	core.lifecycle.setConnectedLocked(true)
	if err := core.setPermissionMode(context.Background(), permission.ModeAuto); err == nil {
		t.Fatal("unsupported runtime accepted automatic review")
	}
}

// TestClaudeAutoModeConfirmation 验证启动和运行中切换都使用 Claude 原生确认，拒绝时不伪装成功。
func TestClaudeAutoModeConfirmation(t *testing.T) {
	for _, startup := range []bool{false, true} {
		for _, outcome := range []string{"auto", "default", "error"} {
			t.Run(fmt.Sprintf("startup=%v/%s", startup, outcome), func(t *testing.T) {
				transport := newScriptedTransport()
				mode := permission.ModeDefault
				if startup {
					mode = permission.ModeAuto
				}
				core := newSessionCoreWithTransport(Options{Transport: transport, Runtime: RuntimeOptions{Kind: RuntimeClaude, PermissionMode: mode, InitializeTimeout: time.Second}}, transport)
				defer func() { _ = core.Disconnect(context.Background()) }()
				done := make(chan error, 1)
				go func() { done <- core.Connect(context.Background()) }()
				assertInitializeRequest(t, receiveWrite(t, transport))
				transport.pushRead(successfulInitializeResponse(map[string]any{"current_permission_mode": "default"}))
				if !startup {
					if err := receiveDone(t, done); err != nil {
						t.Fatal(err)
					}
					go func() { done <- core.setPermissionMode(context.Background(), permission.ModeAuto) }()
				}
				request := receiveWrite(t, transport)
				assertControlRequest(t, request, "set_permission_mode")
				if request["request"].(map[string]any)["mode"] != "auto" {
					t.Fatalf("request=%v", request)
				}
				if outcome == "error" {
					transport.pushRead(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": request["request_id"], "error": "auto mode unavailable"}})
				} else {
					transport.pushRead(successfulControlResponse(request["request_id"].(string), map[string]any{"mode": outcome}))
				}
				err := receiveDone(t, done)
				if (err == nil) != (outcome == "auto") {
					t.Fatalf("outcome=%s error=%v", outcome, err)
				}
				if !startup && outcome != "auto" && core.options.Runtime.PermissionMode != permission.ModeDefault {
					t.Fatal("failed switch changed saved mode")
				}
				if startup && outcome != "auto" && core.isConnected() {
					t.Fatal("failed startup remained connected")
				}
			})
		}
	}
}
