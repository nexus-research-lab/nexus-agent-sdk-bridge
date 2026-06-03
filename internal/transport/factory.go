package transport

// ControlWireDialect 表示 runtime 进程使用的 control wire 字段风格。
type ControlWireDialect string

const (
	// ControlWireDialectClaude 表示 Claude Code 的 camelCase control wire。
	ControlWireDialectClaude ControlWireDialect = "claude"
	// ControlWireDialectSnake 表示 Nexus 原生 snake_case control wire。
	ControlWireDialectSnake ControlWireDialect = "snake"
)

// NewDirectConnectTransport 创建带 Claude Code wire 兼容层的 direct-connect 传输。
func NewDirectConnectTransport(config DirectConnectConfig) Transport {
	return newControlCodecTransport(NewDirectConnectManager(config))
}

// NewProcessTransport 创建本地进程传输。
func NewProcessTransport(config ProcessConfig) Transport {
	manager := NewProcessManager(config)
	if config.ControlWireDialect == ControlWireDialectSnake {
		return manager
	}
	return newControlCodecTransport(manager)
}
