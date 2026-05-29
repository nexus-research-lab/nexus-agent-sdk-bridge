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

	payload := definition.toMap()
	if got := payload["mcp_servers"]; len(got.([]any)) != 1 {
		t.Fatalf("mcp_servers = %#v, want one server", got)
	}
	if payload["background"] != true {
		t.Fatalf("background = %#v, want true", payload["background"])
	}
	if payload["permission_mode"] != permission.ModePlan {
		t.Fatalf("permission_mode = %#v, want plan", payload["permission_mode"])
	}

	definition.MCPServers[0] = "changed"
	if got := payload["mcp_servers"].([]any)[0].(map[string]any)["name"]; got != "slack" {
		t.Fatalf("payload mcp_servers mutated to %q, want clone", got)
	}
	if _, ok := payload["requiredMcpServers"]; ok {
		t.Fatal("payload contains legacy requiredMcpServers alias")
	}
	if _, ok := payload["required_mcp_servers"]; ok {
		t.Fatal("payload contains removed required_mcp_servers field")
	}
	if _, ok := payload["critical_system_reminder_experimental"]; ok {
		t.Fatal("payload contains removed critical_system_reminder_experimental field")
	}
	if _, ok := payload["isolation"]; ok {
		t.Fatal("payload contains removed isolation field")
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
