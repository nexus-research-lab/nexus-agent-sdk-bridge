package protocol

import (
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// ControlRequestEnvelope 表示控制请求包。
type ControlRequestEnvelope struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Request   ControlRequest `json:"request"`
}

// ControlRequest 表示统一控制请求。
//
// 它保留底层 wire 层 subtype union 形状，领域语义由 client、mcp、permission 等公开包承接。
type ControlRequest struct {
	Subtype                string              `json:"subtype"`
	ProtocolCapabilities   []string            `json:"protocol_capabilities,omitempty"`
	Reason                 string              `json:"reason,omitempty"`
	Hooks                  map[string]any      `json:"hooks,omitempty"`
	Agents                 map[string]any      `json:"agents,omitempty"`
	SDKMCPServers          []string            `json:"sdk_mcp_servers,omitempty"`
	Servers                map[string]any      `json:"servers,omitempty"`
	JSONSchema             map[string]any      `json:"json_schema,omitempty"`
	Payload                map[string]any      `json:"payload,omitempty"`
	SystemPrompt           string              `json:"system_prompt,omitempty"`
	AppendSystemPrompt     string              `json:"append_system_prompt,omitempty"`
	ExcludeDynamicSections *bool               `json:"exclude_dynamic_sections,omitempty"`
	AgentProgressSummaries *bool               `json:"agent_progress_summaries,omitempty"`
	Skills                 *[]string           `json:"skills,omitempty"`
	ToolName               string              `json:"tool_name,omitempty"`
	Input                  map[string]any      `json:"input,omitempty"`
	PermissionSuggestions  []permission.Update `json:"permission_suggestions,omitempty"`
	BlockedPath            string              `json:"blocked_path,omitempty"`
	DecisionReason         string              `json:"decision_reason,omitempty"`
	Title                  string              `json:"title,omitempty"`
	Name                   string              `json:"name,omitempty"`
	DisplayName            string              `json:"display_name,omitempty"`
	Description            string              `json:"description,omitempty"`
	Question               string              `json:"question,omitempty"`
	DialogKind             string              `json:"dialog_kind,omitempty"`
	ToolUseID              string              `json:"tool_use_id,omitempty"`
	AgentID                string              `json:"agent_id,omitempty"`
	Mode                   permission.Mode     `json:"mode,omitempty"`
	CallbackID             string              `json:"callback_id,omitempty"`
	Model                  string              `json:"model,omitempty"`
	MaxThinkingTokens      *int                `json:"max_thinking_tokens,omitempty"`
	UserMessageID          string              `json:"user_message_id,omitempty"`
	MessageUUID            string              `json:"message_uuid,omitempty"`
	MessageUUIDs           []string            `json:"message_uuids,omitempty"`
	DryRun                 *bool               `json:"dry_run,omitempty"`
	Persist                *bool               `json:"persist,omitempty"`
	ServerName             string              `json:"server_name,omitempty"`
	Enabled                *bool               `json:"enabled,omitempty"`
	TaskID                 string              `json:"task_id,omitempty"`
	Path                   string              `json:"path,omitempty"`
	MTime                  int64               `json:"mtime,omitempty"`
	Message                map[string]any      `json:"message,omitempty"`
	Settings               map[string]any      `json:"settings,omitempty"`
	MCPServerName          string              `json:"mcp_server_name,omitempty"`
	URL                    string              `json:"url,omitempty"`
	CallbackURL            string              `json:"callback_url,omitempty"`
	ElicitationID          string              `json:"elicitation_id,omitempty"`
	RequestedSchema        map[string]any      `json:"requested_schema,omitempty"`
	LoginWithWeb           *bool               `json:"login_with_web,omitempty"`
	AuthorizationCode      string              `json:"authorization_code,omitempty"`
	State                  string              `json:"state,omitempty"`
}

// ControlResponseEnvelope 表示控制响应包。
type ControlResponseEnvelope struct {
	Type     string          `json:"type"`
	Response ControlResponse `json:"response"`
}

// ControlResponse 表示统一控制响应。
type ControlResponse struct {
	Subtype   string         `json:"subtype"`
	RequestID string         `json:"request_id"`
	Response  map[string]any `json:"response,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// ControlCancelRequest 表示控制取消请求。
type ControlCancelRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
}

// ControlAckEnvelope 表示 runtime 对宿主控制响应的应用确认。
type ControlAckEnvelope struct {
	Type           string `json:"type"`
	RequestID      string `json:"request_id"`
	RequestSubtype string `json:"request_subtype,omitempty"`
	Stage          string `json:"stage,omitempty"`
	HookEventName  string `json:"hook_event_name,omitempty"`
	ToolUseID      string `json:"tool_use_id,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
}

// DecodeControlAck 解析 runtime 发送的控制响应应用确认。
func DecodeControlAck(payload map[string]any) ControlAckEnvelope {
	return ControlAckEnvelope{
		Type:           jsonvalue.StringValue(payload["type"]),
		RequestID:      jsonvalue.StringValue(payload["request_id"]),
		RequestSubtype: jsonvalue.StringValue(payload["request_subtype"]),
		Stage:          jsonvalue.StringValue(payload["stage"]),
		HookEventName:  jsonvalue.StringValue(payload["hook_event_name"]),
		ToolUseID:      jsonvalue.StringValue(payload["tool_use_id"]),
		SessionID:      jsonvalue.StringValue(payload["session_id"]),
	}
}

// NewControlRequestEnvelope 创建控制请求包。
func NewControlRequestEnvelope(requestID string, request ControlRequest) ControlRequestEnvelope {
	return ControlRequestEnvelope{
		Type:      "control_request",
		RequestID: requestID,
		Request:   request,
	}
}

// NewControlCancelRequest 创建控制取消请求。
func NewControlCancelRequest(requestID string) ControlCancelRequest {
	return ControlCancelRequest{
		Type:      "control_cancel_request",
		RequestID: requestID,
	}
}

// NewControlSuccessResponse 创建成功响应。
func NewControlSuccessResponse(requestID string, response map[string]any) ControlResponseEnvelope {
	return ControlResponseEnvelope{
		Type: "control_response",
		Response: ControlResponse{
			Subtype:   "success",
			RequestID: requestID,
			Response:  response,
		},
	}
}

// NewControlErrorResponse 创建失败响应。
func NewControlErrorResponse(requestID string, message string) ControlResponseEnvelope {
	return ControlResponseEnvelope{
		Type: "control_response",
		Response: ControlResponse{
			Subtype:   "error",
			RequestID: requestID,
			Error:     message,
		},
	}
}

// ElicitationAction 表示 elicitation 响应动作。
type ElicitationAction string

const (
	// ElicitationActionAccept 表示接受请求。
	ElicitationActionAccept ElicitationAction = "accept"
	// ElicitationActionDecline 表示拒绝请求。
	ElicitationActionDecline ElicitationAction = "decline"
	// ElicitationActionCancel 表示取消请求。
	ElicitationActionCancel ElicitationAction = "cancel"
)

// ElicitationMode 表示 elicitation 展示模式。
type ElicitationMode string

const (
	// ElicitationModeForm 表示表单模式。
	ElicitationModeForm ElicitationMode = "form"
	// ElicitationModeURL 表示 URL 模式。
	ElicitationModeURL ElicitationMode = "url"
)

// UserDialogAction 表示用户对话响应动作。
type UserDialogAction string

const (
	// UserDialogActionSubmit 表示提交对话。
	UserDialogActionSubmit UserDialogAction = "submit"
	// UserDialogActionCancel 表示取消对话。
	UserDialogActionCancel UserDialogAction = "cancel"
	// UserDialogActionDismiss 表示关闭对话。
	UserDialogActionDismiss UserDialogAction = "dismiss"
)

// ElicitationRequest 表示 MCP 服务发起的用户输入请求。
type ElicitationRequest struct {
	ServerName      string         `json:"server_name,omitempty"`
	Message         string         `json:"message,omitempty"`
	Mode            string         `json:"mode,omitempty"`
	URL             string         `json:"url,omitempty"`
	ElicitationID   string         `json:"elicitation_id,omitempty"`
	RequestedSchema map[string]any `json:"requested_schema,omitempty"`
	Title           string         `json:"title,omitempty"`
	DisplayName     string         `json:"display_name,omitempty"`
	Description     string         `json:"description,omitempty"`
}

// ElicitationResponse 表示 SDK 消费方返回给 CLI 的 elicitation 决策。
type ElicitationResponse struct {
	Action  string         `json:"action,omitempty"`
	Content map[string]any `json:"content,omitempty"`
}

// UserDialogRequest 表示 CLI 请求宿主渲染阻塞式用户对话框。
type UserDialogRequest struct {
	DialogKind string         `json:"dialog_kind,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	ToolUseID  string         `json:"tool_use_id,omitempty"`
}

// UserDialogResponse 表示宿主返回给 CLI 的用户对话结果。
// 中文注释：官方协议对该结构保持开放，这里直接保留原始 map，避免过早收窄。
type UserDialogResponse map[string]any

// DecodeElicitationRequest 解析控制协议中的 elicitation 请求。
func DecodeElicitationRequest(payload map[string]any) ElicitationRequest {
	return ElicitationRequest{
		ServerName:      jsonvalue.FirstNonEmptyString(payload["mcp_server_name"], payload["server_name"]),
		Message:         jsonvalue.StringValue(payload["message"]),
		Mode:            NormalizeElicitationMode(jsonvalue.FirstNonEmptyString(payload["mode"], payload["elicitation_mode"])),
		URL:             jsonvalue.StringValue(payload["url"]),
		ElicitationID:   jsonvalue.StringValue(payload["elicitation_id"]),
		RequestedSchema: jsonvalue.MapValue(payload["requested_schema"]),
		Title:           jsonvalue.StringValue(payload["title"]),
		DisplayName:     jsonvalue.StringValue(payload["display_name"]),
		Description:     jsonvalue.StringValue(payload["description"]),
	}
}

// AcceptElicitation 构造接受响应。
func AcceptElicitation(content map[string]any) ElicitationResponse {
	return ElicitationResponse{Action: string(ElicitationActionAccept), Content: jsonvalue.CloneMap(content)}
}

// DeclineElicitation 构造拒绝响应。
func DeclineElicitation(content map[string]any) ElicitationResponse {
	return ElicitationResponse{Action: string(ElicitationActionDecline), Content: jsonvalue.CloneMap(content)}
}

// CancelElicitation 构造取消响应。
func CancelElicitation(content map[string]any) ElicitationResponse {
	return ElicitationResponse{Action: string(ElicitationActionCancel), Content: jsonvalue.CloneMap(content)}
}

// NormalizeElicitationResponse 归一化 elicitation 响应。
func NormalizeElicitationResponse(response ElicitationResponse) ElicitationResponse {
	response.Action = NormalizeElicitationAction(response.Action)
	if response.Action == "" {
		response.Action = string(ElicitationActionDecline)
	}
	response.Content = jsonvalue.CloneMap(response.Content)
	return response
}

// NormalizeElicitationAction 归一化 elicitation 动作。
func NormalizeElicitationAction(action string) string {
	switch strings.TrimSpace(action) {
	case string(ElicitationActionAccept), string(ElicitationActionDecline), string(ElicitationActionCancel):
		return strings.TrimSpace(action)
	default:
		return ""
	}
}

// NormalizeElicitationMode 归一化 elicitation 展示模式。
func NormalizeElicitationMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case string(ElicitationModeForm), string(ElicitationModeURL):
		return strings.TrimSpace(mode)
	default:
		return ""
	}
}

// ContentMap 将 ElicitationResponse 转换为控制协议需要的 map。
func (r ElicitationResponse) ContentMap() map[string]any {
	r = NormalizeElicitationResponse(r)
	result := map[string]any{
		"action": r.Action,
	}
	if len(r.Content) > 0 {
		result["content"] = jsonvalue.CloneMap(r.Content)
	}
	return result
}

// DecodeUserDialogRequest 解析控制协议中的用户对话请求。
func DecodeUserDialogRequest(payload map[string]any) UserDialogRequest {
	return UserDialogRequest{
		DialogKind: jsonvalue.StringValue(payload["dialog_kind"]),
		Payload:    jsonvalue.MapValue(payload["payload"]),
		ToolUseID:  jsonvalue.StringValue(payload["tool_use_id"]),
	}
}

// NewUserDialogResponse 构造用户对话响应。
func NewUserDialogResponse(action string, payload map[string]any) UserDialogResponse {
	response := UserDialogResponse{}
	if action = strings.TrimSpace(action); action != "" {
		response["action"] = action
	}
	if len(payload) > 0 {
		response["payload"] = jsonvalue.CloneMap(payload)
	}
	return response
}

// SubmitUserDialog 构造用户对话提交响应。
func SubmitUserDialog(payload map[string]any) UserDialogResponse {
	return NewUserDialogResponse(string(UserDialogActionSubmit), payload)
}

// CancelUserDialog 构造用户对话取消响应。
func CancelUserDialog(payload map[string]any) UserDialogResponse {
	return NewUserDialogResponse(string(UserDialogActionCancel), payload)
}

// DismissUserDialog 构造用户对话关闭响应。
func DismissUserDialog(payload map[string]any) UserDialogResponse {
	return NewUserDialogResponse(string(UserDialogActionDismiss), payload)
}

// ContentMap 将 UserDialogResponse 转换为控制协议所需的 map。
func (r UserDialogResponse) ContentMap() map[string]any {
	if r == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(r))
	for key, value := range r {
		result[key] = jsonvalue.CloneValue(value)
	}
	return result
}
