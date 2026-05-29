package client

import (
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

func TestDecodeContextUsageResponse(t *testing.T) {
	payload := map[string]any{
		"categories": []any{
			map[string]any{"name": "system", "tokens": 120, "color": "blue", "is_deferred": true},
		},
		"total_tokens":            500,
		"max_tokens":              1000,
		"raw_max_tokens":          1200,
		"percentage":              50.5,
		"model":                   "model-a",
		"is_auto_compact_enabled": true,
		"memory_files": []any{
			map[string]any{"label": "memory", "file": "summary.md", "tokens": 42},
		},
		"grid_rows": []any{
			[]any{map[string]any{"label": "row", "value": "value", "tokens": 7}},
		},
		"slash_commands": map[string]any{
			"plan": map[string]any{"description": "plan work", "tokens": 8},
		},
		"api_usage": map[string]any{
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
	if len(got.MemoryFiles) != 1 || got.MemoryFiles[0].Name != "memory" || got.MemoryFiles[0].Path != "summary.md" {
		t.Fatalf("memory files = %#v", got.MemoryFiles)
	}
	if len(got.GridRows) != 1 || len(got.GridRows[0]) != 1 || got.GridRows[0][0].Name != "row" {
		t.Fatalf("grid rows = %#v", got.GridRows)
	}
	if len(got.SlashCommands) != 1 || got.SlashCommands[0].Name != "plan" {
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
		"can_rewind":    true,
		"files_changed": []any{"a.go", "b.go"},
		"insertions":    3,
		"deletions":     4,
	})

	if !got.CanRewind || got.Insertions != 3 || got.Deletions != 4 {
		t.Fatalf("rewind result = %#v", got)
	}
	if len(got.FilesChanged) != 2 || got.FilesChanged[1] != "b.go" {
		t.Fatalf("files changed = %#v", got.FilesChanged)
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
			EmailAddress:     "dev@example.com",
			OrganizationName: "Nexus",
			Plan:             "pro",
			Raw: map[string]any{
				"api_provider": "anthropic",
				"token_source": "oauth",
			},
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
