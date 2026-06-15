package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestConnectWithPromptAllowsClaudePromptBeforeSystemInit(t *testing.T) {
	transport := newScriptedTransport()
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{Kind: RuntimeClaude, InitializeTimeout: time.Second},
		},
		transport,
	)

	done := make(chan error, 1)
	go func() {
		done <- core.ConnectWithPrompt(context.Background(), "hello")
	}()
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	assertInitializeRequest(t, receiveWrite(t, transport))
	transport.pushRead(successfulInitializeResponse(map[string]any{}))

	userWrite := receiveWrite(t, transport)
	if userWrite["type"] != "user" || userWrite["session_id"] != "default" {
		t.Fatalf("user write = %#v, want default session before system init", userWrite)
	}
	if err := receiveDone(t, done); err != nil {
		t.Fatalf("ConnectWithPrompt() error = %v", err)
	}

	transport.pushRead(map[string]any{
		"type":       "system",
		"subtype":    "init",
		"session_id": "session-from-init",
	})
	if got := waitForSessionID(t, core, "session-from-init"); got != "session-from-init" {
		t.Fatalf("session ID after system init = %q, want session-from-init", got)
	}
}

func TestConnectWithPromptDefaultRuntimeWaitsForNXSSystemInitSession(t *testing.T) {
	transport := newScriptedTransport()
	options := Options{
		Transport: transport,
		Runtime: RuntimeOptions{
			InitializeTimeout: time.Second,
		},
	}
	core := newSessionCoreWithTransport(options, transport)

	done := make(chan error, 1)
	go func() {
		done <- core.ConnectWithPrompt(context.Background(), "hello")
	}()
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	assertInitializeRequest(t, receiveWrite(t, transport))
	transport.pushRead(successfulInitializeResponse(map[string]any{}))

	select {
	case err := <-done:
		t.Fatalf("ConnectWithPrompt() returned before system init: %v", err)
	case write := <-transport.writes:
		t.Fatalf("unexpected write before system init: %#v", write)
	case <-time.After(50 * time.Millisecond):
	}

	transport.pushRead(map[string]any{
		"type":       "system",
		"subtype":    "init",
		"session_id": "session-from-init",
	})

	userWrite := receiveWrite(t, transport)
	if userWrite["type"] != "user" || userWrite["session_id"] != "session-from-init" {
		t.Fatalf("user write = %#v, want session-from-init", userWrite)
	}
	if err := receiveDone(t, done); err != nil {
		t.Fatalf("ConnectWithPrompt() error = %v", err)
	}
}

func TestConnectWithPromptUsesInitializeResponseSession(t *testing.T) {
	transport := newScriptedTransport()
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)

	done := make(chan error, 1)
	go func() {
		done <- core.ConnectWithPrompt(context.Background(), "hello")
	}()
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	assertInitializeRequest(t, receiveWrite(t, transport))
	transport.pushRead(successfulInitializeResponse(map[string]any{
		"session_id": "session-from-control",
	}))

	userWrite := receiveWrite(t, transport)
	if userWrite["type"] != "user" || userWrite["session_id"] != "session-from-control" {
		t.Fatalf("user write = %#v, want session-from-control", userWrite)
	}
	if err := receiveDone(t, done); err != nil {
		t.Fatalf("ConnectWithPrompt() error = %v", err)
	}
}

func TestSendUsesExplicitSessionIDOption(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(NewOptions().WithSessionID("explicit-session"), transport)
	core.lifecycleState().setConnected(true)

	if err := core.Send(context.Background(), "hello", nil, ""); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got := transport.writes[0]["session_id"]; got != "explicit-session" {
		t.Fatalf("session_id = %#v, want explicit-session", got)
	}
}

func TestReadLoopEmitsMessageStopDiagnostics(t *testing.T) {
	transport := newScriptedTransport()
	events := make(chan DiagnosticEvent, 4)
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
			Callbacks: CallbackOptions{
				Diagnostics: func(event DiagnosticEvent) {
					events <- event
				},
			},
		},
		transport,
	)

	done := make(chan error, 1)
	go func() {
		done <- core.Connect(context.Background())
	}()
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	assertInitializeRequest(t, receiveWrite(t, transport))
	transport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-1"}))
	if err := receiveDone(t, done); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	transport.pushRead(map[string]any{
		"type":       "stream_event",
		"session_id": "session-1",
		"event": map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    "assistant-1",
				"model": "kimi-k2.6",
			},
		},
	})
	transport.pushRead(map[string]any{
		"type":       "stream_event",
		"session_id": "session-1",
		"event": map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason": "tool_use",
			},
		},
	})
	transport.pushRead(map[string]any{
		"type":       "stream_event",
		"session_id": "session-1",
		"event": map[string]any{
			"type": "message_stop",
		},
	})

	select {
	case event := <-events:
		if event.Component != "bridge.stream" || event.Event != "message_stop" {
			t.Fatalf("diagnostic event = %+v, want bridge.stream/message_stop", event)
		}
		if event.Attributes["stop_reason"] != "tool_use" ||
			event.Attributes["session_id"] != "session-1" ||
			event.Attributes["message_id"] != "assistant-1" ||
			event.Attributes["model"] != "kimi-k2.6" {
			t.Fatalf("diagnostic attrs = %+v, want stream stop context", event.Attributes)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bridge.stream message_stop diagnostic")
	}
}

type scriptedTransport struct {
	reads  chan map[string]any
	writes chan map[string]any
	closed chan struct{}
	once   sync.Once
}

func newScriptedTransport() *scriptedTransport {
	return &scriptedTransport{
		reads:  make(chan map[string]any, 8),
		writes: make(chan map[string]any, 8),
		closed: make(chan struct{}),
	}
}

func (t *scriptedTransport) Start(context.Context) error {
	return nil
}

func (t *scriptedTransport) ReadJSON() (map[string]any, error) {
	select {
	case payload, ok := <-t.reads:
		if !ok {
			return nil, io.EOF
		}
		return payload, nil
	case <-t.closed:
		return nil, io.EOF
	}
}

func (t *scriptedTransport) WriteJSON(payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var message map[string]any
	if err := json.Unmarshal(raw, &message); err != nil {
		return errors.New("payload cannot be decoded as a map")
	}
	t.writes <- message
	return nil
}

func (t *scriptedTransport) EndInput() error {
	return nil
}

func (t *scriptedTransport) Interrupt() error {
	return nil
}

func (t *scriptedTransport) Wait() error {
	return nil
}

func (t *scriptedTransport) Close() error {
	t.once.Do(func() {
		close(t.closed)
	})
	return nil
}

func (t *scriptedTransport) pushRead(payload map[string]any) {
	t.reads <- payload
}

func successfulInitializeResponse(response map[string]any) map[string]any {
	return map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": "req_1",
			"response":   response,
		},
	}
}

func assertInitializeRequest(t *testing.T, payload map[string]any) {
	t.Helper()
	if payload["type"] != "control_request" || payload["request_id"] != "req_1" {
		t.Fatalf("initialize request envelope = %#v", payload)
	}
	request, ok := payload["request"].(map[string]any)
	if !ok || request["subtype"] != "initialize" {
		t.Fatalf("initialize request = %#v", payload["request"])
	}
}

func receiveWrite(t *testing.T, transport *scriptedTransport) map[string]any {
	t.Helper()
	select {
	case payload := <-transport.writes:
		return payload
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for transport write")
	}
	return nil
}

func receiveDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ConnectWithPrompt")
	}
	return nil
}

func waitForSessionID(t *testing.T, core *sessionCore, want string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := core.SessionID(); got == want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	return core.SessionID()
}
