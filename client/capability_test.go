package client

import (
	"context"
	"errors"
	"testing"
)

func TestSessionSupportsHostRuntimePrimitives(t *testing.T) {
	session := &Session{core: newSessionCore(Options{})}
	if !session.Supports(CapabilitySendOptions) {
		t.Fatal("Supports(send_options) = false, want true")
	}
	if !session.Supports(CapabilityTypedUsage) {
		t.Fatal("Supports(typed_usage) = false, want true")
	}
	if !session.Supports(CapabilityTerminalCategory) {
		t.Fatal("Supports(terminal_category) = false, want true")
	}
	if !session.Supports(CapabilityInternalContext) {
		t.Fatal("Supports(internal_context) = false, want true")
	}
	if !session.Supports(CapabilityStopTask) {
		t.Fatal("Supports(stop_task) = false, want true")
	}
	if !session.Supports(CapabilityInProcessMCP) {
		t.Fatal("Supports(in_process_mcp) = false, want true")
	}
	if !session.Supports(CapabilitySendTaskMessage) {
		t.Fatal("Supports(send_task_message) = false, want true")
	}
	if !session.Supports(CapabilityAutoDream) {
		t.Fatal("Supports(auto_dream) = false, want true")
	}
}

func TestClaudeSessionDistinguishesSubagentTaskCapabilities(t *testing.T) {
	session := &Session{core: newSessionCore(Options{
		Runtime: RuntimeOptions{Kind: RuntimeClaude},
	})}
	if !session.Supports(CapabilityStopTask) {
		t.Fatal("Supports(stop_task) = false, want true")
	}
	if !session.Supports(CapabilityInProcessMCP) {
		t.Fatal("Supports(in_process_mcp) = false, want true")
	}
	if session.Supports(CapabilitySendTaskMessage) {
		t.Fatal("Supports(send_task_message) = true, want false")
	}
	if session.Supports(CapabilityAutoDream) {
		t.Fatal("Supports(auto_dream) = true, want false")
	}

	err := session.Control().SendTaskMessage(context.Background(), "task-1", "continue", "continue")
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("SendTaskMessage() error = %v, want ErrUnsupportedCapability", err)
	}
	_, err = session.Control().TryAutoDream(context.Background())
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("TryAutoDream() error = %v, want ErrUnsupportedCapability", err)
	}
}

func TestUnsupportedCapabilityError(t *testing.T) {
	err := &UnsupportedCapabilityError{Capability: CapabilityInternalContext}
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("errors.Is(%v, ErrUnsupportedCapability) = false, want true", err)
	}
}

func TestStreamClosedBeforeTerminalError(t *testing.T) {
	err := &StreamClosedBeforeTerminalError{
		LastMessageID:   "msg-1",
		LastMessageType: "assistant",
		SessionID:       "session-1",
	}
	if !errors.Is(err, ErrNoResult) {
		t.Fatalf("errors.Is(%v, ErrNoResult) = false, want true", err)
	}
}
