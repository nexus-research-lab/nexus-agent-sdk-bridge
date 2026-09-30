// INPUT: Claude 原生 sandbox typed launch settings。
// OUTPUT: 证明 Bridge 不接受缺失、重复或可绕过的 settings。
// POS: 不启动真实模型请求的进程准入回归。
package transport

import (
	"context"
	"strings"
	"testing"
)

func TestVerifyClaudeNativeSandboxSettings(t *testing.T) {
	valid := ProcessConfig{
		RequireClaudeNativeSandbox: true,
		Args:                       []string{"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}}`},
	}
	if err := verifyClaudeNativeSandboxSettings(valid); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	for name, arguments := range map[string][]string{
		"missing":       {"--settings", `{"sandbox":{"enabled":true}}`},
		"unsafe":        {"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":true}}`},
		"duplicate":     {"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}}`, "--settings", `{"sandbox":{"enabled":false}}`},
		"invalid":       {"--settings", `{"sandbox":`},
		"not_an_object": {"--settings", `{"sandbox":true}`},
		"trailing":      {"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}} trailing`},
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			config.Args = arguments
			if err := verifyClaudeNativeSandboxSettings(config); err == nil {
				t.Fatal("unsafe native sandbox settings accepted")
			}
		})
	}
}

func TestVerifyClaudeNativeSandboxSettingsIgnoresDisabledContract(t *testing.T) {
	config := ProcessConfig{Args: []string{"--settings", `{"sandbox":{"enabled":false}}`}}
	if err := verifyClaudeNativeSandboxSettings(config); err != nil {
		t.Fatalf("disabled contract was unexpectedly checked: %v", err)
	}
}

func TestProcessArgumentValueUsesLastValueForDiagnostics(t *testing.T) {
	value, count := processArgumentValue([]string{"--settings", "one", "--other", "x", "--settings", "two"}, "--settings")
	if count != 2 || !strings.EqualFold(value, "two") {
		t.Fatalf("value=%q count=%d", value, count)
	}
}

func TestClaudeNativeSandboxProbeUsesSettingsAndScrubsSecrets(t *testing.T) {
	command := newClaudeProbeTestCommand("settings")
	err := verifyClaudeNativeSandboxCommand(context.Background(), command, ProcessConfig{
		Args:                       []string{"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}}`},
		Env:                        map[string]string{"NEXUS_PROBE_SECRET_TOKEN": "probe-secret"},
		RequireClaudeNativeSandbox: true,
		ControlWireDialect:         ControlWireDialectClaude,
	})
	if err != nil {
		t.Fatalf("native sandbox probe rejected a compatible CLI: %v", err)
	}
}

func TestClaudeNativeSandboxProbeFailsClosedWhenSettingsFlagIsMissing(t *testing.T) {
	command := newClaudeProbeTestCommand("missing")
	err := verifyClaudeNativeSandboxCommand(context.Background(), command, ProcessConfig{
		Args:                       []string{"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}}`},
		RequireClaudeNativeSandbox: true,
		ControlWireDialect:         ControlWireDialectClaude,
	})
	if err == nil || !strings.Contains(err.Error(), "did not advertise --settings") {
		t.Fatalf("missing settings flag was accepted: %v", err)
	}
}
