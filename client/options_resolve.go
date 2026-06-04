package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpwire"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"
)

const (
	nxsCommandPathEnvName             = "NEXUS_NXS_COMMAND_PATH"
	nxsRuntimeResolverDisabledEnvName = "NEXUS_NXS_RUNTIME_RESOLVER_DISABLED"
	legacyBundledNXSDisabledEnvName   = "NEXUS_BUNDLED_NXS_DISABLED"
	defaultNXSCommandExecutable       = "nxs"
)

func defaultInitializeTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv(streamCloseTimeoutEnv))
	if raw == "" {
		return defaultInitializeTimeout
	}
	milliseconds, err := strconv.Atoi(raw)
	if err != nil || milliseconds <= 0 {
		return defaultInitializeTimeout
	}
	timeout := time.Duration(milliseconds) * time.Millisecond
	if timeout < defaultInitializeTimeout {
		return defaultInitializeTimeout
	}
	return timeout
}

func diagnosticsToStderrEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(diagnosticsEnv))) {
	case "1", "true", "yes", "on", "stderr", "terminal", "debug":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(debugEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func stderrDiagnosticHandler(event DiagnosticEvent) {
	attributes := "{}"
	if len(event.Attributes) > 0 {
		if payload, err := json.Marshal(event.Attributes); err == nil {
			attributes = string(payload)
		}
	}
	fmt.Fprintf(
		os.Stderr,
		"[nexus-agent-sdk] component=%s event=%s attrs=%s\n",
		strings.TrimSpace(event.Component),
		strings.TrimSpace(event.Event),
		attributes,
	)
}

func hasStructuredSettings(o Options) bool {
	return len(o.SettingsObject) > 0 || o.Sandbox != nil
}

func decodeInlineSettingsObject(raw string) (map[string]any, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false, nil
	}
	if !strings.HasPrefix(trimmed, "{") {
		return nil, false, nil
	}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, true, fmt.Errorf("client: settings inline object is invalid JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, true, fmt.Errorf("client: settings inline object contains trailing JSON values")
	}
	return value, true, nil
}

func (o Options) normalized() (Options, error) {
	result := o
	result.DirectConnect = cloneDirectConnectOptions(result.DirectConnect)
	if result.Runtime.InitializeTimeout <= 0 {
		result.Runtime.InitializeTimeout = defaultInitializeTimeoutFromEnv()
	}
	if result.Callbacks.Diagnostics == nil && diagnosticsToStderrEnabled() {
		result.Callbacks.Diagnostics = stderrDiagnosticHandler
	}
	if err := result.resolveRuntimeCommand(); err != nil {
		return Options{}, err
	}
	if result.System.Text != "" && strings.TrimSpace(result.System.File) != "" {
		return Options{}, fmt.Errorf("client: system prompt text and system prompt file cannot be used together")
	}
	if result.System.Text != "" && result.System.Preset != nil {
		return Options{}, fmt.Errorf("client: system prompt text and system prompt preset cannot be used together")
	}
	if strings.TrimSpace(result.System.File) != "" && result.System.Preset != nil {
		return Options{}, fmt.Errorf("client: system prompt file and system prompt preset cannot be used together")
	}
	if result.System.Preset != nil {
		if result.System.Preset.Preset == "" {
			result.System.Preset.Preset = "nexus"
		}
		if result.System.Preset.Preset != "nexus" {
			return Options{}, fmt.Errorf("client: unsupported system prompt preset %q", result.System.Preset.Preset)
		}
		if result.System.ExcludeDynamicSections == nil && result.System.Preset.ExcludeDynamicSections != nil {
			value := *result.System.Preset.ExcludeDynamicSections
			result.System.ExcludeDynamicSections = &value
		}
	}
	if result.Tools.Available != nil && result.Tools.Preset != nil {
		return Options{}, fmt.Errorf("client: tools and tools preset cannot be used together")
	}
	if result.Tools.Preset != nil {
		if result.Tools.Preset.Preset == "" {
			result.Tools.Preset.Preset = "nexus"
		}
		if result.Tools.Preset.Preset != "nexus" {
			return Options{}, fmt.Errorf("client: unsupported tools preset %q", result.Tools.Preset.Preset)
		}
	}
	skills, err := result.Skills.normalized()
	if err != nil {
		return Options{}, err
	}
	result.Skills = skills

	if result.Callbacks.PermissionHandler != nil {
		if result.Runtime.PermissionPromptToolName != "" && result.Runtime.PermissionPromptToolName != "stdio" {
			return Options{}, fmt.Errorf("client: permission handler requires permission prompt tool stdio")
		}
		result.Runtime.PermissionPromptToolName = "stdio"
	}
	if result.Runtime.PermissionMode == permission.ModeBypassPermissions {
		result.Runtime.AllowDangerouslySkipPermissions = true
	}

	if result.Env == nil {
		result.Env = map[string]string{}
	}
	if result.ExtraArgs == nil {
		result.ExtraArgs = map[string]string{}
	}
	if result.MCP.Servers == nil {
		result.MCP.Servers = map[string]mcp.ServerConfig{}
	}
	if result.Hooks.Matchers == nil {
		result.Hooks.Matchers = map[hook.Event][]hook.Matcher{}
	}
	if result.MCP.SDKServers == nil {
		result.MCP.SDKServers = map[string]mcp.SDKMCPServer{}
	}
	if result.Agents == nil {
		result.Agents = map[string]AgentDefinition{}
	}
	if result.OutputFormat != nil {
		if result.OutputFormat.Type != "" && result.OutputFormat.Type != OutputFormatTypeJSONSchema {
			return Options{}, fmt.Errorf("client: unsupported output format type %q", result.OutputFormat.Type)
		}
		if len(result.OutputFormat.Schema) == 0 {
			return Options{}, fmt.Errorf("client: output format schema is required")
		}
	}
	if result.MCP.Config != "" && len(result.MCP.Servers) > 0 {
		return Options{}, fmt.Errorf("client: mcp config path and MCP.Servers cannot be used together")
	}
	if hasStructuredSettings(result) {
		if _, ok, err := decodeInlineSettingsObject(result.Settings); err != nil {
			return Options{}, err
		} else if strings.TrimSpace(result.Settings) != "" && !ok {
			return Options{}, fmt.Errorf("client: settings path cannot be combined with inline settings object or sandbox")
		}
	}
	if result.ToolConfig != nil && result.ToolConfig.AskUserQuestion != nil {
		switch result.ToolConfig.AskUserQuestion.PreviewFormat {
		case "", QuestionPreviewFormatMarkdown, QuestionPreviewFormatHTML:
		default:
			return Options{}, fmt.Errorf("client: unsupported AskUserQuestion preview format %q", result.ToolConfig.AskUserQuestion.PreviewFormat)
		}
	}
	if len(result.ExecutableArgs) > 0 && strings.TrimSpace(result.Executable) == "" {
		return Options{}, fmt.Errorf("client: executable args require executable")
	}
	if strings.TrimSpace(result.Executable) != "" && strings.TrimSpace(result.PathToExecutable) == "" && strings.TrimSpace(result.CLIPath) == "" {
		return Options{}, fmt.Errorf("client: executable requires path to Claude Code executable")
	}
	if len(result.MCP.Servers) > 0 {
		if _, _, err := mcpwire.SerializeServers(result.MCP.Servers); err != nil {
			return Options{}, err
		}
	}
	if result.Runtime.Thinking != nil {
		switch result.Runtime.Thinking.Type {
		case ThinkingModeAdaptive, ThinkingModeDisabled:
		case ThinkingModeEnabled:
			if result.Runtime.Thinking.BudgetTokens <= 0 {
				return Options{}, fmt.Errorf("client: thinking enabled requires positive budget tokens")
			}
		default:
			return Options{}, fmt.Errorf("client: unsupported thinking mode %q", result.Runtime.Thinking.Type)
		}
		switch result.Runtime.Thinking.Display {
		case "":
		case ThinkingDisplaySummarized, ThinkingDisplayOmitted:
			if result.Runtime.Thinking.Type == ThinkingModeDisabled {
				return Options{}, fmt.Errorf("client: thinking display cannot be used when thinking is disabled")
			}
		default:
			return Options{}, fmt.Errorf("client: unsupported thinking display %q", result.Runtime.Thinking.Display)
		}
	}
	if result.Transport != nil && result.DirectConnect != nil {
		return Options{}, errTransportDirectConnectConflict
	}
	if result.DirectConnect != nil && strings.TrimSpace(result.DirectConnect.URL) == "" {
		return Options{}, fmt.Errorf("client: direct connect url cannot be empty")
	}
	for _, plugin := range result.Plugins {
		if plugin.Type != PluginTypeLocal {
			return Options{}, fmt.Errorf("client: unsupported plugin type %q", plugin.Type)
		}
		if strings.TrimSpace(plugin.Path) == "" {
			return Options{}, fmt.Errorf("client: plugin path cannot be empty")
		}
	}
	for name, server := range result.MCP.SDKServers {
		if server == nil {
			continue
		}
		if _, exists := result.MCP.Servers[name]; exists {
			continue
		}
		result.MCP.Servers[name] = mcp.SDKServerConfig{
			Name:     name,
			Instance: server,
		}
	}

	return result, nil
}

func (o *Options) resolveRuntimeCommand() error {
	switch normalizedRuntimeKind(o.Runtime.Kind) {
	case RuntimeClaude:
		o.Runtime.Kind = RuntimeClaude
		return nil
	case RuntimeNXS:
		o.Runtime.Kind = RuntimeNXS
		if o.Transport != nil || o.DirectConnect != nil || hasExplicitRuntimeCommand(*o) {
			return nil
		}
		if override := strings.TrimSpace(os.Getenv(nxsCommandPathEnvName)); override != "" {
			o.CLIPath = override
			return nil
		}
		if !nxsRuntimeResolverDisabled() {
			runtimePath, err := nxs.RuntimePath()
			if err != nil {
				return fmt.Errorf("client: resolve nxs runtime failed: %w", err)
			}
			if strings.TrimSpace(runtimePath) == "" {
				return fmt.Errorf("client: resolve nxs runtime failed: empty runtime path")
			}
			o.CLIPath = runtimePath
			return nil
		}
		o.CLIPath = defaultNXSCommandExecutable
		return nil
	default:
		return fmt.Errorf("client: unsupported runtime kind %q", o.Runtime.Kind)
	}
}

func normalizedRuntimeKind(kind RuntimeKind) RuntimeKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case "", string(RuntimeClaude), "claude-code", "claudecode":
		return RuntimeClaude
	case string(RuntimeNXS), "go", "go-native", "gonative":
		return RuntimeNXS
	default:
		return RuntimeKind(strings.TrimSpace(string(kind)))
	}
}

func hasExplicitRuntimeCommand(o Options) bool {
	return strings.TrimSpace(o.CLIPath) != "" ||
		strings.TrimSpace(o.Executable) != "" ||
		strings.TrimSpace(o.PathToExecutable) != ""
}

func nxsRuntimeResolverDisabled() bool {
	if envTruthy(os.Getenv(nxsRuntimeResolverDisabledEnvName)) {
		return true
	}
	return envTruthy(os.Getenv(legacyBundledNXSDisabledEnvName))
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s SkillOptions) normalized() (SkillOptions, error) {
	result := SkillOptions{
		Mode:  s.Mode,
		Names: normalizeSkillNames(s.Names),
	}
	if result.Mode == SkillModeDefault && len(result.Names) > 0 {
		result.Mode = SkillModeOnly
	}
	switch result.Mode {
	case SkillModeDefault:
		result.Names = nil
	case SkillModeAll:
		if len(result.Names) > 0 {
			return SkillOptions{}, fmt.Errorf("client: skills mode all cannot include explicit names")
		}
	case SkillModeOnly:
		if len(result.Names) == 0 {
			result.Mode = SkillModeNone
		}
	case SkillModeNone:
		result.Names = nil
	default:
		return SkillOptions{}, fmt.Errorf("client: unsupported skills mode %q", result.Mode)
	}
	return result, nil
}

func (s SkillOptions) controlValue() *[]string {
	switch s.Mode {
	case SkillModeAll:
		return nil
	case SkillModeOnly:
		names := append([]string(nil), s.Names...)
		return &names
	case SkillModeNone:
		names := []string{}
		return &names
	default:
		return nil
	}
}

func normalizeSkillNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result
}

type resolvedDirectConnectConfig struct {
	URL                  string
	SessionKey           string
	DeleteSessionOnClose bool
	DialTimeout          time.Duration
	HTTPClient           *http.Client
}

type resolvedPluginConfig struct {
	Type string
	Path string
}

type resolvedSystemPromptPreset struct {
	Append string
}

type resolvedToolsPreset struct {
	Preset string
}

type resolvedThinkingConfig struct {
	Type         string
	BudgetTokens int
	Display      string
}

type resolvedOutputFormat struct {
	Type   string
	Schema map[string]any
}

type resolvedTaskBudget struct {
	Total int
}

type resolvedOptions struct {
	RuntimeKind                     RuntimeKind
	CommandPath                     string
	Executable                      string
	ExecutableArgs                  []string
	PathToExecutable                string
	CWD                             string
	User                            string
	MaxBufferSize                   int
	SystemPrompt                    string
	SystemPromptFile                string
	SystemPromptPreset              *resolvedSystemPromptPreset
	AppendSystemPrompt              string
	Tools                           []string
	ToolsPreset                     *resolvedToolsPreset
	AllowedTools                    []string
	DisallowedTools                 []string
	Skills                          SkillOptions
	PermissionMode                  permission.Mode
	AllowDangerouslySkipPermissions bool
	PermissionPromptToolName        string
	ContinueConversation            bool
	Resume                          string
	SessionID                       string
	ResumeSessionAt                 string
	SessionTitle                    string
	Agent                           string
	Agents                          map[string]AgentDefinition
	Model                           string
	FallbackModel                   string
	Betas                           []string
	MaxTurns                        int
	MaxBudgetUSD                    float64
	TaskBudget                      *resolvedTaskBudget
	PersistSession                  *bool
	Settings                        string
	SettingsObject                  map[string]any
	Sandbox                         *SandboxSettings
	SettingSources                  []string
	ToolConfig                      *ToolConfig
	OutputFormat                    *resolvedOutputFormat
	PromptSuggestions               *bool
	AgentProgressSummaries          *bool
	ExcludeDynamicSections          *bool
	AdditionalDirectories           []string
	MCPConfig                       string
	MCPSerialized                   map[string]any
	StrictMCPConfig                 bool
	Plugins                         []resolvedPluginConfig
	MaxThinkingTokens               int
	Thinking                        *resolvedThinkingConfig
	Effort                          string
	EnableFileCheckpointing         bool
	DirectConnect                   *resolvedDirectConnectConfig
	Env                             map[string]string
	ExtraArgs                       map[string]string
	ExtraBoolArgs                   []string
	IncludeHookEvents               bool
	IncludePartialMessages          bool
	ForkSession                     bool
	PermissionHandler               permission.Handler
	ElicitationHandler              ElicitationHandler
	UserDialogHandler               UserDialogHandler
	OAuthTokenHandler               OAuthTokenHandler
	Hooks                           map[hook.Event][]hook.Matcher
	Stderr                          func(string)
	Diagnostics                     DiagnosticHandler
	InitializeTimeout               time.Duration
	Debug                           bool
	DebugFile                       string
}

func (o resolvedOptions) OutputSchema() map[string]any {
	if o.OutputFormat != nil && len(o.OutputFormat.Schema) > 0 {
		return o.OutputFormat.Schema
	}
	return nil
}

func (o resolvedOptions) AllowsDangerouslySkipPermissions() bool {
	if o.AllowDangerouslySkipPermissions || o.PermissionMode == permission.ModeBypassPermissions {
		return true
	}
	for _, key := range o.ExtraBoolArgs {
		normalized := strings.TrimPrefix(strings.TrimSpace(key), "--")
		if normalized == "allow-dangerously-skip-permissions" || normalized == "dangerously-skip-permissions" {
			return true
		}
	}
	return false
}
func (o Options) directConnectConfig() (transport.DirectConnectConfig, error) {
	return buildDirectConnectTransportConfig(o.resolveOptionsLenient())
}

func (o Options) processConfig() transport.ProcessConfig {
	return buildProcessTransportConfig(o.resolveOptionsLenient())
}

func (o Options) buildArgs() []string {
	return buildProcessTransportArgs(o.resolveOptionsLenient())
}

func (o Options) allowsDangerouslySkipPermissions() bool {
	if o.Runtime.AllowDangerouslySkipPermissions || o.Runtime.PermissionMode == permission.ModeBypassPermissions {
		return true
	}
	for _, key := range o.ExtraBoolArgs {
		normalized := strings.TrimPrefix(strings.TrimSpace(key), "--")
		if normalized == "allow-dangerously-skip-permissions" || normalized == "dangerously-skip-permissions" {
			return true
		}
	}
	return false
}

func (o Options) outputSchema() map[string]any {
	if o.OutputFormat != nil && len(o.OutputFormat.Schema) > 0 {
		return o.OutputFormat.Schema
	}
	return nil
}

func (o Options) resolveOptions() (resolvedOptions, error) {
	return o.buildResolvedOptions(true)
}

func (o Options) resolveOptionsLenient() resolvedOptions {
	config, _ := o.buildResolvedOptions(false)
	return config
}

func (o Options) buildResolvedOptions(strictMCP bool) (resolvedOptions, error) {
	normalizedOptions, err := o.normalized()
	if err != nil {
		return resolvedOptions{}, err
	}
	o = normalizedOptions
	serializedServers, _, err := mcpwire.SerializeServers(o.resolvedMCPServers())
	if err != nil && strictMCP {
		return resolvedOptions{}, err
	}
	if err != nil {
		serializedServers = map[string]any{}
	}

	var systemPromptPreset *resolvedSystemPromptPreset
	if o.System.Preset != nil {
		systemPromptPreset = &resolvedSystemPromptPreset{
			Append: o.System.Preset.Append,
		}
	}
	var toolsPreset *resolvedToolsPreset
	if o.Tools.Preset != nil {
		toolsPreset = &resolvedToolsPreset{Preset: o.Tools.Preset.Preset}
	}
	var taskBudget *resolvedTaskBudget
	if o.Runtime.TaskBudget != nil {
		taskBudget = &resolvedTaskBudget{Total: o.Runtime.TaskBudget.Total}
	}
	var thinking *resolvedThinkingConfig
	if o.Runtime.Thinking != nil {
		thinking = &resolvedThinkingConfig{
			Type:         string(o.Runtime.Thinking.Type),
			BudgetTokens: o.Runtime.Thinking.BudgetTokens,
			Display:      string(o.Runtime.Thinking.Display),
		}
	}
	var outputFormat *resolvedOutputFormat
	if o.OutputFormat != nil {
		outputFormat = &resolvedOutputFormat{
			Type:   string(o.OutputFormat.Type),
			Schema: cloneAnyMap(o.OutputFormat.Schema),
		}
	}
	var directConnect *resolvedDirectConnectConfig
	if o.DirectConnect != nil {
		directConnect = &resolvedDirectConnectConfig{
			URL:                  o.DirectConnect.URL,
			SessionKey:           o.DirectConnect.SessionKey,
			DeleteSessionOnClose: o.DirectConnect.DeleteSessionOnClose,
			DialTimeout:          o.DirectConnect.DialTimeout,
			HTTPClient:           o.DirectConnect.HTTPClient,
		}
	}

	plugins := make([]resolvedPluginConfig, 0, len(o.Plugins))
	for _, plugin := range o.Plugins {
		plugins = append(plugins, resolvedPluginConfig{
			Type: string(plugin.Type),
			Path: plugin.Path,
		})
	}

	skills, err := o.Skills.normalized()
	if err != nil {
		return resolvedOptions{}, err
	}

	return resolvedOptions{
		RuntimeKind:                     o.Runtime.Kind,
		CommandPath:                     o.CLIPath,
		Executable:                      o.Executable,
		ExecutableArgs:                  append([]string(nil), o.ExecutableArgs...),
		PathToExecutable:                o.PathToExecutable,
		CWD:                             o.CWD,
		User:                            o.User,
		MaxBufferSize:                   o.MaxBufferSize,
		SystemPrompt:                    o.System.Text,
		SystemPromptFile:                o.System.File,
		SystemPromptPreset:              systemPromptPreset,
		AppendSystemPrompt:              o.System.Append,
		Tools:                           append([]string(nil), o.Tools.Available...),
		ToolsPreset:                     toolsPreset,
		AllowedTools:                    append([]string(nil), o.Tools.Allow...),
		DisallowedTools:                 append([]string(nil), o.Tools.Deny...),
		Skills:                          skills,
		PermissionMode:                  o.Runtime.PermissionMode,
		AllowDangerouslySkipPermissions: o.Runtime.AllowDangerouslySkipPermissions,
		PermissionPromptToolName:        o.Runtime.PermissionPromptToolName,
		ContinueConversation:            o.Session.ContinueLatest,
		Resume:                          o.Session.ResumeID,
		SessionID:                       o.Session.ID,
		ResumeSessionAt:                 o.Session.ResumeAt,
		SessionTitle:                    o.Session.Title,
		Agent:                           o.Agent,
		Agents:                          cloneAgentDefinitions(o.Agents),
		Model:                           o.Model,
		FallbackModel:                   o.FallbackModel,
		Betas:                           append([]string(nil), o.Betas...),
		MaxTurns:                        o.Runtime.MaxTurns,
		MaxBudgetUSD:                    o.Runtime.MaxBudgetUSD,
		TaskBudget:                      taskBudget,
		PersistSession:                  cloneBoolPointer(o.Session.Persist),
		Settings:                        o.Settings,
		SettingsObject:                  jsonvalue.CloneMapPreserveTypedSlices(o.SettingsObject),
		Sandbox:                         cloneSandboxSettings(o.Sandbox),
		SettingSources:                  append([]string(nil), o.SettingSources...),
		ToolConfig:                      cloneToolConfig(o.ToolConfig),
		OutputFormat:                    outputFormat,
		PromptSuggestions:               cloneBoolPointer(o.System.PromptSuggestions),
		AgentProgressSummaries:          cloneBoolPointer(o.System.AgentProgressSummaries),
		ExcludeDynamicSections:          cloneBoolPointer(o.System.ExcludeDynamicSections),
		AdditionalDirectories:           append([]string(nil), o.AdditionalDirectories...),
		MCPConfig:                       o.MCP.Config,
		MCPSerialized:                   serializedServers,
		StrictMCPConfig:                 o.MCP.StrictConfig,
		Plugins:                         plugins,
		MaxThinkingTokens:               o.Runtime.MaxThinkingTokens,
		Thinking:                        thinking,
		Effort:                          string(o.Runtime.Effort),
		EnableFileCheckpointing:         o.Runtime.EnableFileCheckpointing,
		DirectConnect:                   directConnect,
		Env:                             cloneStringMap(o.Env),
		ExtraArgs:                       cloneStringMap(o.ExtraArgs),
		ExtraBoolArgs:                   append([]string(nil), o.ExtraBoolArgs...),
		IncludeHookEvents:               o.Hooks.IncludeEvents,
		IncludePartialMessages:          o.IncludePartialMessages,
		ForkSession:                     o.Session.Fork,
		PermissionHandler:               o.Callbacks.PermissionHandler,
		ElicitationHandler:              o.Callbacks.ElicitationHandler,
		UserDialogHandler:               o.Callbacks.UserDialogHandler,
		OAuthTokenHandler:               o.Callbacks.OAuthTokenHandler,
		Hooks:                           cloneHooks(o.Hooks.Matchers),
		Stderr:                          o.Callbacks.Stderr,
		Diagnostics:                     o.Callbacks.Diagnostics,
		InitializeTimeout:               o.Runtime.InitializeTimeout,
		Debug:                           o.Runtime.Debug,
		DebugFile:                       o.Runtime.DebugFile,
	}, nil
}
