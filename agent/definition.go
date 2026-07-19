package agent

import (
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// Definition 表示可通过 client.Options 注入的子 agent 定义。
type Definition struct {
	AgentType                          string          `json:"-"`
	Description                        string          `json:"description"`
	Tools                              []string        `json:"tools,omitempty"`
	DisallowedTools                    []string        `json:"disallowedTools,omitempty"`
	Prompt                             string          `json:"prompt"`
	Source                             string          `json:"-"`
	Model                              string          `json:"model,omitempty"`
	MCPServers                         []any           `json:"mcpServers,omitempty"`
	RequiredMCPServers                 []string        `json:"requiredMcpServers,omitempty"`
	CriticalSystemReminderExperimental string          `json:"criticalSystemReminder_EXPERIMENTAL,omitempty"`
	Skills                             []string        `json:"skills,omitempty"`
	InitialPrompt                      string          `json:"initialPrompt,omitempty"`
	MaxTurns                           int             `json:"maxTurns,omitempty"`
	Background                         *bool           `json:"background,omitempty"`
	Memory                             string          `json:"memory,omitempty"`
	Isolation                          string          `json:"isolation,omitempty"`
	OmitAgentMd                        bool            `json:"-"`
	Effort                             any             `json:"effort,omitempty"`
	PermissionMode                     permission.Mode `json:"permissionMode,omitempty"`
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
		"description": d.Description,
		"prompt":      d.Prompt,
	}
	if len(d.Tools) > 0 {
		result["tools"] = append([]string(nil), d.Tools...)
	}
	if len(d.DisallowedTools) > 0 {
		result["disallowedTools"] = append([]string(nil), d.DisallowedTools...)
	}
	if d.Model != "" {
		result["model"] = d.Model
	}
	if len(d.MCPServers) > 0 {
		result["mcpServers"] = jsonvalue.CloneAnySlice(d.MCPServers)
	}
	if len(d.RequiredMCPServers) > 0 {
		result["requiredMcpServers"] = append([]string(nil), d.RequiredMCPServers...)
	}
	if d.CriticalSystemReminderExperimental != "" {
		result["criticalSystemReminder_EXPERIMENTAL"] = d.CriticalSystemReminderExperimental
	}
	if len(d.Skills) > 0 {
		result["skills"] = append([]string(nil), d.Skills...)
	}
	if d.InitialPrompt != "" {
		result["initialPrompt"] = d.InitialPrompt
	}
	if d.MaxTurns > 0 {
		result["maxTurns"] = d.MaxTurns
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
	if d.Effort != nil {
		result["effort"] = jsonvalue.CloneValue(d.Effort)
	}
	if d.PermissionMode != "" {
		result["permissionMode"] = d.PermissionMode
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
