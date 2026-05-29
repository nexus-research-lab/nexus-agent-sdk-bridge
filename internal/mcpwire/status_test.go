package mcpwire

import "testing"

func TestDecodeStatusResponse(t *testing.T) {
	got := DecodeStatusResponse(map[string]any{
		"mcp_servers": []any{
			map[string]any{
				"name":   "github",
				"status": "connected",
				"server_info": map[string]any{
					"name":    "github-mcp",
					"version": "1.0.0",
				},
				"tools": []any{
					map[string]any{
						"name":         "search",
						"description":  "Search repositories",
						"input_schema": map[string]any{"type": "object"},
						"annotations":  map[string]any{"read_only": true, "open_world_hint": true},
					},
				},
			},
		},
	})

	if len(got.MCPServers) != 1 {
		t.Fatalf("servers = %#v", got.MCPServers)
	}
	server := got.MCPServers[0]
	if server.Name != "github" || server.ServerInfo == nil || server.ServerInfo.Version != "1.0.0" {
		t.Fatalf("server = %#v", server)
	}
	if len(server.Tools) != 1 || server.Tools[0].Name != "search" {
		t.Fatalf("tools = %#v", server.Tools)
	}
	if server.Tools[0].Annotations == nil || !server.Tools[0].Annotations.ReadOnlyHint {
		t.Fatalf("annotations = %#v", server.Tools[0].Annotations)
	}
}

func TestDecodeSetServersResult(t *testing.T) {
	got := DecodeSetServersResult(map[string]any{
		"added":   []any{"a"},
		"removed": []any{"b"},
		"errors":  map[string]any{"c": "failed", "d": ""},
	})

	if len(got.Added) != 1 || got.Added[0] != "a" {
		t.Fatalf("added = %#v", got.Added)
	}
	if len(got.Removed) != 1 || got.Removed[0] != "b" {
		t.Fatalf("removed = %#v", got.Removed)
	}
	if len(got.Errors) != 1 || got.Errors["c"] != "failed" {
		t.Fatalf("errors = %#v", got.Errors)
	}
}
