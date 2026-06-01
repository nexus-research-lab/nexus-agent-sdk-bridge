package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestSetNextTurnContextInjectsSystemReminderIntoNextUserMessage(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)

	err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{
		Name:    "goal",
		Content: "Compare the current state against the goal and continue if needed.",
		Metadata: map[string]string{
			"goal_id": "goal-1",
		},
	}})
	if err != nil {
		t.Fatalf("setNextTurnContext() error = %v", err)
	}
	err = core.SendWithOptions(context.Background(), "Continue.", nil, "session-1", protocol.OutboundMessageOptions{
		Synthetic:      true,
		HiddenFromUser: true,
		Purpose:        "goal_continuation",
		Priority:       "internal",
	})
	if err != nil {
		t.Fatalf("SendWithOptions() error = %v", err)
	}
	if len(transport.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(transport.writes))
	}
	payload := transport.writes[0]
	if payload["hidden_from_user"] != true || payload["is_synthetic"] != true {
		t.Fatalf("payload options = %#v, want hidden synthetic", payload)
	}
	message := payload["message"].(map[string]any)
	content := message["content"].(string)
	for _, want := range []string{
		"<system-reminder>",
		`<internal_context source="goal">`,
		"Compare the current state against the goal",
		"Continue.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q:\n%s", want, content)
		}
	}
}

func TestNextTurnContextIsConsumedOnce(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)

	if err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{Name: "goal", Content: "one-shot context"}}); err != nil {
		t.Fatalf("setNextTurnContext() error = %v", err)
	}
	if err := core.Send(context.Background(), "first", nil, "session-1"); err != nil {
		t.Fatalf("first Send() error = %v", err)
	}
	if err := core.Send(context.Background(), "second", nil, "session-1"); err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	if len(transport.writes) != 2 {
		t.Fatalf("writes = %d, want 2", len(transport.writes))
	}
	firstContent := transport.writes[0]["message"].(map[string]any)["content"].(string)
	secondContent := transport.writes[1]["message"].(map[string]any)["content"].(string)
	if !strings.Contains(firstContent, "one-shot context") {
		t.Fatalf("first content = %q, want injected context", firstContent)
	}
	if strings.Contains(secondContent, "one-shot context") {
		t.Fatalf("second content = %q, want context consumed", secondContent)
	}
}

func TestNextTurnContextPrependsStructuredContentBlock(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)

	if err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{Name: "goal", Content: "structured context"}}); err != nil {
		t.Fatalf("setNextTurnContext() error = %v", err)
	}
	message := protocol.NewUserBlocksMessage(protocol.NewTextContent("visible content"))
	if err := core.SendMessage(context.Background(), message, "session-1"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	content := transport.writes[0]["message"].(map[string]any)["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("len(content) = %d, want 2", len(content))
	}
	firstBlock := content[0].(map[string]any)
	if firstBlock["type"] != "text" || !strings.Contains(firstBlock["text"].(string), "structured context") {
		t.Fatalf("first block = %#v, want internal context reminder", firstBlock)
	}
	secondBlock := content[1].(map[string]any)
	if secondBlock["text"] != "visible content" {
		t.Fatalf("second block = %#v, want original content", secondBlock)
	}
}

type capturingTransport struct {
	writes []map[string]any
}

func (t *capturingTransport) Start(context.Context) error { return nil }

func (t *capturingTransport) ReadJSON() (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (t *capturingTransport) WriteJSON(payload any) error {
	if message, ok := payload.(map[string]any); ok {
		t.writes = append(t.writes, message)
		return nil
	}
	return errors.New("payload is not a map")
}

func (t *capturingTransport) EndInput() error  { return nil }
func (t *capturingTransport) Interrupt() error { return nil }
func (t *capturingTransport) Wait() error      { return nil }
func (t *capturingTransport) Close() error     { return nil }
