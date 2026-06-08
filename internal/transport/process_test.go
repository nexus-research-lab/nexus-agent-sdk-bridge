package transport

import (
	"os"
	"os/exec"
	"path/filepath"
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
