package client

import "slices"

// Capability 表示当前会话后端公开的运行时能力。
type Capability string

// 支持的会话后端能力。
const (
	CapabilityRequiredSandbox        Capability = "required_sandbox"
	CapabilitySandboxFileTools       Capability = "sandbox_file_tools"
	CapabilitySandboxSearchTools     Capability = "sandbox_search_tools"
	CapabilitySandboxMediaFiles      Capability = "sandbox_media_files"
	CapabilitySandboxSkillFiles      Capability = "sandbox_skill_files"
	CapabilitySandboxContextFiles    Capability = "sandbox_context_files"
	CapabilitySandboxProjectFiles    Capability = "sandbox_project_files"
	CapabilitySandboxManagedPolicy   Capability = "sandbox_managed_policy"
	CapabilitySandboxResources       Capability = "sandbox_resources"
	CapabilityAutoReview             Capability = "auto_review"
	CapabilitySendOptions            Capability = "send_options"
	CapabilityInternalContext        Capability = "internal_context"
	CapabilityTypedUsage             Capability = "typed_usage"
	CapabilityTerminalCategory       Capability = "terminal_category"
	CapabilityStopTask               Capability = "stop_task"
	CapabilityInProcessMCP           Capability = "in_process_mcp"
	CapabilitySendTaskMessage        Capability = "send_task_message"
	CapabilityAutoDream              Capability = "auto_dream"
	CapabilityUpdateEnvironment      Capability = "update_environment"
	CapabilityHookResponseAck        Capability = "hook_response_ack"
	CapabilityMessageExecutionPolicy Capability = "message_execution_policy"
	CapabilityRuntimeLifecycle       Capability = "runtime_lifecycle"
	CapabilitySessionFork            Capability = "session_fork"
	CapabilitySubagentControl        Capability = "subagent_control"
)

const (
	subagentControlProtocolCapability        = "subagent_control_v1"
	autoReviewProtocolCapability             = "auto_review_v1"
	requiredSandboxProtocolCapability        = "required_sandbox_v1"
	sandboxFileToolsProtocolCapability       = "sandbox_file_tools_v1"
	sandboxSearchToolsProtocolCapability     = "sandbox_search_tools_v1"
	sandboxMediaFilesProtocolCapability      = "sandbox_media_files_v1"
	sandboxSkillFilesProtocolCapability      = "sandbox_skill_files_v1"
	sandboxContextFilesProtocolCapability    = "sandbox_context_files_v1"
	sandboxProjectFilesProtocolCapability    = "sandbox_project_files_v1"
	sandboxManagedPolicyProtocolCapability   = "sandbox_managed_policy_v1"
	sandboxResourcesProtocolCapability       = "sandbox_resources_v1"
	hookResponseAckProtocolCapability        = "hook_response_ack_v1"
	messageExecutionPolicyProtocolCapability = "message_execution_policy_v1"
)

// InternalContextBlock 表示下一轮可注入的内部上下文块。
type InternalContextBlock struct {
	Name     string            `json:"name,omitempty"`
	Content  string            `json:"content,omitempty"`
	Priority int               `json:"priority,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Supports 判断当前会话是否支持某个能力。
func (s *Session) Supports(capability Capability) bool {
	if s == nil || s.core == nil {
		return false
	}
	return s.core.supports(capability)
}

func (c *sessionCore) supports(capability Capability) bool {
	switch capability {
	case CapabilitySendOptions,
		CapabilityInternalContext,
		CapabilityTypedUsage,
		CapabilityTerminalCategory,
		CapabilityStopTask,
		CapabilityInProcessMCP,
		CapabilityRuntimeLifecycle,
		CapabilitySessionFork:
		return true
	case CapabilitySendTaskMessage, CapabilityAutoDream:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS
	case CapabilityUpdateEnvironment:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS
	case CapabilityRequiredSandbox:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, requiredSandboxProtocolCapability)
	case CapabilitySandboxFileTools:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxFileToolsProtocolCapability)
	case CapabilitySandboxSearchTools:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxSearchToolsProtocolCapability)
	case CapabilitySandboxMediaFiles:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxMediaFilesProtocolCapability)
	case CapabilitySandboxSkillFiles:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxSkillFilesProtocolCapability)
	case CapabilitySandboxContextFiles:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxContextFilesProtocolCapability)
	case CapabilitySandboxProjectFiles:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxProjectFilesProtocolCapability)
	case CapabilitySandboxManagedPolicy:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxManagedPolicyProtocolCapability)
	case CapabilitySandboxResources:
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeNXS && slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, sandboxResourcesProtocolCapability)
	case CapabilityAutoReview:
		// Claude 使用原生控制确认模式；nxs 使用扩展协议协商。此能力不代表账号或模型可用性。
		return normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeClaude || slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, autoReviewProtocolCapability)
	case CapabilityHookResponseAck:
		return slices.Contains(
			c.lifecycle.initializeResponseValue().ProtocolCapabilities,
			hookResponseAckProtocolCapability,
		)
	case CapabilitySubagentControl:
		return slices.Contains(c.lifecycle.initializeResponseValue().ProtocolCapabilities, subagentControlProtocolCapability)
	case CapabilityMessageExecutionPolicy:
		return slices.Contains(
			c.lifecycle.initializeResponseValue().ProtocolCapabilities,
			messageExecutionPolicyProtocolCapability,
		)
	default:
		return false
	}
}
