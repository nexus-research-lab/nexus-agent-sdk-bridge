package transport

// ControlWireDialect 表示 runtime 进程使用的 control wire 方言。
type ControlWireDialect string

const (
	// ControlWireDialectClaude 表示 Claude Code 的 SDK control wire。
	ControlWireDialectClaude ControlWireDialect = "claude"
	// ControlWireDialectNXS 表示原生 nxs 使用的 Claude 对齐 SDK control wire。
	ControlWireDialectNXS ControlWireDialect = "nxs"
)

// NewDirectConnectTransport 创建 direct-connect 传输。
func NewDirectConnectTransport(config DirectConnectConfig) Transport {
	return NewDirectConnectManager(config)
}

// NewProcessTransport 创建本地进程传输。
func NewProcessTransport(config ProcessConfig) Transport {
	return NewProcessManager(config)
}
