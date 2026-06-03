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

func TestDecodeReloadPluginsResponseMapsPublicSurface(t *testing.T) {
	got := decodeReloadPluginsResponse(map[string]any{
		"commands": []any{
			map[string]any{"name": "visible", "description": "Run visible command"},
			map[string]any{"name": "hidden", "description": "Hidden", "user_invocable": false},
		},
		"agents": []any{
			map[string]any{"name": "reviewer", "description": "Review code", "model": "glm-5.1"},
		},
		"plugins": []any{
			map[string]any{"name": "team", "path": "/tmp/team", "source": "project"},
		},
		"mcp_servers": []any{
			map[string]any{
				"name":   "fs",
				"status": "connected",
				"tools": []any{
					map[string]any{"name": "read_file", "annotations": map[string]any{"read_only_hint": true}},
				},
			},
		},
		"command_count":  2,
		"agent_count":    1,
		"mcp_count":      1,
		"error_count":    1,
		"disabled_count": 3,
	})

	if len(got.Commands) != 1 || got.Commands[0].Name != "visible" {
		t.Fatalf("commands = %#v, want only public visible command", got.Commands)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "reviewer" {
		t.Fatalf("agents = %#v, want reviewer", got.Agents)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].Source != "project" {
		t.Fatalf("plugins = %#v, want project plugin", got.Plugins)
	}
	if len(got.MCPServers) != 1 || got.MCPServers[0].Name != "fs" {
		t.Fatalf("mcp servers = %#v, want fs", got.MCPServers)
	}
	if len(got.MCPServers[0].Tools) != 1 || !got.MCPServers[0].Tools[0].Annotations.ReadOnlyHint {
		t.Fatalf("mcp tools = %#v, want read-only hint", got.MCPServers[0].Tools)
	}
	if got.CommandCount != 2 || got.AgentCount != 1 || got.MCPCount != 1 || got.ErrorCount != 1 || got.DisabledCount != 3 {
		t.Fatalf("counts = %#v, want decoded counts", got)
	}
	if got.Raw["command_count"] == nil {
		t.Fatalf("raw payload not preserved: %#v", got.Raw)
	}
}

func TestDecodeSettingsResponse(t *testing.T) {
	got := decodeSettingsResponse(map[string]any{
		"effective": map[string]any{
			"model":               "glm-5.1",
			"permission_mode":     "acceptEdits",
			"max_thinking_tokens": 512,
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

	if got.Effective["model"] != "glm-5.1" || got.Effective["permission_mode"] != "acceptEdits" {
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
