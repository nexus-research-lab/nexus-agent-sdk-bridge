package transport

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveCommandPathUsesExplicitPath(t *testing.T) {
	got, err := resolveCommandPathWith(` C:\tools\claude.cmd `, processCommandResolver{goos: "windows"})
	if err != nil {
		t.Fatalf("resolve explicit command path: %v", err)
	}
	if got != `C:\tools\claude.cmd` {
		t.Fatalf("command path = %q, want explicit path", got)
	}
}

func TestResolveCommandPathUsesClaudeOverride(t *testing.T) {
	expected := `D:\tools\claude.cmd`
	got, err := resolveCommandPathWith("", processCommandResolver{
		goos:       "windows",
		getenv:     fakeProcessCommandEnv(map[string]string{claudeCommandPathEnvName: expected}),
		lookPath:   func(string) (string, error) { return "", exec.ErrNotFound },
		fileExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("resolve override command path: %v", err)
	}
	if got != expected {
		t.Fatalf("command path = %q, want override %q", got, expected)
	}
}

func TestResolveCommandPathPrefersWindowsNPMShimOnPath(t *testing.T) {
	expected := `C:\Users\lee\AppData\Roaming\npm\claude.cmd`
	got, err := resolveCommandPathWith("", processCommandResolver{
		goos:   "windows",
		getenv: fakeProcessCommandEnv(nil),
		lookPath: func(name string) (string, error) {
			if name == "claude.cmd" {
				return expected, nil
			}
			return "", exec.ErrNotFound
		},
		fileExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("resolve PATH shim command path: %v", err)
	}
	if got != expected {
		t.Fatalf("command path = %q, want PATH shim %q", got, expected)
	}
}

func TestResolveCommandPathUsesWindowsNPMShim(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	expected := filepath.Join(appData, "npm", "claude.cmd")
	got, err := resolveCommandPathWith("", processCommandResolver{
		goos:       "windows",
		getenv:     fakeProcessCommandEnv(map[string]string{"APPDATA": appData}),
		lookPath:   func(string) (string, error) { return "", exec.ErrNotFound },
		fileExists: func(path string) bool { return path == expected },
	})
	if err != nil {
		t.Fatalf("resolve Windows npm shim command path: %v", err)
	}
	if got != expected {
		t.Fatalf("command path = %q, want npm shim %q", got, expected)
	}
}

func TestProcessCloseUnblocksWhenDescendantKeepsStderrOpen(t *testing.T) {
	manager, cleanup := newExitedProcessManagerWithOpenStderr(t)
	defer cleanup()

	done := make(chan error, 1)
	go func() {
		done <- manager.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Close() blocked while stderr pipe was still inherited")
	}
}

func TestProcessWaitUnblocksWhenDescendantKeepsStderrOpen(t *testing.T) {
	manager, cleanup := newExitedProcessManagerWithOpenStderr(t)
	defer cleanup()

	done := make(chan error, 1)
	go func() {
		done <- manager.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait() blocked while stderr pipe was still inherited")
	}
}

func TestProcessWaitIncludesStderrTailOnExit(t *testing.T) {
	if os.Getenv("NEXUS_BRIDGE_TEST_EXIT_STDERR") == "1" {
		_, _ = os.Stderr.WriteString("panic: task output failed\nstack line\n")
		os.Exit(2)
	}

	var diagnostics []ProcessDiagnosticEvent
	manager := NewProcessManager(ProcessConfig{
		CommandPath:        os.Args[0],
		CWD:                t.TempDir(),
		Args:               []string{"-test.run=TestProcessWaitIncludesStderrTailOnExit"},
		Env:                map[string]string{"NEXUS_BRIDGE_TEST_EXIT_STDERR": "1"},
		ControlWireDialect: ControlWireDialectNXS,
		Diagnostics: func(event ProcessDiagnosticEvent) {
			diagnostics = append(diagnostics, event)
		},
	})
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	err := manager.Wait()
	if err == nil {
		t.Fatal("Wait() error = nil, want process exit error")
	}
	if !strings.Contains(err.Error(), "exit status 2") ||
		!strings.Contains(err.Error(), "panic: task output failed") {
		t.Fatalf("Wait() error = %v, want exit status and stderr tail", err)
	}

	for _, event := range diagnostics {
		if event.Event != "process_exit" {
			continue
		}
		if !strings.Contains(fmt.Sprint(event.Attributes["stderr_tail"]), "panic: task output failed") {
			t.Fatalf("process_exit diagnostics = %#v, want stderr_tail", event.Attributes)
		}
		return
	}
	t.Fatalf("missing process_exit diagnostics: %#v", diagnostics)
}

func TestProcessCommandVersionCheckSkipsNXSRuntime(t *testing.T) {
	manager := NewProcessManager(ProcessConfig{ControlWireDialect: ControlWireDialectNXS})
	if manager.shouldCheckCommandVersion() {
		t.Fatal("shouldCheckCommandVersion() = true, want false for nxs runtime")
	}
}

func TestBuildEnvironmentUsesRuntimeEntrypointEnv(t *testing.T) {
	claudeEnv := buildEnvironment(nil, "", ControlWireDialectClaude)
	if envValue(claudeEnv, "CLAUDE_CODE_ENTRYPOINT") != "sdk-go" {
		t.Fatalf("CLAUDE_CODE_ENTRYPOINT missing in claude env: %#v", claudeEnv)
	}
	if got := envValue(claudeEnv, "NEXUS_ENTRYPOINT"); got != "" {
		t.Fatalf("NEXUS_ENTRYPOINT = %q, want empty for claude env", got)
	}

	nxsEnv := buildEnvironment(nil, "", ControlWireDialectNXS)
	if envValue(nxsEnv, "NEXUS_ENTRYPOINT") != "sdk-go" {
		t.Fatalf("NEXUS_ENTRYPOINT missing in nxs env: %#v", nxsEnv)
	}
	if got := envValue(nxsEnv, "CLAUDE_CODE_ENTRYPOINT"); got != "" {
		t.Fatalf("CLAUDE_CODE_ENTRYPOINT = %q, want empty for nxs env", got)
	}
}

// TestBuildEnvironmentPreservesResponsesOverrides 验证进程边界不会丢失 Responses 与 Azure 配置。
func TestBuildEnvironmentPreservesResponsesOverrides(t *testing.T) {
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
	environment := buildEnvironment(want, "", ControlWireDialectNXS)
	for key, value := range want {
		if got := envValue(environment, key); got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
}

func envValue(environment []string, key string) string {
	prefix := key + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func newExitedProcessManagerWithOpenStderr(t *testing.T) (*ProcessManager, func()) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("create stderr pipe: %v", err)
	}
	done := make(chan struct{})
	close(done)

	manager := &ProcessManager{
		cmd:    &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}},
		stdout: stdoutReader,
		stderr: stderrReader,
		done:   done,
	}
	manager.stderrWG.Add(1)
	go manager.readStderr(stderrReader)

	cleanup := func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}
	return manager, cleanup
}

func fakeProcessCommandEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
