package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeCommandResolverUsesClaudeOverride(t *testing.T) {
	expected := `D:\tools\claude.exe`
	resolver := runtimeCommandResolver{
		goos:       "windows",
		getenv:     fakeCommandEnv(map[string]string{claudeCommandPathEnvName: expected}),
		lookPath:   func(string) (string, error) { return "", os.ErrNotExist },
		fileExists: func(string) bool { return false },
		globPaths:  fakeCommandGlob(nil),
	}
	if got := resolver.resolveClaudeCommandPath(); got != expected {
		t.Fatalf("claude override = %q, want %q", got, expected)
	}
}

func TestRuntimeCommandResolverUsesWindowsNPMShim(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	expected := filepath.Join(appData, "npm", "claude.cmd")
	resolver := runtimeCommandResolver{
		goos:       "windows",
		getenv:     fakeCommandEnv(map[string]string{"APPDATA": appData}),
		lookPath:   func(string) (string, error) { return "", os.ErrNotExist },
		fileExists: func(path string) bool { return path == expected },
		globPaths:  fakeCommandGlob(nil),
	}
	if got := resolver.resolveClaudeCommandPath(); got != expected {
		t.Fatalf("windows npm shim = %q, want %q", got, expected)
	}
}

func TestRuntimeCommandResolverPrefersClaudeLookPath(t *testing.T) {
	expected := `C:\Users\lee\AppData\Roaming\npm\claude.cmd`
	resolver := runtimeCommandResolver{
		goos:   "windows",
		getenv: fakeCommandEnv(nil),
		lookPath: func(name string) (string, error) {
			if name == "claude.cmd" {
				return expected, nil
			}
			return "", os.ErrNotExist
		},
		fileExists: func(string) bool { return false },
		globPaths:  fakeCommandGlob(nil),
	}
	if got := resolver.resolveClaudeCommandPath(); got != expected {
		t.Fatalf("PATH claude shim = %q, want %q", got, expected)
	}
}

func TestRuntimeCommandResolverUsesNVMClaudeGlobalInstall(t *testing.T) {
	home := "/Users/lee"
	expected := filepath.Join(home, ".nvm", "versions", "node", "v22.11.0", "bin", "claude")
	pattern := filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "claude")
	resolver := runtimeCommandResolver{
		goos:       "darwin",
		getenv:     fakeCommandEnv(map[string]string{"HOME": home}),
		lookPath:   func(string) (string, error) { return "", os.ErrNotExist },
		fileExists: func(path string) bool { return path == expected },
		globPaths:  fakeCommandGlob(map[string][]string{pattern: []string{expected}}),
	}
	if got := resolver.resolveClaudeCommandPath(); got != expected {
		t.Fatalf("nvm claude path = %q, want %q", got, expected)
	}
}

func TestRuntimeCommandResolverFallsBackToClaudeCommandName(t *testing.T) {
	resolver := runtimeCommandResolver{
		goos:       "linux",
		getenv:     fakeCommandEnv(nil),
		lookPath:   func(string) (string, error) { return "", os.ErrNotExist },
		fileExists: func(string) bool { return false },
		globPaths:  fakeCommandGlob(nil),
	}
	if got := resolver.resolveClaudeCommandPath(); got != "claude" {
		t.Fatalf("claude default command = %q, want claude", got)
	}
}

func TestRuntimeCommandResolverUsesAppRootNXSRuntime(t *testing.T) {
	expected := filepath.Join(`C:\Nexus\Resources`, "bin", "nxs.exe")
	resolver := runtimeCommandResolver{
		goos:       "windows",
		getenv:     fakeCommandEnv(map[string]string{nexusAppRootEnvName: `C:\Nexus\Resources`}),
		lookPath:   func(string) (string, error) { return "", os.ErrNotExist },
		fileExists: func(path string) bool { return path == expected },
		globPaths:  fakeCommandGlob(nil),
	}
	if got := resolver.resolvePackagedNXSCommandPath(); got != expected {
		t.Fatalf("packaged nxs path = %q, want %q", got, expected)
	}
}

func fakeCommandEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func fakeCommandGlob(values map[string][]string) func(string) ([]string, error) {
	return func(pattern string) ([]string, error) {
		return values[pattern], nil
	}
}
