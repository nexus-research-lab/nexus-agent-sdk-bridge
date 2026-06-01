package client

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestOptionsTransportConfiguration(t *testing.T) {
	custom := fakeTransport{}
	options := NewOptions().
		WithCLIPath("/tmp/nexus-cli").
		WithDirectConnect(DirectConnectOptions{URL: "cc://127.0.0.1:1234/token"}).
		WithTransport(custom)

	if options.CLIPath != "/tmp/nexus-cli" {
		t.Fatalf("CLIPath = %q", options.CLIPath)
	}
	if options.Transport == nil {
		t.Fatal("Transport is nil")
	}
	if options.DirectConnect != nil {
		t.Fatalf("DirectConnect = %#v, want cleared by WithTransport", options.DirectConnect)
	}
	if got := options.processConfig().CommandPath; got != "/tmp/nexus-cli" {
		t.Fatalf("process command path = %q", got)
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
		WithSettings("/tmp/settings.json").
		WithSandbox(SandboxSettings{Enabled: &enabled}).
		normalized()
	if err == nil {
		t.Fatal("normalized succeeded, want settings path conflict")
	}
}

func TestOptionsRejectInvalidToolConfigPreview(t *testing.T) {
	_, err := NewOptions().
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
	_, err := NewOptions().WithExecutableArgs("--loader", "tsx").normalized()
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
