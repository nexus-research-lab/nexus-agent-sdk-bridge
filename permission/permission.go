package permission

import (
	"context"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

// Mode 表示运行时权限模式。
type Mode string

const (
	// ModeDefault 表示默认权限模式。
	ModeDefault Mode = "default"
	// ModeAcceptEdits 表示自动接受编辑。
	ModeAcceptEdits Mode = "acceptEdits"
	// ModeBypassPermissions 表示跳过权限检查。
	ModeBypassPermissions Mode = "bypassPermissions"
	// ModePlan 表示只规划不执行。
	ModePlan Mode = "plan"
	// ModeDontAsk 表示不询问，直接按预设规则处理。
	ModeDontAsk Mode = "dontAsk"
	// ModeAuto 表示自动决策。
	ModeAuto Mode = "auto"
)

// Behavior 表示权限决策行为。
type Behavior string

const (
	// BehaviorAllow 表示允许执行。
	BehaviorAllow Behavior = "allow"
	// BehaviorDeny 表示拒绝执行。
	BehaviorDeny Behavior = "deny"
	// BehaviorAsk 表示提示询问。
	BehaviorAsk Behavior = "ask"
)

// UpdateDestination 表示权限更新目标。
type UpdateDestination string

const (
	// UpdateDestinationUserSettings 表示用户级配置。
	UpdateDestinationUserSettings UpdateDestination = "userSettings"
	// UpdateDestinationProjectSettings 表示项目级配置。
	UpdateDestinationProjectSettings UpdateDestination = "projectSettings"
	// UpdateDestinationLocalSettings 表示本地级配置。
	UpdateDestinationLocalSettings UpdateDestination = "localSettings"
	// UpdateDestinationSession 表示当前会话。
	UpdateDestinationSession UpdateDestination = "session"
	// UpdateDestinationCLIArg 表示命令行参数层。
	UpdateDestinationCLIArg UpdateDestination = "cliArg"
)

// RuleValue 表示权限规则条目。
type RuleValue struct {
	ToolName    string `json:"tool_name"`
	RuleContent string `json:"rule_content,omitempty"`
}

// RuleSource 表示权限规则来源。
type RuleSource string

const (
	// RuleSourceUserSettings 表示用户级设置来源。
	RuleSourceUserSettings RuleSource = "userSettings"
	// RuleSourceProjectSettings 表示项目级设置来源。
	RuleSourceProjectSettings RuleSource = "projectSettings"
	// RuleSourceLocalSettings 表示本地设置来源。
	RuleSourceLocalSettings RuleSource = "localSettings"
	// RuleSourceFlagSettings 表示启动参数设置来源。
	RuleSourceFlagSettings RuleSource = "flagSettings"
	// RuleSourcePolicySettings 表示策略设置来源。
	RuleSourcePolicySettings RuleSource = "policySettings"
	// RuleSourceCLIArg 表示命令行参数来源。
	RuleSourceCLIArg RuleSource = "cliArg"
	// RuleSourceCommand 表示命令注入来源。
	RuleSourceCommand RuleSource = "command"
	// RuleSourceSession 表示当前会话来源。
	RuleSourceSession RuleSource = "session"
)

// Rule 表示带来源和行为的权限规则。
type Rule struct {
	Source   RuleSource `json:"source,omitempty"`
	Behavior Behavior   `json:"behavior,omitempty"`
	Value    RuleValue  `json:"value"`
}

// Update 表示权限更新建议。
type Update struct {
	Type        string            `json:"type"`
	Rules       []RuleValue       `json:"rules,omitempty"`
	Behavior    Behavior          `json:"behavior,omitempty"`
	Mode        Mode              `json:"mode,omitempty"`
	Directories []string          `json:"directories,omitempty"`
	Destination UpdateDestination `json:"destination,omitempty"`
}

// Request 表示运行时发出的权限请求。
type Request struct {
	ToolName              string         `json:"tool_name"`
	Input                 map[string]any `json:"input,omitempty"`
	PermissionSuggestions []Update       `json:"permission_suggestions,omitempty"`
	BlockedPath           string         `json:"blocked_path,omitempty"`
	DecisionReason        string         `json:"decision_reason,omitempty"`
	Title                 string         `json:"title,omitempty"`
	DisplayName           string         `json:"display_name,omitempty"`
	Description           string         `json:"description,omitempty"`
	ToolUseID             string         `json:"tool_use_id,omitempty"`
	AgentID               string         `json:"agent_id,omitempty"`
}

// Decision 表示权限处理器返回给运行时的权限决策。
type Decision struct {
	Behavior           Behavior         `json:"behavior"`
	Message            string           `json:"message,omitempty"`
	AcceptFeedback     string           `json:"accept_feedback,omitempty"`
	ContentBlocks      []map[string]any `json:"content_blocks,omitempty"`
	Interrupt          bool             `json:"interrupt,omitempty"`
	UpdatedInput       map[string]any   `json:"updated_input,omitempty"`
	UpdatedPermissions []Update         `json:"updated_permissions,omitempty"`
}

// Handler 负责处理工具权限请求。
type Handler func(context.Context, Request) (Decision, error)

// Allow 构造允许决策。
func Allow(updatedInput map[string]any, updates []Update) Decision {
	return Decision{
		Behavior:           BehaviorAllow,
		UpdatedInput:       updatedInput,
		UpdatedPermissions: updates,
	}
}

// Deny 构造拒绝决策。
func Deny(message string, interrupt bool) Decision {
	return Decision{
		Behavior:  BehaviorDeny,
		Message:   message,
		Interrupt: interrupt,
	}
}

// ToMap 将 Decision 转换为控制协议 map。
func (d Decision) ToMap() map[string]any {
	result := map[string]any{
		"behavior": string(d.Behavior),
	}
	if d.Message != "" {
		result["message"] = d.Message
	}
	if d.AcceptFeedback != "" {
		result["accept_feedback"] = d.AcceptFeedback
	}
	if len(d.ContentBlocks) > 0 {
		blocks := make([]map[string]any, len(d.ContentBlocks))
		for i, block := range d.ContentBlocks {
			blocks[i] = jsonvalue.CloneMap(block)
		}
		result["content_blocks"] = blocks
	}
	if d.Interrupt {
		result["interrupt"] = true
	}
	if len(d.UpdatedInput) > 0 {
		result["updated_input"] = jsonvalue.CloneMap(d.UpdatedInput)
	}
	if len(d.UpdatedPermissions) > 0 {
		result["updated_permissions"] = append([]Update(nil), d.UpdatedPermissions...)
	}
	return result
}
