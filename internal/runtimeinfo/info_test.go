package runtimeinfo

import "testing"

func TestDecodeInitializeResponse(t *testing.T) {
	got := DecodeInitializeResponse(map[string]any{
		"commands": []any{
			map[string]any{"name": "plan", "allowed_tools": "Read, Edit", "user_invocable": "true"},
		},
		"agents": []any{
			map[string]any{"name": "reviewer", "description": "Review code"},
		},
		"models": []any{
			map[string]any{"id": "model-a", "display_name": "Model A"},
		},
		"account":                 map[string]any{"email_address": "dev@example.com"},
		"output_style":            "concise",
		"available_output_styles": []any{"concise", "full"},
		"fast_mode_state":         "enabled",
	})

	if len(got.Commands) != 1 || got.Commands[0].Name != "plan" {
		t.Fatalf("commands = %#v", got.Commands)
	}
	if len(got.Commands[0].AllowedTools) != 2 || got.Commands[0].AllowedTools[1] != "Edit" {
		t.Fatalf("allowed tools = %#v", got.Commands[0].AllowedTools)
	}
	if got.Commands[0].UserInvocable == nil || !*got.Commands[0].UserInvocable {
		t.Fatalf("user invocable = %#v", got.Commands[0].UserInvocable)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "reviewer" {
		t.Fatalf("agents = %#v", got.Agents)
	}
	if len(got.Models) != 1 || got.Models[0].ID != "model-a" {
		t.Fatalf("models = %#v", got.Models)
	}
	if got.Account.EmailAddress != "dev@example.com" || got.OutputStyle != "concise" {
		t.Fatalf("initialize response = %#v", got)
	}
}

func TestDecodeReloadPluginsResponse(t *testing.T) {
	got := DecodeReloadPluginsResponse(map[string]any{
		"plugins": []any{
			map[string]any{"name": "local", "path": "/tmp/plugin", "source": "project"},
		},
		"mcp_servers": []any{
			map[string]any{"name": "github", "status": "connected"},
		},
		"enabled_count": 1,
		"mcp_count":     1,
	})

	if len(got.Plugins) != 1 || got.Plugins[0].Name != "local" {
		t.Fatalf("plugins = %#v", got.Plugins)
	}
	if len(got.MCPServers) != 1 || got.MCPServers[0].Name != "github" {
		t.Fatalf("mcp servers = %#v", got.MCPServers)
	}
	if got.EnabledCount != 1 || got.MCPCount != 1 {
		t.Fatalf("counts = %#v", got)
	}
}
