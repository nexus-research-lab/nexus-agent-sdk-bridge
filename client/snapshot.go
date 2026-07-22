package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// RuntimeLaunchSnapshot 表示脱敏后的 runtime 启动快照。
type RuntimeLaunchSnapshot struct {
	RuntimeKind        RuntimeKind                  `json:"runtime_kind"`
	Transport          string                       `json:"transport"`
	CommandPath        string                       `json:"command_path,omitempty"`
	Args               []string                     `json:"args,omitempty"`
	ArgFingerprint     string                       `json:"arg_fingerprint,omitempty"`
	CWD                string                       `json:"cwd,omitempty"`
	User               string                       `json:"user,omitempty"`
	EnvKeys            []string                     `json:"env_keys,omitempty"`
	ExplicitEnvKeys    []string                     `json:"explicit_env_keys,omitempty"`
	SDKEnvKeys         []string                     `json:"sdk_env_keys,omitempty"`
	EnvFingerprint     string                       `json:"env_fingerprint,omitempty"`
	ControlWireDialect string                       `json:"control_wire_dialect,omitempty"`
	DirectConnect      *DirectConnectLaunchSnapshot `json:"direct_connect,omitempty"`
	Fingerprint        OptionsFingerprint           `json:"fingerprint"`
}

// DirectConnectLaunchSnapshot 表示脱敏后的 direct-connect 启动快照。
type DirectConnectLaunchSnapshot struct {
	ServerURL            string `json:"server_url,omitempty"`
	SessionKey           string `json:"session_key,omitempty"`
	DeleteSessionOnClose bool   `json:"delete_session_on_close,omitempty"`
	DialTimeoutMillis    int64  `json:"dial_timeout_millis,omitempty"`
	AuthTokenPresent     bool   `json:"auth_token_present"`
	AuthTokenFingerprint string `json:"auth_token_fingerprint,omitempty"`
}

// OptionsFingerprint 表示可用于定位配置变化的脱敏指纹。
type OptionsFingerprint struct {
	Full             string `json:"full"`
	Launch           string `json:"launch"`
	RestartSensitive string `json:"restart_sensitive"`
	ProcessEnv       string `json:"process_env"`
	ToolPolicy       string `json:"tool_policy"`
	MCPServers       string `json:"mcp_servers"`
	RuntimeControls  string `json:"runtime_controls"`
}

// RuntimeLaunchSnapshot 返回脱敏后的真实启动配置快照。
func (o Options) RuntimeLaunchSnapshot() (RuntimeLaunchSnapshot, error) {
	resolved, err := o.resolveOptions()
	if err != nil {
		return RuntimeLaunchSnapshot{}, err
	}
	fingerprint := fingerprintResolvedOptions(resolved)
	if resolved.DirectConnect != nil {
		return directConnectLaunchSnapshot(resolved, fingerprint)
	}
	config := buildProcessTransportConfig(resolved)
	explicitEnvKeys := sortedStringMapKeys(resolved.Env)
	envKeys := sortedStringMapKeys(config.Env)
	return RuntimeLaunchSnapshot{
		RuntimeKind:        resolved.RuntimeKind,
		Transport:          "process",
		CommandPath:        config.CommandPath,
		Args:               redactRuntimeLaunchArgs(config.Args),
		ArgFingerprint:     digestValue(config.Args),
		CWD:                strings.TrimSpace(config.CWD),
		User:               strings.TrimSpace(config.User),
		EnvKeys:            envKeys,
		ExplicitEnvKeys:    explicitEnvKeys,
		SDKEnvKeys:         subtractSortedStrings(envKeys, explicitEnvKeys),
		EnvFingerprint:     digestValue(config.Env),
		ControlWireDialect: string(config.ControlWireDialect),
		Fingerprint:        fingerprint,
	}, nil
}

// OptionsFingerprint 返回脱敏后的配置指纹。
func (o Options) OptionsFingerprint() (OptionsFingerprint, error) {
	resolved, err := o.resolveOptions()
	if err != nil {
		return OptionsFingerprint{}, err
	}
	return fingerprintResolvedOptions(resolved), nil
}

func directConnectLaunchSnapshot(
	resolved resolvedOptions,
	fingerprint OptionsFingerprint,
) (RuntimeLaunchSnapshot, error) {
	config, err := buildDirectConnectTransportConfig(resolved)
	if err != nil {
		return RuntimeLaunchSnapshot{}, err
	}
	authTokenFingerprint := ""
	if strings.TrimSpace(config.AuthToken) != "" {
		authTokenFingerprint = digestValue(config.AuthToken)
	}
	return RuntimeLaunchSnapshot{
		RuntimeKind: resolved.RuntimeKind,
		Transport:   "direct_connect",
		CWD:         strings.TrimSpace(config.CWD),
		DirectConnect: &DirectConnectLaunchSnapshot{
			ServerURL:            strings.TrimSpace(config.ServerURL),
			SessionKey:           strings.TrimSpace(config.SessionKey),
			DeleteSessionOnClose: config.DeleteSessionOnClose,
			DialTimeoutMillis:    config.DialTimeout.Milliseconds(),
			AuthTokenPresent:     strings.TrimSpace(config.AuthToken) != "",
			AuthTokenFingerprint: authTokenFingerprint,
		},
		Fingerprint: fingerprint,
	}, nil
}

func fingerprintResolvedOptions(o resolvedOptions) OptionsFingerprint {
	processEnv := digestValue(buildProcessTransportEnv(o))
	toolPolicy := digestValue(map[string]any{
		"allowed_tools":    o.AllowedTools,
		"disallowed_tools": o.DisallowedTools,
	})
	skillConfig := digestValue(map[string]any{
		"skills":                 o.Skills,
		"setting_sources":        o.SettingSources,
		"additional_directories": o.AdditionalDirectories,
	})
	mcpServers := digestValue(o.MCPSerialized)
	runtimeControls := digestValue(map[string]any{
		"permission_mode":      string(o.PermissionMode),
		"model":                o.Model,
		"max_thinking_tokens":  o.MaxThinkingTokens,
		"mcp_servers_digest":   mcpServers,
		"allow_skip_at_launch": o.AllowsDangerouslySkipPermissions(),
	})
	launch := digestValue(runtimeLaunchFingerprintPayload(o))
	restartSensitive := digestValue(map[string]any{
		"process_env":  processEnv,
		"tool_policy":  toolPolicy,
		"skill_config": skillConfig,
	})
	full := digestValue(map[string]any{
		"launch":            launch,
		"restart_sensitive": restartSensitive,
		"runtime_controls":  runtimeControls,
		"mcp_servers":       mcpServers,
	})
	return OptionsFingerprint{
		Full:             full,
		Launch:           launch,
		RestartSensitive: restartSensitive,
		ProcessEnv:       processEnv,
		ToolPolicy:       toolPolicy,
		MCPServers:       mcpServers,
		RuntimeControls:  runtimeControls,
	}
}

func runtimeLaunchFingerprintPayload(o resolvedOptions) map[string]any {
	payload := map[string]any{
		"runtime_kind":           string(o.RuntimeKind),
		"command_path":           o.CommandPath,
		"executable":             o.Executable,
		"executable_args":        o.ExecutableArgs,
		"path_to_executable":     o.PathToExecutable,
		"cwd":                    o.CWD,
		"user":                   o.User,
		"args":                   buildProcessTransportArgs(o),
		"env":                    buildProcessTransportEnv(o),
		"direct_connect":         nil,
		"enable_checkpointing":   o.EnableFileCheckpointing,
		"control_wire_dialect":   string(processControlWireDialect(o)),
		"include_partial_events": o.IncludePartialMessages,
	}
	if o.DirectConnect != nil {
		payload["direct_connect"] = map[string]any{
			"url":                     o.DirectConnect.URL,
			"session_key":             o.DirectConnect.SessionKey,
			"delete_session_on_close": o.DirectConnect.DeleteSessionOnClose,
			"dial_timeout":            o.DirectConnect.DialTimeout.String(),
		}
	}
	return payload
}

func redactRuntimeLaunchArgs(args []string) []string {
	result := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		flagName, hasInlineValue := splitRuntimeArg(arg)
		if runtimeArgValueShouldBeRedacted(flagName) {
			if hasInlineValue {
				result = append(result, flagName+"=<redacted>")
				continue
			}
			result = append(result, arg)
			if index+1 < len(args) {
				result = append(result, "<redacted>")
				index++
			}
			continue
		}
		result = append(result, arg)
	}
	return result
}

func splitRuntimeArg(arg string) (string, bool) {
	trimmed := strings.TrimSpace(arg)
	if equalIndex := strings.Index(trimmed, "="); equalIndex >= 0 {
		return strings.TrimSpace(trimmed[:equalIndex]), true
	}
	return trimmed, false
}

func runtimeArgValueShouldBeRedacted(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "--system-prompt",
		"--append-system-prompt",
		"--settings",
		"--mcp-config",
		"--json-schema":
		return true
	default:
		return false
	}
}

func sortedStringMapKeys(input map[string]string) []string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func subtractSortedStrings(left []string, right []string) []string {
	if len(left) == 0 {
		return nil
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	result := make([]string, 0, len(left))
	for _, value := range left {
		if _, ok := rightSet[value]; ok {
			continue
		}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func digestValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte("<unserializable>")
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}
