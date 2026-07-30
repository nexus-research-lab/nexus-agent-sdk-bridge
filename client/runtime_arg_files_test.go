package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestClaudeTransportReceivesSelectedSkillAndAdditionalRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "platform-skills")
	options := NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath("claude").
		WithSkills("ima-skill").
		WithDisabledSkills("workspace-review").
		WithAdditionalDirectories(root)
	resolved, err := options.buildResolvedOptions(false)
	if err != nil {
		t.Fatalf("buildResolvedOptions() error = %v", err)
	}
	args := buildProcessTransportArgs(resolved)
	if !containsArgPair(args, "--allowedTools", "Skill(ima-skill)") {
		t.Fatalf("Claude args = %#v, want selected Skill allow rule", args)
	}
	if !containsArgPair(args, "--disallowedTools", "Skill(workspace-review)") {
		t.Fatalf("Claude args = %#v, want disabled Skill deny rule", args)
	}
	if !containsArgPair(args, "--add-dir", root) {
		t.Fatalf("Claude args = %#v, want platform Skill additional root", args)
	}
}

func TestClaudeTransportKeepsDynamicSkillsAndDeniesUnboundGlobals(t *testing.T) {
	options := NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath("claude").
		WithAllSkills().
		WithDisabledSkills("unused-global", "workspace-off")
	resolved, err := options.buildResolvedOptions(false)
	if err != nil {
		t.Fatalf("buildResolvedOptions() error = %v", err)
	}
	args := buildProcessTransportArgs(resolved)
	if !containsArgPair(args, "--allowedTools", "Skill") {
		t.Fatalf("Claude args = %#v, want dynamic Skill allow rule", args)
	}
	if got := argValue(t, args, "--disallowedTools"); got !=
		"Skill(unused-global),Skill(workspace-off)" {
		t.Fatalf("Claude disabled Skill rules = %q", got)
	}
}

func containsArgPair(args []string, key string, value string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key && args[index+1] == value {
			return true
		}
	}
	return false
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

func TestMaterializeProcessArgFilesForWindowsKeepsNXSSystemPromptParts(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs")
	options.System.AppendStatic = "stable Room rules"
	options.System.AppendDynamic = "dynamic turn context"
	if err := materializeProcessArgFilesForOS("windows", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if options.System.AppendStatic != "stable Room rules" || options.System.AppendDynamic != "dynamic turn context" {
		t.Fatalf("nxs prompt parts = %#v, want parts preserved", options.System)
	}
	resolved, err := options.buildResolvedOptions(false)
	if err != nil {
		t.Fatalf("buildResolvedOptions() error = %v", err)
	}
	if resolved.AppendSystemPrompt != "" {
		t.Fatalf("resolved append prompt = %q, want no inline duplicate", resolved.AppendSystemPrompt)
	}
	args := buildProcessTransportArgs(resolved)
	if got := argValue(t, args, "--append-system-prompt-file"); got == "" {
		t.Fatalf("args = %#v, want append-system-prompt-file", args)
	}
	for _, arg := range args {
		if arg == "--append-system-prompt" {
			t.Fatalf("args = %#v, want no inline append prompt", args)
		}
	}
}

func TestMaterializeProcessArgFilesUsesMCPConfigFileOnEveryOS(t *testing.T) {
	tests := []struct {
		goos     string
		dirMode  os.FileMode
		fileMode os.FileMode
	}{
		{goos: "windows", dirMode: 0o700, fileMode: 0o600},
		{goos: "darwin", dirMode: 0o700, fileMode: 0o600},
		{goos: "linux", dirMode: 0o750 | os.ModeSetgid, fileMode: 0o640},
	}

	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			root := t.TempDir()
			restore := overrideRuntimeArgFilesRoot(root)
			defer restore()

			const secretMarker = "test-secret-marker"
			options := NewOptions().
				WithRuntime(RuntimeNXS).
				WithCLIPath("nxs")
			options.MCP.Servers = map[string]mcp.ServerConfig{
				"nexus_room": mcp.SDKServerConfig{
					Name:     "nexus_room",
					Instance: fakeRuntimeMCPServer{},
				},
				"amap_maps": mcp.HTTPServerConfig{
					URL: "https://mcp.amap.com/mcp?key=" + secretMarker,
					Headers: map[string]string{
						"X-Test": secretMarker,
					},
				},
			}

			if err := materializeProcessArgFilesForOS(test.goos, &options); err != nil {
				t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
			}
			if options.MCP.Config == "" {
				t.Fatalf("MCP config should be written to a file on %s", test.goos)
			}
			if len(options.MCP.Servers) != 0 {
				t.Fatalf("MCP.Servers should be carried by config file: %+v", options.MCP.Servers)
			}
			if len(options.MCP.SDKServers) != 1 || options.MCP.SDKServers["nexus_room"] == nil {
				t.Fatalf("SDK MCP server registry should be preserved: %+v", options.MCP.SDKServers)
			}

			resolved, err := options.buildResolvedOptions(false)
			if err != nil {
				t.Fatalf("buildResolvedOptions() error = %v", err)
			}
			args := buildProcessTransportArgs(resolved)
			if got := argValue(t, args, "--mcp-config"); got != options.MCP.Config {
				t.Fatalf("--mcp-config = %q, want file path %q", got, options.MCP.Config)
			}
			for _, arg := range args {
				if strings.Contains(arg, secretMarker) {
					t.Fatalf("secret leaked into process argv: %#v", args)
				}
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
			assertFileMode(t, root, test.dirMode)
			assertFileMode(t, options.MCP.Config, test.fileMode)
		})
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

func TestMaterializeProcessArgFilesOutsideWindowsKeepsPromptInline(t *testing.T) {
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
	if options.MCP.Config == "" {
		t.Fatal("non-Windows MCP config should be moved out of argv")
	}
}

func TestMaterializeProcessArgFilesMovesInlineMCPConfigOutOfArgv(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	const secretMarker = "inline-test-secret"
	options := Options{}
	options.MCP.Config = `{"mcpServers":{"example":{"type":"http","url":"https://example.com/mcp?key=` +
		secretMarker + `"}}}`

	if err := materializeProcessArgFilesForOS("darwin", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if strings.Contains(options.MCP.Config, secretMarker) {
		t.Fatalf("inline MCP config remains in argv value: %q", options.MCP.Config)
	}
	data, err := os.ReadFile(options.MCP.Config)
	if err != nil {
		t.Fatalf("read MCP arg file: %v", err)
	}
	if !strings.Contains(string(data), secretMarker) {
		t.Fatalf("MCP arg file = %q, want original inline config", string(data))
	}
}

func TestMaterializeProcessArgFilesPreservesExplicitMCPConfigPath(t *testing.T) {
	root := t.TempDir()
	restore := overrideRuntimeArgFilesRoot(filepath.Join(root, "arg-files"))
	defer restore()

	configPath := filepath.Join(root, "mcp.json")
	options := Options{}
	options.MCP.Config = configPath

	if err := materializeProcessArgFilesForOS("darwin", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS() error = %v", err)
	}
	if options.MCP.Config != configPath {
		t.Fatalf("explicit MCP config path = %q, want %q", options.MCP.Config, configPath)
	}
	if _, err := os.Stat(filepath.Join(root, "arg-files")); !os.IsNotExist(err) {
		t.Fatalf("runtime arg file directory should not be created, stat error = %v", err)
	}
}

func TestNormalizedOptionsRejectsInlineMCPConfigCombinedWithServers(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs")
	options.MCP.Config = `{"mcpServers":{}}`
	options.MCP.Servers = map[string]mcp.ServerConfig{
		"example": mcp.HTTPServerConfig{URL: "https://example.com/mcp"},
	}

	if _, err := options.normalized(); err == nil ||
		!strings.Contains(err.Error(), "mcp config path and MCP.Servers cannot be used together") {
		t.Fatalf("normalized() error = %v, want MCP config/server conflict", err)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	got := info.Mode() & (os.ModePerm | os.ModeSetgid)
	if got != want {
		t.Fatalf("%q mode = %v, want %v", path, got, want)
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
