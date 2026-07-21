package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

type failingControlTransport struct {
	writeErr error
	writes   int
}

func (t *failingControlTransport) Start(context.Context) error { return nil }

func (t *failingControlTransport) ReadJSON() (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (t *failingControlTransport) WriteJSON(any) error {
	t.writes++
	return t.writeErr
}

func (t *failingControlTransport) EndInput() error  { return nil }
func (t *failingControlTransport) Interrupt() error { return nil }
func (t *failingControlTransport) Wait() error      { return nil }
func (t *failingControlTransport) Close() error     { return nil }

func TestHandleControlRequestMarksTransportFailedWhenResponseWriteFails(t *testing.T) {
	transport := &failingControlTransport{
		writeErr: errors.New("process: write payload failed: Stream closed"),
	}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)

	core.handleControlRequest(map[string]any{
		"request_id": "request-hook",
		"request": map[string]any{
			"subtype": "unsupported",
		},
	})

	if transport.writes != 1 {
		t.Fatalf("WriteJSON calls = %d, want 1", transport.writes)
	}
	if core.isConnected() {
		t.Fatal("session should be marked disconnected after control response write failure")
	}
	readErr := core.getReadError()
	if readErr == nil || !strings.Contains(readErr.Error(), "send control response failed") ||
		!strings.Contains(readErr.Error(), "Stream closed") {
		t.Fatalf("read error missing control response failure detail: %v", readErr)
	}
}

func TestBuildInitializeRequestAdvertisesHookResponseAckOnlyToNXS(t *testing.T) {
	nxsRequest := newSessionCore(Options{}).buildInitializeRequest()
	if len(nxsRequest.ProtocolCapabilities) != 1 || nxsRequest.ProtocolCapabilities[0] != hookResponseAckProtocolCapability {
		t.Fatalf("nxs protocol capabilities = %#v", nxsRequest.ProtocolCapabilities)
	}

	claudeRequest := newSessionCore(Options{Runtime: RuntimeOptions{Kind: RuntimeClaude}}).buildInitializeRequest()
	if len(claudeRequest.ProtocolCapabilities) != 0 {
		t.Fatalf("claude protocol capabilities = %#v, want none", claudeRequest.ProtocolCapabilities)
	}
}

func TestBuildInitializeRequestCarriesPromptPartsOnlyToNXS(t *testing.T) {
	nxsRequest := newSessionCore(Options{
		System: SystemOptions{
			AppendStatic:  "stable Room rules",
			AppendDynamic: "dynamic Agent context",
		},
	}).buildInitializeRequest()
	if nxsRequest.AppendSystemPromptStatic != "stable Room rules" || nxsRequest.AppendSystemPromptDynamic != "dynamic Agent context" {
		t.Fatalf("nxs prompt parts = %#v, want stable/dynamic fields", nxsRequest)
	}
	if nxsRequest.AppendSystemPrompt != "stable Room rules\n\ndynamic Agent context" {
		t.Fatalf("nxs compatibility prompt = %q, want flattened prompt", nxsRequest.AppendSystemPrompt)
	}

	claudeRequest := newSessionCore(Options{
		Runtime: RuntimeOptions{Kind: RuntimeClaude},
		System:  SystemOptions{AppendStatic: "stable Room rules", AppendDynamic: "dynamic Agent context"},
	}).buildInitializeRequest()
	if claudeRequest.AppendSystemPrompt != "stable Room rules\n\ndynamic Agent context" {
		t.Fatalf("claude compatibility prompt = %q, want flattened prompt", claudeRequest.AppendSystemPrompt)
	}
	if claudeRequest.AppendSystemPromptStatic != "" || claudeRequest.AppendSystemPromptDynamic != "" {
		t.Fatalf("claude prompt parts = %#v, want no nxs-only fields", claudeRequest)
	}
}

func TestHookResponseAppliedAckInvokesCallbackExactlyOnce(t *testing.T) {
	transport := newScriptedTransport()
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)
	core.lifecycleState().setInitializeResponse(runtimeinfo.InitializeResponse{
		ProtocolCapabilities: []string{hookResponseAckProtocolCapability},
	})

	applied := make(chan hook.AppliedAck, 1)
	callbackID := core.hookCallbackRegistry().register(func(context.Context, hook.Input, string) (hook.Output, error) {
		return hook.Output{
			SystemMessage: "continue",
			OnApplied: func(ack hook.AppliedAck) {
				applied <- ack
			},
		}, nil
	})
	core.handleControlRequest(map[string]any{
		"request_id": "request-hook",
		"request": map[string]any{
			"subtype":     "hook_callback",
			"callback_id": callbackID,
			"tool_use_id": "tool-1",
			"input":       map[string]any{"hook_event_name": "PostToolUse"},
		},
	})
	receiveWrite(t, transport)

	ack := map[string]any{
		"type":            "control_ack",
		"request_id":      "request-hook",
		"request_subtype": "hook_callback",
		"stage":           "applied",
		"hook_event_name": "PostToolUse",
		"tool_use_id":     "tool-1",
		"session_id":      "session-1",
	}
	core.handleControlAck(ack)
	core.handleControlAck(ack)

	got := <-applied
	if got.RequestID != "request-hook" || got.HookEventName != hook.EventPostToolUse ||
		got.ToolUseID != "tool-1" || got.SessionID != "session-1" {
		t.Fatalf("applied ack = %#v", got)
	}
	select {
	case duplicate := <-applied:
		t.Fatalf("duplicate applied callback = %#v", duplicate)
	default:
	}
}

func TestHookResponseAppliedAckIsClearedWhenResponseWriteFails(t *testing.T) {
	transport := &failingControlTransport{writeErr: errors.New("write failed")}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)
	core.lifecycleState().setInitializeResponse(runtimeinfo.InitializeResponse{
		ProtocolCapabilities: []string{hookResponseAckProtocolCapability},
	})

	called := false
	callbackID := core.hookCallbackRegistry().register(func(context.Context, hook.Input, string) (hook.Output, error) {
		return hook.Output{OnApplied: func(hook.AppliedAck) { called = true }}, nil
	})
	core.handleControlRequest(map[string]any{
		"request_id": "request-hook",
		"request": map[string]any{
			"subtype":     "hook_callback",
			"callback_id": callbackID,
		},
	})
	core.handleControlAck(map[string]any{
		"type":            "control_ack",
		"request_id":      "request-hook",
		"request_subtype": "hook_callback",
		"stage":           "applied",
	})

	if called {
		t.Fatal("OnApplied called after response write failure")
	}
}
