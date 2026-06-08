package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

type fakeRuntimeMCPServer struct{}

func (fakeRuntimeMCPServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

func TestMaterializeProcessArgFilesForWindowsMovesAppendPromptToFile(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := Options{}
	options.System.Append = "第一行\n第二行"

	if err := materializeProcessArgFilesForOS("windows", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if options.System.Append != "" {
		t.Fatalf("append system prompt should be moved out of argv: %q", options.System.Append)
	}
	path := options.ExtraArgs["append-system-prompt-file"]
	if path == "" {
		t.Fatalf("append-system-prompt-file arg missing: %+v", options.ExtraArgs)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prompt arg file: %v", err)
	}
	if string(content) != "第一行\n第二行" {
		t.Fatalf("prompt arg file = %q", string(content))
	}
}

func TestMaterializeProcessArgFilesForWindowsUsesStableFileName(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	first := Options{}
	first.System.Append = "same prompt"
	if err := materializeProcessArgFilesForOS("windows", &first); err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	second := Options{}
	second.System.Append = "same prompt"
	if err := materializeProcessArgFilesForOS("windows", &second); err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	if first.ExtraArgs["append-system-prompt-file"] != second.ExtraArgs["append-system-prompt-file"] {
		t.Fatalf("arg file path should be stable: first=%q second=%q",
			first.ExtraArgs["append-system-prompt-file"],
			second.ExtraArgs["append-system-prompt-file"])
	}
}

func TestMaterializeProcessArgFilesForWindowsUsesMCPConfigFile(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := Options{}
	options.MCP.Servers = map[string]mcp.ServerConfig{
		"nexus_room": mcp.SDKServerConfig{
			Name:     "nexus_room",
			Instance: fakeRuntimeMCPServer{},
		},
		"amap_maps": mcp.HTTPServerConfig{
			URL: "https://mcp.amap.com/mcp?key=test-key",
			Headers: map[string]string{
				"X-Test": "1",
			},
		},
	}

	if err := materializeProcessArgFilesForOS("windows", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if options.MCP.Config == "" {
		t.Fatal("MCP config should be written to a file on Windows")
	}
	if len(options.MCP.Servers) != 0 {
		t.Fatalf("MCP.Servers should be carried by config file: %+v", options.MCP.Servers)
	}
	if len(options.MCP.SDKServers) != 1 || options.MCP.SDKServers["nexus_room"] == nil {
		t.Fatalf("SDK MCP server registry should be preserved: %+v", options.MCP.SDKServers)
	}
	data, err := os.ReadFile(options.MCP.Config)
	if err != nil {
		t.Fatalf("read MCP arg file: %v", err)
	}
	var payload map[string]map[string]map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("MCP arg file is not JSON: %v", err)
	}
	servers := payload["mcpServers"]
	if servers["nexus_room"]["type"] != "sdk" || servers["nexus_room"]["scope"] != "dynamic" {
		t.Fatalf("SDK MCP server serialized incorrectly: %+v", servers["nexus_room"])
	}
	if servers["amap_maps"]["type"] != "http" || servers["amap_maps"]["url"] == "" {
		t.Fatalf("HTTP MCP server serialized incorrectly: %+v", servers["amap_maps"])
	}
}

func TestNormalizedOptionsPreservesSDKMCPRegistryWithMaterializedMCPConfig(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs").
		WithCWD("C:\\work").
		WithModel("test-model").
		WithSDKMCPServer("nexus_room", fakeRuntimeMCPServer{})
	if err := materializeProcessArgFilesForOS("windows", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if options.MCP.Config == "" {
		t.Fatal("MCP config should be materialized before re-normalizing")
	}

	normalized, err := options.normalized()
	if err != nil {
		t.Fatalf("normalized() error = %v", err)
	}
	if registry := normalized.sdkMCPServerRegistry(); len(registry) != 1 || registry["nexus_room"] == nil {
		t.Fatalf("normalized SDK MCP registry = %+v, want nexus_room", registry)
	}
	config := normalized.processConfig()
	if config.CWD != "C:\\work" {
		t.Fatalf("process config CWD = %q, want C:\\work", config.CWD)
	}
	if got := argValue(t, config.Args, "--model"); got != "test-model" {
		t.Fatalf("--model = %q, want test-model", got)
	}
	if got := argValue(t, config.Args, "--mcp-config"); got != normalized.MCP.Config {
		t.Fatalf("--mcp-config = %q, want %q", got, normalized.MCP.Config)
	}
}

func TestMaterializeProcessArgFilesSkippedOutsideWindows(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := Options{}
	options.System.Append = "保持原样"
	options.MCP.Servers = map[string]mcp.ServerConfig{
		"amap_maps": mcp.HTTPServerConfig{URL: "https://mcp.amap.com/mcp?key=test-key"},
	}

	if err := materializeProcessArgFilesForOS("darwin", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if options.System.Append != "保持原样" {
		t.Fatalf("non-Windows prompt should stay inline: %q", options.System.Append)
	}
	if options.MCP.Config != "" {
		t.Fatalf("non-Windows should not generate MCP config file: %q", options.MCP.Config)
	}
}

func overrideRuntimeArgFilesRoot(root string) func() {
	previous := runtimeArgFilesRoot
	runtimeArgFilesRoot = func(map[string]string) string {
		return filepath.Clean(root)
	}
	return func() {
		runtimeArgFilesRoot = previous
	}
}
