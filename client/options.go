package client

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/agent"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpwire"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var errTransportDirectConnectConflict = errors.New("client: transport and direct connect cannot both be configured")

// ElicitationHandler 负责处理 MCP elicitation 请求。
type ElicitationHandler func(context.Context, protocol.ElicitationRequest) (protocol.ElicitationResponse, error)

// UserDialogHandler 负责处理 CLI transport 发起的阻塞式用户对话请求。
type UserDialogHandler func(context.Context, protocol.UserDialogRequest) (protocol.UserDialogResponse, error)

// OAuthTokenHandler 负责按需刷新 Nexus OAuth access token。
type OAuthTokenHandler func(context.Context) (string, error)

// DiagnosticEvent 表示 SDK transport 产生的一条诊断事件。
type DiagnosticEvent struct {
	Component  string
	Event      string
	Attributes map[string]any
}

// DiagnosticHandler 接收 SDK 内部诊断事件。
type DiagnosticHandler func(DiagnosticEvent)

// AgentDefinition 表示可通过 Options 注入的子 agent 定义。
type AgentDefinition = agent.Definition

// SandboxNetworkConfig 表示 sandbox 网络配置。
type SandboxNetworkConfig struct {
	AllowedDomains          []string `json:"allowedDomains,omitempty"`
	AllowManagedDomainsOnly *bool    `json:"allowManagedDomainsOnly,omitempty"`
	AllowUnixSockets        []string `json:"allowUnixSockets,omitempty"`
	AllowAllUnixSockets     *bool    `json:"allowAllUnixSockets,omitempty"`
	AllowLocalBinding       *bool    `json:"allowLocalBinding,omitempty"`
	HTTPProxyPort           *int     `json:"httpProxyPort,omitempty"`
	SOCKSProxyPort          *int     `json:"socksProxyPort,omitempty"`
}

// SandboxFilesystemConfig 表示 sandbox 文件系统配置。
type SandboxFilesystemConfig struct {
	AllowWrite                []string `json:"allowWrite,omitempty"`
	DenyWrite                 []string `json:"denyWrite,omitempty"`
	DenyRead                  []string `json:"denyRead,omitempty"`
	AllowRead                 []string `json:"allowRead,omitempty"`
	AllowManagedReadPathsOnly *bool    `json:"allowManagedReadPathsOnly,omitempty"`
}

// SandboxRipgrepConfig 表示 sandbox 内 ripgrep 配置。
type SandboxRipgrepConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// SandboxSettings 表示命令执行隔离配置。
type SandboxSettings struct {
	Enabled                      *bool                    `json:"enabled,omitempty"`
	FailIfUnavailable            *bool                    `json:"failIfUnavailable,omitempty"`
	AutoAllowBashIfSandboxed     *bool                    `json:"autoAllowBashIfSandboxed,omitempty"`
	AllowUnsandboxedCommands     *bool                    `json:"allowUnsandboxedCommands,omitempty"`
	Network                      *SandboxNetworkConfig    `json:"network,omitempty"`
	Filesystem                   *SandboxFilesystemConfig `json:"filesystem,omitempty"`
	IgnoreViolations             map[string][]string      `json:"ignoreViolations,omitempty"`
	EnableWeakerNestedSandbox    *bool                    `json:"enableWeakerNestedSandbox,omitempty"`
	EnableWeakerNetworkIsolation *bool                    `json:"enableWeakerNetworkIsolation,omitempty"`
	ExcludedCommands             []string                 `json:"excludedCommands,omitempty"`
	Ripgrep                      *SandboxRipgrepConfig    `json:"ripgrep,omitempty"`
	Extra                        map[string]any           `json:"-"`
}

// QuestionPreviewFormat 表示 AskUserQuestion preview 字段格式。
type QuestionPreviewFormat string

const (
	// QuestionPreviewFormatMarkdown 表示 Markdown/ASCII preview。
	QuestionPreviewFormatMarkdown QuestionPreviewFormat = "markdown"
	// QuestionPreviewFormatHTML 表示自包含 HTML 片段 preview。
	QuestionPreviewFormatHTML QuestionPreviewFormat = "html"
)

// AskUserQuestionToolConfig 表示 AskUserQuestion 工具配置。
type AskUserQuestionToolConfig struct {
	PreviewFormat QuestionPreviewFormat `json:"previewFormat,omitempty"`
}

// ToolConfig 表示内置工具配置；是否生效取决于连接的 bridge runtime 是否支持该字段。
type ToolConfig struct {
	AskUserQuestion *AskUserQuestionToolConfig `json:"askUserQuestion,omitempty"`
	Extra           map[string]any             `json:"-"`
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
	// SkillModeDefault 表示使用 CLI 默认 skill 发现策略。
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
	ID             string
	ResumeAt       string
	Title          string
	Persist        *bool
	Fork           bool
}

// RuntimeKind 表示 bridge 要启动的本地 Agent runtime 类型。
type RuntimeKind string

const (
	// RuntimeClaude 表示兼容 Claude Code CLI 的显式 runtime。
	RuntimeClaude RuntimeKind = "claude"
	// RuntimeNXS 表示 bridge 默认的 Nexus 原生 nxs runtime。
	RuntimeNXS RuntimeKind = "nxs"
)

// RuntimeOptions 表示运行期权限、预算、thinking 与 checkpoint 配置。
type RuntimeOptions struct {
	Kind                            RuntimeKind
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
	Debug                           bool
	DebugFile                       string
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
	CLIPath                string
	Transport              Transport
	DirectConnect          *DirectConnectOptions
	CWD                    string
	User                   string
	MaxBufferSize          int
	System                 SystemOptions
	Tools                  ToolOptions
	Skills                 SkillOptions
	Session                SessionOptions
	Runtime                RuntimeOptions
	Agent                  string
	Agents                 map[string]AgentDefinition
	Model                  string
	FallbackModel          string
	Betas                  []string
	Settings               string
	SettingSources         []string
	OutputFormat           *OutputFormat
	AdditionalDirectories  []string
	MCP                    MCPOptions
	Sandbox                *SandboxSettings
	SettingsObject         map[string]any
	ToolConfig             *ToolConfig
	Env                    map[string]string
	Executable             string
	ExecutableArgs         []string
	PathToExecutable       string
	ExtraArgs              map[string]string
	ExtraBoolArgs          []string
	IncludePartialMessages bool
	Hooks                  HookOptions
	Callbacks              CallbackOptions
}

// NewOptions 创建一份可链式构造的空选项。
func NewOptions() Options {
	return Options{}
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

func cloneAgentDefinitions(input map[string]AgentDefinition) map[string]AgentDefinition {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]AgentDefinition, len(input))
	for key, value := range input {
		result[key] = value.Clone()
	}
	return result
}

func cloneMCPServerConfigs(input map[string]mcp.ServerConfig) map[string]mcp.ServerConfig {
	return mcpwire.CloneServerConfigs(input)
}

func cloneSDKMCPServers(input map[string]mcp.SDKMCPServer) map[string]mcp.SDKMCPServer {
	return mcpwire.CloneSDKServers(input)
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

func cloneSandboxSettings(input *SandboxSettings) *SandboxSettings {
	if input == nil {
		return nil
	}
	result := *input
	if input.Network != nil {
		network := *input.Network
		network.AllowedDomains = append([]string(nil), input.Network.AllowedDomains...)
		network.AllowManagedDomainsOnly = cloneBoolPointer(input.Network.AllowManagedDomainsOnly)
		network.AllowUnixSockets = append([]string(nil), input.Network.AllowUnixSockets...)
		network.AllowAllUnixSockets = cloneBoolPointer(input.Network.AllowAllUnixSockets)
		network.AllowLocalBinding = cloneBoolPointer(input.Network.AllowLocalBinding)
		network.HTTPProxyPort = cloneIntPointer(input.Network.HTTPProxyPort)
		network.SOCKSProxyPort = cloneIntPointer(input.Network.SOCKSProxyPort)
		result.Network = &network
	}
	if input.Filesystem != nil {
		filesystem := *input.Filesystem
		filesystem.AllowWrite = append([]string(nil), input.Filesystem.AllowWrite...)
		filesystem.DenyWrite = append([]string(nil), input.Filesystem.DenyWrite...)
		filesystem.DenyRead = append([]string(nil), input.Filesystem.DenyRead...)
		filesystem.AllowRead = append([]string(nil), input.Filesystem.AllowRead...)
		filesystem.AllowManagedReadPathsOnly = cloneBoolPointer(input.Filesystem.AllowManagedReadPathsOnly)
		result.Filesystem = &filesystem
	}
	if len(input.IgnoreViolations) > 0 {
		result.IgnoreViolations = make(map[string][]string, len(input.IgnoreViolations))
		for key, value := range input.IgnoreViolations {
			result.IgnoreViolations[key] = append([]string(nil), value...)
		}
	}
	if input.Ripgrep != nil {
		ripgrep := *input.Ripgrep
		ripgrep.Args = append([]string(nil), input.Ripgrep.Args...)
		result.Ripgrep = &ripgrep
	}
	result.Enabled = cloneBoolPointer(input.Enabled)
	result.FailIfUnavailable = cloneBoolPointer(input.FailIfUnavailable)
	result.AutoAllowBashIfSandboxed = cloneBoolPointer(input.AutoAllowBashIfSandboxed)
	result.AllowUnsandboxedCommands = cloneBoolPointer(input.AllowUnsandboxedCommands)
	result.EnableWeakerNestedSandbox = cloneBoolPointer(input.EnableWeakerNestedSandbox)
	result.EnableWeakerNetworkIsolation = cloneBoolPointer(input.EnableWeakerNetworkIsolation)
	result.ExcludedCommands = append([]string(nil), input.ExcludedCommands...)
	result.Extra = jsonvalue.CloneMapPreserveTypedSlices(input.Extra)
	return &result
}

func cloneToolConfig(input *ToolConfig) *ToolConfig {
	if input == nil {
		return nil
	}
	result := *input
	if input.AskUserQuestion != nil {
		ask := *input.AskUserQuestion
		result.AskUserQuestion = &ask
	}
	result.Extra = jsonvalue.CloneMapPreserveTypedSlices(input.Extra)
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

func cloneIntPointer(input *int) *int {
	if input == nil {
		return nil
	}
	result := *input
	return &result
}
