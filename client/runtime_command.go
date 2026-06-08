package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	claudeCommandPathEnvName = "NEXUS_CLAUDE_COMMAND_PATH"
	nexusAppRootEnvName      = "NEXUS_APP_ROOT"
)

type runtimeCommandResolver struct {
	goos       string
	getenv     func(string) string
	lookPath   func(string) (string, error)
	fileExists func(string) bool
	globPaths  func(string) ([]string, error)
}

func defaultRuntimeCommandResolver() runtimeCommandResolver {
	return runtimeCommandResolver{
		goos:     runtime.GOOS,
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		fileExists: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
		globPaths: filepath.Glob,
	}
}

func (r runtimeCommandResolver) resolveClaudeCommandPath() string {
	if override := strings.TrimSpace(r.getenv(claudeCommandPathEnvName)); override != "" {
		return override
	}
	for _, name := range claudeCommandNames(r.goos) {
		if path, err := r.lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path
		}
	}
	for _, candidate := range knownClaudeCommandPaths(r.goos, r.getenv) {
		if r.fileExists(candidate) {
			return candidate
		}
	}
	for _, candidate := range knownClaudeCommandPathGlobs(r.goos, r.getenv, r.globPaths) {
		if r.fileExists(candidate) {
			return candidate
		}
	}
	return claudeDefaultCommandName(r.goos)
}

func (r runtimeCommandResolver) resolvePackagedNXSCommandPath() string {
	appRoot := strings.TrimSpace(r.getenv(nexusAppRootEnvName))
	if appRoot == "" {
		return ""
	}
	candidate := filepath.Join(appRoot, "bin", nxsExecutableName(r.goos))
	if r.fileExists(candidate) {
		return candidate
	}
	return ""
}

func claudeCommandNames(goos string) []string {
	if goos == "windows" {
		// Windows 的 npm 全局安装通常只提供 claude.cmd/claude.ps1。
		return []string{"claude.exe", "claude.cmd", "claude.ps1", "claude"}
	}
	return []string{"claude"}
}

func claudeDefaultCommandName(goos string) string {
	if goos == "windows" {
		return "claude.exe"
	}
	return "claude"
}

func nxsExecutableName(goos string) string {
	if goos == "windows" {
		return "nxs.exe"
	}
	return "nxs"
}

func knownClaudeCommandPaths(goos string, getenv func(string) string) []string {
	switch goos {
	case "windows":
		return knownWindowsClaudeCommandPaths(getenv)
	case "darwin":
		return knownDarwinClaudeCommandPaths(getenv)
	default:
		candidates := []string{
			"/usr/local/bin/claude",
			"/usr/bin/claude",
			"/home/linuxbrew/.linuxbrew/bin/claude",
		}
		if homebrewPrefix := strings.TrimSpace(getenv("HOMEBREW_PREFIX")); homebrewPrefix != "" {
			candidates = append([]string{filepath.Join(homebrewPrefix, "bin", "claude")}, candidates...)
		}
		return knownUnixClaudeCommandPaths(getenv, candidates)
	}
}

func knownWindowsClaudeCommandPaths(getenv func(string) string) []string {
	candidates := []string{}
	if appData := strings.TrimSpace(getenv("APPDATA")); appData != "" {
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(appData, "npm"))
	}
	if userProfile := strings.TrimSpace(getenv("USERPROFILE")); userProfile != "" {
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(userProfile, ".local", "bin"))
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(userProfile, ".claude", "local"))
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(userProfile, "node_modules", ".bin"))
	}
	return candidates
}

func knownDarwinClaudeCommandPaths(getenv func(string) string) []string {
	candidates := []string{
		"/opt/homebrew/bin/claude",
		"/usr/local/bin/claude",
	}
	if homebrewPrefix := strings.TrimSpace(getenv("HOMEBREW_PREFIX")); homebrewPrefix != "" {
		candidates = append([]string{filepath.Join(homebrewPrefix, "bin", "claude")}, candidates...)
	}
	candidates = append(candidates, knownUserClaudeCommandPaths(getenv)...)
	if home := strings.TrimSpace(getenv("HOME")); home != "" {
		candidates = append(candidates, filepath.Join(home, "Library", "pnpm", "claude"))
	}
	candidates = append(candidates, knownPackageManagerClaudeCommandPaths(getenv)...)
	return compactClaudeCommandCandidates(candidates)
}

func knownUnixClaudeCommandPaths(getenv func(string) string, systemCandidates []string) []string {
	candidates := append([]string(nil), systemCandidates...)
	candidates = append(candidates, knownUserClaudeCommandPaths(getenv)...)
	candidates = append(candidates, knownPackageManagerClaudeCommandPaths(getenv)...)
	return compactClaudeCommandCandidates(candidates)
}

func knownUserClaudeCommandPaths(getenv func(string) string) []string {
	home := strings.TrimSpace(getenv("HOME"))
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
		filepath.Join(home, ".npm-global", "bin", "claude"),
		filepath.Join(home, ".volta", "bin", "claude"),
		filepath.Join(home, ".asdf", "shims", "claude"),
		filepath.Join(home, "node_modules", ".bin", "claude"),
		filepath.Join(home, ".yarn", "bin", "claude"),
	}
}

func knownPackageManagerClaudeCommandPaths(getenv func(string) string) []string {
	home := strings.TrimSpace(getenv("HOME"))
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".bun", "bin", "claude"),
		filepath.Join(home, ".pnpm", "claude"),
	}
}

func knownClaudeCommandPathGlobs(
	goos string,
	getenv func(string) string,
	globPaths func(string) ([]string, error),
) []string {
	if goos == "windows" {
		return nil
	}
	home := strings.TrimSpace(getenv("HOME"))
	if home == "" {
		return nil
	}
	patterns := []string{
		filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "claude"),
		filepath.Join(home, ".fnm", "node-versions", "*", "installation", "bin", "claude"),
		filepath.Join(home, ".nodenv", "versions", "*", "bin", "claude"),
	}
	candidates := []string{}
	for _, pattern := range patterns {
		matches, err := globPaths(pattern)
		if err != nil {
			continue
		}
		candidates = append(candidates, matches...)
	}
	return compactClaudeCommandCandidates(candidates)
}

func appendWindowsClaudeNames(candidates []string, dir string) []string {
	for _, name := range []string{"claude.exe", "claude.cmd", "claude.ps1", "claude"} {
		candidates = append(candidates, filepath.Join(dir, name))
	}
	return candidates
}

func compactClaudeCommandCandidates(candidates []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		normalized := strings.TrimSpace(candidate)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(normalized))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
