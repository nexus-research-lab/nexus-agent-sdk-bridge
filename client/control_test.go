package client

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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
	core.lifecycle.setConnected(true)

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

func TestBuildInitializeRequestAdvertisesNXSProtocolCapabilities(t *testing.T) {
	nxsRequest := newSessionCore(Options{}).buildInitializeRequest()
	if !reflect.DeepEqual(nxsRequest.ProtocolCapabilities, []string{
		hookResponseAckProtocolCapability,
		messageExecutionPolicyProtocolCapability,
	}) {
		t.Fatalf("nxs protocol capabilities = %#v", nxsRequest.ProtocolCapabilities)
	}

	claudeRequest := newSessionCore(Options{Runtime: RuntimeOptions{Kind: RuntimeClaude}}).buildInitializeRequest()
	if len(claudeRequest.ProtocolCapabilities) != 0 {
		t.Fatalf("claude protocol capabilities = %#v, want none", claudeRequest.ProtocolCapabilities)
	}
}

func TestPermissionErrorCodeFlowsThroughNXSAndClaudeMessages(t *testing.T) {
	handler := func(context.Context, permission.Request) (permission.Decision, error) {
		return permission.DenyWithErrorCode(
			"等待用户确认超时",
			permission.ErrorCodeRequestTimeout,
			true,
		), nil
	}
	request := map[string]any{
		"tool_name":   "AskUserQuestion",
		"tool_use_id": "tool-question",
	}

	nxsCore := newSessionCore(Options{
		Runtime:   RuntimeOptions{Kind: RuntimeNXS},
		Callbacks: CallbackOptions{PermissionHandler: handler},
	})
	nxsResponse := nxsCore.resolvePermissionRequest(context.Background(), request)
	if nxsResponse["errorCode"] != string(permission.ErrorCodeRequestTimeout) {
		t.Fatalf("nxs response = %#v, want structured errorCode", nxsResponse)
	}

	claudeCore := newSessionCore(Options{
		Runtime:   RuntimeOptions{Kind: RuntimeClaude},
		Callbacks: CallbackOptions{PermissionHandler: handler},
	})
	claudeResponse := claudeCore.resolvePermissionRequest(context.Background(), request)
	if _, exists := claudeResponse["errorCode"]; exists {
		t.Fatalf("Claude response = %#v, want no unsupported extension field", claudeResponse)
	}
	message, err := protocol.DecodeMessage(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type":        "tool_result",
				"tool_use_id": "tool-question",
				"content":     "等待用户确认超时",
				"is_error":    true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v", err)
	}
	message = claudeCore.attachPermissionErrorCodes(message)
	toolResult, ok := protocol.AsToolResultBlock(message.User.Message.Content[0])
	if !ok || toolResult.ErrorCode != string(permission.ErrorCodeRequestTimeout) {
		t.Fatalf("Claude tool result = %#v, want bridged error_code", toolResult)
	}
	if toolResult.RawPayload()["error_code"] != string(permission.ErrorCodeRequestTimeout) {
		t.Fatalf("Claude raw tool result = %#v, want bridged error_code", toolResult.RawPayload())
	}
	rawEnvelope := message.Raw["message"].(map[string]any)
	rawContent := rawEnvelope["content"].([]any)
	rawToolResult := rawContent[0].(map[string]any)
	if rawToolResult["error_code"] != string(permission.ErrorCodeRequestTimeout) {
		t.Fatalf("ReceivedMessage.Raw = %#v, want bridged error_code", message.Raw)
	}
}

func TestBuildInitializeRequestCarriesSelectedSkills(t *testing.T) {
	request := newSessionCore(
		NewOptions().
			WithSkills("imagegen", "ima-skill").
			WithDisabledSkills("workspace-review"),
	).buildInitializeRequest()
	if request.Skills == nil {
		t.Fatal("selected Skill filter should be present in initialize request")
	}
	if len(*request.Skills) != 2 || (*request.Skills)[0] != "imagegen" || (*request.Skills)[1] != "ima-skill" {
		t.Fatalf("initialize skills = %#v, want selected names", request.Skills)
	}
	if len(request.DisabledSkills) != 1 || request.DisabledSkills[0] != "workspace-review" {
		t.Fatalf("initialize disabled skills = %#v, want explicit disabled name", request.DisabledSkills)
	}
}

func TestBuildInitializeRequestSendsEmptySkillFilterWhenDisabled(t *testing.T) {
	request := newSessionCore(NewOptions().WithoutSkills()).buildInitializeRequest()
	if request.Skills == nil {
		t.Fatal("disabled Skill filter should be present as an empty list")
	}
	if len(*request.Skills) != 0 {
		t.Fatalf("initialize skills = %#v, want empty list", request.Skills)
	}
}

func TestBuildInitializeRequestOmitsSkillFilterForAll(t *testing.T) {
	request := newSessionCore(NewOptions().WithAllSkills()).buildInitializeRequest()
	if request.Skills != nil {
		t.Fatalf("initialize skills = %#v, want omitted for all", request.Skills)
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
	core.lifecycle.setConnected(true)
	core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{
		ProtocolCapabilities: []string{hookResponseAckProtocolCapability},
	})

	applied := make(chan hook.AppliedAck, 1)
	callbackID := core.hookCallbacks.register(func(context.Context, hook.Input, string) (hook.Output, error) {
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
	core.lifecycle.setConnected(true)
	core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{
		ProtocolCapabilities: []string{hookResponseAckProtocolCapability},
	})

	called := false
	callbackID := core.hookCallbacks.register(func(context.Context, hook.Input, string) (hook.Output, error) {
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

func TestSessionCoreRemoveMessagesSendsControlRequest(t *testing.T) {
	transport := newScriptedTransport()
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)

	connectDone := make(chan error, 1)
	go func() {
		connectDone <- core.Connect(context.Background())
	}()
	assertInitializeRequest(t, receiveWrite(t, transport))
	transport.pushRead(successfulInitializeResponse(map[string]any{
		"session_id": "session-1",
	}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	removeDone := make(chan error, 1)
	go func() {
		removeDone <- core.removeMessages(context.Background(), []string{" msg-1 ", "msg-2", "msg-1"})
	}()

	payload := receiveWrite(t, transport)
	if payload["type"] != "control_request" {
		t.Fatalf("remove_messages envelope = %#v", payload)
	}
	request, ok := payload["request"].(map[string]any)
	if !ok {
		t.Fatalf("remove_messages request = %#v", payload["request"])
	}
	if request["subtype"] != "remove_messages" {
		t.Fatalf("remove_messages subtype = %#v", request["subtype"])
	}
	if got := request["message_uuids"]; !reflect.DeepEqual(got, []any{"msg-1", "msg-2"}) {
		t.Fatalf("message_uuids = %#v, want msg-1/msg-2", got)
	}
	requestID, _ := payload["request_id"].(string)
	transport.pushRead(map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   map[string]any{"removed": 2},
		},
	})
	if err := receiveDone(t, removeDone); err != nil {
		t.Fatalf("removeMessages() error = %v", err)
	}
}
