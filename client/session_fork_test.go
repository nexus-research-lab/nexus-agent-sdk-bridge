package client

import (
	"strings"
	"testing"
)

func TestForkSessionOptionsUsesIndependentResumeBoundary(t *testing.T) {
	prepared, err := forkSessionOptions(" source-session ", " completed-message ", Options{
		Session: SessionOptions{ContinueLatest: true},
	})
	if err != nil {
		t.Fatalf("forkSessionOptions() error = %v", err)
	}
	if prepared.Session.ResumeID != "source-session" ||
		prepared.Session.ResumeAt != "completed-message" ||
		!prepared.Session.Fork ||
		prepared.Session.ContinueLatest {
		t.Fatalf("forkSessionOptions() = %+v", prepared.Session)
	}
}

func TestForkSessionOptionsRequiresSourceAndBoundary(t *testing.T) {
	for _, test := range []struct {
		name      string
		sessionID string
		messageID string
	}{
		{name: "source", messageID: "message"},
		{name: "boundary", sessionID: "session"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := forkSessionOptions(test.sessionID, test.messageID, Options{}); err == nil {
				t.Fatal("forkSessionOptions() error = nil, want validation error")
			}
		})
	}
}

func TestClaudeForkAssignsIndependentSessionID(t *testing.T) {
	prepared, err := forkSessionOptions("source-session", "completed-message", Options{
		Runtime: RuntimeOptions{Kind: RuntimeClaude},
		CLIPath: "claude",
	})
	if err != nil {
		t.Fatalf("forkSessionOptions() error = %v", err)
	}
	normalized, err := prepared.normalized()
	if err != nil {
		t.Fatalf("normalized() error = %v", err)
	}
	if id := normalized.Session.ID; len(id) != 36 || strings.Count(id, "-") != 4 || id == prepared.Session.ResumeID {
		t.Fatalf("fork session id = %q", id)
	}
	assertArgValue(t, normalized.buildArgs(), "--session-id", normalized.Session.ID)
	if id := (&Session{core: newSessionCore(normalized)}).ID(); id != normalized.Session.ID {
		t.Fatalf("Session.ID() = %q, want %q", id, normalized.Session.ID)
	}
}
