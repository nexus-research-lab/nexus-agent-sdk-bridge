package client

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdktools "github.com/nexus-research-lab/nexus-agent-sdk-bridge/tools"
)

// WithCLIPath 设置本地 CLI 可执行文件路径。
func (o Options) WithCLIPath(path string) Options {
	o.CLIPath = path
	return o
}

// WithPathToClaudeCodeExecutable 设置 Claude Code 可执行文件路径。
func (o Options) WithPathToClaudeCodeExecutable(path string) Options {
	o.PathToExecutable = path
	o.CLIPath = path
	return o
}

// WithExecutable 设置用于启动 Claude Code 脚本的运行时可执行文件。
func (o Options) WithExecutable(executable string) Options {
	o.Executable = executable
	return o
}

// WithExecutableArgs 设置传给运行时可执行文件的参数。
func (o Options) WithExecutableArgs(args ...string) Options {
	o.ExecutableArgs = append([]string(nil), args...)
	return o
}

// WithRuntime 设置 bridge 启动的本地 Agent runtime。
func (o Options) WithRuntime(kind RuntimeKind) Options {
	o.Runtime.Kind = kind
	return o
}

// WithTransport 注入自定义 SDK 通信 transport。
func (o Options) WithTransport(transport Transport) Options {
	o.Transport = transport
	o.DirectConnect = nil
	return o
}

// WithDirectConnect 使用 direct-connect transport 连接远端 bridge 会话。
func (o Options) WithDirectConnect(options DirectConnectOptions) Options {
	o.DirectConnect = cloneDirectConnectOptions(&options)
	o.Transport = nil
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

// WithSessionID 设置当前会话使用的固定 session ID。
func (o Options) WithSessionID(sessionID string) Options {
	o.Session.ID = sessionID
	return o
}

// WithResumeSessionAt 设置 resume 时截断到指定消息。
func (o Options) WithResumeSessionAt(messageID string) Options {
	o.Session.ResumeAt = messageID
	return o
}

// WithTitle 设置会话展示名。
func (o Options) WithTitle(title string) Options {
	o.Session.Title = title
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
func (o Options) WithAgents(agents map[string]AgentDefinition) Options {
	o.Agents = cloneAgentDefinitions(agents)
	return o
}

// WithAgentDefinition 添加或替换单个 agent 定义。
func (o Options) WithAgentDefinition(name string, definition AgentDefinition) Options {
	o.Agents = cloneAgentDefinitions(o.Agents)
	if o.Agents == nil {
		o.Agents = map[string]AgentDefinition{}
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

// WithSettingsObject 设置 inline settings 对象。
func (o Options) WithSettingsObject(settings map[string]any) Options {
	o.SettingsObject = jsonvalue.CloneMapPreserveTypedSlices(settings)
	return o
}

// WithSandbox 设置 sandbox 配置。
func (o Options) WithSandbox(settings SandboxSettings) Options {
	o.Sandbox = cloneSandboxSettings(&settings)
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

// WithToolConfig 设置内置工具配置；process CLI 不支持的字段会保留在 Options 中但不伪造传输。
func (o Options) WithToolConfig(config ToolConfig) Options {
	o.ToolConfig = cloneToolConfig(&config)
	return o
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

// WithLoadTimeout 是 WithInitializeTimeout 的官方命名语义别名。
func (o Options) WithLoadTimeout(timeout time.Duration) Options {
	return o.WithInitializeTimeout(timeout)
}

// WithLoadTimeoutMillis 设置初始化加载超时毫秒数。
func (o Options) WithLoadTimeoutMillis(milliseconds int) Options {
	o.Runtime.InitializeTimeout = time.Duration(milliseconds) * time.Millisecond
	return o
}

// WithDebug 设置 CLI debug 模式。
func (o Options) WithDebug(enabled bool) Options {
	o.Runtime.Debug = enabled
	return o
}

// WithDebugFile 设置 CLI debug 日志文件。
func (o Options) WithDebugFile(path string) Options {
	o.Runtime.DebugFile = path
	if strings.TrimSpace(path) != "" {
		o.Runtime.Debug = true
	}
	return o
}
