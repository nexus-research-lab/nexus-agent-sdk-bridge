// INPUT: 当前连接代次确认的沙箱能力及宿主必需选项
// OUTPUT: 模型输入写入前的失败关闭检查
// POS: 沙箱启动与并发消息发送之间的准入边界
package client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// claudeRestrictedRequired 报告宿主是否选择了 Claude Code 原生受限启动合同。
// 它不会从无类型的额外参数推断该合同。
func claudeRestrictedRequired(options Options) bool {
	return options.Sandbox != nil && options.Sandbox.RequireClaudeRestricted
}

// validateClaudeRestrictedOptions 在启动 transport 前校验 Claude 专属的类型化
// 启动合同。标记的实际执行仍由 Claude 负责；Bridge 只证明将以该参数启动所选程序。
func validateClaudeRestrictedOptions(options Options) error {
	settings := options.Sandbox
	if settings == nil || !settings.RequireClaudeRestricted {
		for _, arg := range options.ExtraBoolArgs {
			if strings.TrimLeft(strings.TrimSpace(arg), "-") == "restricted" {
				return fmt.Errorf("client: Claude --restricted must be requested with SandboxSettings.RequireClaudeRestricted")
			}
		}
		for key := range options.ExtraArgs {
			if strings.TrimLeft(strings.TrimSpace(key), "-") == "restricted" {
				return fmt.Errorf("client: Claude --restricted must be requested with SandboxSettings.RequireClaudeRestricted")
			}
		}
		return nil
	}
	if normalizedRuntimeKind(options.Runtime.Kind) != RuntimeClaude {
		return &UnsupportedCapabilityError{Capability: CapabilityClaudeRestricted}
	}
	if options.Runtime.PermissionMode == permission.ModeBypassPermissions || options.Runtime.AllowDangerouslySkipPermissions {
		return errors.New("client: Claude restricted mode cannot be combined with bypass permissions")
	}
	for _, arg := range options.ExtraBoolArgs {
		normalized := strings.TrimLeft(strings.TrimSpace(arg), "-")
		if normalized == "restricted" {
			return errors.New("client: Claude --restricted must be supplied by SandboxSettings.RequireClaudeRestricted exactly once")
		}
		if normalized == "dangerously-skip-permissions" {
			return errors.New("client: Claude restricted mode cannot be combined with bypass permissions")
		}
	}
	for key := range options.ExtraArgs {
		if strings.TrimLeft(strings.TrimSpace(key), "-") == "restricted" {
			return errors.New("client: Claude --restricted must be supplied by SandboxSettings.RequireClaudeRestricted exactly once")
		}
	}
	return nil
}

// validateSandboxRequirements 拒绝互相矛盾的要求，以及未实现该协议的运行时。
func (c *sessionCore) validateSandboxRequirements() error {
	if err := validateClaudeRestrictedOptions(c.options); err != nil {
		return err
	}
	settings := c.options.Sandbox
	if settings == nil {
		return nil
	}
	if err := validateSandboxResources(settings); err != nil {
		return err
	}
	if settings.RequireFileTools && !settings.RequireSandbox {
		return errors.New("文件工具沙箱要求同时启用必需沙箱")
	}
	if settings.RequireSearchTools && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("搜索工具沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireMediaFiles && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("媒体文件沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireSkillFiles && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("Skill 文件沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireContextFiles && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("上下文文件沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireProjectFiles && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("项目定义文件沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireManagedPolicy && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("托管策略沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireSettingsFiles && (!settings.RequireSandbox || !settings.RequireFileTools) {
		return errors.New("普通配置沙箱要求同时启用必需沙箱和文件工具合同")
	}
	if settings.RequireSettingsWrites && (!settings.RequireSandbox || !settings.RequireFileTools || !settings.RequireSettingsFiles) {
		return errors.New("普通配置写入沙箱要求同时启用必需沙箱、文件工具和普通配置读取合同")
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
	if c.options.Sandbox != nil && c.options.Sandbox.RequireSearchTools && !c.supports(CapabilitySandboxSearchTools) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxSearchTools}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireMediaFiles && !c.supports(CapabilitySandboxMediaFiles) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxMediaFiles}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireSkillFiles && !c.supports(CapabilitySandboxSkillFiles) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxSkillFiles}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireContextFiles && !c.supports(CapabilitySandboxContextFiles) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxContextFiles}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireProjectFiles && !c.supports(CapabilitySandboxProjectFiles) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxProjectFiles}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireManagedPolicy && !c.supports(CapabilitySandboxManagedPolicy) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxManagedPolicy}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireSettingsFiles && !c.supports(CapabilitySandboxSettingsFiles) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxSettingsFiles}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.RequireSettingsWrites && !c.supports(CapabilitySandboxSettingsWrites) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxSettingsWrites}
	}
	if c.options.Sandbox != nil && c.options.Sandbox.Resources != nil && !c.supports(CapabilitySandboxResources) {
		return &UnsupportedCapabilityError{Capability: CapabilitySandboxResources}
	}
	return nil
}
