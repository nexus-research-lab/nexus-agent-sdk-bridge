package client

import (
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

func TestDecodeContextUsageResponse(t *testing.T) {
	payload := map[string]any{
		"categories": []any{
			map[string]any{"name": "system", "tokens": 120, "color": "blue", "isDeferred": true},
		},
		"totalTokens":          500,
		"maxTokens":            1000,
		"rawMaxTokens":         1200,
		"percentage":           50.5,
		"model":                "model-a",
		"isAutoCompactEnabled": true,
		"memoryFiles": []any{
			map[string]any{"path": "summary.md", "type": "User", "tokens": 42},
		},
		"gridRows": []any{
			[]any{map[string]any{"categoryName": "system", "isFilled": true, "color": "blue", "tokens": 7}},
		},
		"slashCommands": map[string]any{
			"totalCommands": 4, "includedCommands": 2, "tokens": 8,
		},
		"apiUsage": map[string]any{
			"input_tokens":                10,
			"output_tokens":               11,
			"cache_creation_input_tokens": 12,
			"cache_read_input_tokens":     13,
		},
	}

	got := decodeContextUsageResponse(payload)
	if got.TotalTokens != 500 || got.MaxTokens != 1000 || got.RawMaxTokens != 1200 {
		t.Fatalf("token totals = %#v", got)
	}
	if len(got.Categories) != 1 || got.Categories[0].Name != "system" || !got.Categories[0].IsDeferred {
		t.Fatalf("categories = %#v", got.Categories)
	}
	if len(got.MemoryFiles) != 1 || got.MemoryFiles[0].Path != "summary.md" || got.MemoryFiles[0].Type != "User" {
		t.Fatalf("memory files = %#v", got.MemoryFiles)
	}
	if len(got.GridRows) != 1 || len(got.GridRows[0]) != 1 || got.GridRows[0][0].CategoryName != "system" {
		t.Fatalf("grid rows = %#v", got.GridRows)
	}
	if got.SlashCommands.TotalCommands != 4 || got.SlashCommands.IncludedCommands != 2 {
		t.Fatalf("slash commands = %#v", got.SlashCommands)
	}
	if got.APIUsage.CacheReadInputTokens != 13 {
		t.Fatalf("api usage = %#v", got.APIUsage)
	}
	if got.Raw["model"] != "model-a" {
		t.Fatalf("raw payload not preserved: %#v", got.Raw)
	}
}

func TestDecodeRewindFilesResult(t *testing.T) {
	got := decodeRewindFilesResult(map[string]any{
		"canRewind":    true,
		"filesChanged": []any{"a.go", "b.go"},
		"insertions":   3,
		"deletions":    4,
	})

	if !got.CanRewind || got.Insertions != 3 || got.Deletions != 4 {
		t.Fatalf("rewind result = %#v", got)
	}
	if len(got.FilesChanged) != 2 || got.FilesChanged[1] != "b.go" {
		t.Fatalf("files changed = %#v", got.FilesChanged)
	}
}

func TestDecodeSettingsResponse(t *testing.T) {
	got := decodeSettingsResponse(map[string]any{
		"effective": map[string]any{
			"model":             "glm-5.1",
			"permissionMode":    "acceptEdits",
			"maxThinkingTokens": 512,
		},
		"sources": []any{
			map[string]any{
				"source":   "project",
				"settings": map[string]any{"theme": "compact"},
			},
		},
		"applied": map[string]any{
			"model":  "glm-5.1",
			"effort": "medium",
		},
	})

	if got.Effective["model"] != "glm-5.1" || got.Effective["permissionMode"] != "acceptEdits" {
		t.Fatalf("effective = %#v, want decoded settings", got.Effective)
	}
	if len(got.Sources) != 1 || got.Sources[0].Source != "project" || got.Sources[0].Settings["theme"] != "compact" {
		t.Fatalf("sources = %#v, want project source", got.Sources)
	}
	if got.Applied.Model != "glm-5.1" || got.Applied.Effort == nil || *got.Applied.Effort != "medium" {
		t.Fatalf("applied = %#v, want model and effort", got.Applied)
	}
	if got.Raw["effective"] == nil {
		t.Fatalf("raw payload not preserved: %#v", got.Raw)
	}
}

func TestDecodeAutoDreamResult(t *testing.T) {
	payload := map[string]any{
		"status":            "completed",
		"reason":            "ready",
		"sessions_reviewed": 6,
		"next_check_at_ms":  int64(1_783_651_200_000),
		"summary":           "Consolidated durable memories.",
		"written_paths":     []any{"/memory/user_preferences.md"},
	}

	got := decodeAutoDreamResult(payload)
	if got.Status != AutoDreamStatusCompleted || got.Reason != "ready" {
		t.Fatalf("auto dream result = %#v, want completed/ready", got)
	}
	if got.SessionsReviewed != 6 || got.NextCheckAtMS != 1_783_651_200_000 {
		t.Fatalf("auto dream schedule fields = %#v", got)
	}
	if got.Summary != "Consolidated durable memories." || len(got.WrittenPaths) != 1 || got.WrittenPaths[0] != "/memory/user_preferences.md" || got.Raw["status"] != "completed" {
		t.Fatalf("auto dream summary/raw = %#v", got)
	}
}

func TestInitializationResultFromRuntimeFiltersAndMapsPublicSnapshot(t *testing.T) {
	hidden := false
	got := initializationResultFromRuntime(runtimeinfo.InitializeResponse{
		Commands: []runtimeinfo.SlashCommandInfo{
			{Name: "visible", Description: "Run visible command", ArgumentHint: "<target>"},
			{Name: "hidden", Description: "Hidden", UserInvocable: &hidden},
		},
		Agents: []runtimeinfo.AgentInfo{
			{Name: "reviewer", Description: "Review code", Prompt: "review", Model: "glm-5.1"},
		},
		Models: []runtimeinfo.ModelInfo{
			{ID: "glm-5.1", Name: "glm-5.1", DisplayName: "GLM 5.1", Vendor: "zhipu"},
		},
		Account: runtimeinfo.AccountInfo{
			Email:            "dev@example.com",
			Organization:     "Nexus",
			SubscriptionType: "pro",
			APIProvider:      "anthropic",
			TokenSource:      "oauth",
			Raw:              map[string]any{"apiProvider": "anthropic", "tokenSource": "oauth"},
		},
		OutputStyle:           "default",
		AvailableOutputStyles: []string{"default"},
		Raw:                   map[string]any{"fast_mode_state": "enabled"},
	})

	if len(got.Commands) != 1 || got.Commands[0].Name != "visible" {
		t.Fatalf("commands = %#v, want only user-invocable command", got.Commands)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "reviewer" {
		t.Fatalf("agents = %#v, want reviewer", got.Agents)
	}
	if len(got.Models) != 1 || got.Models[0].ID != "glm-5.1" {
		t.Fatalf("models = %#v, want glm-5.1", got.Models)
	}
	if got.Account.Email != "dev@example.com" || got.Account.Organization != "Nexus" {
		t.Fatalf("account = %#v, want mapped account", got.Account)
	}
	if got.Account.APIProvider != "anthropic" || got.Account.TokenSource != "oauth" {
		t.Fatalf("account sources = %#v, want provider/token source", got.Account)
	}
	if got.Raw["fast_mode_state"] != "enabled" {
		t.Fatalf("raw payload = %#v, want preserved raw", got.Raw)
	}
}
