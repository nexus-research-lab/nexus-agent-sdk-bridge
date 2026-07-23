package protocol

import (
	"encoding/json"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// ContentBlockType 表示内容块类型。
type ContentBlockType string

const (
	// ContentBlockTypeText 表示文本块。
	ContentBlockTypeText ContentBlockType = "text"
	// ContentBlockTypeImage 表示图片块。
	ContentBlockTypeImage ContentBlockType = "image"
	// ContentBlockTypeDocument 表示文档块。
	ContentBlockTypeDocument ContentBlockType = "document"
	// ContentBlockTypeSearchResult 表示搜索结果块。
	ContentBlockTypeSearchResult ContentBlockType = "search_result"
	// ContentBlockTypeResourceLink 表示资源链接块。
	ContentBlockTypeResourceLink ContentBlockType = "resource_link"
	// ContentBlockTypeThinking 表示思考块。
	ContentBlockTypeThinking ContentBlockType = "thinking"
	// ContentBlockTypeToolUse 表示工具调用块。
	ContentBlockTypeToolUse ContentBlockType = "tool_use"
	// ContentBlockTypeToolResult 表示工具结果块。
	ContentBlockTypeToolResult ContentBlockType = "tool_result"
)

// ContentBlock 表示公开暴露的强类型内容块接口。
type ContentBlock interface {
	Type() ContentBlockType
	RawPayload() map[string]any
}

// TextBlock 表示文本内容块。
type TextBlock struct {
	Text string

	raw map[string]any
}

// Type 返回内容块类型。
func (b TextBlock) Type() ContentBlockType {
	return ContentBlockTypeText
}

// RawPayload 返回原始负载副本。
func (b TextBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// ImageBlock 表示图片内容块。
type ImageBlock struct {
	Data     string
	MIMEType string

	raw map[string]any
}

// Type 返回内容块类型。
func (b ImageBlock) Type() ContentBlockType {
	return ContentBlockTypeImage
}

// RawPayload 返回原始负载副本。
func (b ImageBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// DocumentBlock 表示文档内容块。
type DocumentBlock struct {
	MIMEType string
	Source   json.RawMessage
	Title    string

	raw map[string]any
}

// Type 返回内容块类型。
func (b DocumentBlock) Type() ContentBlockType {
	return ContentBlockTypeDocument
}

// RawPayload 返回原始负载副本。
func (b DocumentBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// SearchResultBlock 表示搜索结果内容块。
type SearchResultBlock struct {
	Query   string
	Source  string
	Title   string
	URL     string
	Snippet string

	raw map[string]any
}

// Type 返回内容块类型。
func (b SearchResultBlock) Type() ContentBlockType {
	return ContentBlockTypeSearchResult
}

// RawPayload 返回原始负载副本。
func (b SearchResultBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// ResourceLinkBlock 表示资源链接内容块。
type ResourceLinkBlock struct {
	Description string
	Name        string
	URI         string

	raw map[string]any
}

// Type 返回内容块类型。
func (b ResourceLinkBlock) Type() ContentBlockType {
	return ContentBlockTypeResourceLink
}

// RawPayload 返回原始负载副本。
func (b ResourceLinkBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// ThinkingBlock 表示思考内容块。
type ThinkingBlock struct {
	Thinking  string
	Signature string

	raw map[string]any
}

// Type 返回内容块类型。
func (b ThinkingBlock) Type() ContentBlockType {
	return ContentBlockTypeThinking
}

// RawPayload 返回原始负载副本。
func (b ThinkingBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// ToolUseBlock 表示工具调用内容块。
type ToolUseBlock struct {
	ID    string
	Name  string
	Input json.RawMessage

	raw map[string]any
}

// Type 返回内容块类型。
func (b ToolUseBlock) Type() ContentBlockType {
	return ContentBlockTypeToolUse
}

// RawPayload 返回原始负载副本。
func (b ToolUseBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// DecodeInput 将工具输入解码到目标结构。
func (b ToolUseBlock) DecodeInput(target any) error {
	if len(b.Input) == 0 || target == nil {
		return nil
	}
	return json.Unmarshal(b.Input, target)
}

// InputMap 返回工具输入的 map 表示。
func (b ToolUseBlock) InputMap() map[string]any {
	if len(b.Input) == 0 {
		return map[string]any{}
	}
	result := map[string]any{}
	if err := json.Unmarshal(b.Input, &result); err != nil {
		return map[string]any{}
	}
	return result
}

// ToolResultBlock 表示工具结果内容块。
type ToolResultBlock struct {
	ToolUseID string
	Content   json.RawMessage
	IsError   bool
	MimeType  string

	raw map[string]any
}

// Type 返回内容块类型。
func (b ToolResultBlock) Type() ContentBlockType {
	return ContentBlockTypeToolResult
}

// RawPayload 返回原始负载副本。
func (b ToolResultBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// DecodeContent 将工具结果内容解码到目标结构。
func (b ToolResultBlock) DecodeContent(target any) error {
	if len(b.Content) == 0 || target == nil {
		return nil
	}
	return json.Unmarshal(b.Content, target)
}

// ContentString 返回字符串形式的工具结果内容。
func (b ToolResultBlock) ContentString() (string, bool) {
	if len(b.Content) == 0 {
		return "", false
	}
	var result string
	if err := json.Unmarshal(b.Content, &result); err != nil {
		return "", false
	}
	return result, true
}

// UnknownBlock 表示未知内容块。
type UnknownBlock struct {
	BlockType ContentBlockType

	raw map[string]any
}

// Type 返回内容块类型。
func (b UnknownBlock) Type() ContentBlockType {
	return b.BlockType
}

// RawPayload 返回原始负载副本。
func (b UnknownBlock) RawPayload() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.raw)
}

// AsTextBlock 将内容块断言为文本块。
func AsTextBlock(block ContentBlock) (TextBlock, bool) {
	switch value := block.(type) {
	case TextBlock:
		return value, true
	case *TextBlock:
		if value != nil {
			return *value, true
		}
	}
	return TextBlock{}, false
}

// AsThinkingBlock 将内容块断言为思考块。
func AsThinkingBlock(block ContentBlock) (ThinkingBlock, bool) {
	switch value := block.(type) {
	case ThinkingBlock:
		return value, true
	case *ThinkingBlock:
		if value != nil {
			return *value, true
		}
	}
	return ThinkingBlock{}, false
}

// AsImageBlock 将内容块断言为图片块。
func AsImageBlock(block ContentBlock) (ImageBlock, bool) {
	switch value := block.(type) {
	case ImageBlock:
		return value, true
	case *ImageBlock:
		if value != nil {
			return *value, true
		}
	}
	return ImageBlock{}, false
}

// AsDocumentBlock 将内容块断言为文档块。
func AsDocumentBlock(block ContentBlock) (DocumentBlock, bool) {
	switch value := block.(type) {
	case DocumentBlock:
		return value, true
	case *DocumentBlock:
		if value != nil {
			return *value, true
		}
	}
	return DocumentBlock{}, false
}

// AsSearchResultBlock 将内容块断言为搜索结果块。
func AsSearchResultBlock(block ContentBlock) (SearchResultBlock, bool) {
	switch value := block.(type) {
	case SearchResultBlock:
		return value, true
	case *SearchResultBlock:
		if value != nil {
			return *value, true
		}
	}
	return SearchResultBlock{}, false
}

// AsResourceLinkBlock 将内容块断言为资源链接块。
func AsResourceLinkBlock(block ContentBlock) (ResourceLinkBlock, bool) {
	switch value := block.(type) {
	case ResourceLinkBlock:
		return value, true
	case *ResourceLinkBlock:
		if value != nil {
			return *value, true
		}
	}
	return ResourceLinkBlock{}, false
}

// AsToolUseBlock 将内容块断言为工具调用块。
func AsToolUseBlock(block ContentBlock) (ToolUseBlock, bool) {
	switch value := block.(type) {
	case ToolUseBlock:
		return value, true
	case *ToolUseBlock:
		if value != nil {
			return *value, true
		}
	}
	return ToolUseBlock{}, false
}

// AsToolResultBlock 将内容块断言为工具结果块。
func AsToolResultBlock(block ContentBlock) (ToolResultBlock, bool) {
	switch value := block.(type) {
	case ToolResultBlock:
		return value, true
	case *ToolResultBlock:
		if value != nil {
			return *value, true
		}
	}
	return ToolResultBlock{}, false
}

// FirstTextBlockText 返回首个文本块的文本内容。
func FirstTextBlockText(blocks []ContentBlock) (string, bool) {
	for _, block := range blocks {
		text, ok := AsTextBlock(block)
		if !ok || text.Text == "" {
			continue
		}
		return text.Text, true
	}
	return "", false
}

func rawJSONValue(raw any) json.RawMessage {
	if raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	return encoded
}

// OutboundMessage 表示发送给 SDK 的强类型消息。
type OutboundMessage interface {
	encodeOutboundMessage(defaultSessionID string) map[string]any
}

// OutboundMessageOptions 描述发送给 SDK 的用户消息附加语义。
type OutboundMessageOptions struct {
	Meta           bool              `json:"is_meta,omitempty"`
	Synthetic      bool              `json:"is_synthetic,omitempty"`
	HiddenFromUser bool              `json:"hidden_from_user,omitempty"`
	Purpose        string            `json:"purpose,omitempty"`
	Priority       string            `json:"priority,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// OutboundContentBlock 表示发送给 SDK 的结构化内容块。
type OutboundContentBlock interface {
	encodeOutboundContentBlock() map[string]any
}

// UserTextMessage 表示文本用户消息。
type UserTextMessage struct {
	Text            string
	ParentToolUseID *string
	Options         OutboundMessageOptions
}

// NewUserTextMessage 创建文本用户消息。
func NewUserTextMessage(text string) UserTextMessage {
	return UserTextMessage{Text: text}
}

// NewUserTextMessageWithOptions 创建带附加语义的文本用户消息。
func NewUserTextMessageWithOptions(text string, options OutboundMessageOptions) UserTextMessage {
	return UserTextMessage{Text: text, Options: options.normalized()}
}

// WithParentToolUseID 为文本用户消息设置父工具调用 ID。
func (m UserTextMessage) WithParentToolUseID(toolUseID string) UserTextMessage {
	m.ParentToolUseID = &toolUseID
	return m
}

// WithOptions 为文本用户消息设置附加发送语义。
func (m UserTextMessage) WithOptions(options OutboundMessageOptions) UserTextMessage {
	m.Options = options.normalized()
	return m
}

func (m UserTextMessage) encodeOutboundMessage(defaultSessionID string) map[string]any {
	return buildUserOutboundMessage(defaultSessionID, m.ParentToolUseID, m.Text, m.Options)
}

// UserBlocksMessage 表示结构化用户消息。
type UserBlocksMessage struct {
	Blocks          []OutboundContentBlock
	ParentToolUseID *string
	Options         OutboundMessageOptions
}

// NewUserBlocksMessage 创建结构化用户消息。
func NewUserBlocksMessage(blocks ...OutboundContentBlock) UserBlocksMessage {
	return UserBlocksMessage{Blocks: append([]OutboundContentBlock(nil), blocks...)}
}

// NewUserBlocksMessageWithOptions 创建带附加语义的结构化用户消息。
func NewUserBlocksMessageWithOptions(options OutboundMessageOptions, blocks ...OutboundContentBlock) UserBlocksMessage {
	return UserBlocksMessage{
		Blocks:  append([]OutboundContentBlock(nil), blocks...),
		Options: options.normalized(),
	}
}

// WithParentToolUseID 为结构化用户消息设置父工具调用 ID。
func (m UserBlocksMessage) WithParentToolUseID(toolUseID string) UserBlocksMessage {
	m.ParentToolUseID = &toolUseID
	return m
}

// WithOptions 为结构化用户消息设置附加发送语义。
func (m UserBlocksMessage) WithOptions(options OutboundMessageOptions) UserBlocksMessage {
	m.Options = options.normalized()
	return m
}

func (m UserBlocksMessage) encodeOutboundMessage(defaultSessionID string) map[string]any {
	content := make([]map[string]any, 0, len(m.Blocks))
	for _, block := range m.Blocks {
		if block == nil {
			continue
		}
		content = append(content, block.encodeOutboundContentBlock())
	}
	return buildUserOutboundMessage(defaultSessionID, m.ParentToolUseID, content, m.Options)
}

// RawMessage 表示兼容场景下的原始 SDK 消息。
type RawMessage struct {
	Payload map[string]any
}

// NewRawMessage 创建原始 SDK 消息。
func NewRawMessage(payload map[string]any) RawMessage {
	return RawMessage{Payload: jsonvalue.CloneMapOrEmpty(payload)}
}

func (m RawMessage) encodeOutboundMessage(defaultSessionID string) map[string]any {
	payload := jsonvalue.CloneMapOrEmpty(m.Payload)
	if payload["session_id"] == nil && defaultSessionID != "" {
		payload["session_id"] = defaultSessionID
	}
	return payload
}

// TextContent 表示结构化文本块。
type TextContent struct {
	Text string
}

// NewTextContent 创建结构化文本块。
func NewTextContent(text string) TextContent {
	return TextContent{Text: text}
}

func (b TextContent) encodeOutboundContentBlock() map[string]any {
	return map[string]any{
		"type": "text",
		"text": b.Text,
	}
}

// ImageContent 表示结构化图片块。
type ImageContent struct {
	Data     string
	MIMEType string
}

// NewImageContent 创建结构化图片块。
func NewImageContent(data string, mimeType string) ImageContent {
	return ImageContent{Data: data, MIMEType: mimeType}
}

func (b ImageContent) encodeOutboundContentBlock() map[string]any {
	payload := map[string]any{
		"type": "image",
		"data": b.Data,
	}
	if b.MIMEType != "" {
		payload["mime_type"] = b.MIMEType
	}
	return payload
}

// DocumentContent 表示结构化文档块。
type DocumentContent struct {
	MIMEType string
	Source   any
	Title    string
}

// NewDocumentContent 创建结构化文档块。
func NewDocumentContent(source any, mimeType string, title string) DocumentContent {
	return DocumentContent{Source: source, MIMEType: mimeType, Title: title}
}

func (b DocumentContent) encodeOutboundContentBlock() map[string]any {
	payload := map[string]any{
		"type": "document",
	}
	if b.Source != nil {
		payload["source"] = b.Source
	}
	if b.MIMEType != "" {
		payload["mime_type"] = b.MIMEType
	}
	if b.Title != "" {
		payload["title"] = b.Title
	}
	return payload
}

// SearchResultContent 表示结构化搜索结果块。
type SearchResultContent struct {
	Query   string
	Source  string
	Snippet string
	Title   string
	URL     string
}

// NewSearchResultContent 创建结构化搜索结果块。
func NewSearchResultContent(query string, title string, url string, snippet string) SearchResultContent {
	return SearchResultContent{
		Query:   query,
		Title:   title,
		URL:     url,
		Snippet: snippet,
	}
}

func (b SearchResultContent) encodeOutboundContentBlock() map[string]any {
	payload := map[string]any{
		"type": "search_result",
	}
	if b.Query != "" {
		payload["query"] = b.Query
	}
	if b.Source != "" {
		payload["source"] = b.Source
	}
	if b.Title != "" {
		payload["title"] = b.Title
	}
	if b.URL != "" {
		payload["url"] = b.URL
	}
	if b.Snippet != "" {
		payload["snippet"] = b.Snippet
	}
	return payload
}

// ResourceLinkContent 表示结构化资源链接块。
type ResourceLinkContent struct {
	Description string
	Name        string
	URI         string
}

// NewResourceLinkContent 创建结构化资源链接块。
func NewResourceLinkContent(name string, uri string, description string) ResourceLinkContent {
	return ResourceLinkContent{Name: name, URI: uri, Description: description}
}

func (b ResourceLinkContent) encodeOutboundContentBlock() map[string]any {
	payload := map[string]any{
		"type": "resource_link",
	}
	if b.Name != "" {
		payload["name"] = b.Name
	}
	if b.URI != "" {
		payload["uri"] = b.URI
	}
	if b.Description != "" {
		payload["description"] = b.Description
	}
	return payload
}

// ToolResultContent 表示结构化工具结果块。
type ToolResultContent struct {
	ToolUseID string
	Content   any
	IsError   bool
}

// NewToolResultContent 创建结构化工具结果块。
func NewToolResultContent(toolUseID string, content any, isError bool) ToolResultContent {
	return ToolResultContent{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

func (b ToolResultContent) encodeOutboundContentBlock() map[string]any {
	payload := map[string]any{
		"type":        "tool_result",
		"tool_use_id": b.ToolUseID,
	}
	if b.Content != nil {
		payload["content"] = b.Content
	}
	if b.IsError {
		payload["is_error"] = true
	}
	return payload
}

// RawContent 表示兼容场景下的原始内容块。
type RawContent struct {
	Payload map[string]any
}

// NewRawContent 创建原始内容块。
func NewRawContent(payload map[string]any) RawContent {
	return RawContent{Payload: jsonvalue.CloneMapOrEmpty(payload)}
}

func (b RawContent) encodeOutboundContentBlock() map[string]any {
	return jsonvalue.CloneMapOrEmpty(b.Payload)
}

// EncodeOutboundMessage 将强类型消息编码为 SDK 原始负载。
func EncodeOutboundMessage(message OutboundMessage, defaultSessionID string) map[string]any {
	if message == nil {
		return map[string]any{}
	}
	return message.encodeOutboundMessage(defaultSessionID)
}

// EncodeOutboundMessageWithOptions 将强类型消息编码为 SDK 原始负载并应用发送选项。
func EncodeOutboundMessageWithOptions(message OutboundMessage, defaultSessionID string, options OutboundMessageOptions) map[string]any {
	payload := EncodeOutboundMessage(message, defaultSessionID)
	return ApplyOutboundMessageOptions(payload, options)
}

// ApplyOutboundMessageOptions 将发送选项应用到原始 SDK 消息副本。
func ApplyOutboundMessageOptions(payload map[string]any, options OutboundMessageOptions) map[string]any {
	result := jsonvalue.CloneMapPreserveTypedSlices(payload)
	if result == nil {
		result = map[string]any{}
	}
	options = options.normalized()
	if options.Meta {
		result["is_meta"] = true
	}
	if options.Synthetic || options.Meta {
		result["is_synthetic"] = true
	}
	if options.HiddenFromUser {
		result["hidden_from_user"] = true
	}
	if options.Purpose != "" {
		result["purpose"] = options.Purpose
	}
	if options.Priority != "" {
		result["priority"] = options.Priority
	}
	if len(options.Metadata) > 0 {
		result["metadata"] = jsonvalue.CloneStringMap(options.Metadata)
	}
	return result
}

func buildUserOutboundMessage(sessionID string, parentToolUseID *string, content any, options OutboundMessageOptions) map[string]any {
	payload := map[string]any{
		"type":               "user",
		"parent_tool_use_id": parentToolUseID,
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}
	if sessionID != "" {
		payload["session_id"] = sessionID
	}
	return ApplyOutboundMessageOptions(payload, options)
}

func (o OutboundMessageOptions) normalized() OutboundMessageOptions {
	o.Purpose = strings.TrimSpace(o.Purpose)
	o.Priority = strings.TrimSpace(o.Priority)
	o.Metadata = jsonvalue.CloneStringMap(o.Metadata)
	if o.Meta {
		o.Synthetic = true
	}
	return o
}

// MessageType 表示 SDK 接收消息的顶层类型。
type MessageType string

const (
	// MessageTypeSystem 表示系统消息。
	MessageTypeSystem MessageType = "system"
	// MessageTypeUser 表示用户消息。
	MessageTypeUser MessageType = "user"
	// MessageTypeAssistant 表示助手消息。
	MessageTypeAssistant MessageType = "assistant"
	// MessageTypeResult 表示结果消息。
	MessageTypeResult MessageType = "result"
	// MessageTypeStreamEvent 表示流式事件。
	MessageTypeStreamEvent MessageType = "stream_event"
	// MessageTypeStreamRequestStart 表示一次模型流式请求即将开始。
	MessageTypeStreamRequestStart MessageType = "stream_request_start"
	// MessageTypeToolProgress 表示工具进度消息。
	MessageTypeToolProgress MessageType = "tool_progress"
	// MessageTypeToolUseSummary 表示工具摘要消息。
	MessageTypeToolUseSummary MessageType = "tool_use_summary"
	// MessageTypeRateLimitEvent 表示限流消息。
	MessageTypeRateLimitEvent MessageType = "rate_limit_event"
	// MessageTypeTaskStarted 表示任务开始消息。
	MessageTypeTaskStarted MessageType = "task_started"
	// MessageTypeTaskProgress 表示任务进度消息。
	MessageTypeTaskProgress MessageType = "task_progress"
	// MessageTypeTaskNotification 表示任务通知消息。
	MessageTypeTaskNotification MessageType = "task_notification"
	// MessageTypePromptSuggestion 表示提示建议消息。
	MessageTypePromptSuggestion MessageType = "prompt_suggestion"
	// MessageTypeAuthStatus 表示鉴权状态消息。
	MessageTypeAuthStatus MessageType = "auth_status"
	// MessageTypeUnknown 表示未知消息。
	MessageTypeUnknown MessageType = "unknown"
)

// ConversationEnvelope 表示 assistant / user 内部 message 结构。
type ConversationEnvelope struct {
	ID         string         `json:"id,omitempty"`
	Role       string         `json:"role,omitempty"`
	Model      string         `json:"model,omitempty"`
	Content    []ContentBlock `json:"content,omitempty"`
	Usage      map[string]any `json:"usage,omitempty"`
	StopReason any            `json:"stop_reason,omitempty"`
	Additional map[string]any `json:"additional,omitempty"`
}

// UserMessage 表示用户消息。
type UserMessage struct {
	Message         ConversationEnvelope `json:"message"`
	IsMeta          bool                 `json:"is_meta,omitempty"`
	IsReplay        bool                 `json:"is_replay,omitempty"`
	IsSynthetic     bool                 `json:"is_synthetic,omitempty"`
	ToolUseResult   any                  `json:"tool_use_result,omitempty"`
	Priority        string               `json:"priority,omitempty"`
	Timestamp       string               `json:"timestamp,omitempty"`
	ParentToolUseID *string              `json:"parent_tool_use_id,omitempty"`
}

// AssistantMessage 表示助手消息。
type AssistantMessage struct {
	Message         ConversationEnvelope `json:"message"`
	Error           string               `json:"error,omitempty"`
	APIError        string               `json:"api_error,omitempty"`
	ErrorDetails    string               `json:"error_details,omitempty"`
	IsAPIError      bool                 `json:"is_api_error_message,omitempty"`
	ParentToolUseID *string              `json:"parent_tool_use_id,omitempty"`
}

// StreamEvent 表示流式事件。
type StreamEvent struct {
	Event any            `json:"event,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}

// ToolProgressMessage 表示工具运行进度。
type ToolProgressMessage struct {
	ToolUseID          string         `json:"tool_use_id,omitempty"`
	ToolName           string         `json:"tool_name,omitempty"`
	ParentToolUseID    *string        `json:"parent_tool_use_id,omitempty"`
	ElapsedTimeSeconds float64        `json:"elapsed_time_seconds,omitempty"`
	TaskID             string         `json:"task_id,omitempty"`
	Additional         map[string]any `json:"additional,omitempty"`
}

// ToolUseSummaryMessage 表示工具摘要。
type ToolUseSummaryMessage struct {
	Summary             string         `json:"summary,omitempty"`
	PrecedingToolUseIDs []string       `json:"preceding_tool_use_ids,omitempty"`
	Additional          map[string]any `json:"additional,omitempty"`
}

// RateLimitEvent 表示限流信息。
type RateLimitEvent struct {
	RateLimitInfo map[string]any `json:"rate_limit_info,omitempty"`
}

// PromptSuggestionMessage 表示提示建议。
type PromptSuggestionMessage struct {
	Suggestion string `json:"suggestion,omitempty"`
}

// AuthStatusMessage 表示认证状态消息。
type AuthStatusMessage struct {
	IsAuthenticating bool           `json:"is_authenticating,omitempty"`
	Output           []string       `json:"output,omitempty"`
	Error            string         `json:"error,omitempty"`
	Additional       map[string]any `json:"additional,omitempty"`
}

// ReceivedMessage 表示统一接收消息。
type ReceivedMessage struct {
	Type             MessageType              `json:"type"`
	Subtype          string                   `json:"subtype,omitempty"`
	SessionID        string                   `json:"session_id,omitempty"`
	UUID             string                   `json:"uuid,omitempty"`
	ParentToolUseID  *string                  `json:"parent_tool_use_id,omitempty"`
	User             *UserMessage             `json:"user,omitempty"`
	Assistant        *AssistantMessage        `json:"assistant,omitempty"`
	System           *SystemMessage           `json:"system,omitempty"`
	Result           *ResultMessage           `json:"result,omitempty"`
	Stream           *StreamEvent             `json:"stream,omitempty"`
	ToolProgress     *ToolProgressMessage     `json:"tool_progress,omitempty"`
	ToolUseSummary   *ToolUseSummaryMessage   `json:"tool_use_summary,omitempty"`
	RateLimit        *RateLimitEvent          `json:"rate_limit,omitempty"`
	TaskStarted      *TaskStartedMessage      `json:"task_started,omitempty"`
	TaskProgress     *TaskProgressMessage     `json:"task_progress,omitempty"`
	TaskNotification *TaskNotificationMessage `json:"task_notification,omitempty"`
	TaskUpdated      *TaskUpdatedMessage      `json:"task_updated,omitempty"`
	PromptSuggestion *PromptSuggestionMessage `json:"prompt_suggestion,omitempty"`
	AuthStatus       *AuthStatusMessage       `json:"auth_status,omitempty"`
	Raw              map[string]any           `json:"raw,omitempty"`
}

// PermissionDenial 表示权限拒绝明细。
type PermissionDenial struct {
	ToolName  string         `json:"tool_name,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`
}

// ResultMessage 表示最终结果。
type ResultMessage struct {
	Subtype           string             `json:"subtype,omitempty"`
	DurationMS        int                `json:"duration_ms,omitempty"`
	DurationAPIMS     int                `json:"duration_api_ms,omitempty"`
	IsError           bool               `json:"is_error,omitempty"`
	NumTurns          int                `json:"num_turns,omitempty"`
	Result            string             `json:"result,omitempty"`
	StopReason        any                `json:"stop_reason,omitempty"`
	TerminalReason    string             `json:"terminal_reason,omitempty"`
	TotalCostUSD      float64            `json:"total_cost_usd,omitempty"`
	Usage             map[string]any     `json:"usage,omitempty"`
	ModelUsage        map[string]any     `json:"model_usage,omitempty"`
	PermissionDenials []PermissionDenial `json:"permission_denials,omitempty"`
	Errors            []string           `json:"errors,omitempty"`
	StructuredOutput  any                `json:"structured_output,omitempty"`
	FastModeState     string             `json:"fast_mode_state,omitempty"`
	Additional        map[string]any     `json:"additional,omitempty"`
}

func decodeResultMessage(payload map[string]any) *ResultMessage {
	return &ResultMessage{
		Subtype:        jsonvalue.StringValue(payload["subtype"]),
		DurationMS:     jsonvalue.IntValue(payload["duration_ms"]),
		DurationAPIMS:  jsonvalue.IntValue(payload["duration_api_ms"]),
		IsError:        jsonvalue.BoolValue(payload["is_error"]),
		NumTurns:       jsonvalue.IntValue(payload["num_turns"]),
		Result:         jsonvalue.StringValue(payload["result"]),
		StopReason:     payload["stop_reason"],
		TerminalReason: jsonvalue.StringValue(payload["terminal_reason"]),
		TotalCostUSD:   jsonvalue.FloatValue(payload["total_cost_usd"]),
		Usage:          jsonvalue.MapValue(payload["usage"]),
		// nxs 使用 canonical snake_case，Claude Code 的公开 SDK 协议使用
		// modelUsage。这里只对公共字段显式兼容，避免全局键名转换污染控制协议。
		ModelUsage:        jsonvalue.MapValue(jsonvalue.FirstNonNil(payload["model_usage"], payload["modelUsage"])),
		PermissionDenials: decodePermissionDenials(payload["permission_denials"]),
		Errors:            jsonvalue.StringSliceValue(payload["errors"]),
		StructuredOutput:  payload["structured_output"],
		FastModeState:     jsonvalue.StringValue(payload["fast_mode_state"]),
		Additional:        payload,
	}
}

func decodePermissionDenials(raw any) []PermissionDenial {
	items := jsonvalue.SliceValue(raw)
	results := make([]PermissionDenial, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}
		results = append(results, PermissionDenial{
			ToolName:  jsonvalue.StringValue(payload["tool_name"]),
			ToolUseID: jsonvalue.StringValue(payload["tool_use_id"]),
			ToolInput: jsonvalue.MapValue(payload["tool_input"]),
		})
	}
	return results
}

// InitMCPServerStatus 表示 init 消息中的 MCP 服务状态。
type InitMCPServerStatus struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

// InitPlugin 表示 init 消息中已加载的插件。
type InitPlugin struct {
	Name   string `json:"name,omitempty"`
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
}

// InitSystemMessage 表示 `system/init` 消息。
type InitSystemMessage struct {
	Agents            []string              `json:"agents,omitempty"`
	APIKeySource      string                `json:"api_key_source,omitempty"`
	Betas             []string              `json:"betas,omitempty"`
	RuntimeVersion    string                `json:"runtime_version,omitempty"`
	ClaudeCodeVersion string                `json:"claude_code_version,omitempty"`
	CWD               string                `json:"cwd,omitempty"`
	Tools             []string              `json:"tools,omitempty"`
	MCPServers        []InitMCPServerStatus `json:"mcp_servers,omitempty"`
	Model             string                `json:"model,omitempty"`
	PermissionMode    permission.Mode       `json:"permission_mode,omitempty"`
	SlashCommands     []string              `json:"slash_commands,omitempty"`
	OutputStyle       string                `json:"output_style,omitempty"`
	Skills            []string              `json:"skills,omitempty"`
	Plugins           []InitPlugin          `json:"plugins,omitempty"`
	FastModeState     string                `json:"fast_mode_state,omitempty"`
	Additional        map[string]any        `json:"additional,omitempty"`
}

// StatusSystemMessage 表示 `system/status` 消息。
type StatusSystemMessage struct {
	Status         string          `json:"status,omitempty"`
	PermissionMode permission.Mode `json:"permission_mode,omitempty"`
	Additional     map[string]any  `json:"additional,omitempty"`
}

// InformationalSystemMessage 表示 informational 系统消息。
type InformationalSystemMessage struct {
	Content             string         `json:"content,omitempty"`
	Level               string         `json:"level,omitempty"`
	ToolUseID           string         `json:"tool_use_id,omitempty"`
	PreventContinuation bool           `json:"prevent_continuation,omitempty"`
	Additional          map[string]any `json:"additional,omitempty"`
}

// MemorySavedMessage 表示后台记忆任务实际新增或改进的 topic 文件。
type MemorySavedMessage struct {
	Verb         string         `json:"verb,omitempty"`
	WrittenPaths []string       `json:"written_paths,omitempty"`
	Additional   map[string]any `json:"additional,omitempty"`
}

// TaskStartedMessage 表示任务开始消息。
type TaskStartedMessage struct {
	TaskID         string         `json:"task_id,omitempty"`
	ToolUseID      string         `json:"tool_use_id,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	AgentType      string         `json:"agent_type,omitempty"`
	ChildSessionID string         `json:"child_session_id,omitempty"`
	Description    string         `json:"description,omitempty"`
	TaskType       string         `json:"task_type,omitempty"`
	WorkflowName   string         `json:"workflow_name,omitempty"`
	OutputFile     string         `json:"output_file,omitempty"`
	ParentTaskID   string         `json:"parent_task_id,omitempty"`
	Prompt         string         `json:"prompt,omitempty"`
	Additional     map[string]any `json:"additional,omitempty"`
}

// TaskUsage 表示任务进度中的 usage。
type TaskUsage struct {
	TotalTokens int `json:"total_tokens,omitempty"`
	ToolUses    int `json:"tool_uses,omitempty"`
	DurationMS  int `json:"duration_ms,omitempty"`
}

// TaskProgressMessage 表示任务进度消息。
type TaskProgressMessage struct {
	TaskID         string         `json:"task_id,omitempty"`
	ToolUseID      string         `json:"tool_use_id,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	AgentType      string         `json:"agent_type,omitempty"`
	ChildSessionID string         `json:"child_session_id,omitempty"`
	Description    string         `json:"description,omitempty"`
	TaskType       string         `json:"task_type,omitempty"`
	LastToolName   string         `json:"last_tool_name,omitempty"`
	ParentTaskID   string         `json:"parent_task_id,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	Usage          TaskUsage      `json:"usage,omitempty"`
	Additional     map[string]any `json:"additional,omitempty"`
}

// TaskNotificationMessage 表示任务通知消息。
type TaskNotificationMessage struct {
	TaskID         string         `json:"task_id,omitempty"`
	ToolUseID      string         `json:"tool_use_id,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	AgentType      string         `json:"agent_type,omitempty"`
	ChildSessionID string         `json:"child_session_id,omitempty"`
	ParentTaskID   string         `json:"parent_task_id,omitempty"`
	Status         string         `json:"status,omitempty"`
	OutputFile     string         `json:"output_file,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	TranscriptPath string         `json:"transcript_path,omitempty"`
	Usage          TaskUsage      `json:"usage,omitempty"`
	Additional     map[string]any `json:"additional,omitempty"`
}

// TaskUpdatedPatch 表示 task_updated 生命周期补丁。
type TaskUpdatedPatch struct {
	Status         string         `json:"status,omitempty"`
	Description    string         `json:"description,omitempty"`
	EndTime        int64          `json:"end_time,omitempty"`
	TotalPausedMS  int            `json:"total_paused_ms,omitempty"`
	Error          string         `json:"error,omitempty"`
	IsBackgrounded bool           `json:"is_backgrounded,omitempty"`
	Additional     map[string]any `json:"additional,omitempty"`
}

// TaskUpdatedMessage 表示后台任务状态变更消息。
type TaskUpdatedMessage struct {
	TaskID     string           `json:"task_id,omitempty"`
	Patch      TaskUpdatedPatch `json:"patch,omitempty"`
	Status     string           `json:"status,omitempty"`
	Additional map[string]any   `json:"additional,omitempty"`
}

// SystemMessage 表示统一系统消息。
type SystemMessage struct {
	Subtype          string                      `json:"subtype,omitempty"`
	Init             *InitSystemMessage          `json:"init,omitempty"`
	Status           *StatusSystemMessage        `json:"status,omitempty"`
	Informational    *InformationalSystemMessage `json:"informational,omitempty"`
	MemorySaved      *MemorySavedMessage         `json:"memory_saved,omitempty"`
	TaskStarted      *TaskStartedMessage         `json:"task_started,omitempty"`
	TaskProgress     *TaskProgressMessage        `json:"task_progress,omitempty"`
	TaskNotification *TaskNotificationMessage    `json:"task_notification,omitempty"`
	TaskUpdated      *TaskUpdatedMessage         `json:"task_updated,omitempty"`
	Data             map[string]any              `json:"data,omitempty"`
}

func decodeSystemMessage(payload map[string]any) *SystemMessage {
	system := &SystemMessage{
		Subtype: jsonvalue.StringValue(payload["subtype"]),
		Data:    payload,
	}

	switch system.Subtype {
	case "init":
		claudeCodeVersion := jsonvalue.StringValue(payload["claude_code_version"])
		system.Init = &InitSystemMessage{
			Agents:            jsonvalue.StringSliceValue(payload["agents"]),
			APIKeySource:      jsonvalue.FirstNonEmptyString(payload["api_key_source"], payload["apiKeySource"]),
			Betas:             jsonvalue.StringSliceValue(payload["betas"]),
			RuntimeVersion:    jsonvalue.FirstNonEmptyString(payload["runtime_version"], claudeCodeVersion),
			ClaudeCodeVersion: claudeCodeVersion,
			CWD:               jsonvalue.StringValue(payload["cwd"]),
			Tools:             jsonvalue.StringSliceValue(payload["tools"]),
			MCPServers:        decodeMCPServerStatus(payload["mcp_servers"]),
			Model:             jsonvalue.StringValue(payload["model"]),
			PermissionMode:    permission.Mode(jsonvalue.FirstNonEmptyString(payload["permission_mode"], payload["permissionMode"])),
			SlashCommands:     jsonvalue.StringSliceValue(payload["slash_commands"]),
			OutputStyle:       jsonvalue.StringValue(payload["output_style"]),
			Skills:            jsonvalue.StringSliceValue(payload["skills"]),
			Plugins:           decodeInitPlugins(payload["plugins"]),
			FastModeState:     jsonvalue.StringValue(payload["fast_mode_state"]),
			Additional:        payload,
		}
	case "status":
		system.Status = &StatusSystemMessage{
			Status:         jsonvalue.StringValue(payload["status"]),
			PermissionMode: permission.Mode(jsonvalue.FirstNonEmptyString(payload["permission_mode"], payload["permissionMode"])),
			Additional:     payload,
		}
	case "informational":
		system.Informational = &InformationalSystemMessage{
			Content:             jsonvalue.StringValue(payload["content"]),
			Level:               jsonvalue.StringValue(payload["level"]),
			ToolUseID:           jsonvalue.StringValue(payload["tool_use_id"]),
			PreventContinuation: jsonvalue.BoolValue(payload["prevent_continuation"]),
			Additional:          payload,
		}
	case "memory_saved":
		system.MemorySaved = &MemorySavedMessage{
			Verb:         jsonvalue.StringValue(payload["verb"]),
			WrittenPaths: jsonvalue.StringSliceValue(jsonvalue.FirstNonNil(payload["written_paths"], payload["writtenPaths"])),
			Additional:   payload,
		}
	case "task_started":
		system.TaskStarted = decodeTaskStartedMessage(payload)
	case "task_progress":
		system.TaskProgress = decodeTaskProgressMessage(payload)
	case "task_notification":
		system.TaskNotification = decodeTaskNotificationMessage(payload)
	case "task_updated":
		system.TaskUpdated = decodeTaskUpdatedMessage(payload)
	}

	return system
}

func decodeInitPlugins(raw any) []InitPlugin {
	items := jsonvalue.SliceValue(raw)
	plugins := make([]InitPlugin, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}
		plugins = append(plugins, InitPlugin{
			Name:   jsonvalue.StringValue(payload["name"]),
			Path:   jsonvalue.StringValue(payload["path"]),
			Source: jsonvalue.StringValue(payload["source"]),
		})
	}
	return plugins
}

func normalizeSDKAssistantMessageError(raw string, status *int) string {
	switch raw {
	case "authentication_failed", "billing_error", "rate_limit", "invalid_request", "server_error", "unknown", "max_output_tokens":
		return raw
	}
	if strings.Contains(raw, "max_output_tokens") {
		return "max_output_tokens"
	}
	if strings.Contains(raw, "billing") {
		return "billing_error"
	}
	if strings.Contains(raw, "invalid_request") || strings.Contains(raw, "request_too_large") {
		return "invalid_request"
	}
	if strings.Contains(raw, "overloaded") || strings.Contains(raw, "rate_limit") {
		return "rate_limit"
	}
	if strings.Contains(raw, "authentication") || strings.Contains(raw, "permission") || strings.Contains(raw, "oauth") || strings.Contains(raw, "token") {
		return "authentication_failed"
	}
	if status == nil {
		if raw == "" {
			return ""
		}
		return "unknown"
	}
	switch {
	case *status == 402:
		return "billing_error"
	case *status == 401 || *status == 403:
		return "authentication_failed"
	case *status == 400 || *status == 404 || *status == 413:
		return "invalid_request"
	case *status == 429 || *status == 529:
		return "rate_limit"
	case *status >= 408:
		return "server_error"
	default:
		return "unknown"
	}
}

func decodeMCPServerStatus(raw any) []InitMCPServerStatus {
	items := jsonvalue.SliceValue(raw)
	servers := make([]InitMCPServerStatus, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}
		servers = append(servers, InitMCPServerStatus{
			Name:   jsonvalue.StringValue(payload["name"]),
			Status: jsonvalue.StringValue(payload["status"]),
		})
	}
	return servers
}

func decodeTaskStartedMessage(payload map[string]any) *TaskStartedMessage {
	return &TaskStartedMessage{
		TaskID:         jsonvalue.StringValue(payload["task_id"]),
		ToolUseID:      jsonvalue.StringValue(payload["tool_use_id"]),
		AgentID:        jsonvalue.StringValue(payload["agent_id"]),
		AgentType:      jsonvalue.StringValue(payload["agent_type"]),
		ChildSessionID: jsonvalue.StringValue(payload["child_session_id"]),
		Description:    jsonvalue.StringValue(payload["description"]),
		TaskType:       jsonvalue.StringValue(payload["task_type"]),
		WorkflowName:   jsonvalue.StringValue(payload["workflow_name"]),
		OutputFile:     jsonvalue.StringValue(payload["output_file"]),
		ParentTaskID:   jsonvalue.StringValue(payload["parent_task_id"]),
		Prompt:         jsonvalue.StringValue(payload["prompt"]),
		Additional:     payload,
	}
}

func decodeTaskProgressMessage(payload map[string]any) *TaskProgressMessage {
	return &TaskProgressMessage{
		TaskID:         jsonvalue.StringValue(payload["task_id"]),
		ToolUseID:      jsonvalue.StringValue(payload["tool_use_id"]),
		AgentID:        jsonvalue.StringValue(payload["agent_id"]),
		AgentType:      jsonvalue.StringValue(payload["agent_type"]),
		ChildSessionID: jsonvalue.StringValue(payload["child_session_id"]),
		Description:    jsonvalue.StringValue(payload["description"]),
		TaskType:       jsonvalue.StringValue(payload["task_type"]),
		LastToolName:   jsonvalue.StringValue(payload["last_tool_name"]),
		ParentTaskID:   jsonvalue.StringValue(payload["parent_task_id"]),
		Summary:        jsonvalue.StringValue(payload["summary"]),
		Usage:          decodeTaskUsage(payload["usage"]),
		Additional:     payload,
	}
}

func decodeTaskNotificationMessage(payload map[string]any) *TaskNotificationMessage {
	return &TaskNotificationMessage{
		TaskID:         jsonvalue.StringValue(payload["task_id"]),
		ToolUseID:      jsonvalue.StringValue(payload["tool_use_id"]),
		AgentID:        jsonvalue.StringValue(payload["agent_id"]),
		AgentType:      jsonvalue.StringValue(payload["agent_type"]),
		ChildSessionID: jsonvalue.StringValue(payload["child_session_id"]),
		ParentTaskID:   jsonvalue.StringValue(payload["parent_task_id"]),
		Status:         jsonvalue.StringValue(payload["status"]),
		OutputFile:     jsonvalue.StringValue(payload["output_file"]),
		Summary:        jsonvalue.StringValue(payload["summary"]),
		TranscriptPath: jsonvalue.StringValue(payload["transcript_path"]),
		Usage:          decodeTaskUsage(payload["usage"]),
		Additional:     payload,
	}
}

func decodeTaskUpdatedMessage(payload map[string]any) *TaskUpdatedMessage {
	patch := decodeTaskUpdatedPatch(payload["patch"])
	return &TaskUpdatedMessage{
		TaskID:     jsonvalue.StringValue(payload["task_id"]),
		Patch:      patch,
		Status:     patch.Status,
		Additional: payload,
	}
}

func decodeTaskUpdatedPatch(raw any) TaskUpdatedPatch {
	payload := jsonvalue.MapValue(raw)
	return TaskUpdatedPatch{
		Status:         jsonvalue.StringValue(payload["status"]),
		Description:    jsonvalue.StringValue(payload["description"]),
		EndTime:        int64(jsonvalue.IntValue(payload["end_time"])),
		TotalPausedMS:  jsonvalue.IntValue(payload["total_paused_ms"]),
		Error:          jsonvalue.StringValue(payload["error"]),
		IsBackgrounded: jsonvalue.BoolValue(payload["is_backgrounded"]),
		Additional:     payload,
	}
}

func decodeTaskUsage(raw any) TaskUsage {
	payload := jsonvalue.MapValue(raw)
	return TaskUsage{
		TotalTokens: jsonvalue.IntValue(payload["total_tokens"]),
		ToolUses:    jsonvalue.IntValue(payload["tool_uses"]),
		DurationMS:  jsonvalue.IntValue(payload["duration_ms"]),
	}
}

// ParseMessage 解析原始 JSON 消息。
func ParseMessage(raw []byte) (ReceivedMessage, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ReceivedMessage{}, NewJSONDecodeErrorWithCause("protocol: unmarshal message failed", string(raw), err)
	}
	return DecodeMessage(payload)
}

// DecodeMessage 将 map 解析为统一消息结构。
func DecodeMessage(payload map[string]any) (ReceivedMessage, error) {
	messageType := MessageType(jsonvalue.StringValue(payload["type"]))
	if messageType == "" {
		return ReceivedMessage{}, NewMessageParseError("protocol: message type missing")
	}

	message := ReceivedMessage{
		Type:            normalizeMessageType(messageType),
		Subtype:         jsonvalue.StringValue(payload["subtype"]),
		SessionID:       jsonvalue.StringValue(payload["session_id"]),
		UUID:            jsonvalue.StringValue(payload["uuid"]),
		ParentToolUseID: jsonvalue.StringPointer(payload["parent_tool_use_id"]),
		Raw:             payload,
	}

	switch message.Type {
	case MessageTypeUser:
		isMeta := jsonvalue.BoolValue(payload["is_meta"])
		message.User = &UserMessage{
			Message:         decodeConversationEnvelope(payload["message"]),
			IsMeta:          isMeta,
			IsReplay:        jsonvalue.BoolValue(payload["is_replay"]) || jsonvalue.BoolValue(payload["isReplay"]),
			IsSynthetic:     jsonvalue.BoolValue(payload["is_synthetic"]) || jsonvalue.BoolValue(payload["isSynthetic"]) || isMeta,
			ToolUseResult:   payload["tool_use_result"],
			Priority:        jsonvalue.StringValue(payload["priority"]),
			Timestamp:       jsonvalue.StringValue(payload["timestamp"]),
			ParentToolUseID: message.ParentToolUseID,
		}
	case MessageTypeAssistant:
		apiError := jsonvalue.StringValue(payload["api_error"])
		errorStatus := jsonvalue.FirstNonNilIntPointer(payload["error_status"], payload["status"], jsonvalue.MapValue(payload["error"])["status"])
		rawError := jsonvalue.FirstNonEmptyString(
			payload["error"],
			payload["error_type"],
			payload["category"],
			apiError,
			jsonvalue.MapValue(payload["error"])["type"],
			jsonvalue.MapValue(payload["error"])["message"],
		)
		message.Assistant = &AssistantMessage{
			Message:         decodeConversationEnvelope(payload["message"]),
			Error:           normalizeSDKAssistantMessageError(rawError, errorStatus),
			APIError:        apiError,
			ErrorDetails:    jsonvalue.StringValue(payload["error_details"]),
			IsAPIError:      jsonvalue.BoolValue(payload["is_api_error_message"]),
			ParentToolUseID: message.ParentToolUseID,
		}
	case MessageTypeSystem:
		message.System = decodeSystemMessage(payload)
	case MessageTypeResult:
		message.Result = decodeResultMessage(payload)
	case MessageTypeStreamEvent:
		message.Stream = &StreamEvent{
			Event: payload["event"],
			Data:  payload,
		}
	case MessageTypeStreamRequestStart:
	case MessageTypeToolProgress:
		message.ToolProgress = &ToolProgressMessage{
			ToolUseID:          jsonvalue.StringValue(payload["tool_use_id"]),
			ToolName:           jsonvalue.StringValue(payload["tool_name"]),
			ParentToolUseID:    jsonvalue.StringPointer(payload["parent_tool_use_id"]),
			ElapsedTimeSeconds: jsonvalue.FloatValue(payload["elapsed_time_seconds"]),
			TaskID:             jsonvalue.StringValue(payload["task_id"]),
			Additional:         payload,
		}
	case MessageTypeToolUseSummary:
		message.ToolUseSummary = &ToolUseSummaryMessage{
			Summary:             jsonvalue.StringValue(payload["summary"]),
			PrecedingToolUseIDs: jsonvalue.StringSliceValue(payload["preceding_tool_use_ids"]),
			Additional:          payload,
		}
	case MessageTypeRateLimitEvent:
		message.RateLimit = &RateLimitEvent{
			RateLimitInfo: jsonvalue.MapValue(payload["rate_limit_info"]),
		}
	case MessageTypeTaskStarted:
		message.TaskStarted = decodeTaskStartedMessage(payload)
	case MessageTypeTaskProgress:
		message.TaskProgress = decodeTaskProgressMessage(payload)
	case MessageTypeTaskNotification:
		message.TaskNotification = decodeTaskNotificationMessage(payload)
	case MessageTypePromptSuggestion:
		message.PromptSuggestion = &PromptSuggestionMessage{
			Suggestion: jsonvalue.StringValue(payload["suggestion"]),
		}
	case MessageTypeAuthStatus:
		message.AuthStatus = &AuthStatusMessage{
			IsAuthenticating: jsonvalue.BoolValue(payload["is_authenticating"]) || jsonvalue.BoolValue(payload["isAuthenticating"]),
			Output:           jsonvalue.StringSliceValue(payload["output"]),
			Error:            jsonvalue.StringValue(payload["error"]),
			Additional:       payload,
		}
	default:
	}

	return message, nil
}

func normalizeMessageType(messageType MessageType) MessageType {
	switch messageType {
	case MessageTypeSystem,
		MessageTypeUser,
		MessageTypeAssistant,
		MessageTypeResult,
		MessageTypeStreamEvent,
		MessageTypeStreamRequestStart,
		MessageTypeToolProgress,
		MessageTypeToolUseSummary,
		MessageTypeRateLimitEvent,
		MessageTypeTaskStarted,
		MessageTypeTaskProgress,
		MessageTypeTaskNotification,
		MessageTypePromptSuggestion,
		MessageTypeAuthStatus:
		return messageType
	default:
		return messageType
	}
}

func decodeConversationEnvelope(raw any) ConversationEnvelope {
	payload := jsonvalue.MapValue(raw)
	return ConversationEnvelope{
		ID:         jsonvalue.StringValue(payload["id"]),
		Role:       jsonvalue.StringValue(payload["role"]),
		Model:      jsonvalue.StringValue(payload["model"]),
		Content:    decodeConversationContent(payload["content"]),
		Usage:      jsonvalue.MapValue(payload["usage"]),
		StopReason: payload["stop_reason"],
		Additional: payload,
	}
}

func decodeConversationContent(raw any) []ContentBlock {
	if text := jsonvalue.StringValue(raw); text != "" {
		return []ContentBlock{TextBlock{
			Text: text,
			raw: map[string]any{
				"type": "text",
				"text": text,
			},
		}}
	}
	return decodeContentBlocks(raw)
}

func decodeContentBlocks(raw any) []ContentBlock {
	items := jsonvalue.SliceValue(raw)
	blocks := make([]ContentBlock, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}

		switch ContentBlockType(jsonvalue.StringValue(payload["type"])) {
		case ContentBlockTypeText:
			blocks = append(blocks, TextBlock{
				Text: jsonvalue.StringValue(payload["text"]),
				raw:  jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeImage:
			blocks = append(blocks, ImageBlock{
				Data:     jsonvalue.StringValue(payload["data"]),
				MIMEType: jsonvalue.StringValue(payload["mime_type"]),
				raw:      jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeDocument:
			blocks = append(blocks, DocumentBlock{
				MIMEType: jsonvalue.StringValue(payload["mime_type"]),
				Source:   rawJSONValue(payload["source"]),
				Title:    jsonvalue.StringValue(payload["title"]),
				raw:      jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeSearchResult:
			blocks = append(blocks, SearchResultBlock{
				Query:   jsonvalue.StringValue(payload["query"]),
				Source:  jsonvalue.StringValue(payload["source"]),
				Title:   jsonvalue.StringValue(payload["title"]),
				URL:     jsonvalue.StringValue(payload["url"]),
				Snippet: jsonvalue.StringValue(payload["snippet"]),
				raw:     jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeResourceLink:
			blocks = append(blocks, ResourceLinkBlock{
				Description: jsonvalue.StringValue(payload["description"]),
				Name:        jsonvalue.StringValue(payload["name"]),
				URI:         jsonvalue.StringValue(payload["uri"]),
				raw:         jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeThinking:
			blocks = append(blocks, ThinkingBlock{
				Thinking:  jsonvalue.StringValue(payload["thinking"]),
				Signature: jsonvalue.StringValue(payload["signature"]),
				raw:       jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeToolUse:
			blocks = append(blocks, ToolUseBlock{
				ID:    jsonvalue.StringValue(payload["id"]),
				Name:  jsonvalue.StringValue(payload["name"]),
				Input: rawJSONValue(payload["input"]),
				raw:   jsonvalue.CloneMapOrEmpty(payload),
			})
		case ContentBlockTypeToolResult:
			blocks = append(blocks, ToolResultBlock{
				ToolUseID: jsonvalue.StringValue(payload["tool_use_id"]),
				Content:   rawJSONValue(payload["content"]),
				IsError:   jsonvalue.BoolValue(payload["is_error"]),
				MimeType:  jsonvalue.StringValue(payload["mime_type"]),
				raw:       jsonvalue.CloneMapOrEmpty(payload),
			})
		default:
			blocks = append(blocks, UnknownBlock{
				BlockType: ContentBlockType(jsonvalue.StringValue(payload["type"])),
				raw:       jsonvalue.CloneMapOrEmpty(payload),
			})
		}
	}
	return blocks
}
