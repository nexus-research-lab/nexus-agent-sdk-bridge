package mcp

// ToolStatusAnnotations 表示 MCP status 中的工具注解。
type ToolStatusAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint,omitempty"`
	DestructiveHint bool `json:"destructiveHint,omitempty"`
	IdempotentHint  bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   bool `json:"openWorldHint,omitempty"`
	ReadOnly        bool `json:"readOnly,omitempty"`
	Destructive     bool `json:"destructive,omitempty"`
	OpenWorld       bool `json:"openWorld,omitempty"`
}

// ToolInfo 表示 MCP status 中的工具信息。
type ToolInfo struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]any         `json:"inputSchema,omitempty"`
	Meta        map[string]any         `json:"_meta,omitempty"`
	Annotations *ToolStatusAnnotations `json:"annotations,omitempty"`
}

// ServerInfo 表示 MCP 服务器信息。
type ServerInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// ServerStatus 表示 MCP 连接状态。
type ServerStatus struct {
	Name       string         `json:"name,omitempty"`
	Status     string         `json:"status,omitempty"`
	ServerInfo *ServerInfo    `json:"serverInfo,omitempty"`
	Error      string         `json:"error,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Tools      []ToolInfo     `json:"tools,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// StatusResponse 表示 MCP 状态响应。
type StatusResponse struct {
	MCPServers []ServerStatus `json:"mcpServers,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// SetServersResult 表示动态替换 MCP 服务的结果。
type SetServersResult struct {
	Added   []string          `json:"added,omitempty"`
	Removed []string          `json:"removed,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
	Raw     map[string]any    `json:"raw,omitempty"`
}
