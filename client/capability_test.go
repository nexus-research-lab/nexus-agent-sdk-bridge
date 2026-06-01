package client

import (
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
