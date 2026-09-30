//go:build darwin && cgo

// INPUT: 尚未放行任务的已认证引导连接与登记集合。
// OUTPUT: 在执行前绑定的 kqueue 退出事件，不依赖 PID 轮询或 launchd job 消失。
// POS: 仅观察根进程，不把 NOTE_EXIT 当作脱离后代回收证明。
package processscope

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// WatchRoot 必须在向 helper 放行之前调用；注册前后均绑定同一个连接内核身份。
func (s *Scope) WatchRoot(conn *net.UnixConn, pid int) (*ExitMonitor, error) {
	if s == nil {
		return nil, errors.New("process scope is required")
	}
	verify := func() error {
		current, err := CapturePeer(conn, pid)
		if err != nil {
			return err
		}
		if current.Registration() != s.registration {
			return errors.New("root observer does not match registered scope")
		}
		return nil
	}
	if err := verify(); err != nil {
		return nil, err
	}
	// kqueue 没有可移植的原子 CLOEXEC 创建入口，和 Go 的并发 ForkExec 共用锁，
	// 避免创建与设标记之间把观察句柄借给其他 runtime。
	syscall.ForkLock.Lock()
	fd, err := unix.Kqueue()
	if err == nil {
		if _, err = unix.FcntlInt(uintptr(fd), unix.F_SETFD, unix.FD_CLOEXEC); err != nil {
			_ = unix.Close(fd)
		}
	}
	syscall.ForkLock.Unlock()
	if err != nil {
		return nil, err
	}
	changes := []unix.Kevent_t{
		{Ident: 1, Filter: unix.EVFILT_USER, Flags: unix.EV_ADD | unix.EV_CLEAR},
		{Ident: uint64(pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS},
	}
	if _, err := unix.Kevent(fd, changes, nil, nil); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("register root exit: %w", err)
	}
	if err := verify(); err != nil {
		unix.Close(fd)
		return nil, err
	}
	monitor := &ExitMonitor{fd: fd, pid: pid, done: make(chan struct{})}
	go monitor.observe()
	return monitor, nil
}

func (m *ExitMonitor) observe() {
	var result RootExit
	var observed error
	defer func() {
		m.mu.Lock()
		closeErr := unix.Close(m.fd)
		m.fd = -1
		m.result = result
		m.err = errors.Join(observed, closeErr)
		close(m.done)
		m.mu.Unlock()
	}()
	var events [2]unix.Kevent_t
	for {
		n, err := unix.Kevent(m.fd, nil, events[:], nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			observed = fmt.Errorf("observe root exit: %w", err)
			return
		}
		stopped := false
		for _, event := range events[:n] {
			if event.Flags&unix.EV_ERROR != 0 {
				observed = fmt.Errorf("root exit event: %w", syscall.Errno(event.Data))
				return
			}
			if event.Filter == unix.EVFILT_PROC && event.Ident == uint64(m.pid) && event.Fflags&unix.NOTE_EXIT != 0 {
				if event.Fflags&unix.NOTE_EXITSTATUS == 0 {
					observed = errors.New("root exit event lacks status evidence")
					return
				}
				result, observed = decodeRootExit(m.pid, event.Data)
				return
			}
			if event.Filter == unix.EVFILT_USER && event.Ident == 1 {
				stopped = true
			} else {
				observed = errors.New("unexpected root observation event")
				return
			}
		}
		if stopped {
			observed = ErrObservationStopped
			return
		}
	}
}

func decodeRootExit(pid int, data int64) (RootExit, error) {
	if data < 0 || data > 65535 {
		return RootExit{}, errors.New("invalid root exit status")
	}
	status := syscall.WaitStatus(data)
	switch {
	case status.Exited():
		return RootExit{PID: pid, Code: status.ExitStatus()}, nil
	case status.Signaled():
		return RootExit{PID: pid, Signal: int(status.Signal())}, nil
	default:
		return RootExit{}, errors.New("root event has no terminal status")
	}
}

// Close 主动停止观察并释放句柄；它不终止进程，也不产生清理成功证明。
func (m *ExitMonitor) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.fd < 0 {
		m.mu.Unlock()
		return nil
	}
	change := []unix.Kevent_t{{Ident: 1, Filter: unix.EVFILT_USER, Fflags: unix.NOTE_TRIGGER}}
	var err error
	for {
		_, err = unix.Kevent(m.fd, change, nil, nil)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	m.mu.Unlock()
	if err != nil {
		return fmt.Errorf("stop root observation: %w", err)
	}
	<-m.done
	return nil
}
