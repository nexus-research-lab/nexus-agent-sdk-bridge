package client

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// StreamStopDiagnostics 表示最近一次 provider message_stop 的定位信息。
type StreamStopDiagnostics struct {
	Observed      bool
	MessageIndex  int
	MessagesAfter int
	Summary       string
	StopReason    string
	SessionID     string
	MessageID     string
	Model         string
}

func (d StreamStopDiagnostics) attributes() map[string]any {
	if !d.Observed {
		return nil
	}
	return map[string]any{
		"summary":        d.Summary,
		"stop_reason":    d.StopReason,
		"session_id":     d.SessionID,
		"message_id":     d.MessageID,
		"model":          d.Model,
		"message_index":  d.MessageIndex,
		"messages_after": d.MessagesAfter,
	}
}

type streamDiagnosticsTracker struct {
	currentMessageID  string
	currentModel      string
	currentStopReason string
	lastStreamStop    StreamStopDiagnostics
}

func (t *streamDiagnosticsTracker) observe(message protocol.ReceivedMessage, messageIndex int) StreamStopDiagnostics {
	if message.Type != protocol.MessageTypeStreamEvent || message.Stream == nil {
		return StreamStopDiagnostics{}
	}
	payload := streamEventPayload(message)
	eventType := strings.TrimSpace(jsonvalue.StringValue(payload["type"]))
	switch eventType {
	case "message_start":
		startMessage := jsonvalue.MapValue(payload["message"])
		t.currentMessageID = strings.TrimSpace(jsonvalue.StringValue(startMessage["id"]))
		t.currentModel = strings.TrimSpace(jsonvalue.StringValue(startMessage["model"]))
		t.currentStopReason = ""
	case "message_delta":
		delta := jsonvalue.MapValue(payload["delta"])
		if stopReason := strings.TrimSpace(jsonvalue.StringValue(delta["stop_reason"])); stopReason != "" {
			t.currentStopReason = stopReason
		}
	case "message_stop":
		t.lastStreamStop = StreamStopDiagnostics{
			Observed:     true,
			MessageIndex: messageIndex,
			Summary:      "stream message_stop",
			StopReason: firstNonEmptyString(
				strings.TrimSpace(jsonvalue.StringValue(payload["stop_reason"])),
				t.currentStopReason,
			),
			SessionID: strings.TrimSpace(message.SessionID),
			MessageID: firstNonEmptyString(
				strings.TrimSpace(receivedMessageID(message)),
				t.currentMessageID,
			),
			Model: firstNonEmptyString(
				strings.TrimSpace(jsonvalue.StringValue(payload["model"])),
				t.currentModel,
			),
		}
		return t.lastStreamStop
	}
	return StreamStopDiagnostics{}
}

func (t streamDiagnosticsTracker) snapshot(messagesSeen int) StreamStopDiagnostics {
	result := t.lastStreamStop
	if !result.Observed {
		return result
	}
	if messagesSeen > result.MessageIndex {
		result.MessagesAfter = messagesSeen - result.MessageIndex
	}
	return result
}

func streamEventPayload(message protocol.ReceivedMessage) map[string]any {
	if message.Stream == nil {
		return nil
	}
	if payload := jsonvalue.MapValue(message.Stream.Event); len(payload) > 0 {
		return payload
	}
	return jsonvalue.MapValue(message.Stream.Data)
}

func appendStreamStopErrorDetail(message string, diagnostics StreamStopDiagnostics) string {
	if !diagnostics.Observed {
		return message
	}
	return fmt.Sprintf(
		"%s; last_stream_stop_summary=%q; last_stream_stop_reason=%s; messages_after_last_stream_stop=%d",
		message,
		diagnostics.Summary,
		diagnostics.StopReason,
		diagnostics.MessagesAfter,
	)
}

func receivedMessageID(message protocol.ReceivedMessage) string {
	if strings.TrimSpace(message.UUID) != "" {
		return strings.TrimSpace(message.UUID)
	}
	if message.Assistant != nil && strings.TrimSpace(message.Assistant.Message.ID) != "" {
		return strings.TrimSpace(message.Assistant.Message.ID)
	}
	if message.Stream != nil {
		payload := streamEventPayload(message)
		if messagePayload := jsonvalue.MapValue(payload["message"]); len(messagePayload) > 0 {
			return strings.TrimSpace(jsonvalue.StringValue(messagePayload["id"]))
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
