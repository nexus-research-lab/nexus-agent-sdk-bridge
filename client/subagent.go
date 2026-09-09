// INPUT: 活跃 SDK MCP 调用 identity 与宿主批准的子智能体操作。
// OUTPUT: 能力协商后的原生子智能体控制请求；取消沿同一 control request 传播。
// POS: 宿主 MCP 到 runtime 执行器的窄接口，不创建额外进程。
package client

import (
	"context"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ControlSubagent 只在 runtime 的当前 SDK MCP 调用期间操作当前会话的子智能体。
func (c *SessionControl) ControlSubagent(ctx context.Context, toolUseID, operation string, input map[string]any) (map[string]any, error) {
	core, err := c.activeCore()
	if err != nil {
		return nil, err
	}
	if !core.supports(CapabilitySubagentControl) {
		return nil, &UnsupportedCapabilityError{Capability: CapabilitySubagentControl}
	}
	return core.sendControlRequest(ctx, protocol.ControlRequest{
		Subtype: "subagent_control", ToolUseID: toolUseID, Operation: operation, Input: input,
	}, 0)
}
