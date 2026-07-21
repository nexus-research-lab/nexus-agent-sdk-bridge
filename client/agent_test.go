package client

import (
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestAgentDefinitionToMapMatchesAgentShape(t *testing.T) {
	background := true
	definition := AgentDefinition{
		Description:     "review code",
		Prompt:          "Review the change.",
		Tools:           []string{"Read"},
		DisallowedTools: []string{"Write"},
		Model:           "sonnet",
		MCPServers:      []any{map[string]any{"name": "slack"}},
		Skills:          []string{"review"},
		InitialPrompt:   "Start here.",
		MaxTurns:        3,
		Background:      &background,
		Memory:          "project",
		Effort:          "high",
		PermissionMode:  permission.ModePlan,
	}

	payload := definition.ToMap()
	if got := payload["mcpServers"]; len(got.([]any)) != 1 {
		t.Fatalf("mcpServers = %#v, want one server", got)
	}
	if payload["background"] != true {
		t.Fatalf("background = %#v, want true", payload["background"])
	}
	if payload["permissionMode"] != permission.ModePlan {
		t.Fatalf("permissionMode = %#v, want plan", payload["permissionMode"])
	}

	definition.MCPServers[0] = "changed"
	if got := payload["mcpServers"].([]any)[0].(map[string]any)["name"]; got != "slack" {
		t.Fatalf("payload mcpServers mutated to %q, want clone", got)
	}
	for _, key := range []string{"mcp_servers", "permission_mode", "disallowed_tools", "initial_prompt", "max_turns"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("payload contains non-CC key %q", key)
		}
	}
}

func TestAgentDefinitionCloneCopiesMutableSlices(t *testing.T) {
	definition := AgentDefinition{
		Tools:           []string{"Read"},
		DisallowedTools: []string{"Write"},
		MCPServers:      []any{"slack"},
		Skills:          []string{"review"},
	}

	clone := definition.Clone()
	definition.Tools[0] = "Changed"
	definition.DisallowedTools[0] = "Changed"
	definition.MCPServers[0] = "changed"
	definition.Skills[0] = "changed"

	if clone.Tools[0] != "Read" || clone.DisallowedTools[0] != "Write" || clone.MCPServers[0] != "slack" || clone.Skills[0] != "review" {
		t.Fatalf("clone = %#v, want independent mutable slices", clone)
	}
}
