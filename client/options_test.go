package client

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
)

func TestOptionsTransportConfiguration(t *testing.T) {
	custom := fakeTransport{}
	options := NewOptions().
		WithCLIPath("nxs").
		WithDirectConnect(DirectConnectOptions{URL: "cc://127.0.0.1:1234/token"}).
		WithTransport(custom)

	if options.CLIPath != "nxs" {
		t.Fatalf("CLIPath = %q", options.CLIPath)
	}
	if options.Transport == nil {
		t.Fatal("Transport is nil")
	}
	if options.DirectConnect != nil {
		t.Fatalf("DirectConnect = %#v, want cleared by WithTransport", options.DirectConnect)
	}
	if got := options.processConfig().CommandPath; got != "nxs" {
		t.Fatalf("process command path = %q", got)
	}
}

func TestOptionsDefaultRuntimeUsesNXSControlWire(t *testing.T) {
	config := NewOptions().WithCLIPath("nxs").processConfig()
	if config.ControlWireDialect != transport.ControlWireDialectNXS {
		t.Fatalf("control wire dialect = %q, want nxs", config.ControlWireDialect)
	}
}

func TestOptionsWithRuntimeClaudeUsesClaudeControlWire(t *testing.T) {
	config := NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude").processConfig()
	if config.ControlWireDialect != transport.ControlWireDialectClaude {
		t.Fatalf("control wire dialect = %q, want claude", config.ControlWireDialect)
	}
}

func TestOptionsWithRuntimeNXSRequiresExplicitCommandPath(t *testing.T) {
	t.Setenv("NEXUS_NXS_COMMAND_PATH", "")
	_, err := NewOptions().WithRuntime(RuntimeNXS).normalized()
	if err == nil || !strings.Contains(err.Error(), "NEXUS_NXS_COMMAND_PATH") {
		t.Fatalf("normalized() error = %v, want explicit nxs command path error", err)
	}
}

func TestOptionsWithRuntimeNXSUsesEnvOverride(t *testing.T) {
	t.Setenv("NEXUS_NXS_COMMAND_PATH", "/tmp/custom-nxs")
	config := NewOptions().WithRuntime(RuntimeNXS).processConfig()
	if config.CommandPath != "/tmp/custom-nxs" {
		t.Fatalf("nxs command path = %q, want env override", config.CommandPath)
	}
}

func TestOptionsWithRuntimeNXSInjectsDefaultEnv(t *testing.T) {
	config := NewOptions().WithRuntime(RuntimeNXS).WithCLIPath("nxs").processConfig()
	want := map[string]string{
		nxsAPIClearToolResultsEnvName:               "1",
		nxsAPIClearToolUsesEnvName:                  "1",
		nxsAPILocalClearToolHistoryEnvName:          "1",
		nxsPromptCache1hAllowlistEnvName:            "repl_main_thread*,agent:*,sdk",
		nxsAgentSDKDiagnosticsEnvName:               "",
		nxsAgentSDKDiagnosticsStreamProgressEnvName: "0",
		nxsAgentSDKDebugEnvName:                     "",
		nxsAgentSDKProviderDebugBodyEnvName:         "",
	}
	for key, value := range want {
		if config.Env[key] != value {
			t.Fatalf("%s = %q, want %q; env=%+v", key, config.Env[key], value, config.Env)
		}
	}
	if _, ok := config.Env[nxsCachedMicrocompactEnvName]; ok {
		t.Fatalf("%s should require explicit cache-editing opt-in: env=%+v", nxsCachedMicrocompactEnvName, config.Env)
	}
	if _, ok := config.Env[nxsPromptCache1hEligibleEnvName]; ok {
		t.Fatalf("%s should require explicit host/user eligibility: env=%+v", nxsPromptCache1hEligibleEnvName, config.Env)
	}
}

// TestOptionsWithRuntimeNXSPreservesResponsesEnv 验证 bridge 不解释但完整透传 Responses provider 配置。
func TestOptionsWithRuntimeNXSPreservesResponsesEnv(t *testing.T) {
	want := map[string]string{
		"NEXUS_API_PROVIDER":             "openai",
		"NEXUS_OPENAI_PROTOCOL":          "responses",
		"NEXUS_OPENAI_PROMPT_CACHE":      "1",
		"NEXUS_OPENAI_PROMPT_CACHE_MODE": "explicit",
		"NEXUS_OPENAI_PROMPT_CACHE_TTL":  "30m",
		"OPENAI_BASE_URL":                "https://sample.openai.azure.com/openai/",
		"OPENAI_API_KEY":                 "test-key",
		"OPENAI_MODEL":                   "gpt-test",
	}
	config := NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs").
		WithEnv(want).
		processConfig()
	for key, value := range want {
		if config.Env[key] != value {
			t.Fatalf("%s = %q, want %q", key, config.Env[key], value)
		}
	}
}

func TestRuntimeNXSCacheOptInDefaultsMatchClaude(t *testing.T) {
	claudeConfig := NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude").processConfig()
	nxsConfig := NewOptions().WithRuntime(RuntimeNXS).WithCLIPath("nxs").processConfig()
	for _, key := range []string{
		nxsCachedMicrocompactEnvName,
		nxsPromptCache1hEligibleEnvName,
	} {
		if _, ok := claudeConfig.Env[key]; ok {
			t.Fatalf("RuntimeClaude %s should be unset by default: env=%+v", key, claudeConfig.Env)
		}
		if _, ok := nxsConfig.Env[key]; ok {
			t.Fatalf("RuntimeNXS %s should match RuntimeClaude opt-in default: env=%+v", key, nxsConfig.Env)
		}
	}
}

func TestOptionsWithRuntimeNXSDoesNotEnableCachedMicrocompactForCustomAnthropicBaseURL(t *testing.T) {
	config := NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs").
		WithEnv(map[string]string{
			anthropicBaseURLEnvName: "https://open.bigmodel.cn/api/anthropic/v1/messages",
		}).
		processConfig()
	if _, ok := config.Env[nxsCachedMicrocompactEnvName]; ok {
		t.Fatalf("%s should require explicit cache-editing opt-in for custom Anthropic base URL; env=%+v", nxsCachedMicrocompactEnvName, config.Env)
	}

	config = NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs").
		WithEnv(map[string]string{
			anthropicBaseURLEnvName:      "https://open.bigmodel.cn/api/anthropic/v1/messages",
			nxsCachedMicrocompactEnvName: "0",
		}).
		processConfig()
	if config.Env[nxsCachedMicrocompactEnvName] != "0" {
		t.Fatalf("%s explicit override = %q, want preserved 0; env=%+v",
			nxsCachedMicrocompactEnvName,
			config.Env[nxsCachedMicrocompactEnvName],
			config.Env)
	}

	config = NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs").
		WithEnv(map[string]string{
			anthropicBaseURLEnvName:      "https://api.anthropic.com",
			nxsCachedMicrocompactEnvName: "1",
		}).
		processConfig()
	if config.Env[nxsCachedMicrocompactEnvName] != "1" {
		t.Fatalf("%s explicit override = %q, want preserved 1; env=%+v",
			nxsCachedMicrocompactEnvName,
			config.Env[nxsCachedMicrocompactEnvName],
			config.Env)
	}
}

func TestOptionsWithRuntimeNXSAllowsDefaultEnvOverride(t *testing.T) {
	config := NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath("nxs").
		WithEnv(map[string]string{
			nxsCachedMicrocompactEnvName:                "0",
			nxsAPIClearToolResultsEnvName:               "",
			nxsAPILocalClearToolHistoryEnvName:          "0",
			nxsPromptCache1hEligibleEnvName:             "0",
			nxsPromptCache1hAllowlistEnvName:            "agent:*",
			nxsAgentSDKDiagnosticsEnvName:               "stderr",
			nxsAgentSDKDiagnosticsStreamProgressEnvName: "1",
		}).
		processConfig()
	if config.Env[nxsCachedMicrocompactEnvName] != "0" ||
		config.Env[nxsAPIClearToolResultsEnvName] != "" ||
		config.Env[nxsAPILocalClearToolHistoryEnvName] != "0" ||
		config.Env[nxsPromptCache1hEligibleEnvName] != "0" ||
		config.Env[nxsPromptCache1hAllowlistEnvName] != "agent:*" ||
		config.Env[nxsAgentSDKDiagnosticsEnvName] != "stderr" ||
		config.Env[nxsAgentSDKDiagnosticsStreamProgressEnvName] != "1" {
		t.Fatalf("nxs default env override failed: %+v", config.Env)
	}
	if config.Env[nxsAPIClearToolUsesEnvName] != "1" {
		t.Fatalf("nxs tool use clear default missing: %+v", config.Env)
	}
}

func TestOptionsWithRuntimeNXSKeepsExplicitCLIPath(t *testing.T) {
	t.Setenv("NEXUS_NXS_COMMAND_PATH", "/tmp/custom-nxs")
	config := NewOptions().WithRuntime(RuntimeNXS).WithCLIPath("/tmp/manual-nxs").processConfig()
	if config.CommandPath != "/tmp/manual-nxs" {
		t.Fatalf("nxs command path = %q, want explicit CLIPath", config.CommandPath)
	}
}

func TestOptionsRejectUnsupportedRuntimeKind(t *testing.T) {
	_, err := NewOptions().WithRuntime(RuntimeKind("unknown")).normalized()
	if err == nil || !strings.Contains(err.Error(), "unsupported runtime kind") {
		t.Fatalf("normalized() error = %v, want unsupported runtime kind", err)
	}
}

func TestProcessOptionsExposeOfficialCLIFlags(t *testing.T) {
	options := NewOptions().
		WithPathToClaudeCodeExecutable("/opt/claude-code").
		WithExecutable("node").
		WithExecutableArgs("--loader", "tsx").
		WithResume("parent-session").
		WithSessionID("00000000-0000-0000-0000-000000000001").
		WithResumeSessionAt("11111111-1111-1111-1111-111111111111").
		WithTitle("bridge session").
		WithDebugFile("/tmp/bridge.log")

	config := options.processConfig()
	if config.CommandPath != "node" {
		t.Fatalf("command path = %q, want node", config.CommandPath)
	}
	wantPrefix := []string{"--loader", "tsx", "/opt/claude-code"}
	if len(config.Args) < len(wantPrefix) {
		t.Fatalf("args = %#v, want prefix %#v", config.Args, wantPrefix)
	}
	for i, want := range wantPrefix {
		if config.Args[i] != want {
			t.Fatalf("args[%d] = %q, want %q in %#v", i, config.Args[i], want, config.Args)
		}
	}
	assertArgValue(t, config.Args, "--resume", "parent-session")
	assertArgValue(t, config.Args, "--session-id", "00000000-0000-0000-0000-000000000001")
	assertArgValue(t, config.Args, "--resume-session-at", "11111111-1111-1111-1111-111111111111")
	assertArgValue(t, config.Args, "--name", "bridge session")
	assertArg(t, config.Args, "--debug")
	assertArgValue(t, config.Args, "--debug-file", "/tmp/bridge.log")
}

func TestSettingsObjectAndSandboxBecomeInlineSettings(t *testing.T) {
	enabled := true
	allowLocalBinding := true
	allowAppleEvents := true
	allowGitConfig := true
	options := NewOptions().
		WithCLIPath("nxs").
		WithSettingsObject(map[string]any{
			"model": "sonnet",
		}).
		WithSandbox(SandboxSettings{
			Enabled:          &enabled,
			EnabledPlatforms: []string{"macos"},
			AllowAppleEvents: &allowAppleEvents,
			Filesystem:       &SandboxFilesystemConfig{AllowGitConfig: &allowGitConfig},
			Network: &SandboxNetworkConfig{
				DeniedDomains:     []string{"blocked.example"},
				AllowLocalBinding: &allowLocalBinding,
				AllowMachLookup:   []string{"com.example.service*"},
			},
			Extra: map[string]any{"mode": "strict"},
		})

	raw := argValue(t, options.processConfig().Args, "--settings")
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings JSON decode: %v\nraw=%s", err, raw)
	}
	if settings["model"] != "sonnet" {
		t.Fatalf("settings.model = %#v", settings["model"])
	}
	sandbox, ok := settings["sandbox"].(map[string]any)
	if !ok {
		t.Fatalf("settings.sandbox = %#v", settings["sandbox"])
	}
	if sandbox["enabled"] != true {
		t.Fatalf("sandbox.enabled = %#v", sandbox["enabled"])
	}
	if sandbox["allowAppleEvents"] != true {
		t.Fatalf("sandbox.allowAppleEvents = %#v", sandbox["allowAppleEvents"])
	}
	filesystem, ok := sandbox["filesystem"].(map[string]any)
	if !ok || filesystem["allowGitConfig"] != true {
		t.Fatalf("sandbox.filesystem.allowGitConfig = %#v, want true", sandbox["filesystem"])
	}
	if sandbox["mode"] != "strict" {
		t.Fatalf("sandbox.mode = %#v", sandbox["mode"])
	}
	network, ok := sandbox["network"].(map[string]any)
	if !ok || network["allowLocalBinding"] != true {
		t.Fatalf("sandbox.network = %#v", sandbox["network"])
	}
	if network["deniedDomains"] == nil || network["allowMachLookup"] == nil {
		t.Fatalf("sandbox.network missing complete fields: %#v", network)
	}
}

func TestInlineSettingsMergeSandbox(t *testing.T) {
	enabled := true
	options := NewOptions().
		WithCLIPath("nxs").
		WithSettings(`{"model":"sonnet","permissions":{"allow":["Read(*)"]}}`).
		WithSandbox(SandboxSettings{Enabled: &enabled})

	raw := argValue(t, options.processConfig().Args, "--settings")
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings JSON decode: %v\nraw=%s", err, raw)
	}
	if settings["model"] != "sonnet" {
		t.Fatalf("settings.model = %#v", settings["model"])
	}
	if _, ok := settings["sandbox"].(map[string]any); !ok {
		t.Fatalf("settings.sandbox = %#v", settings["sandbox"])
	}
}

func TestSandboxPreservesExplicitEmptyEnabledPlatforms(t *testing.T) {
	options := NewOptions().
		WithCLIPath("nxs").
		WithSandbox(SandboxSettings{EnabledPlatforms: []string{}})

	raw := argValue(t, options.processConfig().Args, "--settings")
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings JSON decode: %v\nraw=%s", err, raw)
	}
	sandboxSettings, ok := settings["sandbox"].(map[string]any)
	if !ok {
		t.Fatalf("settings.sandbox = %#v", settings["sandbox"])
	}
	platforms, exists := sandboxSettings["enabledPlatforms"]
	if !exists {
		t.Fatal("sandbox.enabledPlatforms explicit empty list was omitted")
	}
	if values, ok := platforms.([]any); !ok || len(values) != 0 {
		t.Fatalf("sandbox.enabledPlatforms = %#v, want []", platforms)
	}
}

func TestSandboxNetworkPreservesExplicitEmptyDomainLists(t *testing.T) {
	data, err := json.Marshal(SandboxNetworkConfig{})
	if err != nil {
		t.Fatalf("marshal network config: %v", err)
	}
	var network map[string]any
	if err := json.Unmarshal(data, &network); err != nil {
		t.Fatalf("unmarshal network config: %v", err)
	}
	for _, key := range []string{"allowedDomains", "deniedDomains"} {
		value, ok := network[key]
		if !ok {
			t.Fatalf("network.%s was omitted: %#v", key, network)
		}
		items, ok := value.([]any)
		if !ok || len(items) != 0 {
			t.Fatalf("network.%s = %#v, want []", key, value)
		}
	}
}

func TestOptionsRejectSettingsPathWithStructuredSettings(t *testing.T) {
	enabled := true
	_, err := NewOptions().
		WithCLIPath("nxs").
		WithSettings("/tmp/settings.json").
		WithSandbox(SandboxSettings{Enabled: &enabled}).
		normalized()
	if err == nil {
		t.Fatal("normalized succeeded, want settings path conflict")
	}
}

func TestOptionsRejectInvalidToolConfigPreview(t *testing.T) {
	_, err := NewOptions().
		WithCLIPath("nxs").
		WithToolConfig(ToolConfig{
			AskUserQuestion: &AskUserQuestionToolConfig{
				PreviewFormat: QuestionPreviewFormat("pdf"),
			},
		}).
		normalized()
	if err == nil {
		t.Fatal("normalized succeeded, want invalid preview format error")
	}
}

func TestOptionsRejectExecutableArgsWithoutExecutable(t *testing.T) {
	_, err := NewOptions().WithCLIPath("nxs").WithExecutableArgs("--loader", "tsx").normalized()
	if err == nil {
		t.Fatal("normalized succeeded, want executable args conflict")
	}
}

func TestOptionsRejectsTransportAndDirectConnectConflict(t *testing.T) {
	options := NewOptions()
	options.Transport = fakeTransport{}
	options.DirectConnect = &DirectConnectOptions{URL: "cc://127.0.0.1:1234/token"}

	_, err := options.normalized()
	if err == nil || !errors.Is(err, errTransportDirectConnectConflict) {
		t.Fatalf("normalized error = %v, want transport/direct-connect conflict", err)
	}
}

type fakeTransport struct{}

func (fakeTransport) Start(context.Context) error { return nil }
func (fakeTransport) ReadJSON() (map[string]any, error) {
	return nil, errors.New("not implemented")
}
func (fakeTransport) WriteJSON(any) error { return nil }
func (fakeTransport) EndInput() error     { return nil }
func (fakeTransport) Interrupt() error    { return nil }
func (fakeTransport) Wait() error         { return nil }
func (fakeTransport) Close() error        { return nil }

func assertArg(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, arg := range args {
		if arg == flag {
			return
		}
	}
	t.Fatalf("missing arg %q in %#v", flag, args)
}

func assertArgValue(t *testing.T, args []string, flag string, want string) {
	t.Helper()
	got := argValue(t, args, flag)
	if got != want {
		t.Fatalf("%s = %q, want %q in %#v", flag, got, want, args)
	}
}

func argValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, arg := range args {
		if arg == flag {
			if i+1 >= len(args) {
				t.Fatalf("%s has no value in %#v", flag, args)
			}
			return args[i+1]
		}
	}
	t.Fatalf("missing arg %q in %#v", flag, args)
	return ""
}
