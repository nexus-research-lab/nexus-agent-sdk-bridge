// INPUT: 当前连接代次确认的沙箱能力及宿主必需选项
// OUTPUT: 模型输入写入前的失败关闭检查
// POS: 沙箱启动与并发消息发送之间的准入边界
package client

import "errors"

// validateSandboxRequirements 拒绝互相矛盾的要求，以及未实现该协议的运行时。
func (c *sessionCore) validateSandboxRequirements() error {
	settings := c.options.Sandbox
	if settings == nil {
		return nil
	}
	if settings.RequireFileTools && !settings.RequireSandbox {
		return errors.New("文件工具沙箱要求同时启用必需沙箱")
	}
	if settings.RequireSandbox && normalizedRuntimeKind(c.options.Runtime.Kind) != RuntimeNXS {
		return &UnsupportedCapabilityError{Capability: CapabilityRequiredSandbox}
	}
	return nil
}

// requireSandboxReadyForSend 独立于 connected 标记：transport 建立后仍可能正在握手。
// 普通会话保留既有启动语义；必需沙箱未确认时不得消费下一轮上下文或写入用户任务。
func (c *sessionCore) requireSandboxReadyForSend() error {
	if err := c.validateSandboxRequirements(); err != nil {
		return err
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireSandbox && !c.supports(CapabilityRequiredSandbox) {
		return &UnsupportedCapabilityError{Capability: CapabilityRequiredSandbox}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireFileTools && !c.supports(CapabilitySandboxFileTools) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxFileTools}
	}
	return nil
}
