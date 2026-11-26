package client

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRunningInputRequiresNegotiation(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(NewOptions().WithRuntime(RuntimeNXS).WithSessionID("session-running"), transport)
	core.lifecycle.setConnected(true)
	session := &Session{core: core}

	err := session.SendRunningInput(context.Background(), "input-1", protocol.UserTextMessage{Text: "补充"})
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("SendRunningInput() before negotiation error = %v, want unsupported capability", err)
	}
	if len(transport.writes) != 0 {
		t.Fatalf("writes = %#v, want none before negotiation", transport.writes)
	}
}

func TestRunningInputCarriesInputIDWithoutNextTurnContext(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(NewOptions().WithRuntime(RuntimeNXS).WithSessionID("session-running"), transport)
	core.lifecycle.setConnected(true)
	core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{runningInputProtocolCapability}})
	session := &Session{core: core}
	if err := session.Control().SetNextTurnContext(context.Background(), []InternalContextBlock{{Name: "next", Content: "只属于下一轮"}}); err != nil {
		t.Fatal(err)
	}

	if err := session.SendRunningInput(context.Background(), "input-1", protocol.UserTextMessage{Text: "补充"}); err != nil {
		t.Fatalf("SendRunningInput() error = %v", err)
	}
	payload := transport.writes[len(transport.writes)-1]
	if payload["type"] != "user" || payload["input_id"] != "input-1" || payload["session_id"] != "session-running" {
		t.Fatalf("payload = %#v, want user message with input_id", payload)
	}
	if _, ok := payload[protocol.InternalContextPayloadKey]; ok {
		t.Fatalf("running input carried next turn context: %#v", payload)
	}
	if err := core.Send(context.Background(), "下一轮", nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := transport.writes[len(transport.writes)-1][protocol.InternalContextPayloadKey]; !ok {
		t.Fatal("next turn context was consumed by running input")
	}

	if err := session.SendRunningInput(context.Background(), " ", protocol.UserTextMessage{Text: "补充"}); err == nil {
		t.Fatal("SendRunningInput() accepted empty input_id")
	}
}

func TestDecodeInputConsumedSystemMessage(t *testing.T) {
	message, err := protocol.DecodeMessage(map[string]any{
		"type":       "system",
		"subtype":    protocol.SystemSubtypeInputConsumed,
		"session_id": "session-running",
		"input_ids":  []any{"input-1", "input-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if message.System == nil || message.System.InputConsumed == nil ||
		!slices.Equal(message.System.InputConsumed.InputIDs, []string{"input-1", "input-2"}) {
		t.Fatalf("system = %#v, want input_consumed ids", message.System)
	}
}
