package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestRestartReasonForReconfigureDetectsProcessEnvChange(t *testing.T) {
	currentOptions, err := NewOptions().WithCLIPath("nxs").WithEnv(map[string]string{
		"ANTHROPIC_AUTH_TOKEN": "old-token",
		"ANTHROPIC_API_KEY":    "",
	}).normalized()
	if err != nil {
		t.Fatalf("normalize current options: %v", err)
	}
	nextOptions, err := NewOptions().WithCLIPath("nxs").WithEnv(map[string]string{
		"ANTHROPIC_AUTH_TOKEN": "new-token",
		"ANTHROPIC_API_KEY":    "",
	}).normalized()
	if err != nil {
		t.Fatalf("normalize next options: %v", err)
	}

	reason, ok := restartReasonForReconfigure(currentOptions, nextOptions)
	if !ok || reason != RestartReasonProcessEnvChanged {
		t.Fatalf("restart reason = %q, %v; want process env changed", reason, ok)
	}
	if _, ok := restartReasonForReconfigure(nextOptions, nextOptions); ok {
		t.Fatal("unchanged options should not require restart")
	}
	emptyKeyOptions, err := NewOptions().WithCLIPath("nxs").WithEnv(map[string]string{"ANTHROPIC_API_KEY": ""}).normalized()
	if err != nil {
		t.Fatalf("normalize empty key options: %v", err)
	}
	emptyTokenOptions, err := NewOptions().WithCLIPath("nxs").WithEnv(map[string]string{"ANTHROPIC_AUTH_TOKEN": ""}).normalized()
	if err != nil {
		t.Fatalf("normalize empty token options: %v", err)
	}
	reason, ok = restartReasonForReconfigure(emptyKeyOptions, emptyTokenOptions)
	if !ok || reason != RestartReasonProcessEnvChanged {
		t.Fatalf("restart reason = %q, %v; want key-only env change", reason, ok)
	}
}

func TestRestartReasonForReconfigureDetectsToolPolicyChange(t *testing.T) {
	currentOptions, err := NewOptions().WithCLIPath("nxs").WithAllowedTools("Read", "create_goal").normalized()
	if err != nil {
		t.Fatalf("normalize current options: %v", err)
	}
	nextOptions, err := NewOptions().WithCLIPath("nxs").WithAllowedTools("Read", "create_goal", "mcp__nexus_goal__update_goal").normalized()
	if err != nil {
		t.Fatalf("normalize next options: %v", err)
	}

	reason, ok := restartReasonForReconfigure(currentOptions, nextOptions)
	if !ok || reason != RestartReasonToolPolicyChanged {
		t.Fatalf("restart reason = %q, %v; want tool policy changed", reason, ok)
	}
}

func TestReconfigureAppliesRuntimeControls(t *testing.T) {
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

	reconfigureDone := make(chan error, 1)
	go func() {
		reconfigureDone <- core.reconfigure(context.Background(), NewOptions().
			WithTransport(transport).
			WithPermissionMode(permission.ModeAcceptEdits).
			WithModel("runtime-model").
			WithMaxThinkingTokens(512).
			WithMCPServer("test", mcp.HTTPServerConfig{URL: "https://example.test/mcp"}).
			WithSDKMCPServer("sdk-test", fakeSDKMCPServer{}))
	}()

	assertControlRequest(t, receiveWrite(t, transport), "set_permission_mode")
	transport.pushRead(successfulControlResponse("req_2", map[string]any{}))
	assertControlRequest(t, receiveWrite(t, transport), "set_model")
	transport.pushRead(successfulControlResponse("req_3", map[string]any{}))
	assertControlRequest(t, receiveWrite(t, transport), "set_max_thinking_tokens")
	transport.pushRead(successfulControlResponse("req_4", map[string]any{}))
	assertControlRequest(t, receiveWrite(t, transport), "mcp_set_servers")
	transport.pushRead(successfulControlResponse("req_5", map[string]any{}))

	if err := receiveDone(t, reconfigureDone); err != nil {
		t.Fatalf("reconfigure() error = %v", err)
	}
	if core.options.Model != "runtime-model" ||
		core.options.Runtime.PermissionMode != permission.ModeAcceptEdits ||
		core.options.Runtime.MaxThinkingTokens != 512 {
		t.Fatalf("options were not updated after reconfigure: %+v", core.options)
	}
	if _, ok := core.sdkMCPServer("sdk-test"); !ok {
		t.Fatal("sdk mcp server registry was not updated after reconfigure")
	}
}

func TestSendTaskMessageSendsControlRequest(t *testing.T) {
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
	transport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-1"}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	sendDone := make(chan error, 1)
	session := &Session{core: core}
	go func() {
		sendDone <- session.Control().SendTaskMessage(context.Background(), "task-1", "please continue", "continue")
	}()
	payload := receiveWrite(t, transport)
	assertControlRequest(t, payload, "send_task_message")
	request := payload["request"].(map[string]any)
	if request["task_id"] != "task-1" {
		t.Fatalf("task_id = %#v, want task-1", request["task_id"])
	}
	requestPayload, ok := request["payload"].(map[string]any)
	if !ok || requestPayload["message"] != "please continue" || requestPayload["summary"] != "continue" {
		t.Fatalf("payload = %#v, want message and summary", request["payload"])
	}
	transport.pushRead(successfulControlResponse("req_2", map[string]any{}))
	if err := receiveDone(t, sendDone); err != nil {
		t.Fatalf("sendTaskMessage() error = %v", err)
	}
}

func TestReconfigureReturnsRestartRequiredWhenMCPControlUnsupported(t *testing.T) {
	err := errors.New("unsupported control request subtype: mcp_set_servers")
	if !isMCPSetServersUnsupported(err) {
		t.Fatal("mcp_set_servers unsupported error should require restart")
	}
	if isMCPSetServersUnsupported(errors.New("mcp_set_servers failed: invalid url")) {
		t.Fatal("ordinary mcp_set_servers failure should not require restart")
	}
}

func successfulControlResponse(requestID string, response map[string]any) map[string]any {
	return map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   response,
		},
	}
}

func TestInterruptWithReasonSendsControlRequest(t *testing.T) {
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
	transport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-1"}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	interruptDone := make(chan error, 1)
	go func() {
		interruptDone <- core.interruptWithReason(context.Background(), "interrupt")
	}()
	payload := receiveWrite(t, transport)
	assertControlRequest(t, payload, "interrupt")
	request := payload["request"].(map[string]any)
	if request["reason"] != "interrupt" {
		t.Fatalf("interrupt reason = %#v, want interrupt", request["reason"])
	}
	requestID, _ := payload["request_id"].(string)
	transport.pushRead(map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
		},
	})
	if err := receiveDone(t, interruptDone); err != nil {
		t.Fatalf("interruptWithReason() error = %v", err)
	}
}

func assertControlRequest(t *testing.T, payload map[string]any, subtype string) {
	t.Helper()
	if payload["type"] != "control_request" {
		t.Fatalf("control request envelope = %#v", payload)
	}
	request, ok := payload["request"].(map[string]any)
	if !ok || request["subtype"] != subtype {
		t.Fatalf("control request = %#v, want subtype %s", payload["request"], subtype)
	}
}

type fakeSDKMCPServer struct{}

func (fakeSDKMCPServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
