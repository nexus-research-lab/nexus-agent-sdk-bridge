// INPUT: 当前连接代次确认的沙箱能力及宿主必需选项
// OUTPUT: 模型输入写入前的失败关闭检查
// POS: 沙箱启动与并发消息发送之间的准入边界
package client

// requireSandboxReadyForSend 独立于 connected 标记：transport 建立后仍可能正在握手。
// 普通会话保留既有启动语义；必需沙箱未确认时不得消费下一轮上下文或写入用户任务。
func (c *sessionCore) requireSandboxReadyForSend() error {
	if c.options.Sandbox != nil && c.options.Sandbox.RequireSandbox && !c.supports(CapabilityRequiredSandbox) {
		return &UnsupportedCapabilityError{Capability: CapabilityRequiredSandbox}
	}
	return nil
}
