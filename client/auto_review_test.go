// INPUT: 有无协商自动审核能力的 runtime initialize 响应。
// OUTPUT: 自动审核模式不会落到不支持能力的运行时。
// POS: bridge 能力协商与结构化审核投影回归。
package client

import (
	"context"
	"testing"

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
