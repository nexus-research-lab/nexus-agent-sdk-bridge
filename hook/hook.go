package hook

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// Event 表示 SDK 支持的 hook 事件名。
type Event string

const (
	EventPreToolUse         Event = "PreToolUse"
	EventPostToolUse        Event = "PostToolUse"
	EventPostToolUseFailure Event = "PostToolUseFailure"
	EventNotification       Event = "Notification"
	EventUserPromptSubmit   Event = "UserPromptSubmit"
	EventSessionStart       Event = "SessionStart"
	EventSessionEnd         Event = "SessionEnd"
	EventStop               Event = "Stop"
	EventStopFailure        Event = "StopFailure"
	EventSubagentStart      Event = "SubagentStart"
	EventSubagentStop       Event = "SubagentStop"
	EventPreCompact         Event = "PreCompact"
	EventPostCompact        Event = "PostCompact"
	EventPermissionRequest  Event = "PermissionRequest"
	EventPermissionDenied   Event = "PermissionDenied"
	EventSetup              Event = "Setup"
	EventTeammateIdle       Event = "TeammateIdle"
	EventTaskCreated        Event = "TaskCreated"
	EventTaskCompleted      Event = "TaskCompleted"
	EventElicitation        Event = "Elicitation"
	EventElicitationResult  Event = "ElicitationResult"
	EventConfigChange       Event = "ConfigChange"
	EventWorktreeCreate     Event = "WorktreeCreate"
	EventWorktreeRemove     Event = "WorktreeRemove"
	EventInstructionsLoaded Event = "InstructionsLoaded"
	EventCWDChanged         Event = "CwdChanged"
	EventFileChanged        Event = "FileChanged"
)

// Input 表示 hook 输入。
type Input struct {
	EventName            Event          `json:"hook_event_name,omitempty"`
	SessionID            string         `json:"session_id,omitempty"`
	TranscriptPath       string         `json:"transcript_path,omitempty"`
	CWD                  string         `json:"cwd,omitempty"`
	PermissionMode       string         `json:"permission_mode,omitempty"`
	AgentID              string         `json:"agent_id,omitempty"`
	AgentType            string         `json:"agent_type,omitempty"`
	ToolName             string         `json:"tool_name,omitempty"`
	ToolInput            any            `json:"tool_input,omitempty"`
	ToolResponse         any            `json:"tool_response,omitempty"`
	ToolUseID            string         `json:"tool_use_id,omitempty"`
	Error                string         `json:"error,omitempty"`
	ErrorDetails         string         `json:"error_details,omitempty"`
	IsInterrupt          bool           `json:"is_interrupt,omitempty"`
	Reason               string         `json:"reason,omitempty"`
	Message              string         `json:"message,omitempty"`
	Title                string         `json:"title,omitempty"`
	NotificationType     string         `json:"notification_type,omitempty"`
	Prompt               string         `json:"prompt,omitempty"`
	Source               string         `json:"source,omitempty"`
	Model                string         `json:"model,omitempty"`
	Trigger              string         `json:"trigger,omitempty"`
	StopHookActive       *bool          `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string         `json:"last_assistant_message,omitempty"`
	AgentTranscriptPath  string         `json:"agent_transcript_path,omitempty"`
	CustomInstructions   any            `json:"custom_instructions,omitempty"`
	CompactSummary       string         `json:"compact_summary,omitempty"`
	TeammateName         string         `json:"teammate_name,omitempty"`
	TeamName             string         `json:"team_name,omitempty"`
	TaskID               string         `json:"task_id,omitempty"`
	TaskSubject          string         `json:"task_subject,omitempty"`
	TaskDescription      string         `json:"task_description,omitempty"`
	MCPServerName        string         `json:"mcp_server_name,omitempty"`
	Mode                 string         `json:"mode,omitempty"`
	URL                  string         `json:"url,omitempty"`
	ElicitationID        string         `json:"elicitation_id,omitempty"`
	RequestedSchema      map[string]any `json:"requested_schema,omitempty"`
	Action               string         `json:"action,omitempty"`
	Content              map[string]any `json:"content,omitempty"`
	FilePath             string         `json:"file_path,omitempty"`
	MemoryType           string         `json:"memory_type,omitempty"`
	LoadReason           string         `json:"load_reason,omitempty"`
	Globs                []string       `json:"globs,omitempty"`
	TriggerFilePath      string         `json:"trigger_file_path,omitempty"`
	ParentFilePath       string         `json:"parent_file_path,omitempty"`
	Name                 string         `json:"name,omitempty"`
	WorktreePath         string         `json:"worktree_path,omitempty"`
	OldCWD               string         `json:"old_cwd,omitempty"`
	NewCWD               string         `json:"new_cwd,omitempty"`
	FileEvent            string         `json:"event,omitempty"`
	Data                 map[string]any `json:"data,omitempty"`
}

// Output 表示 hook 输出。
type Output struct {
	Async             *bool           `json:"async,omitempty"`
	AsyncTimeout      time.Duration   `json:"-"`
	Continue          *bool           `json:"continue,omitempty"`
	SuppressOutput    *bool           `json:"suppress_output,omitempty"`
	StopReason        string          `json:"stop_reason,omitempty"`
	Decision          string          `json:"decision,omitempty"`
	SystemMessage     string          `json:"system_message,omitempty"`
	Reason            string          `json:"reason,omitempty"`
	SpecificOutput    *SpecificOutput `json:"-"`
	RawSpecificOutput map[string]any  `json:"hook_specific_output,omitempty"`
}

// ToMap 将 Output 转换为控制协议所需的 map。
func (o Output) ToMap() map[string]any {
	result := map[string]any{}
	if o.Async != nil {
		result["async"] = *o.Async
		if o.AsyncTimeout > 0 {
			result["async_timeout"] = o.AsyncTimeout.Milliseconds()
		}
	}
	if o.Continue != nil {
		result["continue"] = *o.Continue
	}
	if o.SuppressOutput != nil {
		result["suppress_output"] = *o.SuppressOutput
	}
	if o.StopReason != "" {
		result["stop_reason"] = o.StopReason
	}
	if o.Decision != "" {
		result["decision"] = o.Decision
	}
	if o.SystemMessage != "" {
		result["system_message"] = o.SystemMessage
	}
	if o.Reason != "" {
		result["reason"] = o.Reason
	}
	if len(o.RawSpecificOutput) > 0 {
		result["hook_specific_output"] = o.RawSpecificOutput
	} else if o.SpecificOutput != nil {
		result["hook_specific_output"] = o.SpecificOutput.ToMap()
	}
	return result
}

// SpecificOutput 表示 hook_specific_output 的常用强类型变体，同时保留开放扩展。
type SpecificOutput struct {
	HookEventName             Event
	PermissionDecision        permission.Behavior
	PermissionDecisionReason  string
	UpdatedInput              map[string]any
	AdditionalContext         string
	InitialUserMessage        string
	WatchPaths                []string
	UpdatedMCPToolOutput      any
	Retry                     *bool
	PermissionRequestDecision *permission.Decision
	Action                    string
	Content                   map[string]any
	WorktreePath              string
	Extra                     map[string]any
}

// ToMap 将强类型 hook-specific output 转换为 JSON 形状。
func (o SpecificOutput) ToMap() map[string]any {
	result := jsonvalue.CloneMapPreserveTypedSlices(o.Extra)
	if result == nil {
		result = map[string]any{}
	}
	if o.HookEventName != "" {
		result["hook_event_name"] = string(o.HookEventName)
	}
	if o.PermissionDecision != "" {
		result["permission_decision"] = string(o.PermissionDecision)
	}
	if o.PermissionDecisionReason != "" {
		result["permission_decision_reason"] = o.PermissionDecisionReason
	}
	if len(o.UpdatedInput) > 0 {
		result["updated_input"] = jsonvalue.CloneMapPreserveTypedSlices(o.UpdatedInput)
	}
	if o.AdditionalContext != "" {
		result["additional_context"] = o.AdditionalContext
	}
	if o.InitialUserMessage != "" {
		result["initial_user_message"] = o.InitialUserMessage
	}
	if len(o.WatchPaths) > 0 {
		result["watch_paths"] = append([]string(nil), o.WatchPaths...)
	}
	if o.UpdatedMCPToolOutput != nil {
		result["updated_mcp_tool_output"] = jsonvalue.CloneValuePreserveTypedSlices(o.UpdatedMCPToolOutput)
	}
	if o.Retry != nil {
		result["retry"] = *o.Retry
	}
	if o.PermissionRequestDecision != nil {
		result["decision"] = o.PermissionRequestDecision.ToMap()
	}
	if o.Action != "" {
		result["action"] = o.Action
	}
	if len(o.Content) > 0 {
		result["content"] = jsonvalue.CloneMapPreserveTypedSlices(o.Content)
	}
	if o.WorktreePath != "" {
		result["worktree_path"] = o.WorktreePath
	}
	return result
}

// NewInput 将 hook callback 输入 map 解码为强类型 Input。
func NewInput(input map[string]any) Input {
	return Input{
		EventName:            Event(jsonvalue.StringValue(input["hook_event_name"])),
		SessionID:            jsonvalue.StringValue(input["session_id"]),
		TranscriptPath:       jsonvalue.StringValue(input["transcript_path"]),
		CWD:                  jsonvalue.StringValue(input["cwd"]),
		PermissionMode:       jsonvalue.StringValue(input["permission_mode"]),
		AgentID:              jsonvalue.StringValue(input["agent_id"]),
		AgentType:            jsonvalue.StringValue(input["agent_type"]),
		ToolName:             jsonvalue.StringValue(input["tool_name"]),
		ToolInput:            jsonvalue.CloneValuePreserveTypedSlices(input["tool_input"]),
		ToolResponse:         jsonvalue.CloneValuePreserveTypedSlices(input["tool_response"]),
		ToolUseID:            jsonvalue.StringValue(input["tool_use_id"]),
		Error:                jsonvalue.StringValue(input["error"]),
		ErrorDetails:         jsonvalue.StringValue(input["error_details"]),
		IsInterrupt:          jsonvalue.BoolValue(input["is_interrupt"]),
		Reason:               jsonvalue.StringValue(input["reason"]),
		Message:              jsonvalue.StringValue(input["message"]),
		Title:                jsonvalue.StringValue(input["title"]),
		NotificationType:     jsonvalue.StringValue(input["notification_type"]),
		Prompt:               jsonvalue.StringValue(input["prompt"]),
		Source:               jsonvalue.StringValue(input["source"]),
		Model:                jsonvalue.StringValue(input["model"]),
		Trigger:              jsonvalue.StringValue(input["trigger"]),
		StopHookActive:       jsonvalue.BoolPointer(input["stop_hook_active"]),
		LastAssistantMessage: jsonvalue.StringValue(input["last_assistant_message"]),
		AgentTranscriptPath:  jsonvalue.StringValue(input["agent_transcript_path"]),
		CustomInstructions:   jsonvalue.CloneValuePreserveTypedSlices(input["custom_instructions"]),
		CompactSummary:       jsonvalue.StringValue(input["compact_summary"]),
		TeammateName:         jsonvalue.StringValue(input["teammate_name"]),
		TeamName:             jsonvalue.StringValue(input["team_name"]),
		TaskID:               jsonvalue.StringValue(input["task_id"]),
		TaskSubject:          jsonvalue.StringValue(input["task_subject"]),
		TaskDescription:      jsonvalue.StringValue(input["task_description"]),
		MCPServerName:        jsonvalue.StringValue(input["mcp_server_name"]),
		Mode:                 jsonvalue.StringValue(input["mode"]),
		URL:                  jsonvalue.StringValue(input["url"]),
		ElicitationID:        jsonvalue.StringValue(input["elicitation_id"]),
		RequestedSchema:      jsonvalue.CloneMapValue(input["requested_schema"]),
		Action:               jsonvalue.StringValue(input["action"]),
		Content:              jsonvalue.CloneMapValue(input["content"]),
		FilePath:             jsonvalue.StringValue(input["file_path"]),
		MemoryType:           jsonvalue.StringValue(input["memory_type"]),
		LoadReason:           jsonvalue.StringValue(input["load_reason"]),
		Globs:                jsonvalue.StringSliceValue(input["globs"]),
		TriggerFilePath:      jsonvalue.StringValue(input["trigger_file_path"]),
		ParentFilePath:       jsonvalue.StringValue(input["parent_file_path"]),
		Name:                 jsonvalue.StringValue(input["name"]),
		WorktreePath:         jsonvalue.StringValue(input["worktree_path"]),
		OldCWD:               jsonvalue.StringValue(input["old_cwd"]),
		NewCWD:               jsonvalue.StringValue(input["new_cwd"]),
		FileEvent:            jsonvalue.StringValue(input["event"]),
		Data:                 jsonvalue.CloneMapPreserveTypedSlices(input),
	}
}

// Callback 表示 hook 回调。
type Callback func(context.Context, Input, string) (Output, error)

// Matcher 表示 hook 匹配器。
type Matcher struct {
	Matcher string
	Hooks   []Callback
	Timeout time.Duration
}
