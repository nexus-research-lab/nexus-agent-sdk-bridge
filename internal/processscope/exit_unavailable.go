//go:build !darwin || !cgo

// INPUT: 不支持原生退出观察的构建。
// OUTPUT: 不可用，不退回 PID 存活轮询。
// POS: 原生根进程观察的平台拒绝边界。
package processscope

import "net"

func (*Scope) WatchRoot(*net.UnixConn, int) (*ExitMonitor, error) { return nil, ErrUnavailable }
func (*ExitMonitor) Close() error                                 { return ErrUnavailable }
