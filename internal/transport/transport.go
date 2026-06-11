package transport

import (
	"context"
	"errors"
	"os"
	"strings"
)

// ErrInterruptUnsupported 表示传输层不能直接中断底层执行。
var ErrInterruptUnsupported = errors.New("transport interrupt unsupported")

// Transport 表示 SDK client 与底层 runtime 之间的传输契约。
type Transport interface {
	Start(context.Context) error
	ReadJSON() (map[string]any, error)
	WriteJSON(any) error
	EndInput() error
	Interrupt() error
	Wait() error
	Close() error
}

// IsTransportWriteFailure 判断 err 是否表示 runtime 输入流已不可写。
func IsTransportWriteFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrClosed) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "write payload failed") ||
		strings.Contains(message, "stdin unavailable") ||
		strings.Contains(message, "websocket unavailable") ||
		strings.Contains(message, "file already closed") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "closed pipe")
}

// ChannelClosed 判断 signal 是否已经关闭；nil channel 按未关闭处理。
func ChannelClosed(signal <-chan struct{}) bool {
	if signal == nil {
		return false
	}
	select {
	case <-signal:
		return true
	default:
		return false
	}
}
