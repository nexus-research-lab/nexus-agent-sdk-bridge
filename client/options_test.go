package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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

func TestOptionsWithRuntimeNXSUsesDownloadedRuntime(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("NEXUS_NXS_RUNTIME_CACHE_DIR", cacheDir)
	runtimeBytes := []byte("#!/bin/sh\nexit 0\n")
	digest := sha256.Sum256(runtimeBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest":
			_, _ = w.Write([]byte(`{"schema_version":1,"version":"0.9.0","assets":[{"goos":"` + runtime.GOOS + `","goarch":"` + runtime.GOARCH + `","filename":"nxs","url":"/nxs","sha256":"` + hex.EncodeToString(digest[:]) + `","archive":"raw"}]}`))
		case "/nxs":
			_, _ = w.Write(runtimeBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NEXUS_NXS_RUNTIME_MANIFEST_URL", server.URL+"/manifest")

	config := NewOptions().WithRuntime(RuntimeNXS).processConfig()
	if !strings.HasPrefix(config.CommandPath, cacheDir) {
		t.Fatalf("nxs command path = %q, want under %q", config.CommandPath, cacheDir)
	}
	info, err := os.Stat(config.CommandPath)
	if err != nil || info.IsDir() {
		t.Fatalf("nxs command path is not executable: info=%v err=%v", info, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		t.Fatalf("nxs command path is not executable: info=%v err=%v", info, err)
	}
	if config.ControlWireDialect != transport.ControlWireDialectSnake {
		t.Fatalf("control wire dialect = %q, want snake", config.ControlWireDialect)
	}
}

func TestOptionsWithRuntimeNXSReportsRuntimeResolverError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	t.Setenv("NEXUS_NXS_RUNTIME_MANIFEST_URL", server.URL+"/missing")

	_, err := NewOptions().WithRuntime(RuntimeNXS).normalized()
	if err == nil ||
		!strings.Contains(err.Error(), "resolve nxs runtime failed") ||
		!strings.Contains(err.Error(), "download nxs runtime manifest") {
		t.Fatalf("normalized() error = %v, want nxs runtime resolver error", err)
	}
}

func TestOptionsDefaultRuntimeUsesNXSControlWire(t *testing.T) {
	config := NewOptions().WithCLIPath("nxs").processConfig()
	if config.ControlWireDialect != transport.ControlWireDialectSnake {
		t.Fatalf("control wire dialect = %q, want snake", config.ControlWireDialect)
	}
}

func TestOptionsWithRuntimeClaudeUsesClaudeControlWire(t *testing.T) {
	config := NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude").processConfig()
	if config.ControlWireDialect != transport.ControlWireDialectClaude {
		t.Fatalf("control wire dialect = %q, want claude", config.ControlWireDialect)
	}
}

func TestOptionsWithRuntimeNXSUsesEnvOverride(t *testing.T) {
	t.Setenv("NEXUS_NXS_COMMAND_PATH", "/tmp/custom-nxs")
	config := NewOptions().WithRuntime(RuntimeNXS).processConfig()
	if config.CommandPath != "/tmp/custom-nxs" {
		t.Fatalf("nxs command path = %q, want env override", config.CommandPath)
	}
}

func TestOptionsWithRuntimeNXSCanDisableRuntimeResolver(t *testing.T) {
	t.Setenv("NEXUS_NXS_RUNTIME_RESOLVER_DISABLED", "1")
	config := NewOptions().WithRuntime(RuntimeNXS).processConfig()
	if config.CommandPath != nxsExecutableName(runtime.GOOS) {
		t.Fatalf("nxs command path = %q, want PATH fallback", config.CommandPath)
	}
}

func TestOptionsWithRuntimeNXSUsesPackagedAppRootRuntime(t *testing.T) {
	root := t.TempDir()
	commandPath := filepath.Join(root, "bin", nxsExecutableName(runtime.GOOS))
	if err := os.MkdirAll(filepath.Dir(commandPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write nxs: %v", err)
	}
	t.Setenv(nexusAppRootEnvName, root)

	config := NewOptions().WithRuntime(RuntimeNXS).processConfig()
	if config.CommandPath != commandPath {
		t.Fatalf("nxs command path = %q, want packaged %q", config.CommandPath, commandPath)
	}
}

func TestOptionsWithRuntimeNXSInjectsDefaultEnv(t *testing.T) {
	t.Setenv("NEXUS_NXS_RUNTIME_RESOLVER_DISABLED", "1")
	config := NewOptions().WithRuntime(RuntimeNXS).processConfig()
	want := map[string]string{
		nxsCachedMicrocompactEnvName:        "1",
		nxsAPIClearToolResultsEnvName:       "1",
		nxsAPIClearToolUsesEnvName:          "1",
		nxsAPILocalClearToolHistoryEnvName:  "1",
		nxsPromptCache1hEligibleEnvName:     "1",
		nxsPromptCache1hAllowlistEnvName:    "sdk",
		nxsAgentSDKDiagnosticsEnvName:       "",
		nxsAgentSDKDebugEnvName:             "",
		nxsAgentSDKProviderDebugBodyEnvName: "",
	}
	for key, value := range want {
		if config.Env[key] != value {
			t.Fatalf("%s = %q, want %q; env=%+v", key, config.Env[key], value, config.Env)
		}
	}
}

func TestOptionsWithRuntimeNXSAllowsDefaultEnvOverride(t *testing.T) {
	t.Setenv("NEXUS_NXS_RUNTIME_RESOLVER_DISABLED", "1")
	config := NewOptions().
		WithRuntime(RuntimeNXS).
		WithEnv(map[string]string{
			nxsCachedMicrocompactEnvName:       "0",
			nxsAPIClearToolResultsEnvName:      "",
			nxsAPILocalClearToolHistoryEnvName: "0",
			nxsPromptCache1hEligibleEnvName:    "0",
			nxsPromptCache1hAllowlistEnvName:   "agent:*",
			nxsAgentSDKDiagnosticsEnvName:      "stderr",
		}).
		processConfig()
	if config.Env[nxsCachedMicrocompactEnvName] != "0" ||
		config.Env[nxsAPIClearToolResultsEnvName] != "" ||
		config.Env[nxsAPILocalClearToolHistoryEnvName] != "0" ||
		config.Env[nxsPromptCache1hEligibleEnvName] != "0" ||
		config.Env[nxsPromptCache1hAllowlistEnvName] != "agent:*" ||
		config.Env[nxsAgentSDKDiagnosticsEnvName] != "stderr" {
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
	options := NewOptions().
		WithCLIPath("nxs").
		WithSettingsObject(map[string]any{
			"model": "sonnet",
		}).
		WithSandbox(SandboxSettings{
			Enabled: &enabled,
			Network: &SandboxNetworkConfig{
				AllowLocalBinding: &allowLocalBinding,
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
	if sandbox["mode"] != "strict" {
		t.Fatalf("sandbox.mode = %#v", sandbox["mode"])
	}
	network, ok := sandbox["network"].(map[string]any)
	if !ok || network["allowLocalBinding"] != true {
		t.Fatalf("sandbox.network = %#v", sandbox["network"])
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
