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
// 它保留底层 wire 层 subtype union 形状，领域语义由根包 permission、interaction.go 等入口提供。
type ControlRequest struct {
	Subtype                string              `json:"subtype"`
	Hooks                  map[string]any      `json:"hooks,omitempty"`
	Agents                 map[string]any      `json:"agents,omitempty"`
	SDKMCPServers          []string            `json:"sdk_mcp_servers,omitempty"`
	Servers                map[string]any      `json:"servers,omitempty"`
	JSONSchema             map[string]any      `json:"json_schema,omitempty"`
	Payload                map[string]any      `json:"payload,omitempty"`
	SystemPrompt           string              `json:"system_prompt,omitempty"`
	AppendSystemPrompt     string              `json:"append_system_prompt,omitempty"`
	ExcludeDynamicSections *bool               `json:"exclude_dynamic_sections,omitempty"`
	PromptSuggestions      *bool               `json:"prompt_suggestions,omitempty"`
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

// NewControlRequestEnvelope 创建控制请求包。
func NewControlRequestEnvelope(requestID string, request ControlRequest) ControlRequestEnvelope {
	return ControlRequestEnvelope{
		Type:      "control_request",
		RequestID: requestID,
		Request:   request,
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

// InitializeResponse 表示初始化响应。
type InitializeResponse struct {
	Commands              []SlashCommandInfo `json:"commands,omitempty"`
	Agents                []AgentInfo        `json:"agents,omitempty"`
	OutputStyle           string             `json:"output_style,omitempty"`
	AvailableOutputStyles []string           `json:"available_output_styles,omitempty"`
	Models                []ModelInfo        `json:"models,omitempty"`
	Account               AccountInfo        `json:"account,omitempty"`
	FastModeState         string             `json:"fast_mode_state,omitempty"`
	Raw                   map[string]any     `json:"raw,omitempty"`
}

// SlashCommandInfo 表示初始化响应中的 slash command。
type SlashCommandInfo struct {
	Name          string         `json:"name,omitempty"`
	Type          string         `json:"type,omitempty"`
	Description   string         `json:"description,omitempty"`
	ArgumentHint  string         `json:"argument_hint,omitempty"`
	AllowedTools  []string       `json:"allowed_tools,omitempty"`
	Category      string         `json:"category,omitempty"`
	Source        string         `json:"source,omitempty"`
	LoadedFrom    string         `json:"loaded_from,omitempty"`
	Kind          string         `json:"kind,omitempty"`
	UserInvocable *bool          `json:"user_invocable,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

// ModelInfo 表示初始化响应中的模型信息。
type ModelInfo struct {
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
	Vendor      string         `json:"vendor,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// AgentInfo 表示初始化响应中的 agent 信息。
type AgentInfo struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	Model       string         `json:"model,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// AccountInfo 表示初始化响应中的账号信息。
type AccountInfo struct {
	EmailAddress       string         `json:"email_address,omitempty"`
	OrganizationName   string         `json:"organization_name,omitempty"`
	Plan               string         `json:"plan,omitempty"`
	SubscriptionStatus string         `json:"subscription_status,omitempty"`
	Raw                map[string]any `json:"raw,omitempty"`
}

// MCPToolAnnotations 表示 MCP 工具注解。
type MCPToolAnnotations struct {
	ReadOnlyHint    bool `json:"read_only_hint,omitempty"`
	DestructiveHint bool `json:"destructive_hint,omitempty"`
	IdempotentHint  bool `json:"idempotent_hint,omitempty"`
	OpenWorldHint   bool `json:"open_world_hint,omitempty"`
	ReadOnly        bool `json:"read_only,omitempty"`
	Destructive     bool `json:"destructive,omitempty"`
	OpenWorld       bool `json:"open_world,omitempty"`
}

// MCPToolInfo 表示 MCP 工具信息。
type MCPToolInfo struct {
	Name        string              `json:"name,omitempty"`
	Description string              `json:"description,omitempty"`
	InputSchema map[string]any      `json:"input_schema,omitempty"`
	Meta        map[string]any      `json:"_meta,omitempty"`
	Annotations *MCPToolAnnotations `json:"annotations,omitempty"`
}

// MCPServerInfo 表示 MCP 服务器信息。
type MCPServerInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// MCPServerStatus 表示 MCP 连接状态。
type MCPServerStatus struct {
	Name       string         `json:"name,omitempty"`
	Status     string         `json:"status,omitempty"`
	ServerInfo *MCPServerInfo `json:"server_info,omitempty"`
	Error      string         `json:"error,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Tools      []MCPToolInfo  `json:"tools,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// MCPStatusResponse 表示 MCP 状态响应。
type MCPStatusResponse struct {
	MCPServers []MCPServerStatus `json:"mcp_servers,omitempty"`
	Raw        map[string]any    `json:"raw,omitempty"`
}

// MCPSetServersResult 表示动态替换 MCP 服务的结果。
type MCPSetServersResult struct {
	Added   []string          `json:"added,omitempty"`
	Removed []string          `json:"removed,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
	Raw     map[string]any    `json:"raw,omitempty"`
}

// PluginStatus 表示插件重载后的插件信息。
type PluginStatus struct {
	Name   string `json:"name,omitempty"`
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
}

// ReloadPluginsResponse 表示插件热重载响应。
type ReloadPluginsResponse struct {
	Commands      []SlashCommandInfo `json:"commands,omitempty"`
	Agents        []AgentInfo        `json:"agents,omitempty"`
	Plugins       []PluginStatus     `json:"plugins,omitempty"`
	MCPServers    []MCPServerStatus  `json:"mcp_servers,omitempty"`
	EnabledCount  int                `json:"enabled_count,omitempty"`
	DisabledCount int                `json:"disabled_count,omitempty"`
	CommandCount  int                `json:"command_count,omitempty"`
	AgentCount    int                `json:"agent_count,omitempty"`
	HookCount     int                `json:"hook_count,omitempty"`
	MCPCount      int                `json:"mcp_count,omitempty"`
	LSPCount      int                `json:"lsp_count,omitempty"`
	ErrorCount    int                `json:"error_count,omitempty"`
	Raw           map[string]any     `json:"raw,omitempty"`
}

// ContextUsageCategory 表示上下文使用分类。
type ContextUsageCategory struct {
	Name       string `json:"name,omitempty"`
	Tokens     int    `json:"tokens,omitempty"`
	Color      string `json:"color,omitempty"`
	IsDeferred bool   `json:"is_deferred,omitempty"`
}

// ContextUsageEntry 表示上下文使用明细中的通用条目。
type ContextUsageEntry struct {
	Name        string         `json:"name,omitempty"`
	Path        string         `json:"path,omitempty"`
	Description string         `json:"description,omitempty"`
	Type        string         `json:"type,omitempty"`
	Scope       string         `json:"scope,omitempty"`
	Tokens      int            `json:"tokens,omitempty"`
	Count       int            `json:"count,omitempty"`
	Percentage  float64        `json:"percentage,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// ContextUsageGridCell 表示网格中的单个单元格。
type ContextUsageGridCell struct {
	Name       string         `json:"name,omitempty"`
	Value      string         `json:"value,omitempty"`
	Tokens     int            `json:"tokens,omitempty"`
	Count      int            `json:"count,omitempty"`
	Color      string         `json:"color,omitempty"`
	Percentage float64        `json:"percentage,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// ContextUsageSlashCommand 表示 slash command 使用信息。
type ContextUsageSlashCommand struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Tokens      int            `json:"tokens,omitempty"`
	Count       int            `json:"count,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// ContextUsageAPIUsage 表示 API 账单相关使用情况。
type ContextUsageAPIUsage struct {
	InputTokens              int            `json:"input_tokens,omitempty"`
	OutputTokens             int            `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int            `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int            `json:"cache_read_input_tokens,omitempty"`
	Raw                      map[string]any `json:"raw,omitempty"`
}

// ContextUsageResponse 表示上下文使用明细。
type ContextUsageResponse struct {
	Categories           []ContextUsageCategory     `json:"categories,omitempty"`
	TotalTokens          int                        `json:"total_tokens,omitempty"`
	MaxTokens            int                        `json:"max_tokens,omitempty"`
	RawMaxTokens         int                        `json:"raw_max_tokens,omitempty"`
	Percentage           float64                    `json:"percentage,omitempty"`
	Model                string                     `json:"model,omitempty"`
	IsAutoCompactEnabled bool                       `json:"is_auto_compact_enabled,omitempty"`
	MemoryFiles          []ContextUsageEntry        `json:"memory_files,omitempty"`
	MCPTools             []ContextUsageEntry        `json:"mcp_tools,omitempty"`
	Agents               []ContextUsageEntry        `json:"agents,omitempty"`
	GridRows             [][]ContextUsageGridCell   `json:"grid_rows,omitempty"`
	AutoCompactThreshold int                        `json:"auto_compact_threshold,omitempty"`
	DeferredBuiltinTools []ContextUsageEntry        `json:"deferred_builtin_tools,omitempty"`
	SystemTools          []ContextUsageEntry        `json:"system_tools,omitempty"`
	SystemPromptSections []ContextUsageEntry        `json:"system_prompt_sections,omitempty"`
	SlashCommands        []ContextUsageSlashCommand `json:"slash_commands,omitempty"`
	APIUsage             ContextUsageAPIUsage       `json:"api_usage,omitempty"`
	Raw                  map[string]any             `json:"raw,omitempty"`
}

// RewindFilesResult 表示文件回滚结果。
type RewindFilesResult struct {
	CanRewind    bool           `json:"can_rewind,omitempty"`
	Error        string         `json:"error,omitempty"`
	FilesChanged []string       `json:"files_changed,omitempty"`
	Insertions   int            `json:"insertions,omitempty"`
	Deletions    int            `json:"deletions,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

// ContentMap 将文件回滚结果转换为控制协议需要的 map。
func (r RewindFilesResult) ContentMap() map[string]any {
	result := map[string]any{
		"can_rewind": r.CanRewind,
	}
	if r.Error != "" {
		result["error"] = r.Error
	}
	if len(r.FilesChanged) > 0 {
		result["files_changed"] = append([]string(nil), r.FilesChanged...)
	}
	if r.Insertions != 0 {
		result["insertions"] = r.Insertions
	}
	if r.Deletions != 0 {
		result["deletions"] = r.Deletions
	}
	return result
}

// DecodeMCPStatusResponse 解析 MCP 状态响应。
func DecodeMCPStatusResponse(payload map[string]any) MCPStatusResponse {
	items := jsonvalue.SliceValue(payload["mcp_servers"])
	servers := make([]MCPServerStatus, 0, len(items))
	for _, item := range items {
		server := jsonvalue.MapValue(item)
		if len(server) == 0 {
			continue
		}

		var annotations *MCPToolAnnotations
		tools := make([]MCPToolInfo, 0, len(jsonvalue.SliceValue(server["tools"])))
		for _, rawTool := range jsonvalue.SliceValue(server["tools"]) {
			tool := jsonvalue.MapValue(rawTool)
			if len(tool) == 0 {
				continue
			}
			if annotationData := jsonvalue.MapValue(tool["annotations"]); len(annotationData) > 0 {
				readOnlyHint := jsonvalue.BoolValue(annotationData["read_only_hint"])
				readOnly := jsonvalue.BoolValue(annotationData["read_only"])
				destructiveHint := jsonvalue.BoolValue(annotationData["destructive_hint"])
				destructive := jsonvalue.BoolValue(annotationData["destructive"])
				openWorldHint := jsonvalue.BoolValue(annotationData["open_world_hint"])
				openWorld := jsonvalue.BoolValue(annotationData["open_world"])
				annotations = &MCPToolAnnotations{
					ReadOnlyHint:    readOnlyHint || readOnly,
					DestructiveHint: destructiveHint || destructive,
					IdempotentHint:  jsonvalue.BoolValue(annotationData["idempotent_hint"]),
					OpenWorldHint:   openWorldHint || openWorld,
					ReadOnly:        readOnly,
					Destructive:     destructive,
					OpenWorld:       openWorld,
				}
			} else {
				annotations = nil
			}
			meta := jsonvalue.MapValue(tool["_meta"])
			inputSchema := jsonvalue.MapValue(tool["input_schema"])

			tools = append(tools, MCPToolInfo{
				Name:        jsonvalue.StringValue(tool["name"]),
				Description: jsonvalue.StringValue(tool["description"]),
				InputSchema: inputSchema,
				Meta:        meta,
				Annotations: annotations,
			})
		}

		var serverInfo *MCPServerInfo
		if info := jsonvalue.MapValue(server["server_info"]); len(info) > 0 {
			serverInfo = &MCPServerInfo{
				Name:    jsonvalue.StringValue(info["name"]),
				Version: jsonvalue.StringValue(info["version"]),
			}
		}

		servers = append(servers, MCPServerStatus{
			Name:       jsonvalue.StringValue(server["name"]),
			Status:     jsonvalue.StringValue(server["status"]),
			ServerInfo: serverInfo,
			Error:      jsonvalue.StringValue(server["error"]),
			Config:     jsonvalue.MapValue(server["config"]),
			Scope:      jsonvalue.StringValue(server["scope"]),
			Tools:      tools,
			Raw:        server,
		})
	}

	return MCPStatusResponse{
		MCPServers: servers,
		Raw:        payload,
	}
}

// DecodeContextUsageResponse 解析上下文使用响应。
func DecodeContextUsageResponse(payload map[string]any) ContextUsageResponse {
	categories := make([]ContextUsageCategory, 0, len(jsonvalue.SliceValue(payload["categories"])))
	for _, item := range jsonvalue.SliceValue(payload["categories"]) {
		category := jsonvalue.MapValue(item)
		if len(category) == 0 {
			continue
		}
		categories = append(categories, ContextUsageCategory{
			Name:       jsonvalue.StringValue(category["name"]),
			Tokens:     jsonvalue.IntValue(category["tokens"]),
			Color:      jsonvalue.StringValue(category["color"]),
			IsDeferred: jsonvalue.BoolValue(category["is_deferred"]),
		})
	}

	return ContextUsageResponse{
		Categories:           categories,
		TotalTokens:          jsonvalue.IntValue(payload["total_tokens"]),
		MaxTokens:            jsonvalue.IntValue(payload["max_tokens"]),
		RawMaxTokens:         jsonvalue.IntValue(payload["raw_max_tokens"]),
		Percentage:           jsonvalue.FloatValue(payload["percentage"]),
		Model:                jsonvalue.StringValue(payload["model"]),
		IsAutoCompactEnabled: jsonvalue.BoolValue(payload["is_auto_compact_enabled"]),
		MemoryFiles:          decodeContextUsageEntries(payload["memory_files"]),
		MCPTools:             decodeContextUsageEntries(payload["mcp_tools"]),
		Agents:               decodeContextUsageEntries(payload["agents"]),
		GridRows:             decodeContextUsageGridRows(payload["grid_rows"]),
		AutoCompactThreshold: jsonvalue.IntValue(payload["auto_compact_threshold"]),
		DeferredBuiltinTools: decodeContextUsageEntries(payload["deferred_builtin_tools"]),
		SystemTools:          decodeContextUsageEntries(payload["system_tools"]),
		SystemPromptSections: decodeContextUsageEntries(payload["system_prompt_sections"]),
		SlashCommands:        decodeContextUsageSlashCommands(payload["slash_commands"]),
		APIUsage:             decodeContextUsageAPIUsage(payload["api_usage"]),
		Raw:                  payload,
	}
}

// DecodeRewindFilesResult 解析文件回滚结果。
func DecodeRewindFilesResult(payload map[string]any) RewindFilesResult {
	return RewindFilesResult{
		CanRewind:    jsonvalue.BoolValue(payload["can_rewind"]),
		Error:        jsonvalue.StringValue(payload["error"]),
		FilesChanged: jsonvalue.StringSliceValue(payload["files_changed"]),
		Insertions:   jsonvalue.IntValue(payload["insertions"]),
		Deletions:    jsonvalue.IntValue(payload["deletions"]),
		Raw:          payload,
	}
}

// DecodeMCPSetServersResult 解析动态 MCP 服务更新结果。
func DecodeMCPSetServersResult(payload map[string]any) MCPSetServersResult {
	errorsResult := map[string]string{}
	for key, value := range jsonvalue.MapValue(payload["errors"]) {
		text := jsonvalue.StringValue(value)
		if text == "" {
			continue
		}
		errorsResult[key] = text
	}

	return MCPSetServersResult{
		Added:   jsonvalue.StringSliceValue(payload["added"]),
		Removed: jsonvalue.StringSliceValue(payload["removed"]),
		Errors:  errorsResult,
		Raw:     payload,
	}
}

// DecodeInitializeResponse 解析初始化响应。
func DecodeInitializeResponse(payload map[string]any) InitializeResponse {
	return InitializeResponse{
		Commands:              decodeSlashCommands(payload["commands"]),
		Agents:                decodeAgents(payload["agents"]),
		OutputStyle:           jsonvalue.StringValue(payload["output_style"]),
		AvailableOutputStyles: jsonvalue.StringSliceValue(payload["available_output_styles"]),
		Models:                decodeModels(payload["models"]),
		Account:               decodeAccountInfo(payload["account"]),
		FastModeState:         jsonvalue.StringValue(payload["fast_mode_state"]),
		Raw:                   payload,
	}
}

// DecodeReloadPluginsResponse 解析插件热重载结果。
func DecodeReloadPluginsResponse(payload map[string]any) ReloadPluginsResponse {
	plugins := make([]PluginStatus, 0, len(jsonvalue.SliceValue(payload["plugins"])))
	for _, item := range jsonvalue.SliceValue(payload["plugins"]) {
		plugin := jsonvalue.MapValue(item)
		if len(plugin) == 0 {
			continue
		}
		plugins = append(plugins, PluginStatus{
			Name:   jsonvalue.StringValue(plugin["name"]),
			Path:   jsonvalue.StringValue(plugin["path"]),
			Source: jsonvalue.StringValue(plugin["source"]),
		})
	}

	return ReloadPluginsResponse{
		Commands:      decodeSlashCommands(payload["commands"]),
		Agents:        decodeAgents(payload["agents"]),
		Plugins:       plugins,
		MCPServers:    DecodeMCPStatusResponse(map[string]any{"mcp_servers": payload["mcp_servers"]}).MCPServers,
		EnabledCount:  jsonvalue.IntValue(payload["enabled_count"]),
		DisabledCount: jsonvalue.IntValue(payload["disabled_count"]),
		CommandCount:  jsonvalue.IntValue(payload["command_count"]),
		AgentCount:    jsonvalue.IntValue(payload["agent_count"]),
		HookCount:     jsonvalue.IntValue(payload["hook_count"]),
		MCPCount:      jsonvalue.IntValue(payload["mcp_count"]),
		LSPCount:      jsonvalue.IntValue(payload["lsp_count"]),
		ErrorCount:    jsonvalue.IntValue(payload["error_count"]),
		Raw:           payload,
	}
}

func decodeSlashCommands(raw any) []SlashCommandInfo {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]SlashCommandInfo, 0, len(items))
	for _, item := range items {
		result = append(result, SlashCommandInfo{
			Name:          jsonvalue.FirstNonEmptyString(item["name"], item["command"]),
			Type:          jsonvalue.FirstNonEmptyString(item["type"], item["command_type"]),
			Description:   jsonvalue.StringValue(item["description"]),
			ArgumentHint:  jsonvalue.StringValue(item["argument_hint"]),
			AllowedTools:  parseSlashCommandTools(item["allowed_tools"]),
			Category:      jsonvalue.StringValue(item["category"]),
			Source:        jsonvalue.StringValue(item["source"]),
			LoadedFrom:    jsonvalue.StringValue(item["loaded_from"]),
			Kind:          jsonvalue.StringValue(item["kind"]),
			UserInvocable: parseOptionalBool(item["user_invocable"]),
			Raw:           item,
		})
	}
	return result
}

func parseOptionalBool(values ...any) *bool {
	for _, value := range values {
		switch typed := value.(type) {
		case bool:
			result := typed
			return &result
		case string:
			normalized := strings.TrimSpace(strings.ToLower(typed))
			switch normalized {
			case "true":
				result := true
				return &result
			case "false":
				result := false
				return &result
			}
		}
	}
	return nil
}

func parseSlashCommandTools(values ...any) []string {
	for _, value := range values {
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			tools := make([]string, 0, len(typed))
			for _, item := range typed {
				if text := jsonvalue.StringValue(item); text != "" {
					tools = append(tools, text)
				}
			}
			if len(tools) > 0 {
				return tools
			}
		case string:
			return splitToolList(typed)
		}
	}
	return nil
}

func splitToolList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		raw = strings.TrimSpace(raw[1 : len(raw)-1])
	}
	if raw == "" {
		return nil
	}
	values := []string{}
	current := strings.Builder{}
	depth := 0
	var quote rune
	flush := func() {
		part := strings.TrimSpace(current.String())
		current.Reset()
		part = strings.Trim(part, `"'`)
		if part != "" {
			values = append(values, part)
		}
	}
	for _, r := range raw {
		switch {
		case quote != 0:
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			current.WriteRune(r)
		case r == '(' || r == '[' || r == '{':
			depth++
			current.WriteRune(r)
		case r == ')' || r == ']' || r == '}':
			if depth > 0 {
				depth--
			}
			current.WriteRune(r)
		case r == ',' && depth == 0:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return values
}

func decodeModels(raw any) []ModelInfo {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]ModelInfo, 0, len(items))
	for _, item := range items {
		result = append(result, ModelInfo{
			ID:          jsonvalue.FirstNonEmptyString(item["id"], item["name"]),
			Name:        jsonvalue.StringValue(item["name"]),
			DisplayName: jsonvalue.StringValue(item["display_name"]),
			Vendor:      jsonvalue.StringValue(item["vendor"]),
			Raw:         item,
		})
	}
	return result
}

func decodeAgents(raw any) []AgentInfo {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]AgentInfo, 0, len(items))
	for _, item := range items {
		result = append(result, AgentInfo{
			Name:        jsonvalue.StringValue(item["name"]),
			Description: jsonvalue.StringValue(item["description"]),
			Prompt:      jsonvalue.StringValue(item["prompt"]),
			Model:       jsonvalue.StringValue(item["model"]),
			Raw:         item,
		})
	}
	return result
}

func decodeAccountInfo(raw any) AccountInfo {
	item := jsonvalue.MapValue(raw)
	return AccountInfo{
		EmailAddress:       jsonvalue.FirstNonEmptyString(item["email"], item["email_address"]),
		OrganizationName:   jsonvalue.StringValue(item["organization_name"]),
		Plan:               jsonvalue.FirstNonEmptyString(item["plan"], item["subscription_plan"]),
		SubscriptionStatus: jsonvalue.StringValue(item["subscription_status"]),
		Raw:                item,
	}
}

func decodeContextUsageEntries(raw any) []ContextUsageEntry {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]ContextUsageEntry, 0, len(items))
	for _, item := range items {
		result = append(result, ContextUsageEntry{
			Name:        jsonvalue.FirstNonEmptyString(item["name"], item["label"]),
			Path:        jsonvalue.FirstNonEmptyString(item["path"], item["file"]),
			Description: jsonvalue.StringValue(item["description"]),
			Type:        jsonvalue.StringValue(item["type"]),
			Scope:       jsonvalue.StringValue(item["scope"]),
			Tokens:      jsonvalue.IntValue(item["tokens"]),
			Count:       jsonvalue.IntValue(item["count"]),
			Percentage:  jsonvalue.FloatValue(item["percentage"]),
			Raw:         item,
		})
	}
	return result
}

func decodeContextUsageGridRows(raw any) [][]ContextUsageGridCell {
	rows := jsonvalue.SliceValue(raw)
	result := make([][]ContextUsageGridCell, 0, len(rows))
	for _, rowRaw := range rows {
		cells := jsonvalue.SliceValue(rowRaw)
		row := make([]ContextUsageGridCell, 0, len(cells))
		for _, cellRaw := range cells {
			cell := jsonvalue.MapValue(cellRaw)
			if len(cell) == 0 {
				continue
			}
			row = append(row, ContextUsageGridCell{
				Name:       jsonvalue.FirstNonEmptyString(cell["name"], cell["label"]),
				Value:      jsonvalue.StringValue(cell["value"]),
				Tokens:     jsonvalue.IntValue(cell["tokens"]),
				Count:      jsonvalue.IntValue(cell["count"]),
				Color:      jsonvalue.StringValue(cell["color"]),
				Percentage: jsonvalue.FloatValue(cell["percentage"]),
				Raw:        cell,
			})
		}
		result = append(result, row)
	}
	return result
}

func decodeContextUsageSlashCommands(raw any) []ContextUsageSlashCommand {
	objectValue := jsonvalue.MapValue(raw)
	if len(objectValue) > 0 {
		result := make([]ContextUsageSlashCommand, 0, len(objectValue))
		for key, rawValue := range objectValue {
			payload := jsonvalue.MapValue(rawValue)
			if len(payload) == 0 {
				payload = map[string]any{"name": key, "value": rawValue}
			}
			if payload["name"] == nil {
				payload["name"] = key
			}
			result = append(result, ContextUsageSlashCommand{
				Name:        jsonvalue.StringValue(payload["name"]),
				Description: jsonvalue.StringValue(payload["description"]),
				Tokens:      jsonvalue.IntValue(payload["tokens"]),
				Count:       jsonvalue.IntValue(payload["count"]),
				Raw:         payload,
			})
		}
		return result
	}

	items := jsonvalue.MapSliceValue(raw)
	result := make([]ContextUsageSlashCommand, 0, len(items))
	for _, item := range items {
		result = append(result, ContextUsageSlashCommand{
			Name:        jsonvalue.FirstNonEmptyString(item["name"], item["command"]),
			Description: jsonvalue.StringValue(item["description"]),
			Tokens:      jsonvalue.IntValue(item["tokens"]),
			Count:       jsonvalue.IntValue(item["count"]),
			Raw:         item,
		})
	}
	return result
}

func decodeContextUsageAPIUsage(raw any) ContextUsageAPIUsage {
	payload := jsonvalue.MapValue(raw)
	return ContextUsageAPIUsage{
		InputTokens:              jsonvalue.IntValue(payload["input_tokens"]),
		OutputTokens:             jsonvalue.IntValue(payload["output_tokens"]),
		CacheCreationInputTokens: jsonvalue.IntValue(payload["cache_creation_input_tokens"]),
		CacheReadInputTokens:     jsonvalue.IntValue(payload["cache_read_input_tokens"]),
		Raw:                      payload,
	}
}
