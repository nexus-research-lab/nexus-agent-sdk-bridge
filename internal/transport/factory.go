package transport

// NewDirectConnectTransport 创建带 Claude Code wire 兼容层的 direct-connect 传输。
func NewDirectConnectTransport(config DirectConnectConfig) Transport {
	return newControlCodecTransport(NewDirectConnectManager(config))
}

// NewProcessTransport 创建带 Claude Code wire 兼容层的本地进程传输。
func NewProcessTransport(config ProcessConfig) Transport {
	return newControlCodecTransport(NewProcessManager(config))
}
