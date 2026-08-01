package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeCommandResolverResolveClaudeCommandPath(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	home := "/Users/lee"
	nvmPath := filepath.Join(home, ".nvm", "versions", "node", "v22.11.0", "bin", "claude")
	nvmPattern := filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "claude")

	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		lookPath map[string]string
		exists   map[string]bool
		globs    map[string][]string
		want     string
	}{
		{
			name: "override",
			goos: "windows",
			env:  map[string]string{claudeCommandPathEnvName: `D:\tools\claude.exe`},
			want: `D:\tools\claude.exe`,
		},
		{
			name:   "windows npm shim",
			goos:   "windows",
			env:    map[string]string{"APPDATA": appData},
			exists: map[string]bool{filepath.Join(appData, "npm", "claude.cmd"): true},
			want:   filepath.Join(appData, "npm", "claude.cmd"),
		},
		{
			name: "windows prefers PowerShell shim",
			goos: "windows",
			env:  map[string]string{"APPDATA": appData},
			exists: map[string]bool{
				filepath.Join(appData, "npm", "claude.ps1"): true,
				filepath.Join(appData, "npm", "claude.cmd"): true,
			},
			want: filepath.Join(appData, "npm", "claude.ps1"),
		},
		{
			name:     "look path",
			goos:     "windows",
			lookPath: map[string]string{"claude.cmd": `C:\Users\lee\AppData\Roaming\npm\claude.cmd`},
			want:     `C:\Users\lee\AppData\Roaming\npm\claude.cmd`,
		},
		{
			name: "look path prefers PowerShell shim",
			goos: "windows",
			lookPath: map[string]string{
				"claude.ps1": `C:\Users\lee\AppData\Roaming\npm\claude.ps1`,
				"claude.cmd": `C:\Users\lee\AppData\Roaming\npm\claude.cmd`,
			},
			want: `C:\Users\lee\AppData\Roaming\npm\claude.ps1`,
		},
		{
			name:   "nvm install",
			goos:   "darwin",
			env:    map[string]string{"HOME": home},
			exists: map[string]bool{nvmPath: true},
			globs:  map[string][]string{nvmPattern: []string{nvmPath}},
			want:   nvmPath,
		},
		{name: "fallback", goos: "linux", want: "claude"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := runtimeCommandResolver{
				goos:   test.goos,
				getenv: fakeCommandEnv(test.env),
				lookPath: func(name string) (string, error) {
					if value := test.lookPath[name]; value != "" {
						return value, nil
					}
					return "", os.ErrNotExist
				},
				fileExists: func(path string) bool { return test.exists[path] },
				globPaths:  fakeCommandGlob(test.globs),
			}
			if got := resolver.resolveClaudeCommandPath(); got != test.want {
				t.Fatalf("resolveClaudeCommandPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClaudeCommandNamesOnWindowsAreSafestFirst(t *testing.T) {
	want := []string{"claude.exe", "claude.ps1", "claude.cmd", "claude"}
	got := claudeCommandNames("windows")
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("claudeCommandNames(windows) = %q, want %q", got, want)
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
