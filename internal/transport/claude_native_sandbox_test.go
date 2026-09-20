// INPUT: Claude 原生 sandbox typed launch settings。
// OUTPUT: 证明 Bridge 不接受缺失、重复或可绕过的 settings。
// POS: 不启动真实模型请求的进程准入回归。
package transport

import (
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
