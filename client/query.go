package client

import (
	"context"
	"errors"
	"io"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// QueryRequest 表示一次性查询请求。
type QueryRequest struct {
	Prompt   string
	Messages <-chan protocol.OutboundMessage
	Options  Options
}

// PromptRequest 表示只关心最终 result 的一次性查询请求。
type PromptRequest struct {
	Prompt   string
	Messages <-chan protocol.OutboundMessage
	Options  Options
}

// Stream 表示 SDK 返回的消息流。
type Stream struct {
	client     *sessionClient
	ownsClient bool
	closeInput bool
}

// Query 创建新会话并执行一次性查询，调用方负责在读完后 Close。
func Query(ctx context.Context, request QueryRequest) (*Stream, error) {
	session, err := newSession(ctx, request.Options)
	if err != nil {
		return nil, err
	}

	stream := &Stream{
		client:     session.client,
		ownsClient: true,
		closeInput: request.Messages == nil,
	}
	if request.Messages != nil {
		session.client.startMessageStream(request.Messages)
		return stream, nil
	}
	if err := session.client.Query(ctx, request.Prompt); err != nil {
		_ = session.client.Disconnect(ctx)
		return nil, err
	}
	go stream.closeInputAfterResult()
	return stream, nil
}

// Prompt 执行一次性查询并返回最终 result。
func Prompt(ctx context.Context, request PromptRequest) (protocol.ResultMessage, error) {
	stream, err := Query(ctx, QueryRequest{
		Prompt:   request.Prompt,
		Messages: request.Messages,
		Options:  request.Options,
	})
	if err != nil {
		return protocol.ResultMessage{}, err
	}
	defer stream.Close(ctx)
	return stream.Result(ctx)
}

// Recv 读取下一条 SDK 消息。
func (s *Stream) Recv(ctx context.Context) (protocol.ReceivedMessage, error) {
	if s == nil || s.client == nil {
		return protocol.ReceivedMessage{}, ErrNotConnected
	}

	messages := s.client.Messages()
	select {
	case <-ctx.Done():
		return protocol.ReceivedMessage{}, ctx.Err()
	case message, ok := <-messages:
		if !ok {
			return protocol.ReceivedMessage{}, io.EOF
		}
		return message, nil
	}
}

// Result 读取消息直到首个 result，并返回 result payload。
func (s *Stream) Result(ctx context.Context) (protocol.ResultMessage, error) {
	var lastMessage protocol.ReceivedMessage
	for {
		message, err := s.Recv(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if waitErr := s.client.Wait(); waitErr != nil {
					return protocol.ResultMessage{}, waitErr
				}
				return protocol.ResultMessage{}, &StreamClosedBeforeTerminalError{
					LastMessageID:   lastMessage.UUID,
					LastMessageType: string(lastMessage.Type),
					SessionID:       lastMessage.SessionID,
				}
			}
			return protocol.ResultMessage{}, err
		}
		lastMessage = message
		if message.Type == protocol.MessageTypeResult {
			if message.Result == nil {
				return protocol.ResultMessage{}, ErrNoResult
			}
			return *message.Result, nil
		}
	}
}

// Close 释放一次性查询持有的底层会话。
func (s *Stream) Close(ctx context.Context) error {
	if s == nil || s.client == nil || !s.ownsClient {
		return nil
	}
	return s.client.Disconnect(ctx)
}

func (s *Stream) closeInputAfterResult() {
	if s == nil || s.client == nil || !s.closeInput {
		return
	}
	s.client.finishInputStream()
}
