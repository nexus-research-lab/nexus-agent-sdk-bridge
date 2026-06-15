package client

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestErrAbortedWrapsContextCancellation(t *testing.T) {
	err := abortError(context.Canceled)
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("abortError(context.Canceled) does not match ErrAborted: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("abortError(context.Canceled) does not preserve context.Canceled: %v", err)
	}

	deadlineErr := abortError(context.DeadlineExceeded)
	if errors.Is(deadlineErr, ErrAborted) {
		t.Fatalf("abortError(context.DeadlineExceeded) unexpectedly matches ErrAborted")
	}
}

func TestStreamRecvReturnsErrAbortedOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := &Stream{core: newSessionCore(Options{})}
	_, err := stream.Recv(ctx)
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("Recv() error = %v, want ErrAborted", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Recv() error = %v, want context.Canceled", err)
	}
}

func TestSessionWaitReturnsErrAbortedForCancelledRead(t *testing.T) {
	core := newSessionCore(Options{})
	core.setReadError(context.Canceled)
	close(core.streamState().readDone)

	err := core.Wait()
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("Wait() error = %v, want ErrAborted", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v, want context.Canceled", err)
	}
}

func TestStreamResultReturnsLastStreamStopDiagnostics(t *testing.T) {
	core := newSessionCore(Options{})
	streams := core.streamState()
	streams.messages <- protocol.ReceivedMessage{
		Type:      protocol.MessageTypeStreamEvent,
		SessionID: "session-1",
		Stream: &protocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-1",
					"model": "kimi-k2.6",
				},
			},
		},
	}
	streams.messages <- protocol.ReceivedMessage{
		Type:      protocol.MessageTypeStreamEvent,
		SessionID: "session-1",
		Stream: &protocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "tool_use",
				},
			},
		},
	}
	streams.messages <- protocol.ReceivedMessage{
		Type:      protocol.MessageTypeStreamEvent,
		SessionID: "session-1",
		Stream: &protocol.StreamEvent{
			Event: map[string]any{"type": "message_stop"},
		},
	}
	streams.messages <- protocol.ReceivedMessage{
		Type:      protocol.MessageTypeTaskProgress,
		SessionID: "session-1",
	}
	close(streams.messages)
	close(streams.readDone)

	stream := &Stream{core: core}
	_, err := stream.Result(context.Background())
	if !errors.Is(err, ErrNoResult) {
		t.Fatalf("Result() error = %v, want ErrNoResult", err)
	}
	if errors.Is(err, io.EOF) {
		t.Fatalf("Result() should wrap EOF as StreamClosedBeforeTerminalError: %v", err)
	}
	var streamErr *StreamClosedBeforeTerminalError
	if !errors.As(err, &streamErr) {
		t.Fatalf("Result() error = %T %[1]v, want StreamClosedBeforeTerminalError", err)
	}
	stop := streamErr.LastStreamStop
	if !stop.Observed ||
		stop.MessageIndex != 3 ||
		stop.MessagesAfter != 1 ||
		stop.StopReason != "tool_use" ||
		stop.SessionID != "session-1" ||
		stop.MessageID != "assistant-1" ||
		stop.Model != "kimi-k2.6" {
		t.Fatalf("LastStreamStop = %+v, want populated message_stop diagnostics", stop)
	}
	if !strings.Contains(err.Error(), "messages_after_last_stream_stop=1") {
		t.Fatalf("error string missing stream stop diagnostics: %v", err)
	}
}
