package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

func TestConnectIncludesRuntimeStartupDiagnosticsOnInitialSessionTimeout(t *testing.T) {
	transport := newScriptedTransport()
	transport.stderrTail = `API error (attempt 1/11): 529 {"type":"overloaded_error","message":"[1305] 当前访问量过大，请稍后再试"}`
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: 20 * time.Millisecond},
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
	transport.pushRead(successfulInitializeResponse(map[string]any{}))

	err := receiveDone(t, done)
	if err == nil {
		t.Fatal("Connect() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Connect() error = %v, want context deadline exceeded", err)
	}
	message := err.Error()
	for _, want := range []string{"provider_error=server_overload", "overloaded_error", "context deadline exceeded"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Connect() error = %q, want %q", message, want)
		}
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
	core.lifecycle.setConnected(true)

	if err := core.Send(context.Background(), "hello", nil, ""); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got := transport.writes[0]["session_id"]; got != "explicit-session" {
		t.Fatalf("session_id = %#v, want explicit-session", got)
	}
}

func TestSendSlashCommandUsesNormalUserMessage(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(NewOptions().WithSessionID("session-slash"), transport)
	core.lifecycle.setConnected(true)

	if err := core.Send(context.Background(), "/review target", nil, ""); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(transport.writes) != 1 {
		t.Fatalf("writes = %#v, want one user message", transport.writes)
	}
	payload := transport.writes[0]
	if payload["type"] != "user" || payload["session_id"] != "session-slash" {
		t.Fatalf("payload = %#v, want normal user envelope", payload)
	}
	message, ok := payload["message"].(map[string]any)
	if !ok || message["role"] != "user" || message["content"] != "/review target" {
		t.Fatalf("message = %#v, want slash text as ordinary user content", payload["message"])
	}
	if _, hasControlSubtype := payload["subtype"]; hasControlSubtype {
		t.Fatalf("payload = %#v, slash commands must not use control subtype", payload)
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

func TestDisconnectUnblocksReadLoopWhenMessageBufferIsFull(t *testing.T) {
	transport := newObservedReadTransport()
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)
	connectDone := make(chan error, 1)
	go func() { connectDone <- core.Connect(context.Background()) }()
	assertInitializeRequest(t, receiveWrite(t, transport.scriptedTransport))
	transport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-full-buffer"}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	streams := core.streams
	for len(streams.messages) < cap(streams.messages) {
		streams.messages <- protocol.ReceivedMessage{}
	}
	transport.pushRead(map[string]any{
		"type":       "assistant",
		"session_id": "session-full-buffer",
		"message": map[string]any{
			"role":    "assistant",
			"content": []any{},
		},
	})
	select {
	case <-transport.assistantRead:
	case <-time.After(time.Second):
		t.Fatal("readLoop 未读取用于填塞 emit 的消息")
	}

	disconnectDone := make(chan error, 1)
	go func() { disconnectDone <- core.Disconnect(context.Background()) }()
	select {
	case err := <-disconnectDone:
		if err != nil {
			t.Fatalf("Disconnect() error = %v", err)
		}
	case <-time.After(time.Second):
		// 给旧实现释放一个槽位，避免失败测试遗留永久阻塞的 goroutine。
		<-streams.messages
		t.Fatal("Disconnect() blocked behind the full message buffer")
	}
}

func TestDisconnectReadLoopWaitHonorsContext(t *testing.T) {
	transport := &nonClosingScriptedTransport{scriptedTransport: newScriptedTransport()}
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)
	connectDone := make(chan error, 1)
	go func() { connectDone <- core.Connect(context.Background()) }()
	assertInitializeRequest(t, receiveWrite(t, transport.scriptedTransport))
	transport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-context-close"}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := core.Disconnect(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Disconnect() error = %v, want context deadline exceeded", err)
	}
	_ = transport.scriptedTransport.Close()
	select {
	case <-core.streams.readDone:
	case <-time.After(time.Second):
		t.Fatal("测试 transport 释放后 readLoop 未结束")
	}
}

func TestDisconnectTransportCloseHonorsContext(t *testing.T) {
	transport := newBlockingCloseTransport()
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)
	connectDone := make(chan error, 1)
	go func() { connectDone <- core.Connect(context.Background()) }()
	assertInitializeRequest(t, receiveWrite(t, transport.scriptedTransport))
	transport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-blocking-close"}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	disconnectDone := make(chan error, 1)
	go func() { disconnectDone <- core.Disconnect(ctx) }()
	select {
	case <-transport.closeStarted:
	case <-time.After(time.Second):
		t.Fatal("transport Close 未启动")
	}
	select {
	case err := <-disconnectDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Disconnect() error = %v, want context deadline exceeded", err)
		}
	case <-time.After(250 * time.Millisecond):
		close(transport.closeRelease)
		t.Fatal("Disconnect() blocked inside transport Close")
	}
	closeState := core.streams.closeState
	if closeState == nil {
		close(transport.closeRelease)
		t.Fatal("Disconnect() 未发布本代 close state")
	}

	close(transport.closeRelease)
	select {
	case <-closeState.done:
	case <-time.After(time.Second):
		t.Fatal("释放 transport 后 Close 未结束")
	}
	select {
	case <-core.streams.readDone:
	case <-time.After(time.Second):
		t.Fatal("释放 transport 后 readLoop 未结束")
	}
}

func TestConnectWaitsForPreviousCloseGenerationWithoutBlockingReadLoop(t *testing.T) {
	firstTransport := newBlockingCloseTransport()
	secondTransport := newScriptedTransport()
	transportQueue := make(chan Transport, 2)
	transportQueue <- firstTransport
	transportQueue <- secondTransport
	factoryCalls := make(chan Transport, 2)
	core := &sessionCore{runner: newRunner(
		Options{
			Transport: firstTransport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		nil,
		func(Options) Transport {
			transport := <-transportQueue
			factoryCalls <- transport
			return transport
		},
		false,
	)}

	firstConnectDone := make(chan error, 1)
	go func() { firstConnectDone <- core.Connect(context.Background()) }()
	select {
	case transport := <-factoryCalls:
		if transport != firstTransport {
			t.Fatalf("首次 transport = %T, want firstTransport", transport)
		}
	case <-time.After(time.Second):
		t.Fatal("首次 Connect 未创建 transport")
	}
	assertInitializeRequest(t, receiveWrite(t, firstTransport.scriptedTransport))
	firstTransport.pushRead(successfulInitializeResponse(map[string]any{"session_id": "session-first"}))
	if err := receiveDone(t, firstConnectDone); err != nil {
		t.Fatalf("首次 Connect() error = %v", err)
	}

	oldStreams := core.streams
	oldReadDone := oldStreams.readDone
	releasedFirstClose := false
	defer func() {
		if !releasedFirstClose {
			close(firstTransport.closeRelease)
		}
		_ = secondTransport.Close()
	}()
	disconnectCtx, cancelDisconnect := context.WithCancel(context.Background())
	disconnectDone := make(chan error, 1)
	go func() { disconnectDone <- core.Disconnect(disconnectCtx) }()
	select {
	case <-firstTransport.closeStarted:
	case <-time.After(time.Second):
		t.Fatal("旧 transport Close 未启动")
	}
	oldCloseState := oldStreams.closeState
	if oldCloseState == nil {
		t.Fatal("Disconnect() 未发布旧代 close state")
	}
	cancelDisconnect()
	if err := receiveDone(t, disconnectDone); !errors.Is(err, context.Canceled) {
		t.Fatalf("超时前代 Disconnect() error = %v, want context canceled", err)
	}

	reconnectParent, cancelReconnect := context.WithCancel(context.Background())
	defer cancelReconnect()
	waitContext := newObservedDoneContext(reconnectParent)
	reconnectDone := make(chan error, 1)
	go func() { reconnectDone <- core.Connect(waitContext) }()
	select {
	case <-waitContext.observed:
	case <-time.After(time.Second):
		t.Fatal("Connect 未进入前代 close 等待")
	}
	select {
	case transport := <-factoryCalls:
		t.Fatalf("前代 close 完成前创建了新 transport: %T", transport)
	default:
	}
	select {
	case payload := <-firstTransport.writes:
		t.Fatalf("前代 close 完成前向旧 transport 写入: %#v", payload)
	default:
	}

	// 先让旧 readLoop 退出；若 Connect 持有 connectedMu 等待 Close，这里会死锁。
	_ = firstTransport.scriptedTransport.Close()
	select {
	case <-oldReadDone:
	case <-time.After(time.Second):
		t.Fatal("等待前代 close 的 Connect 阻塞了旧 readLoop 退出")
	}
	close(firstTransport.closeRelease)
	releasedFirstClose = true
	select {
	case <-oldCloseState.done:
	case <-time.After(time.Second):
		t.Fatal("旧代 close state 未完成")
	}

	select {
	case transport := <-factoryCalls:
		if transport != secondTransport {
			t.Fatalf("重连 transport = %T, want secondTransport", transport)
		}
	case <-time.After(time.Second):
		t.Fatal("前代 close 完成后未创建新 transport")
	}
	assertInitializeRequestWithID(t, receiveWrite(t, secondTransport), "req_2")
	secondTransport.pushRead(successfulInitializeResponseWithID(
		"req_2",
		map[string]any{"session_id": "session-second"},
	))
	if err := receiveDone(t, reconnectDone); err != nil {
		t.Fatalf("前代 close 完成后 Connect() error = %v", err)
	}
	if core.streams.closeState != nil {
		t.Fatal("新 generation 不应继承旧 close state")
	}

	if err := core.Disconnect(context.Background()); err != nil {
		t.Fatalf("新 generation Disconnect() error = %v", err)
	}
	newCloseState := core.streams.closeState
	if newCloseState == nil || newCloseState == oldCloseState {
		t.Fatal("新 generation 应持有独立 close state")
	}
}

func TestDisconnectAfterTransportStartFailureCompletes(t *testing.T) {
	startErr := errors.New("transport start failed")
	transport := &startFailTransport{
		scriptedTransport: newScriptedTransport(),
		startErr:          startErr,
		closeCalled:       make(chan struct{}, 1),
	}
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)
	if err := core.Connect(context.Background()); !errors.Is(err, startErr) {
		t.Fatalf("Connect() error = %v, want start error", err)
	}
	if err := core.Disconnect(context.Background()); err != nil {
		t.Fatalf("Start 失败后的 Disconnect() error = %v", err)
	}
	select {
	case <-transport.closeCalled:
	default:
		t.Fatal("Start 失败后的 transport 未关闭")
	}
}

type observedReadTransport struct {
	*scriptedTransport
	assistantRead chan struct{}
	readOnce      sync.Once
}

func newObservedReadTransport() *observedReadTransport {
	return &observedReadTransport{
		scriptedTransport: newScriptedTransport(),
		assistantRead:     make(chan struct{}),
	}
}

func (t *observedReadTransport) ReadJSON() (map[string]any, error) {
	payload, err := t.scriptedTransport.ReadJSON()
	if err == nil && payload["type"] == "assistant" {
		t.readOnce.Do(func() { close(t.assistantRead) })
	}
	return payload, err
}

type nonClosingScriptedTransport struct {
	*scriptedTransport
}

func (t *nonClosingScriptedTransport) Close() error { return nil }

type blockingCloseTransport struct {
	*scriptedTransport
	closeStarted chan struct{}
	closeRelease chan struct{}
}

type observedDoneContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func newObservedDoneContext(parent context.Context) *observedDoneContext {
	return &observedDoneContext{
		Context:  parent,
		observed: make(chan struct{}),
	}
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func newBlockingCloseTransport() *blockingCloseTransport {
	return &blockingCloseTransport{
		scriptedTransport: newScriptedTransport(),
		closeStarted:      make(chan struct{}),
		closeRelease:      make(chan struct{}),
	}
}

func (t *blockingCloseTransport) Close() error {
	close(t.closeStarted)
	<-t.closeRelease
	return t.scriptedTransport.Close()
}

type startFailTransport struct {
	*scriptedTransport
	startErr    error
	closeCalled chan struct{}
}

func (t *startFailTransport) Start(context.Context) error {
	return t.startErr
}

func (t *startFailTransport) Close() error {
	t.closeCalled <- struct{}{}
	return t.scriptedTransport.Close()
}

type scriptedTransport struct {
	reads      chan map[string]any
	writes     chan map[string]any
	closed     chan struct{}
	once       sync.Once
	stderrTail string
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

func (t *scriptedTransport) StderrTail() string {
	return t.stderrTail
}

func (t *scriptedTransport) pushRead(payload map[string]any) {
	t.reads <- payload
}

func successfulInitializeResponse(response map[string]any) map[string]any {
	return successfulInitializeResponseWithID("req_1", response)
}

func successfulInitializeResponseWithID(requestID string, response map[string]any) map[string]any {
	return map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   response,
		},
	}
}

func assertInitializeRequest(t *testing.T, payload map[string]any) {
	assertInitializeRequestWithID(t, payload, "req_1")
}

func assertInitializeRequestWithID(t *testing.T, payload map[string]any, requestID string) {
	t.Helper()
	if payload["type"] != "control_request" || payload["request_id"] != requestID {
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
