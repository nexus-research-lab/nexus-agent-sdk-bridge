package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (c *sessionCore) Connect(ctx context.Context) error {
	lifecycle := c.lifecycleState()
	lifecycle.lockConnection()
	if lifecycle.connectedLocked() {
		lifecycle.unlockConnection()
		return nil
	}
	c.resetLifecycleIfNeededLocked()

	normalizedOptions, err := c.options.normalized()
	if err != nil {
		lifecycle.unlockConnection()
		return err
	}
	c.options = normalizedOptions
	c.replaceSDKMCPServers(c.options.sdkMCPServerRegistry())

	if c.transport == nil {
		c.transport, err = c.buildTransport(c.options)
		if err != nil {
			lifecycle.setConnectedLocked(false)
			lifecycle.unlockConnection()
			return err
		}
	}
	lifecycle.setConnectedLocked(true)
	lifecycle.unlockConnection()

	if err := c.transport.Start(ctx); err != nil {
		lifecycle.setConnected(false)
		return classifyTransportStartError(c.options, err)
	}

	go c.readLoop()

	response, err := c.sendControlRequest(
		ctx,
		c.buildInitializeRequest(),
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		_ = c.Disconnect(ctx)
		return err
	}

	lifecycle.setInitializeResponse(runtimeinfo.DecodeInitializeResponse(response))
	return nil
}

// Wait 等待会话结束。
func (c *sessionCore) Wait() error {
	streams := c.streamState()
	<-streams.readDone
	if c.lifecycleState().inputStreamActiveValue() && !transport.ChannelClosed(streams.inputClosed) {
		c.finishInputStream()
	}

	var result error
	if c.transport != nil {
		result = c.transport.Wait()
	}
	if c.getReadError() == nil {
		result = withLastErrorResult(result, c.lifecycleState().lastErrorResultValue())
	}
	return abortError(classifyProcessExitError(joinErrors(c.getReadError(), result)))
}

// Disconnect 断开连接。
func (c *sessionCore) Disconnect(ctx context.Context) error {
	if !c.isConnected() && c.transport == nil {
		return nil
	}
	streams := c.streamState()
	var disconnectErr error
	c.lifecycleState().closeOnceDo(func() {
		c.lifecycleState().setConnected(false)

		if c.transport != nil {
			disconnectErr = joinErrors(disconnectErr, c.transport.Close())
		}

		<-streams.readDone
		disconnectErr = joinErrors(disconnectErr, c.getReadError())
	})
	return disconnectErr
}

// SessionID 返回当前会话 ID。
func (c *sessionCore) SessionID() string {
	return c.lifecycleState().sessionIDValue()
}

func (c *sessionCore) isConnected() bool {
	return c.lifecycleState().isConnected()
}

func (c *sessionCore) resetLifecycleIfNeededLocked() {
	streams := c.streamState()
	if !transport.ChannelClosed(streams.readDone) {
		return
	}
	streams.reset()
	c.lifecycleState().resetRuntimeState("")
	if !c.customTransport {
		c.transport = nil
	}
}

func (c *sessionCore) markDisconnected() {
	c.lifecycleState().setConnected(false)
}

func (c *sessionCore) markTransportFailed(err error) {
	if err != nil {
		c.setReadError(err)
	}
	c.markDisconnected()
	c.failPendingRequests(joinErrors(c.getReadError(), ErrNotConnected))
}

func (c *sessionCore) currentSessionID() string {
	return c.lifecycleState().currentSessionID(defaultSessionID, "", c.options.Session.ResumeID)
}

func (c *sessionCore) buildTransport(options Options) (Transport, error) {
	if c.transportFactory == nil {
		if options.DirectConnect != nil {
			directConnectConfig, err := options.directConnectConfig()
			if err != nil {
				return nil, err
			}
			return transport.NewDirectConnectTransport(directConnectConfig), nil
		}
		return transport.NewProcessTransport(options.processConfig()), nil
	}
	transport := c.transportFactory(options)
	if transport == nil {
		return nil, errors.New("client: transport factory returned nil")
	}
	return transport, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return jsonvalue.CloneMap(input)
}

func (c *sessionCore) ConnectWithPrompt(ctx context.Context, prompt string) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	return c.Query(ctx, prompt)
}

// ConnectWithMessages 建立连接并在后台流式发送强类型 SDK 消息。
func (c *sessionCore) ConnectWithMessages(ctx context.Context, messages <-chan protocol.OutboundMessage) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	c.lifecycleState().setInputStreamActive(true)
	c.startMessageStream(messages)
	return nil
}

// ConnectWithRawMessages 建立连接并在后台流式发送原始 SDK 消息。
func (c *sessionCore) ConnectWithRawMessages(ctx context.Context, messages <-chan map[string]any) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	c.lifecycleState().setInputStreamActive(true)
	c.startRawMessageStream(messages)
	return nil
}

// Query 发送一条字符串 prompt。
func (c *sessionCore) Query(ctx context.Context, prompt string) error {
	return c.Send(ctx, prompt, nil, "")
}

// Send 发送一条用户消息。
func (c *sessionCore) Send(ctx context.Context, prompt string, parentToolUseID *string, sessionID string) error {
	return c.sendContent(ctx, prompt, parentToolUseID, sessionID)
}

// SendMessage 发送一条强类型 SDK 消息。
func (c *sessionCore) SendMessage(ctx context.Context, message protocol.OutboundMessage, sessionID string) error {
	if message == nil {
		return nil
	}
	return c.SendRawMessage(ctx, protocol.EncodeOutboundMessage(message, sessionID), sessionID)
}

// SendRawMessage 发送一条原始 SDK 消息。
func (c *sessionCore) SendRawMessage(ctx context.Context, message map[string]any, sessionID string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	if len(message) == 0 {
		return nil
	}

	payload := cloneMap(message)
	if payload["session_id"] == nil {
		effectiveSessionID := sessionID
		if effectiveSessionID == "" {
			effectiveSessionID = c.currentSessionID()
		}
		payload["session_id"] = effectiveSessionID
	}

	if c.transport == nil {
		return ErrNotConnected
	}
	if err := c.transport.WriteJSON(payload); err != nil {
		if transport.IsTransportWriteFailure(err) {
			c.markTransportFailed(fmt.Errorf("client: send sdk message failed: %w", err))
		}
		return fmt.Errorf("client: send sdk message failed: %w", err)
	}
	return nil
}

func (c *sessionCore) startMessageStream(messages <-chan protocol.OutboundMessage) {
	streams := c.streamState()
	go func() {
		for {
			select {
			case <-streams.readDone:
				c.finishInputStreamFor(streams)
				return
			case message, ok := <-messages:
				if !ok {
					c.finishInputStreamFor(streams)
					return
				}
				if err := c.SendMessage(context.Background(), message, ""); err != nil {
					c.setReadError(err)
					_ = c.Disconnect(context.Background())
					return
				}
			}
		}
	}()
}

func (c *sessionCore) startRawMessageStream(messages <-chan map[string]any) {
	streams := c.streamState()
	go func() {
		for {
			select {
			case <-streams.readDone:
				c.finishInputStreamFor(streams)
				return
			case message, ok := <-messages:
				if !ok {
					c.finishInputStreamFor(streams)
					return
				}
				if err := c.SendRawMessage(context.Background(), message, ""); err != nil {
					c.setReadError(err)
					_ = c.Disconnect(context.Background())
					return
				}
			}
		}
	}()
}

func (c *sessionCore) finishInputStream() {
	c.finishInputStreamFor(c.streamState())
}

func (c *sessionCore) finishInputStreamFor(streams *sessionStreams) {
	select {
	case <-streams.firstResult:
	case <-streams.readDone:
	}
	_ = c.CloseInput()
	c.lifecycleState().inputCloseOnceDo(func() {
		c.lifecycleState().setInputStreamActive(false)
		close(streams.inputClosed)
	})
}

func (c *sessionCore) sendContent(ctx context.Context, content any, parentToolUseID *string, sessionID string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	effectiveSessionID := sessionID
	if effectiveSessionID == "" {
		effectiveSessionID = c.currentSessionID()
	}

	payload := map[string]any{
		"type":               "user",
		"session_id":         effectiveSessionID,
		"parent_tool_use_id": parentToolUseID,
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}

	return c.SendRawMessage(ctx, payload, effectiveSessionID)
}

// ReceiveMessages 返回消息流。
func (c *sessionCore) ReceiveMessages(ctx context.Context) <-chan protocol.ReceivedMessage {
	return c.streamState().messages
}

// Messages 返回消息流。
func (c *sessionCore) Messages() <-chan protocol.ReceivedMessage {
	return c.streamState().messages
}

// ReceiveResponse 读取直到首个 result 的消息流。
func (c *sessionCore) ReceiveResponse(ctx context.Context) <-chan protocol.ReceivedMessage {
	response := make(chan protocol.ReceivedMessage, 16)
	go func() {
		defer close(response)
		streams := c.streamState()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-streams.messages:
				if !ok {
					return
				}
				response <- message
				if message.Type == protocol.MessageTypeResult {
					return
				}
			}
		}
	}()
	return response
}

// CloseInput 主动关闭输入流。
func (c *sessionCore) CloseInput() error {
	if c.transport == nil {
		return ErrNotConnected
	}
	return c.transport.EndInput()
}

func (c *sessionCore) readLoop() {
	streams := c.streamState()
	defer c.markDisconnected()
	defer close(streams.readDone)
	defer close(streams.messages)

	for {
		payload, err := c.transport.ReadJSON()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.setReadError(fmt.Errorf("client: read message failed: %w", err))
			}
			c.failPendingRequests(joinErrors(c.getReadError(), ErrNotConnected))
			return
		}

		switch payloadType := jsonvalue.StringValue(payload["type"]); payloadType {
		case "control_response":
			c.handleControlResponse(payload)
		case "control_request":
			go c.handleControlRequest(payload)
		case "control_cancel_request":
			c.handleControlCancelRequest(payload)
		default:
			message, decodeErr := protocol.DecodeMessage(payload)
			if decodeErr != nil {
				c.setReadError(decodeErr)
				c.failPendingRequests(decodeErr)
				return
			}

			if message.SessionID != "" {
				c.lifecycleState().setSessionID(message.SessionID)
			}
			c.trackLastErrorResult(message)
			if message.Type == protocol.MessageTypeResult {
				c.signalFirstResult()
			}
			c.emitMessage(message)
		}
	}
}

func (c *sessionCore) sendInternalRawMessage(message map[string]any, sessionID string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	if len(message) == 0 {
		return nil
	}

	payload := cloneMap(message)
	if payload["session_id"] == nil {
		effectiveSessionID := sessionID
		if effectiveSessionID == "" {
			effectiveSessionID = c.currentSessionID()
		}
		payload["session_id"] = effectiveSessionID
	}
	if c.transport == nil {
		return ErrNotConnected
	}
	if err := c.transport.WriteJSON(payload); err != nil {
		if transport.IsTransportWriteFailure(err) {
			c.markTransportFailed(fmt.Errorf("client: send continuation message failed: %w", err))
		}
		return fmt.Errorf("client: send continuation message failed: %w", err)
	}
	return nil
}

func (c *sessionCore) emitMessage(message protocol.ReceivedMessage) {
	streams := c.streamState()
	if message.Type == protocol.MessageTypeResult {
		c.signalFirstResult()
	}
	streams.messages <- message
}

func (c *sessionCore) signalFirstResult() {
	streams := c.streamState()
	c.lifecycleState().firstResultOnceDo(func() {
		close(streams.firstResult)
	})
}

func (c *sessionCore) trackLastErrorResult(message protocol.ReceivedMessage) {
	if message.Type == protocol.MessageTypeResult && message.Result != nil {
		if message.Result.IsError {
			c.lifecycleState().setLastErrorResult(resultErrorText(message.Result))
			return
		}
		c.lifecycleState().setLastErrorResult("")
		return
	}
	if message.Type == protocol.MessageTypeSystem && message.Subtype == "session_state_changed" {
		return
	}
	c.lifecycleState().setLastErrorResult("")
}

func resultErrorText(result *protocol.ResultMessage) string {
	if result == nil {
		return "unknown error"
	}
	if len(result.Errors) > 0 {
		parts := make([]string, 0, len(result.Errors))
		for _, item := range result.Errors {
			if text := strings.TrimSpace(item); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	if text := strings.TrimSpace(result.Subtype); text != "" {
		return text
	}
	return "unknown error"
}
