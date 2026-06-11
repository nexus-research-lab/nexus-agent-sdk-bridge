package client

// Capability 表示当前会话后端公开的运行时能力。
type Capability string

// 支持的会话后端能力。
const (
	CapabilitySendOptions      Capability = "send_options"
	CapabilityInternalContext  Capability = "internal_context"
	CapabilityTypedUsage       Capability = "typed_usage"
	CapabilityTerminalCategory Capability = "terminal_category"
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
	case CapabilitySendOptions, CapabilityInternalContext, CapabilityTypedUsage, CapabilityTerminalCategory:
		return true
	default:
		return false
	}
}
