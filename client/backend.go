package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
)

// Backend 描述可选的执行后端；未设置时使用本地 process bridge。
type Backend interface {
	applyBackend(*Options) backendRuntime
}

type backendRuntime struct {
	transport Transport
	external  bool
}

// ProcessOptions 表示外部进程后端配置。
type ProcessOptions struct {
	CommandPath string
}

type processBackend struct {
	options ProcessOptions
}

// ProcessBackend 选择外部进程后端。
func ProcessBackend(options ProcessOptions) Backend {
	return processBackend{options: options}
}

func (b processBackend) applyBackend(options *Options) backendRuntime {
	options.commandPath = b.options.CommandPath
	options.directConnect = nil
	return backendRuntime{external: true}
}

type directConnectBackend struct {
	options DirectConnectOptions
}

// DirectConnectOptions 表示 direct-connect 后端配置。
type DirectConnectOptions struct {
	URL                  string
	SessionKey           string
	DeleteSessionOnClose bool
	DialTimeout          time.Duration
	HTTPClient           *http.Client
}

// DirectConnectBackend 选择 direct-connect 后端。
func DirectConnectBackend(options DirectConnectOptions) Backend {
	return directConnectBackend{options: options}
}

func (b directConnectBackend) applyBackend(options *Options) backendRuntime {
	options.directConnect = cloneDirectConnectOptions(&b.options)
	return backendRuntime{external: true}
}

type transportBackend struct {
	transport Transport
}

// TransportBackend 注入自定义后端传输。
func TransportBackend(transport Transport) Backend {
	return transportBackend{transport: transport}
}

func (b transportBackend) applyBackend(*Options) backendRuntime {
	return backendRuntime{transport: b.transport, external: true}
}

func applyBackend(options Options) (Options, Transport) {
	if options.Backend == nil {
		return options, nil
	}
	runtime := options.Backend.applyBackend(&options)
	options.usesExternalBackend = runtime.external
	return options, runtime.transport
}

// DirectConnectEndpoint 表示解析后的 direct-connect 地址。
type DirectConnectEndpoint struct {
	ServerURL string
	AuthToken string
}

// DirectConnectError 暴露底层 direct-connect 传输错误类型。
type DirectConnectError = transport.DirectConnectError

// ParseDirectConnectURL 解析官方 TS SDK 同款 direct-connect 地址格式。
func ParseDirectConnectURL(raw string) (DirectConnectEndpoint, error) {
	endpoint, err := transport.ParseDirectConnectURL(raw)
	if err != nil {
		return DirectConnectEndpoint{}, err
	}
	return DirectConnectEndpoint{
		ServerURL: endpoint.ServerURL,
		AuthToken: endpoint.AuthToken,
	}, nil
}

// Transport 表示可替换的 SDK 底层传输实现。
type Transport = transport.Transport

func buildProcessBackendArgs(o resolvedOptions) []string {
	args := []string{
		"--output-format", "stream-json",
		"--verbose",
		"--input-format", "stream-json",
	}
	effectiveAllowedTools, effectiveSettingSources := processBackendSkillDefaults(o)

	switch {
	case strings.TrimSpace(o.SystemPromptFile) != "":
		args = append(args, "--system-prompt-file", o.SystemPromptFile)
	case o.SystemPromptPreset != nil:
		if o.SystemPromptPreset.Append != "" {
			args = append(args, "--append-system-prompt", o.SystemPromptPreset.Append)
		}
	default:
		args = append(args, "--system-prompt", o.SystemPrompt)
	}
	if o.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", o.AppendSystemPrompt)
	}

	if o.Tools != nil {
		args = append(args, "--tools", strings.Join(o.Tools, ","))
	} else if o.ToolsPreset != nil {
		args = append(args, "--tools", "default")
	}
	if len(effectiveAllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(effectiveAllowedTools, ","))
	}
	if len(o.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(o.DisallowedTools, ","))
	}
	if o.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(o.MaxTurns))
	}
	if o.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", strconv.FormatFloat(o.MaxBudgetUSD, 'f', -1, 64))
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	if o.FallbackModel != "" {
		args = append(args, "--fallback-model", o.FallbackModel)
	}
	if len(o.Betas) > 0 {
		args = append(args, "--betas", strings.Join(o.Betas, ","))
	}
	if o.PermissionPromptToolName != "" {
		args = append(args, "--permission-prompt-tool", o.PermissionPromptToolName)
	}
	if o.PermissionMode != "" {
		args = append(args, "--permission-mode", string(o.PermissionMode))
	}
	if o.AllowsDangerouslySkipPermissions() {
		args = append(args, "--allow-dangerously-skip-permissions")
	}
	if o.ContinueConversation {
		args = append(args, "--continue")
	}
	if o.Resume != "" {
		args = append(args, "--resume", o.Resume)
	}
	if o.Agent != "" {
		args = append(args, "--agent", o.Agent)
	}
	if settingsValue := buildProcessBackendSettingsValue(o); settingsValue != "" {
		args = append(args, "--settings", settingsValue)
	}
	if len(effectiveSettingSources) > 0 {
		args = append(args, "--setting-sources", strings.Join(effectiveSettingSources, ","))
	}
	for _, directory := range o.AdditionalDirectories {
		args = append(args, "--add-dir", directory)
	}
	if mcpConfigValue := buildProcessBackendMCPConfigValue(o); mcpConfigValue != "" {
		args = append(args, "--mcp-config", mcpConfigValue)
	}
	for _, plugin := range o.Plugins {
		if plugin.Type == "local" && plugin.Path != "" {
			args = append(args, "--plugin-dir", plugin.Path)
		}
	}
	if o.TaskBudget != nil && o.TaskBudget.Total > 0 {
		args = append(args, "--task-budget", strconv.Itoa(o.TaskBudget.Total))
	}
	if o.Thinking != nil {
		switch o.Thinking.Type {
		case "adaptive":
			args = append(args, "--thinking", "adaptive")
		case "disabled":
			args = append(args, "--thinking", "disabled")
		case "enabled":
			args = append(args, "--max-thinking-tokens", strconv.Itoa(o.Thinking.BudgetTokens))
		}
		if o.Thinking.Type != "disabled" && o.Thinking.Display != "" {
			args = append(args, "--thinking-display", o.Thinking.Display)
		}
	} else if o.MaxThinkingTokens > 0 {
		args = append(args, "--max-thinking-tokens", strconv.Itoa(o.MaxThinkingTokens))
	}
	if o.Effort != "" {
		args = append(args, "--effort", o.Effort)
	}
	if schema := o.OutputSchema(); len(schema) > 0 {
		args = append(args, "--json-schema", mustJSONString(schema))
	}
	if o.PersistSession != nil && !*o.PersistSession {
		args = append(args, "--no-session-persistence")
	}
	if o.IncludeHookEvents {
		args = append(args, "--include-hook-events")
	}
	if o.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}
	if o.IncludePartialMessages {
		args = append(args, "--include-partial-messages")
	}
	if o.ForkSession {
		args = append(args, "--fork-session")
	}
	if o.ExcludeDynamicSections != nil && *o.ExcludeDynamicSections {
		args = append(args, "--exclude-dynamic-system-prompt-sections")
	}

	boolArgKeys := append([]string(nil), o.ExtraBoolArgs...)
	sort.Strings(boolArgKeys)
	for _, key := range boolArgKeys {
		args = append(args, "--"+key)
	}

	extraArgKeys := make([]string, 0, len(o.ExtraArgs))
	for key := range o.ExtraArgs {
		extraArgKeys = append(extraArgKeys, key)
	}
	sort.Strings(extraArgKeys)
	for _, key := range extraArgKeys {
		args = append(args, "--"+key, o.ExtraArgs[key])
	}

	return args
}

func processBackendSkillDefaults(o resolvedOptions) ([]string, []string) {
	allowedTools := append([]string(nil), o.AllowedTools...)
	settingSources := append([]string(nil), o.SettingSources...)

	switch o.Skills.Mode {
	case SkillModeAll:
		allowedTools = appendStringOnce(allowedTools, "Skill")
	case SkillModeOnly:
		for _, name := range o.Skills.Names {
			if name == "" {
				continue
			}
			allowedTools = appendStringOnce(allowedTools, "Skill("+name+")")
		}
	case SkillModeNone:
	default:
		return allowedTools, settingSources
	}

	if len(settingSources) == 0 {
		settingSources = []string{"user", "project"}
	}
	return allowedTools, settingSources
}

func appendStringOnce(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
func buildProcessBackendSettingsValue(o resolvedOptions) string {
	return o.Settings
}

func buildProcessBackendMCPConfigValue(o resolvedOptions) string {
	if strings.TrimSpace(o.MCPConfig) != "" {
		return o.MCPConfig
	}
	if len(o.MCPSerialized) == 0 {
		return ""
	}
	return mustJSONString(map[string]any{
		"mcpServers": o.MCPSerialized,
	})
}

func mustJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

const (
	nexusConfigDirEnv           = "NEXUS_CONFIG_DIR"
	processConfigDirEnv         = "CLAUDE_CONFIG_DIR"
	processFileCheckpointingEnv = "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING"
	processOAuthRefreshEnv      = "CLAUDE_CODE_SDK_HAS_OAUTH_REFRESH"
)

func buildDirectConnectBackendConfig(o resolvedOptions) (transport.DirectConnectConfig, error) {
	if o.DirectConnect == nil {
		return transport.DirectConnectConfig{}, errors.New("client: direct connect config is not configured")
	}

	endpoint, err := transport.ParseDirectConnectURL(o.DirectConnect.URL)
	if err != nil {
		return transport.DirectConnectConfig{}, err
	}

	return transport.DirectConnectConfig{
		ServerURL:                       endpoint.ServerURL,
		AuthToken:                       endpoint.AuthToken,
		SessionKey:                      o.DirectConnect.SessionKey,
		CWD:                             o.CWD,
		PermissionMode:                  string(o.PermissionMode),
		AllowDangerouslySkipPermissions: o.AllowsDangerouslySkipPermissions(),
		DeleteSessionOnClose:            o.DirectConnect.DeleteSessionOnClose,
		HTTPClient:                      o.DirectConnect.HTTPClient,
		DialTimeout:                     o.DirectConnect.DialTimeout,
	}, nil
}

func buildProcessBackendConfig(o resolvedOptions) transport.ProcessConfig {
	processEnv := map[string]string{}
	for key, value := range o.Env {
		processEnv[key] = value
	}
	processEnv[nexusConfigDirEnv] = resolveConfigDir(processEnv)
	processEnv[processConfigDirEnv] = processEnv[nexusConfigDirEnv]
	if o.EnableFileCheckpointing {
		processEnv[processFileCheckpointingEnv] = "true"
	}
	if o.OAuthTokenHandler != nil {
		processEnv[processOAuthRefreshEnv] = "1"
	}
	return transport.ProcessConfig{
		CommandPath:   o.CommandPath,
		CWD:           o.CWD,
		User:          o.User,
		MaxBufferSize: o.MaxBufferSize,
		Args:          buildProcessBackendArgs(o),
		Env:           processEnv,
		Stderr:        o.Stderr,
		Diagnostics:   processDiagnosticHandler(o.Diagnostics),
	}
}

func resolveConfigDir(env map[string]string) string {
	if value := strings.TrimSpace(env[nexusConfigDirEnv]); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(nexusConfigDirEnv)); value != "" {
		return value
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".nexus")
	}
	return filepath.Join(homeDir, ".nexus")
}

func processDiagnosticHandler(handler DiagnosticHandler) func(transport.ProcessDiagnosticEvent) {
	if handler == nil {
		return nil
	}
	return func(event transport.ProcessDiagnosticEvent) {
		handler(DiagnosticEvent{
			Component:  event.Component,
			Event:      event.Event,
			Attributes: cloneAnyMap(event.Attributes),
		})
	}
}
