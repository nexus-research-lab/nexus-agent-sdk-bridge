package transport

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

func TestResolveCommandPathPrefersWindowsPowerShellShimOnPath(t *testing.T) {
	powerShellPath := `C:\Users\lee\AppData\Roaming\npm\claude.ps1`
	batchPath := `C:\Users\lee\AppData\Roaming\npm\claude.cmd`
	got, err := resolveCommandPathWith("", processCommandResolver{
		goos:   "windows",
		getenv: fakeProcessCommandEnv(nil),
		lookPath: func(name string) (string, error) {
			switch name {
			case "claude.ps1":
				return powerShellPath, nil
			case "claude.cmd":
				return batchPath, nil
			default:
				return "", exec.ErrNotFound
			}
		},
		fileExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("resolve PATH PowerShell shim command path: %v", err)
	}
	if got != powerShellPath {
		t.Fatalf("command path = %q, want PowerShell shim %q", got, powerShellPath)
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

func TestResolveProcessCommandWrapsWindowsPowerShellScript(t *testing.T) {
	scriptPath := `C:\Users\lee\AppData\Roaming\npm\claude.ps1`
	launcherPath := `C:\Program Files\PowerShell\7\pwsh.exe`
	command, err := resolveProcessCommandWith(scriptPath, processCommandResolver{
		goos:   "windows",
		getenv: fakeProcessCommandEnv(nil),
		lookPath: func(name string) (string, error) {
			if name == "pwsh.exe" {
				return launcherPath, nil
			}
			return "", exec.ErrNotFound
		},
	})
	if err != nil {
		t.Fatalf("resolve PowerShell command: %v", err)
	}
	if command.path != scriptPath || command.executable != launcherPath {
		t.Fatalf("resolved command = %#v", command)
	}
	wantArgs := strings.Join([]string{
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-File",
		scriptPath,
		"-v",
	}, "|")
	if got := strings.Join(command.arguments([]string{"-v"}), "|"); got != wantArgs {
		t.Fatalf("PowerShell args = %q, want %q", got, wantArgs)
	}
}

func TestResolveProcessCommandUsesPowerShellSiblingForWindowsBatch(t *testing.T) {
	for _, extension := range []string{".cmd", ".bat", ".CMD", ".BAT"} {
		t.Run(extension, func(t *testing.T) {
			batchPath := `C:\tools\claude` + extension
			scriptPath := `C:\tools\claude.ps1`
			launcherPath := `C:\Program Files\PowerShell\7\pwsh.exe`
			command, err := resolveProcessCommandWith(batchPath, processCommandResolver{
				goos:       "windows",
				getenv:     fakeProcessCommandEnv(nil),
				fileExists: func(path string) bool { return path == scriptPath },
				lookPath: func(name string) (string, error) {
					if name == "pwsh.exe" {
						return launcherPath, nil
					}
					return "", exec.ErrNotFound
				},
			})
			if err != nil {
				t.Fatalf("resolve Windows batch shim: %v", err)
			}
			if command.path != batchPath || command.executable != launcherPath {
				t.Fatalf("resolved command = %#v", command)
			}
			wantArgs := strings.Join([]string{
				"-NoProfile",
				"-NonInteractive",
				"-ExecutionPolicy",
				"Bypass",
				"-File",
				scriptPath,
				`SAFE&echo injected`,
			}, "|")
			if got := strings.Join(command.arguments([]string{`SAFE&echo injected`}), "|"); got != wantArgs {
				t.Fatalf("PowerShell sibling args = %q, want %q", got, wantArgs)
			}
		})
	}
}

func TestResolveProcessCommandRejectsWindowsBatchWithoutPowerShellSibling(t *testing.T) {
	for _, extension := range []string{".cmd", ".bat", ".CMD", ".BAT"} {
		t.Run(extension, func(t *testing.T) {
			batchPath := `C:\tools\claude` + extension
			scriptPath := `C:\tools\claude.ps1`
			_, err := resolveProcessCommandWith(batchPath, processCommandResolver{
				goos:       "windows",
				getenv:     fakeProcessCommandEnv(nil),
				lookPath:   func(string) (string, error) { return "", exec.ErrNotFound },
				fileExists: func(string) bool { return false },
			})
			if err == nil {
				t.Fatal("resolve Windows batch shim succeeded without a PowerShell sibling")
			}
			if !strings.Contains(err.Error(), "cannot safely receive SDK arguments") ||
				!strings.Contains(err.Error(), strconv.Quote(scriptPath)) {
				t.Fatalf("resolve Windows batch shim error = %q", err)
			}
		})
	}
}

func TestResolveProcessCommandFallsBackToWindowsPowerShell(t *testing.T) {
	scriptPath := `C:\tools\claude.ps1`
	launcherPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	command, err := resolveProcessCommandWith(scriptPath, processCommandResolver{
		goos:   "windows",
		getenv: fakeProcessCommandEnv(nil),
		lookPath: func(name string) (string, error) {
			if name == "powershell.exe" {
				return launcherPath, nil
			}
			return "", exec.ErrNotFound
		},
	})
	if err != nil {
		t.Fatalf("resolve Windows PowerShell command: %v", err)
	}
	if command.executable != launcherPath {
		t.Fatalf("launcher path = %q, want %q", command.executable, launcherPath)
	}
}

func TestProcessManagerStartsWindowsPowerShellScript(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell script launcher is Windows-only")
	}
	launcherDir := filepath.Join(
		os.Getenv("SystemRoot"),
		"System32",
		"WindowsPowerShell",
		"v1.0",
	)
	launcherPath := filepath.Join(launcherDir, "powershell.exe")
	if _, err := os.Stat(launcherPath); err != nil {
		t.Skipf("Windows PowerShell is unavailable: %v", err)
	}
	t.Setenv("PATH", launcherDir)
	t.Setenv(skipVersionCheckEnv, "true")
	temporaryRoot := t.TempDir()
	scriptPath := filepath.Join(temporaryRoot, "claude shim.ps1")
	markerPath := filepath.Join(temporaryRoot, "runtime-args.txt")
	script := `
if ($args -contains '-v') {
    [Console]::Out.WriteLine('1.0.0')
    exit 0
}
[IO.File]::WriteAllText($env:NEXUS_BRIDGE_PS1_MARKER, ($args -join '|'))
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write PowerShell shim: %v", err)
	}

	var diagnostics []ProcessDiagnosticEvent
	manager := NewProcessManager(ProcessConfig{
		CommandPath: scriptPath,
		CWD:         temporaryRoot,
		Args:        []string{"runtime", "two words"},
		Env:         map[string]string{"NEXUS_BRIDGE_PS1_MARKER": markerPath},
		Diagnostics: func(event ProcessDiagnosticEvent) {
			diagnostics = append(diagnostics, event)
		},
	})
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Start() PowerShell shim: %v", err)
	}
	if err := manager.Wait(); err != nil {
		t.Fatalf("Wait() PowerShell shim: %v", err)
	}
	args, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read PowerShell runtime marker: %v", err)
	}
	if got := string(args); got != "runtime|two words" {
		t.Fatalf("PowerShell runtime args = %q", got)
	}

	var sawLauncherDiagnostic bool
	for _, event := range diagnostics {
		switch event.Event {
		case "process_start":
			sawLauncherDiagnostic = event.Attributes["command_path"] == scriptPath &&
				strings.EqualFold(
					strings.TrimSpace(fmt.Sprint(event.Attributes["launcher_path"])),
					launcherPath,
				)
		}
	}
	if !sawLauncherDiagnostic {
		t.Fatalf("PowerShell diagnostics = %#v", diagnostics)
	}
}

func TestProcessManagerRedirectsWindowsBatchShimToPowerShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows batch shim redirection is Windows-only")
	}
	launcherDir := filepath.Join(
		os.Getenv("SystemRoot"),
		"System32",
		"WindowsPowerShell",
		"v1.0",
	)
	launcherPath := filepath.Join(launcherDir, "powershell.exe")
	if _, err := os.Stat(launcherPath); err != nil {
		t.Skipf("Windows PowerShell is unavailable: %v", err)
	}
	t.Setenv("PATH", launcherDir)
	t.Setenv(skipVersionCheckEnv, "true")

	temporaryRoot := t.TempDir()
	batchPath := filepath.Join(temporaryRoot, "claude shim.cmd")
	scriptPath := filepath.Join(temporaryRoot, "claude shim.ps1")
	batchMarkerPath := filepath.Join(temporaryRoot, "batch-ran.txt")
	powerShellMarkerPath := filepath.Join(temporaryRoot, "powershell-args.txt")
	injectedMarkerPath := filepath.Join(temporaryRoot, "injected-marker.txt")
	batch := `@echo off
> "%NEXUS_BRIDGE_BATCH_MARKER%" echo batch-ran
exit /b 97
`
	script := `
if ($args -contains '-v') {
    [Console]::Out.WriteLine('1.0.0')
    exit 0
}
[IO.File]::WriteAllText($env:NEXUS_BRIDGE_PS1_MARKER, ($args -join [char]31))
`
	if err := os.WriteFile(batchPath, []byte(batch), 0o600); err != nil {
		t.Fatalf("write batch shim: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write PowerShell shim: %v", err)
	}

	hostileArgs := []string{
		"",
		`SAFE&ver`,
		`SAFE|ver`,
		`SAFE<input`,
		`SAFE>output`,
		`SAFE^caret`,
		`SAFE;Write-Output INJECTED`,
		`%PATH%`,
		`{"key":"value&still-data"}`,
		`two words`,
		"embedded \"quote\"",
		"line1\r\nline2",
		`SAFE&echo.INJECTED>injected-marker.txt`,
		`$([IO.File]::WriteAllText($env:NEXUS_BRIDGE_INJECTED_MARKER,'powershell'))`,
	}
	var diagnostics []ProcessDiagnosticEvent
	manager := NewProcessManager(ProcessConfig{
		CommandPath: batchPath,
		CWD:         temporaryRoot,
		Args:        hostileArgs,
		Env: map[string]string{
			"NEXUS_BRIDGE_BATCH_MARKER":    batchMarkerPath,
			"NEXUS_BRIDGE_INJECTED_MARKER": injectedMarkerPath,
			"NEXUS_BRIDGE_PS1_MARKER":      powerShellMarkerPath,
		},
		Diagnostics: func(event ProcessDiagnosticEvent) {
			diagnostics = append(diagnostics, event)
		},
	})
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Start() Windows batch shim: %v", err)
	}
	if err := manager.Wait(); err != nil {
		t.Fatalf("Wait() Windows batch shim: %v", err)
	}

	args, err := os.ReadFile(powerShellMarkerPath)
	if err != nil {
		t.Fatalf("read PowerShell runtime marker: %v", err)
	}
	if got, want := string(args), strings.Join(hostileArgs, string(rune(31))); got != want {
		t.Fatalf("PowerShell runtime args = %q, want %q", got, want)
	}
	for description, path := range map[string]string{
		"batch shim":       batchMarkerPath,
		"injected command": injectedMarkerPath,
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s marker exists or cannot be checked: %v", description, err)
		}
	}

	var sawLauncherDiagnostic bool
	for _, event := range diagnostics {
		switch event.Event {
		case "process_start":
			sawLauncherDiagnostic = event.Attributes["command_path"] == batchPath &&
				strings.EqualFold(
					strings.TrimSpace(fmt.Sprint(event.Attributes["launcher_path"])),
					launcherPath,
				)
		}
	}
	if !sawLauncherDiagnostic {
		t.Fatalf("Windows batch shim diagnostics = %#v", diagnostics)
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

func TestProcessInterruptSignalFallsBackOnWindows(t *testing.T) {
	signal, err := processInterruptSignal("windows")
	if signal != nil {
		t.Fatalf("processInterruptSignal(windows) signal = %v, want nil", signal)
	}
	if !errors.Is(err, ErrInterruptUnsupported) {
		t.Fatalf("processInterruptSignal(windows) error = %v, want ErrInterruptUnsupported", err)
	}
}

func TestProcessInterruptSignalUsesOSInterruptElsewhere(t *testing.T) {
	signal, err := processInterruptSignal("linux")
	if err != nil {
		t.Fatalf("processInterruptSignal(linux) error = %v", err)
	}
	if signal != os.Interrupt {
		t.Fatalf("processInterruptSignal(linux) signal = %v, want os.Interrupt", signal)
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

func TestBuildEnvironmentForPlatformMergesWindowsKeysCaseInsensitively(t *testing.T) {
	environment := buildEnvironmentForPlatform(
		[]string{
			`Path=C:\Windows\System32`,
			`=C:=C:\work`,
			`=D:=D:\tmp`,
			"ClaudeCode=parent-session",
			"Claude_Code_Entrypoint=parent-entrypoint",
		},
		map[string]string{
			"":     "invalid-empty-key",
			"   ":  "invalid-blank-key",
			"Path": `D:\mixed-case\bin`,
			"PATH": `D:\runtime\bin`,
		},
		"",
		ControlWireDialectNXS,
		"windows",
	)

	if value, count := foldedEnvValue(environment, "PATH"); count != 1 || value != `D:\runtime\bin` {
		t.Fatalf("Windows PATH entries = %d value=%q, want one override: %#v", count, value, environment)
	}
	if _, count := foldedEnvValue(environment, "CLAUDECODE"); count != 0 {
		t.Fatalf("Windows inherited CLAUDECODE should be removed: %#v", environment)
	}
	if _, count := foldedEnvValue(environment, "CLAUDE_CODE_ENTRYPOINT"); count != 0 {
		t.Fatalf("NXS env should remove inherited Claude entrypoint case-insensitively: %#v", environment)
	}
	if value, count := foldedEnvValue(environment, "NEXUS_ENTRYPOINT"); count != 1 || value != "sdk-go" {
		t.Fatalf("NXS entrypoint entries = %d value=%q: %#v", count, value, environment)
	}
	if got := envValue(environment, "=C:"); got != `C:\work` {
		t.Fatalf("Windows hidden =C: entry = %q: %#v", got, environment)
	}
	if got := envValue(environment, "=D:"); got != `D:\tmp` {
		t.Fatalf("Windows hidden =D: entry = %q: %#v", got, environment)
	}
	for _, entry := range environment {
		if entry == "=invalid-empty-key" || entry == "   =invalid-blank-key" {
			t.Fatalf("invalid empty override key leaked into environment: %#v", environment)
		}
	}
}

func TestBuildEnvironmentForPlatformPreservesUnixKeyCasing(t *testing.T) {
	environment := buildEnvironmentForPlatform(
		[]string{"Path=/base/bin"},
		map[string]string{"PATH": "/override/bin"},
		"",
		ControlWireDialectNXS,
		"linux",
	)
	if got := envValue(environment, "Path"); got != "/base/bin" {
		t.Fatalf("Unix Path = %q, want inherited value: %#v", got, environment)
	}
	if got := envValue(environment, "PATH"); got != "/override/bin" {
		t.Fatalf("Unix PATH = %q, want override value: %#v", got, environment)
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

func foldedEnvValue(environment []string, key string) (string, int) {
	value := ""
	count := 0
	for _, entry := range environment {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], key) {
			continue
		}
		value = parts[1]
		count++
	}
	return value, count
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
