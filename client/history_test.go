package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionCatalogLifecycle(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(nexusConfigDirEnv, configDir)

	projectDir := filepath.Join(configDir, "projects", encodeProjectDirectory(t.TempDir()))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(projectDir, "session-1.jsonl")
	if err := os.WriteFile(sessionPath, []byte(
		`{"type":"system","subtype":"init","session_id":"session-1","timestamp":"2026-05-01T00:00:00Z","cwd":"/tmp/project","gitBranch":"main"}`+"\n"+
			`{"type":"user","uuid":"u1","sessionId":"session-1","timestamp":"2026-05-01T00:00:01Z","message":{"role":"user","content":"hello"}}`+"\n"+
			`{"type":"assistant","uuid":"a1","sessionId":"session-1","timestamp":"2026-05-01T00:00:02Z","message":{"role":"assistant","content":[{"type":"text","text":"world"}]}}`+"\n"+
			`{"type":"ai-title","aiTitle":"Greeting","sessionId":"session-1"}`+"\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := ListSessions(ListSessionsOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}
	if sessions[0].Summary != "Greeting" {
		t.Fatalf("summary = %q, want Greeting", sessions[0].Summary)
	}
	if sessions[0].FirstPrompt == nil || *sessions[0].FirstPrompt != "hello" {
		t.Fatalf("first prompt = %#v, want hello", sessions[0].FirstPrompt)
	}

	messages, err := GetSessionMessages("session-1", GetSessionMessagesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if messages[0].Type != "user" || messages[1].UUID != "a1" {
		t.Fatalf("messages = %#v", messages)
	}

	if err := RenameSession("session-1", "Custom title", SessionLookupOptions{}); err != nil {
		t.Fatal(err)
	}
	tag := "review"
	if err := TagSession("session-1", &tag, SessionLookupOptions{}); err != nil {
		t.Fatal(err)
	}

	info, err := GetSessionInfo("session-1", SessionLookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("info = nil")
	}
	if info.CustomTitle == nil || *info.CustomTitle != "Custom title" {
		t.Fatalf("custom title = %#v, want Custom title", info.CustomTitle)
	}
	if info.Tag == nil || *info.Tag != "review" {
		t.Fatalf("tag = %#v, want review", info.Tag)
	}
}

func TestSessionCatalogReturnsNilForMissingInfo(t *testing.T) {
	t.Setenv(nexusConfigDirEnv, t.TempDir())
	info, err := GetSessionInfo("missing", SessionLookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Fatalf("info = %#v, want nil", info)
	}
}
