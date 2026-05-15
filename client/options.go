package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	sdktools "github.com/nexus-research-lab/nexus-agent-sdk-bridge/tools"
)

// ElicitationHandler 负责处理 MCP elicitation 请求。
type ElicitationHandler func(context.Context, protocol.ElicitationRequest) (protocol.ElicitationResponse, error)

// UserDialogHandler 负责处理后端发起的阻塞式用户对话请求。
type UserDialogHandler func(context.Context, protocol.UserDialogRequest) (protocol.UserDialogResponse, error)

// OAuthTokenHandler 负责按需刷新 Nexus OAuth access token。
type OAuthTokenHandler func(context.Context) (string, error)

// DiagnosticEvent 表示 SDK backend 产生的一条诊断事件。
type DiagnosticEvent struct {
	Component  string
	Event      string
	Attributes map[string]any
}

// DiagnosticHandler 接收 SDK 内部诊断事件。
type DiagnosticHandler func(DiagnosticEvent)

// PluginType 表示 SDK 显式加载的插件来源类型。
type PluginType string

const (
	// PluginTypeLocal 表示从本地目录加载插件。
	PluginTypeLocal PluginType = "local"
)

// PluginConfig 表示 SDK 显式加载的本地插件配置。
type PluginConfig struct {
	Type PluginType
	Path string
}

const (
	defaultInitializeTimeout = time.Minute
	diagnosticsEnv           = "NEXUS_AGENT_SDK_DIAGNOSTICS"
	debugEnv                 = "NEXUS_AGENT_SDK_DEBUG"
	streamCloseTimeoutEnv    = "NEXUS_AGENT_SDK_STREAM_CLOSE_TIMEOUT"
)

// SystemPromptPreset 表示内置 preset system prompt 配置。
type SystemPromptPreset struct {
	Preset                 string
	Append                 string
	ExcludeDynamicSections *bool
}

// SystemOptions 表示系统提示词与初始化展示偏好。
type SystemOptions struct {
	Text                   string
	File                   string
	Preset                 *SystemPromptPreset
	Append                 string
	PromptSuggestions      *bool
	AgentProgressSummaries *bool
	ExcludeDynamicSections *bool
}

// ToolsPreset 表示内置 tools preset 配置。
type ToolsPreset struct {
	Preset string
}

// ThinkingMode 表示思考模式。
type ThinkingMode string

const (
	// ThinkingModeAdaptive 表示自适应思考。
	ThinkingModeAdaptive ThinkingMode = "adaptive"
	// ThinkingModeEnabled 表示启用思考预算。
	ThinkingModeEnabled ThinkingMode = "enabled"
	// ThinkingModeDisabled 表示关闭思考。
	ThinkingModeDisabled ThinkingMode = "disabled"
)

// ThinkingDisplay 表示是否输出 thinking 文本。
type ThinkingDisplay string

const (
	// ThinkingDisplaySummarized 表示输出摘要版 thinking 文本。
	ThinkingDisplaySummarized ThinkingDisplay = "summarized"
	// ThinkingDisplayOmitted 表示省略 thinking 文本，只保留签名。
	ThinkingDisplayOmitted ThinkingDisplay = "omitted"
)

// ThinkingConfig 表示思考参数。
type ThinkingConfig struct {
	Type         ThinkingMode
	BudgetTokens int
	Display      ThinkingDisplay
}

// EffortLevel 表示思考深度。
type EffortLevel string

const (
	// EffortLevelLow 表示低推理强度。
	EffortLevelLow EffortLevel = "low"
	// EffortLevelMedium 表示中等推理强度。
	EffortLevelMedium EffortLevel = "medium"
	// EffortLevelHigh 表示高推理强度。
	EffortLevelHigh EffortLevel = "high"
	// EffortLevelXHigh 表示扩展高推理强度。
	EffortLevelXHigh EffortLevel = "xhigh"
	// EffortLevelMax 表示最大推理强度。
	EffortLevelMax EffortLevel = "max"
)

// OutputFormatType 表示结构化输出类型。
type OutputFormatType string

const (
	// OutputFormatTypeJSONSchema 表示 JSON Schema 输出。
	OutputFormatTypeJSONSchema OutputFormatType = "json_schema"
)

// OutputFormat 表示结构化输出格式。
type OutputFormat struct {
	Type   OutputFormatType
	Schema map[string]any
}

// TaskBudget 表示 API 侧任务预算。
type TaskBudget struct {
	Total int
}

// ToolOptions 表示 agent 可见工具和权限规则配置。
type ToolOptions struct {
	Available []string
	Preset    *ToolsPreset
	Allow     []string
	Deny      []string
}

// SkillMode 表示主会话 skill 选择模式。
type SkillMode string

const (
	// SkillModeDefault 表示使用后端默认 skill 发现策略。
	SkillModeDefault SkillMode = ""
	// SkillModeAll 表示允许本会话使用所有已发现 skills。
	SkillModeAll SkillMode = "all"
	// SkillModeOnly 表示只允许本会话使用 Names 中列出的 skills。
	SkillModeOnly SkillMode = "only"
	// SkillModeNone 表示关闭本会话 skills。
	SkillModeNone SkillMode = "none"
)

// SkillOptions 表示主会话 Skill 工具可见的 skill 集合。
type SkillOptions struct {
	Mode  SkillMode
	Names []string
}

// SessionOptions 表示 transcript 恢复、继续、fork 与持久化配置。
type SessionOptions struct {
	ContinueLatest bool
	ResumeID       string
	Persist        *bool
	Fork           bool
}

// RuntimeOptions 表示运行期权限、预算、thinking 与 checkpoint 配置。
type RuntimeOptions struct {
	PermissionMode                  permission.Mode
	AllowDangerouslySkipPermissions bool
	PermissionPromptToolName        string
	MaxTurns                        int
	MaxBudgetUSD                    float64
	TaskBudget                      *TaskBudget
	MaxThinkingTokens               int
	Thinking                        *ThinkingConfig
	Effort                          EffortLevel
	EnableFileCheckpointing         bool
	InitializeTimeout               time.Duration
}

// MCPOptions 表示外部 MCP 配置与进程内 SDK MCP server。
type MCPOptions struct {
	Servers      map[string]mcp.ServerConfig
	Config       string
	SDKServers   map[string]mcp.SDKMCPServer
	StrictConfig bool
}

// HookOptions 表示 hook matcher 配置与 hook 事件输出开关。
type HookOptions struct {
	Matchers      map[hook.Event][]hook.Matcher
	IncludeEvents bool
}

// CallbackOptions 表示宿主注入的回调和外部运行边界。
type CallbackOptions struct {
	PermissionHandler  permission.Handler
	ElicitationHandler ElicitationHandler
	UserDialogHandler  UserDialogHandler
	OAuthTokenHandler  OAuthTokenHandler
	Stderr             func(string)
	Diagnostics        DiagnosticHandler
}

// Options 表示 Nexus Agent SDK Go 客户端选项。
type Options struct {
	Backend                Backend
	CWD                    string
	User                   string
	MaxBufferSize          int
	System                 SystemOptions
	Tools                  ToolOptions
	Skills                 SkillOptions
	Session                SessionOptions
	Runtime                RuntimeOptions
	Agent                  string
	Agents                 map[string]protocol.AgentDefinition
	Model                  string
	FallbackModel          string
	Betas                  []string
	Settings               string
	SettingSources         []string
	OutputFormat           *OutputFormat
	AdditionalDirectories  []string
	MCP                    MCPOptions
	Plugins                []PluginConfig
	Env                    map[string]string
	ExtraArgs              map[string]string
	ExtraBoolArgs          []string
	IncludePartialMessages bool
	Hooks                  HookOptions
	Callbacks              CallbackOptions
	commandPath            string
	directConnect          *DirectConnectOptions
	usesExternalBackend    bool
}

// NewOptions 创建一份可链式构造的空选项。
func NewOptions() Options {
	return Options{}
}

// WithBackend 设置可选执行后端；默认不设置时使用本地 process bridge。
func (o Options) WithBackend(backend Backend) Options {
	o.Backend = backend
	return o
}

// WithCWD 设置工作目录。
func (o Options) WithCWD(cwd string) Options {
	o.CWD = cwd
	return o
}

// WithUser 设置运行用户。
func (o Options) WithUser(user string) Options {
	o.User = user
	return o
}

// WithMaxBufferSize 设置最大缓冲区大小。
func (o Options) WithMaxBufferSize(size int) Options {
	o.MaxBufferSize = size
	return o
}

// WithSystemPrompt 设置系统提示词。
func (o Options) WithSystemPrompt(prompt string) Options {
	o.System.Text = prompt
	return o
}

// WithSystemPromptFile 设置系统提示词文件。
func (o Options) WithSystemPromptFile(path string) Options {
	o.System.File = path
	return o
}

// WithSystemPromptPreset 设置系统提示词预设。
func (o Options) WithSystemPromptPreset(preset SystemPromptPreset) Options {
	o.System.Preset = &preset
	return o
}

// WithAppendSystemPrompt 追加系统提示词。
func (o Options) WithAppendSystemPrompt(prompt string) Options {
	o.System.Append = prompt
	return o
}

// WithPromptSuggestions 设置是否启用提示建议。
func (o Options) WithPromptSuggestions(enabled bool) Options {
	o.System.PromptSuggestions = pointerTo(enabled)
	return o
}

// WithExcludeDynamicSections 设置是否排除动态系统提示片段。
func (o Options) WithExcludeDynamicSections(enabled bool) Options {
	o.System.ExcludeDynamicSections = pointerTo(enabled)
	return o
}

// WithTools 设置 agent 可见工具集合。
func (o Options) WithTools(tools ...string) Options {
	o.Tools.Available = append([]string(nil), tools...)
	return o
}

// WithToolsPreset 设置工具预设。
func (o Options) WithToolsPreset(preset ToolsPreset) Options {
	o.Tools.Preset = &preset
	return o
}

// WithAllowedTools 设置权限允许规则。
func (o Options) WithAllowedTools(tools ...string) Options {
	o.Tools.Allow = append([]string(nil), tools...)
	return o
}

// AddAllowedTool 追加权限允许规则。
func (o Options) AddAllowedTool(tool string) Options {
	o.Tools.Allow = append(append([]string(nil), o.Tools.Allow...), tool)
	return o
}

// WithDisallowedTools 设置权限禁止规则。
func (o Options) WithDisallowedTools(tools ...string) Options {
	o.Tools.Deny = append([]string(nil), tools...)
	return o
}

// AddDisallowedTool 追加权限禁止规则。
func (o Options) AddDisallowedTool(tool string) Options {
	o.Tools.Deny = append(append([]string(nil), o.Tools.Deny...), tool)
	return o
}

// WithSkills 只允许本会话使用指定 skills；不传名称表示关闭 skills。
func (o Options) WithSkills(names ...string) Options {
	o.Skills = SkillOptions{
		Mode:  SkillModeOnly,
		Names: normalizeSkillNames(names),
	}
	if len(o.Skills.Names) == 0 {
		o.Skills.Mode = SkillModeNone
	}
	return o
}

// WithAllSkills 允许本会话使用所有已发现 skills。
func (o Options) WithAllSkills() Options {
	o.Skills = SkillOptions{Mode: SkillModeAll}
	return o
}

// WithoutSkills 关闭本会话 skills。
func (o Options) WithoutSkills() Options {
	o.Skills = SkillOptions{Mode: SkillModeNone}
	return o
}

// WithPermissionMode 设置权限模式。
func (o Options) WithPermissionMode(mode permission.Mode) Options {
	o.Runtime.PermissionMode = mode
	return o
}

// WithAllowDangerouslySkipPermissions 允许会话在运行期切换到 bypassPermissions。
func (o Options) WithAllowDangerouslySkipPermissions(enabled bool) Options {
	o.Runtime.AllowDangerouslySkipPermissions = enabled
	return o
}

// WithPermissionPromptToolName 设置权限提示工具。
func (o Options) WithPermissionPromptToolName(name string) Options {
	o.Runtime.PermissionPromptToolName = name
	return o
}

// WithContinueConversation 设置是否继续会话。
func (o Options) WithContinueConversation(enabled bool) Options {
	o.Session.ContinueLatest = enabled
	return o
}

// WithResume 设置恢复会话 ID。
func (o Options) WithResume(sessionID string) Options {
	o.Session.ResumeID = sessionID
	return o
}

// WithForkSession 设置是否 fork 会话。
func (o Options) WithForkSession(enabled bool) Options {
	o.Session.Fork = enabled
	return o
}

// WithPersistSession 设置是否持久化会话。
func (o Options) WithPersistSession(enabled bool) Options {
	o.Session.Persist = pointerTo(enabled)
	return o
}

// WithAgent 设置 agent 名称。
func (o Options) WithAgent(agent string) Options {
	o.Agent = agent
	return o
}

// WithAgents 批量设置 agent 定义。
func (o Options) WithAgents(agents map[string]protocol.AgentDefinition) Options {
	o.Agents = cloneAgentDefinitions(agents)
	return o
}

// WithAgentDefinition 添加或替换单个 agent 定义。
func (o Options) WithAgentDefinition(name string, definition protocol.AgentDefinition) Options {
	o.Agents = cloneAgentDefinitions(o.Agents)
	if o.Agents == nil {
		o.Agents = map[string]protocol.AgentDefinition{}
	}
	o.Agents[name] = definition
	return o
}

// WithAgentProgressSummaries 设置是否启用 agent 进度摘要。
func (o Options) WithAgentProgressSummaries(enabled bool) Options {
	o.System.AgentProgressSummaries = pointerTo(enabled)
	return o
}

// WithModel 设置主模型。
func (o Options) WithModel(model string) Options {
	o.Model = model
	return o
}

// WithFallbackModel 设置回退模型。
func (o Options) WithFallbackModel(model string) Options {
	o.FallbackModel = model
	return o
}

// WithBetas 设置 beta 开关。
func (o Options) WithBetas(betas ...string) Options {
	o.Betas = append([]string(nil), betas...)
	return o
}

// AddBeta 追加 beta 开关。
func (o Options) AddBeta(beta string) Options {
	o.Betas = append(append([]string(nil), o.Betas...), beta)
	return o
}

// WithMaxTurns 设置最大轮次。
func (o Options) WithMaxTurns(turns int) Options {
	o.Runtime.MaxTurns = turns
	return o
}

// WithMaxBudgetUSD 设置最大预算。
func (o Options) WithMaxBudgetUSD(value float64) Options {
	o.Runtime.MaxBudgetUSD = value
	return o
}

// WithTaskBudget 设置任务预算。
func (o Options) WithTaskBudget(total int) Options {
	o.Runtime.TaskBudget = &TaskBudget{Total: total}
	return o
}

// WithOutputFormat 设置输出格式。
func (o Options) WithOutputFormat(format *OutputFormat) Options {
	o.OutputFormat = cloneOutputFormat(format)
	return o
}

// WithMaxThinkingTokens 设置最大思考 token。
func (o Options) WithMaxThinkingTokens(tokens int) Options {
	o.Runtime.MaxThinkingTokens = tokens
	return o
}

// WithThinking 设置思考模式。
func (o Options) WithThinking(thinking *ThinkingConfig) Options {
	o.Runtime.Thinking = cloneThinkingConfig(thinking)
	return o
}

// WithEffort 设置推理强度。
func (o Options) WithEffort(effort EffortLevel) Options {
	o.Runtime.Effort = effort
	return o
}

// WithEnableFileCheckpointing 设置是否启用文件检查点。
func (o Options) WithEnableFileCheckpointing(enabled bool) Options {
	o.Runtime.EnableFileCheckpointing = enabled
	return o
}

// WithSettings 设置 settings 字符串或路径。
func (o Options) WithSettings(settings string) Options {
	o.Settings = settings
	return o
}

// WithSettingSources 设置 setting source 列表。
func (o Options) WithSettingSources(sources ...string) Options {
	o.SettingSources = append([]string(nil), sources...)
	return o
}

// AddSettingSource 追加 setting source。
func (o Options) AddSettingSource(source string) Options {
	o.SettingSources = append(append([]string(nil), o.SettingSources...), source)
	return o
}

// WithAdditionalDirectories 设置附加目录。
func (o Options) WithAdditionalDirectories(directories ...string) Options {
	o.AdditionalDirectories = append([]string(nil), directories...)
	return o
}

// AddDirectory 追加附加目录。
func (o Options) AddDirectory(directory string) Options {
	o.AdditionalDirectories = append(append([]string(nil), o.AdditionalDirectories...), directory)
	return o
}

// WithMCPServers 批量设置 MCP 服务。
func (o Options) WithMCPServers(servers map[string]mcp.ServerConfig) Options {
	o.MCP.Servers = cloneMCPServerConfigs(servers)
	return o
}

// WithMCPServer 添加或替换单个 MCP 服务。
func (o Options) WithMCPServer(name string, config mcp.ServerConfig) Options {
	o.MCP.Servers = cloneMCPServerConfigs(o.MCP.Servers)
	if o.MCP.Servers == nil {
		o.MCP.Servers = map[string]mcp.ServerConfig{}
	}
	o.MCP.Servers[name] = config
	return o
}

// WithMCPConfig 设置 MCP 配置文件或 JSON。
func (o Options) WithMCPConfig(config string) Options {
	o.MCP.Config = config
	return o
}

// WithStrictMCPConfig 设置是否只使用显式 MCP 配置。
func (o Options) WithStrictMCPConfig(enabled bool) Options {
	o.MCP.StrictConfig = enabled
	return o
}

// WithPlugins 批量设置插件。
func (o Options) WithPlugins(items ...PluginConfig) Options {
	o.Plugins = append([]PluginConfig(nil), items...)
	return o
}

// AddPlugin 追加插件。
func (o Options) AddPlugin(plugin PluginConfig) Options {
	o.Plugins = append(append([]PluginConfig(nil), o.Plugins...), plugin)
	return o
}

// WithEnv 批量设置环境变量。
func (o Options) WithEnv(env map[string]string) Options {
	o.Env = cloneStringMap(env)
	return o
}

// AddEnv 追加环境变量。
func (o Options) AddEnv(key string, value string) Options {
	o.Env = cloneStringMap(o.Env)
	if o.Env == nil {
		o.Env = map[string]string{}
	}
	o.Env[key] = value
	return o
}

// WithExtraArgs 批量设置额外参数。
func (o Options) WithExtraArgs(args map[string]string) Options {
	o.ExtraArgs = cloneStringMap(args)
	return o
}

// AddExtraArg 追加额外参数。
func (o Options) AddExtraArg(key string, value string) Options {
	o.ExtraArgs = cloneStringMap(o.ExtraArgs)
	if o.ExtraArgs == nil {
		o.ExtraArgs = map[string]string{}
	}
	o.ExtraArgs[key] = value
	return o
}

// WithExtraBoolArgs 批量设置布尔额外参数。
func (o Options) WithExtraBoolArgs(args ...string) Options {
	o.ExtraBoolArgs = append([]string(nil), args...)
	return o
}

// AddExtraBoolArg 追加布尔额外参数。
func (o Options) AddExtraBoolArg(arg string) Options {
	o.ExtraBoolArgs = append(append([]string(nil), o.ExtraBoolArgs...), arg)
	return o
}

// WithIncludeHookEvents 设置是否包含 hook 事件。
func (o Options) WithIncludeHookEvents(enabled bool) Options {
	o.Hooks.IncludeEvents = enabled
	return o
}

// WithIncludePartialMessages 设置是否包含 partial messages。
func (o Options) WithIncludePartialMessages(enabled bool) Options {
	o.IncludePartialMessages = enabled
	return o
}

// WithPermissionHandler 设置权限处理器。
func (o Options) WithPermissionHandler(handler permission.Handler) Options {
	o.Callbacks.PermissionHandler = handler
	return o
}

// WithElicitationHandler 设置 elicitation 处理器。
func (o Options) WithElicitationHandler(handler ElicitationHandler) Options {
	o.Callbacks.ElicitationHandler = handler
	return o
}

// WithUserDialogHandler 设置用户对话处理器。
func (o Options) WithUserDialogHandler(handler UserDialogHandler) Options {
	o.Callbacks.UserDialogHandler = handler
	return o
}

// WithOAuthTokenHandler 设置 OAuth token 处理器。
func (o Options) WithOAuthTokenHandler(handler OAuthTokenHandler) Options {
	o.Callbacks.OAuthTokenHandler = handler
	return o
}

// WithHooks 批量设置 hooks。
func (o Options) WithHooks(hooks map[hook.Event][]hook.Matcher) Options {
	o.Hooks.Matchers = cloneHooks(hooks)
	return o
}

// AddHookMatcher 为指定事件追加 hook matcher。
func (o Options) AddHookMatcher(event hook.Event, matcher hook.Matcher) Options {
	o.Hooks.Matchers = cloneHooks(o.Hooks.Matchers)
	if o.Hooks.Matchers == nil {
		o.Hooks.Matchers = map[hook.Event][]hook.Matcher{}
	}
	o.Hooks.Matchers[event] = append(append([]hook.Matcher(nil), o.Hooks.Matchers[event]...), matcher)
	return o
}

// WithSDKMCPServers 批量设置 SDK MCP 服务。
func (o Options) WithSDKMCPServers(servers map[string]mcp.SDKMCPServer) Options {
	o.MCP.SDKServers = cloneSDKMCPServers(servers)
	return o
}

// WithSDKMCPServer 添加或替换单个 SDK MCP 服务。
func (o Options) WithSDKMCPServer(name string, server mcp.SDKMCPServer) Options {
	o.MCP.SDKServers = cloneSDKMCPServers(o.MCP.SDKServers)
	if o.MCP.SDKServers == nil {
		o.MCP.SDKServers = map[string]mcp.SDKMCPServer{}
	}
	o.MCP.SDKServers[name] = server
	return o
}

// WithCustomTools registers Go-native custom tools on an in-process SDK MCP server.
func (o Options) WithCustomTools(serverName string, customTools ...sdktools.Tool) Options {
	name := strings.TrimSpace(serverName)
	if name == "" {
		name = "custom"
	}
	server := sdktools.CreateSDKMCPServer(sdktools.SDKMCPServerOptions{
		Name:  name,
		Tools: append([]sdktools.Tool(nil), customTools...),
	})
	return o.WithSDKMCPServer(name, server)
}

// WithStderr 设置 stderr 回调。
func (o Options) WithStderr(callback func(string)) Options {
	o.Callbacks.Stderr = callback
	return o
}

// WithDiagnostics 设置 SDK 内部诊断事件回调。
func (o Options) WithDiagnostics(callback DiagnosticHandler) Options {
	o.Callbacks.Diagnostics = callback
	return o
}

// WithInitializeTimeout 设置初始化超时。
func (o Options) WithInitializeTimeout(timeout time.Duration) Options {
	o.Runtime.InitializeTimeout = timeout
	return o
}
func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	return jsonvalue.CloneStringMap(input)
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	return jsonvalue.CloneMap(input)
}

func cloneAgentDefinitions(input map[string]protocol.AgentDefinition) map[string]protocol.AgentDefinition {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]protocol.AgentDefinition, len(input))
	for key, value := range input {
		result[key] = value.Clone()
	}
	return result
}

func cloneMCPServerConfigs(input map[string]mcp.ServerConfig) map[string]mcp.ServerConfig {
	return mcp.CloneServerConfigs(input)
}

func cloneSDKMCPServers(input map[string]mcp.SDKMCPServer) map[string]mcp.SDKMCPServer {
	return mcp.CloneSDKServers(input)
}

func cloneHooks(input map[hook.Event][]hook.Matcher) map[hook.Event][]hook.Matcher {
	if len(input) == 0 {
		return nil
	}
	result := make(map[hook.Event][]hook.Matcher, len(input))
	for key, value := range input {
		result[key] = append([]hook.Matcher(nil), value...)
	}
	return result
}

func cloneThinkingConfig(input *ThinkingConfig) *ThinkingConfig {
	if input == nil {
		return nil
	}
	result := *input
	return &result
}

func cloneOutputFormat(input *OutputFormat) *OutputFormat {
	if input == nil {
		return nil
	}
	result := *input
	result.Schema = cloneAnyMap(input.Schema)
	return &result
}

func cloneDirectConnectOptions(input *DirectConnectOptions) *DirectConnectOptions {
	if input == nil {
		return nil
	}
	result := *input
	return &result
}

func cloneBoolPointer(input *bool) *bool {
	if input == nil {
		return nil
	}
	result := *input
	return &result
}
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
func (o Options) normalized() (Options, error) {
	result := o
	if result.Runtime.InitializeTimeout <= 0 {
		result.Runtime.InitializeTimeout = defaultInitializeTimeoutFromEnv()
	}
	if result.Callbacks.Diagnostics == nil && diagnosticsToStderrEnabled() {
		result.Callbacks.Diagnostics = stderrDiagnosticHandler
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
		result.Agents = map[string]protocol.AgentDefinition{}
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
	if len(result.MCP.Servers) > 0 {
		if _, _, err := mcp.SerializeServers(result.MCP.Servers); err != nil {
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
	if result.directConnect != nil && strings.TrimSpace(result.directConnect.URL) == "" {
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
	CommandPath                     string
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
	Agent                           string
	Agents                          map[string]protocol.AgentDefinition
	Model                           string
	FallbackModel                   string
	Betas                           []string
	MaxTurns                        int
	MaxBudgetUSD                    float64
	TaskBudget                      *resolvedTaskBudget
	PersistSession                  *bool
	Settings                        string
	SettingSources                  []string
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
	return buildDirectConnectBackendConfig(o.resolveOptionsLenient())
}

func (o Options) processConfig() transport.ProcessConfig {
	return buildProcessBackendConfig(o.resolveOptionsLenient())
}

func (o Options) buildArgs() []string {
	return buildProcessBackendArgs(o.resolveOptionsLenient())
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
	serializedServers, _, err := mcp.SerializeServers(o.resolvedMCPServers())
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
	if o.directConnect != nil {
		directConnect = &resolvedDirectConnectConfig{
			URL:                  o.directConnect.URL,
			SessionKey:           o.directConnect.SessionKey,
			DeleteSessionOnClose: o.directConnect.DeleteSessionOnClose,
			DialTimeout:          o.directConnect.DialTimeout,
			HTTPClient:           o.directConnect.HTTPClient,
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
		CommandPath:                     o.commandPath,
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
		SettingSources:                  append([]string(nil), o.SettingSources...),
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
	}, nil
}
