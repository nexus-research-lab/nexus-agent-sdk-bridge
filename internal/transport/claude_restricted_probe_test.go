package transport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestClaudeRestrictedProbeRequiresAdvertisedFlagAndScrubsSecrets(t *testing.T) {
	command := newClaudeProbeTestCommand("restricted")

	err := verifyClaudeRestrictedCommand(context.Background(), command, ProcessConfig{
		Env: map[string]string{
			"NEXUS_PROBE_SECRET_TOKEN": "probe-secret",
		},
		ControlWireDialect: ControlWireDialectClaude,
	})
	if err != nil {
		t.Fatalf("restricted probe rejected a compatible CLI: %v", err)
	}
}

func TestClaudeRestrictedProbeFailsClosedWhenFlagIsMissing(t *testing.T) {
	command := newClaudeProbeTestCommand("missing")
	err := verifyClaudeRestrictedCommand(context.Background(), command, ProcessConfig{
		ControlWireDialect: ControlWireDialectClaude,
	})
	if err == nil || !strings.Contains(err.Error(), "did not advertise --restricted") {
		t.Fatalf("missing restricted flag was accepted: %v", err)
	}
}

// newClaudeProbeTestCommand 以原生测试二进制代替 shell，所有平台执行同一预检合同。
func newClaudeProbeTestCommand(mode string) processCommand {
	return processCommand{path: os.Args[0], executable: os.Args[0], prefixArgs: []string{
		"-test.run=^TestClaudeProbeCLIHelper$", "--", "claude-probe-test", mode,
	}}
}

// TestClaudeProbeCLIHelper 校验真实 argv/环境；直接退出避免测试框架输出伪造能力标记。
func TestClaudeProbeCLIHelper(t *testing.T) {
	index := slices.Index(os.Args, "--")
	if index < 0 || len(os.Args) <= index+2 || os.Args[index+1] != "claude-probe-test" {
		return
	}
	if os.Getenv("NEXUS_PROBE_SECRET_TOKEN") != "" {
		os.Exit(91)
	}
	arguments := os.Args[index+3:]
	switch os.Args[index+2] {
	case "settings":
		if len(arguments) != 3 || arguments[0] != "--settings" || arguments[2] != "--help" || !strings.Contains(arguments[1], `"failIfUnavailable":true`) {
			os.Exit(92)
		}
		fmt.Println("Claude Code options: --settings <file-or-json>")
	case "restricted":
		if !slices.Equal(arguments, []string{"--restricted", "--help"}) {
			os.Exit(92)
		}
		fmt.Println("Claude Code options: --restricted")
	case "missing":
		fmt.Println("Claude Code options: --print")
	default:
		os.Exit(93)
	}
	os.Exit(0)
}

func writeProbeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
