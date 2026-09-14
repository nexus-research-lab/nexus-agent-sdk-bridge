package client

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestSandboxPermissionBoundaryTransport(t *testing.T) {
	for _, tc := range []struct {
		name              string
		raw               any
		include, accepted bool
	}{
		{name: "legacy", accepted: true},
		{name: "ordinary", raw: "tool", include: true, accepted: true},
		{name: "escape", raw: "sandbox_escape", include: true, accepted: true},
		{name: "unknown", raw: "future_boundary", include: true},
		{name: "malformed", raw: 42, include: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			core := newSessionCoreWithTransport(Options{Callbacks: CallbackOptions{PermissionHandler: func(_ context.Context, request permission.Request) (permission.Decision, error) {
				calls++
				if tc.include && string(request.Boundary) != tc.raw {
					t.Fatalf("lost boundary: %q", request.Boundary)
				}
				if !request.RequiresHuman || request.ToolUseID != "exact-call" || request.Review == nil || request.Review.Rationale != "review evidence" {
					t.Fatalf("lost exact review context: %+v", request)
				}
				return permission.Allow(nil, nil), nil
			}}}, newScriptedTransport())
			request := map[string]any{"tool_name": "Bash", "tool_use_id": "exact-call", "requires_human": true, "review": map[string]any{"status": "needs_approval", "rationale": "review evidence"}}
			if tc.include {
				request["permission_boundary"] = tc.raw
			}
			result := core.resolvePermissionRequest(context.Background(), request)
			if (calls == 1) != tc.accepted {
				t.Fatalf("handler calls=%d accepted=%v", calls, tc.accepted)
			}
			if !tc.accepted && result["behavior"] != "deny" {
				t.Fatalf("unknown boundary was not denied: %#v", result)
			}
		})
	}
}
