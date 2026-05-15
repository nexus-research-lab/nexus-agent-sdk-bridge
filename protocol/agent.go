package protocol

import "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

// AgentDefinition 表示自定义子代理定义。
type AgentDefinition struct {
	Description     string          `json:"description"`
	Tools           []string        `json:"tools,omitempty"`
	DisallowedTools []string        `json:"disallowed_tools,omitempty"`
	Prompt          string          `json:"prompt"`
	Model           string          `json:"model,omitempty"`
	MCPServers      []any           `json:"mcp_servers,omitempty"`
	Skills          []string        `json:"skills,omitempty"`
	InitialPrompt   string          `json:"initial_prompt,omitempty"`
	MaxTurns        int             `json:"max_turns,omitempty"`
	Background      *bool           `json:"background,omitempty"`
	Memory          string          `json:"memory,omitempty"`
	Effort          any             `json:"effort,omitempty"`
	PermissionMode  permission.Mode `json:"permission_mode,omitempty"`
}

// Clone 返回 AgentDefinition 的可变切片副本。
func (d AgentDefinition) Clone() AgentDefinition {
	d.Tools = append([]string(nil), d.Tools...)
	d.DisallowedTools = append([]string(nil), d.DisallowedTools...)
	d.MCPServers = append([]any(nil), d.MCPServers...)
	d.Skills = append([]string(nil), d.Skills...)
	return d
}

// ToMap 将 AgentDefinition 编码为控制协议 agents 负载。
func (d AgentDefinition) ToMap() map[string]any {
	result := map[string]any{
		"description": d.Description,
		"prompt":      d.Prompt,
	}
	if len(d.Tools) > 0 {
		result["tools"] = append([]string(nil), d.Tools...)
	}
	if len(d.DisallowedTools) > 0 {
		result["disallowed_tools"] = append([]string(nil), d.DisallowedTools...)
	}
	if d.Model != "" {
		result["model"] = d.Model
	}
	if len(d.MCPServers) > 0 {
		result["mcp_servers"] = append([]any(nil), d.MCPServers...)
	}
	if len(d.Skills) > 0 {
		result["skills"] = append([]string(nil), d.Skills...)
	}
	if d.InitialPrompt != "" {
		result["initial_prompt"] = d.InitialPrompt
	}
	if d.MaxTurns > 0 {
		result["max_turns"] = d.MaxTurns
	}
	if d.Background != nil {
		result["background"] = *d.Background
	}
	if d.Memory != "" {
		result["memory"] = d.Memory
	}
	if d.Effort != nil {
		result["effort"] = d.Effort
	}
	if d.PermissionMode != "" {
		result["permission_mode"] = d.PermissionMode
	}
	return result
}

// EncodeAgentDefinitions 编码 agents 控制负载。
func EncodeAgentDefinitions(definitions map[string]AgentDefinition) map[string]any {
	if len(definitions) == 0 {
		return nil
	}
	result := make(map[string]any, len(definitions))
	for name, definition := range definitions {
		result[name] = definition.ToMap()
	}
	return result
}
