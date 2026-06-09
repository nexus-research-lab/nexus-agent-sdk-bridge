package client

import (
	"os"
	"path/filepath"
	"strings"
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

	if err := RenameSession("session-1", "Custom title", SessionMutationOptions{}); err != nil {
		t.Fatal(err)
	}
	tag := "review"
	if err := TagSession("session-1", &tag, SessionMutationOptions{}); err != nil {
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

func TestEncodeProjectDirectoryMatchesClaudeCode(t *testing.T) {
	if got := encodeProjectDirectory("/Users/foo/my_project-测试"); got != "-Users-foo-my-project---" {
		t.Fatalf("encodeProjectDirectory() = %q, want Claude Code ASCII replacement", got)
	}

	longPath := strings.Repeat("a", maxProjectDirectoryNameLength+1)
	expected := strings.Repeat("a", maxProjectDirectoryNameLength) + "-2lljc4d1ph1qx"
	if got := encodeProjectDirectory(longPath); got != expected {
		t.Fatalf("encodeProjectDirectory() = %q, want %q", got, expected)
	}
}

func TestProjectPathHashSuffixMatchesBunHashFixtures(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: "27k1wwwhf13t"},
		{name: "ascii", input: "abc", expected: "1g45uqqks6lu"},
		{name: "unicode", input: "/Users/foo/my_project-测试", expected: "2a16ot6asyzsy"},
		{name: "emoji", input: strings.Repeat("😀", 101), expected: "1wlro20j1vo13"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := projectPathHashSuffix(test.input); got != test.expected {
				t.Fatalf("projectPathHashSuffix() = %q, want %q", got, test.expected)
			}
		})
	}
}

func TestBuildProcessTransportEnvUnifiesClaudeAndNexusConfigRoot(t *testing.T) {
	processEnv := buildProcessTransportEnv(resolvedOptions{
		Env: map[string]string{
			processConfigDirEnv: "/tmp/from-claude",
		},
	})

	if processEnv[nexusConfigDirEnv] != "/tmp/from-claude" {
		t.Fatalf("%s = %q, want /tmp/from-claude", nexusConfigDirEnv, processEnv[nexusConfigDirEnv])
	}
	if processEnv[processConfigDirEnv] != "/tmp/from-claude" {
		t.Fatalf("%s = %q, want /tmp/from-claude", processConfigDirEnv, processEnv[processConfigDirEnv])
	}
}
