// INPUT: 已登记根进程的内核退出观察。
// OUTPUT: 主进程退出码或信号；取消等待和停止观察不能成为退出证据。
// POS: 主进程生命周期与集合回收独立，RootExit 不能授权 scratch 回收。
package processscope

import (
	"context"
	"errors"
	"sync"
)

var ErrObservationStopped = errors.New("root exit observation stopped without exit evidence")

// RootExit 只描述被登记主进程；Code 为正常退出码，Signal 为终止信号，两者互斥。
type RootExit struct {
	PID    int
	Code   int
	Signal int
}

// ExitMonitor 的内核等待由组件拥有，调用者取消不取消共享观察。
type ExitMonitor struct {
	mu     sync.Mutex
	fd     int
	pid    int
	done   chan struct{}
	result RootExit
	err    error
}

// Wait 只取消本次等待；已捕获的退出事实可被后续调用继续读取。
func (m *ExitMonitor) Wait(ctx context.Context) (RootExit, error) {
	if ctx == nil {
		return RootExit{}, errors.New("root exit wait requires context")
	}
	select {
	case <-m.done:
		return m.result, m.err
	default:
	}
	select {
	case <-m.done:
		return m.result, m.err
	case <-ctx.Done():
		return RootExit{}, ctx.Err()
	}
}
