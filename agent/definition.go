package agent

import (
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// Definition 表示可通过 client.Options 注入的子 agent 定义。
type Definition struct {
	AgentType                          string          `json:"agent_type"`
	Description                        string          `json:"description"`
	Tools                              []string        `json:"tools,omitempty"`
	DisallowedTools                    []string        `json:"disallowed_tools,omitempty"`
	Prompt                             string          `json:"prompt"`
	Source                             string          `json:"source,omitempty"`
	Model                              string          `json:"model,omitempty"`
	MCPServers                         []any           `json:"mcp_servers,omitempty"`
	RequiredMCPServers                 []string        `json:"required_mcp_servers,omitempty"`
	CriticalSystemReminderExperimental string          `json:"critical_system_reminder_experimental,omitempty"`
	Skills                             []string        `json:"skills,omitempty"`
	InitialPrompt                      string          `json:"initial_prompt,omitempty"`
	MaxTurns                           int             `json:"max_turns,omitempty"`
	Background                         *bool           `json:"background,omitempty"`
	Memory                             string          `json:"memory,omitempty"`
	Isolation                          string          `json:"isolation,omitempty"`
	OmitAgentMd                        bool            `json:"omit_agent_md,omitempty"`
	Effort                             any             `json:"effort,omitempty"`
	PermissionMode                     permission.Mode `json:"permission_mode,omitempty"`
}

// Clone 返回 Definition 的可变字段副本。
func (d Definition) Clone() Definition {
	d.Tools = append([]string(nil), d.Tools...)
	d.DisallowedTools = append([]string(nil), d.DisallowedTools...)
	d.MCPServers = jsonvalue.CloneAnySlice(d.MCPServers)
	d.RequiredMCPServers = append([]string(nil), d.RequiredMCPServers...)
	d.Skills = append([]string(nil), d.Skills...)
	d.Effort = jsonvalue.CloneValue(d.Effort)
	return d
}

// ToMap 编码为 runtime 控制面使用的 agent payload。
func (d Definition) ToMap() map[string]any {
	result := map[string]any{
		"agent_type":  d.AgentType,
		"description": d.Description,
		"prompt":      d.Prompt,
	}
	if len(d.Tools) > 0 {
		result["tools"] = append([]string(nil), d.Tools...)
	}
	if len(d.DisallowedTools) > 0 {
		result["disallowed_tools"] = append([]string(nil), d.DisallowedTools...)
	}
	if d.Source != "" {
		result["source"] = d.Source
	}
	if d.Model != "" {
		result["model"] = d.Model
	}
	if len(d.MCPServers) > 0 {
		result["mcp_servers"] = jsonvalue.CloneAnySlice(d.MCPServers)
	}
	if len(d.RequiredMCPServers) > 0 {
		result["required_mcp_servers"] = append([]string(nil), d.RequiredMCPServers...)
	}
	if d.CriticalSystemReminderExperimental != "" {
		result["critical_system_reminder_experimental"] = d.CriticalSystemReminderExperimental
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
	if d.Isolation != "" {
		result["isolation"] = d.Isolation
	}
	if d.OmitAgentMd {
		result["omit_agent_md"] = true
	}
	if d.Effort != nil {
		result["effort"] = jsonvalue.CloneValue(d.Effort)
	}
	if d.PermissionMode != "" {
		result["permission_mode"] = d.PermissionMode
	}
	return result
}

// EncodeDefinitions 编码一组 agent 定义。
func EncodeDefinitions(definitions map[string]Definition) map[string]any {
	if len(definitions) == 0 {
		return nil
	}
	result := make(map[string]any, len(definitions))
	for name, definition := range definitions {
		result[name] = definition.ToMap()
	}
	return result
}
