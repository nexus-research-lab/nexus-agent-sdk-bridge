package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestSetNextTurnContextAttachesSystemReminderForNXS(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycle.setConnected(true)

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
	if content := message["content"].(string); content != "Continue." {
		t.Fatalf("content = %q, want original user message", content)
	}
	reminder := payload[protocol.InternalContextPayloadKey].(string)
	for _, want := range []string{
		"<system-reminder>",
		`<internal_context source="goal">`,
		"Compare the current state against the goal",
	} {
		if !strings.Contains(reminder, want) {
			t.Fatalf("reminder missing %q:\n%s", want, reminder)
		}
	}
}

func TestSetNextTurnContextUsesClaudeUserPromptSubmitHook(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{Runtime: RuntimeOptions{Kind: RuntimeClaude}}, transport)
	core.lifecycle.setConnected(true)
	request := core.buildInitializeRequest()

	if err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{Name: "goal", Content: "Claude context"}}); err != nil {
		t.Fatalf("setNextTurnContext() error = %v", err)
	}
	if err := core.Send(context.Background(), "visible", nil, "session-1"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	payload := transport.writes[0]
	if _, exists := payload[protocol.InternalContextPayloadKey]; exists {
		t.Fatalf("payload = %#v, Claude stdin must not receive nxs-only context field", payload)
	}
	content := payload["message"].(map[string]any)["content"].(string)
	if content != "visible" {
		t.Fatalf("content = %q, want untouched user text", content)
	}

	matchers := request.Hooks[string(hook.EventUserPromptSubmit)].([]map[string]any)
	callbackIDs := matchers[len(matchers)-1]["hookCallbackIds"].([]string)
	response, _, err := core.resolveHookCallback(context.Background(), map[string]any{
		"callback_id": callbackIDs[0],
		"input":       map[string]any{"hook_event_name": string(hook.EventUserPromptSubmit)},
	})
	if err != nil {
		t.Fatalf("resolveHookCallback() error = %v", err)
	}
	specific := response["hookSpecificOutput"].(map[string]any)
	additionalContext := specific["additionalContext"].(string)
	if !strings.Contains(additionalContext, "Claude context") || strings.Contains(additionalContext, "<system-reminder>") {
		t.Fatalf("additionalContext = %q, want raw CC hook context", additionalContext)
	}
}

func TestNextTurnContextBindsOnlyToNextMessage(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycle.setConnected(true)

	if err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{Name: "goal", Content: "bound context"}}); err != nil {
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
	firstContext := transport.writes[0][protocol.InternalContextPayloadKey].(string)
	if !strings.Contains(firstContext, "bound context") {
		t.Fatalf("first context = %q, want attached context", firstContext)
	}
	if _, exists := transport.writes[1][protocol.InternalContextPayloadKey]; exists {
		t.Fatalf("second payload = %#v, want no duplicate binding", transport.writes[1])
	}
}

func TestNextTurnContextPrependsStructuredContentBlock(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycle.setConnected(true)

	if err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{Name: "goal", Content: "structured context"}}); err != nil {
		t.Fatalf("setNextTurnContext() error = %v", err)
	}
	message := protocol.NewUserBlocksMessage(protocol.NewTextContent("visible content"))
	if err := core.SendMessage(context.Background(), message, "session-1"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	content := transport.writes[0]["message"].(map[string]any)["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("len(content) = %d, want original block only", len(content))
	}
	if content[0].(map[string]any)["text"] != "visible content" {
		t.Fatalf("content = %#v, want original content", content)
	}
	if reminder := transport.writes[0][protocol.InternalContextPayloadKey].(string); !strings.Contains(reminder, "structured context") {
		t.Fatalf("reminder = %q, want structured context", reminder)
	}
}

func TestClearNextTurnContextKeepsAtomicUserInputUntouched(t *testing.T) {
	transport := &capturingTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycle.setConnected(true)

	if err := core.setNextTurnContext(context.Background(), []InternalContextBlock{{
		Name:    "goal",
		Content: "must not reach the command",
	}}); err != nil {
		t.Fatalf("setNextTurnContext() error = %v", err)
	}
	if err := core.clearNextTurnContext(context.Background()); err != nil {
		t.Fatalf("clearNextTurnContext() error = %v", err)
	}
	if err := core.Send(context.Background(), "/model sonnet", nil, "session-1"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	content := transport.writes[0]["message"].(map[string]any)["content"].(string)
	if content != "/model sonnet" {
		t.Fatalf("atomic command content = %q, want unchanged slash input", content)
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
