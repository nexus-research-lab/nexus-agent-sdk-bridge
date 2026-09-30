// INPUT: 已准备的启动状态、主进程观察和 exact 集合身份。
// OUTPUT: 不可重放的运行句柄与共享关闭结果；调用者取消仅停止等待。
// POS: 主进程退出与完整集合回收分离，资源终态由 Host 持久确认。
package supervision

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

type Process struct {
	intent     Intent
	host       Host
	paths      Paths
	reserved   bool
	submitted  bool
	scope      *processscope.Scope
	monitor    *processscope.ExitMonitor
	streams    Streams
	childFiles [3]*os.File
	closeOnce  sync.Once
	closed     chan struct{}
	closeErr   error
	startErr   error
}

func (p *Process) Intent() Intent   { return p.intent }
func (p *Process) Streams() Streams { return p.streams }

// Wait 等主进程退出后再收口全部集合；退出码不能替代清理结果。
func (p *Process) Wait(ctx context.Context) (Exit, error) {
	if p.monitor == nil {
		return Exit{}, errors.Join(p.startErr, p.Close(ctx))
	}
	status, err := p.monitor.Wait(ctx)
	if err != nil {
		if ctx != nil && ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return Exit{}, err
		}
		return Exit{}, errors.Join(err, p.Close(ctx))
	}
	return status, p.Close(ctx)
}

// Close 发起一次共享清理；调用者取消不撤销清理，失败结果不会被重复 Close 隐藏。
func (p *Process) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("supervised close requires context")
	}
	p.closeOnce.Do(func() {
		go func() {
			defer close(p.closed)
			cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			p.closeErr = p.cleanup(cleanup)
		}()
	})
	select {
	case <-p.closed:
		return p.closeErr
	default:
	}
	select {
	case <-p.closed:
		return p.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Process) cleanup(ctx context.Context) error {
	if p.streams.Stdin != nil {
		_ = p.streams.Stdin.Close()
	}
	if p.startErr != nil {
		if p.streams.Stdout != nil {
			_ = p.streams.Stdout.Close()
		}
		if p.streams.Stderr != nil {
			_ = p.streams.Stderr.Close()
		}
	}

	for _, file := range p.childFiles {
		if file != nil {
			_ = file.Close()
		}
	}
	var cleanupErr error
	if p.submitted {
		cleanupErr = removeJob(ctx, p.intent)
	}
	var proof *Evidence
	if p.scope != nil {
		// 即使撤销失败也尽量停止原集合，但不得提交已回收终态或清除持久记录。
		observed, err := p.scope.Reap(ctx)
		cleanupErr = errors.Join(cleanupErr, err)
		if err == nil {
			boot, err := processscope.CurrentBootID()
			cleanupErr = errors.Join(cleanupErr, err)
			if err == nil {
				proof = &Evidence{Registration: observed.Registration, Reason: observed.Reason, ObservedBootID: boot}
			}
		}
	}
	if p.monitor != nil {
		cleanupErr = errors.Join(cleanupErr, p.monitor.Close())
	}
	if cleanupErr == nil && p.reserved {
		cleanupErr = p.host.Finish(ctx, p.intent, proof)
	}
	return cleanupErr
}
