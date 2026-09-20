// INPUT: Bridge 已解析的 Claude 进程参数和原生 sandbox typed 要求。
// OUTPUT: 在正式 stream-json 进程前确认强制 sandbox settings 没有被覆盖。
// POS: Claude 原生命令沙箱的参数准入；不把它冒充为 nxs wire 能力。
package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// verifyClaudeNativeSandboxSettings checks the exact generated --settings JSON
// before starting Claude. The CLI owns the actual OS enforcement; this check
// prevents a duplicate or user-supplied settings argument from replacing the
// host's fail-closed policy.
func verifyClaudeNativeSandboxSettings(config ProcessConfig) error {
	if !config.RequireClaudeNativeSandbox {
		return nil
	}
	settingsValue, count := processArgumentValue(config.Args, "--settings")
	if count != 1 || strings.TrimSpace(settingsValue) == "" {
		return errors.New("process: Claude native sandbox requires one generated --settings object")
	}
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(settingsValue))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return fmt.Errorf("process: Claude native sandbox settings are invalid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("process: Claude native sandbox settings contain trailing JSON")
	}
	sandbox, ok := root["sandbox"].(map[string]any)
	if !ok {
		return errors.New("process: Claude native sandbox settings are missing sandbox object")
	}
	if value, ok := sandbox["enabled"].(bool); !ok || !value {
		return errors.New("process: Claude native sandbox requires sandbox.enabled=true")
	}
	if value, ok := sandbox["failIfUnavailable"].(bool); !ok || !value {
		return errors.New("process: Claude native sandbox requires sandbox.failIfUnavailable=true")
	}
	if value, ok := sandbox["allowUnsandboxedCommands"].(bool); !ok || value {
		return errors.New("process: Claude native sandbox must reject unsandboxed commands")
	}
	return nil
}

func processArgumentValue(arguments []string, name string) (string, int) {
	count := 0
	value := ""
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != name {
			continue
		}
		count++
		if index+1 < len(arguments) {
			value = arguments[index+1]
		}
	}
	return value, count
}
