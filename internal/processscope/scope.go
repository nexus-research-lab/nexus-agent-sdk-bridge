// INPUT: 已认证引导进程，或由可信存储恢复的执行登记。
// OUTPUT: 同 boot/coalition 的观察、精确终止及内核回收证明。
// POS: 进程生命周期组件；尚未接入默认 transport 或宿主资源回收。
package processscope

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
)

var (
	ErrUnavailable = errors.New("exact process scope supervision is unavailable")
	ErrGone        = errors.New("kernel process identity no longer exists")
)

// Registration 只能由 Capture 产生并在执行前持久保存，不能作为外部授权输入。
type Registration struct {
	Version     int    `json:"version"`
	BootID      string `json:"boot_id"`
	CoalitionID uint64 `json:"coalition_id"`
	OwnerUID    uint32 `json:"owner_uid"`
}

// Evidence 只证明原登记范围已经消失；不证明工具副作用已经回滚或可以重放。
type Evidence struct {
	Registration Registration
	Reason       string
}

type process struct {
	coalition uint64
	audit     [8]uint32
}

type kernel interface {
	available() error
	bootID() (string, error)
	self() (process, error)
	inspect(int) (process, error)
	exists(uint64) error
	members(uint64) ([]process, error)
	signal(process, int) error
}

// Scope 只持有一个已登记集合。gate 串行化本对象清理，取消的等待者不能发信号。
type Scope struct {
	registration Registration
	kernel       kernel
	gate         chan struct{}
}

// Capture 在可信 helper 仍存活、且尚未执行任务时核验原生身份。
func Capture(pid int) (*Scope, error) { return capture(newKernel(), pid) }

func capture(k kernel, pid int) (*Scope, error) {
	if pid <= 1 || pid > 1<<31-1 {
		return nil, errors.New("invalid bootstrap process identity")
	}
	if err := k.available(); err != nil {
		return nil, err
	}
	boot, err := k.bootID()
	if err != nil {
		return nil, err
	}
	self, err := k.self()
	if err != nil {
		return nil, err
	}
	root, err := k.inspect(pid)
	if err != nil {
		return nil, err
	}
	if root.audit[5] != uint32(pid) || root.audit[7] == 0 || root.audit[1] != self.audit[1] || root.coalition == 0 || root.coalition == self.coalition {
		return nil, errors.New("bootstrap does not own a distinct same-user process scope")
	}
	if err := k.exists(root.coalition); err != nil {
		return nil, err
	}
	return restore(k, Registration{Version: 1, BootID: boot, CoalitionID: root.coalition, OwnerUID: root.audit[1]})
}

// Restore 不把任意不存在的 ID 当作退出证据；调用者必须保证输入来自可信原登记。
func Restore(registration Registration) (*Scope, error) { return restore(newKernel(), registration) }

func restore(k kernel, r Registration) (*Scope, error) {
	if r.Version != 1 || r.CoalitionID == 0 || !validBootID(r.BootID) {
		return nil, errors.New("invalid stored process scope registration")
	}
	r.BootID = strings.ToLower(r.BootID)
	if err := k.available(); err != nil {
		return nil, err
	}
	self, err := k.self()
	if err != nil {
		return nil, err
	}
	boot, err := k.bootID()
	if err != nil {
		return nil, err
	}
	if !validBootID(boot) {
		return nil, errors.New("invalid current boot identity")
	}
	if self.audit[1] != r.OwnerUID || (strings.EqualFold(boot, r.BootID) && self.coalition == r.CoalitionID) {
		return nil, errors.New("stored scope conflicts with observer identity")
	}
	return &Scope{registration: r, kernel: k, gate: make(chan struct{}, 1)}, nil
}

// Registration 返回不可变登记副本；持久层仍负责 owner/session/generation 绑定。
func (s *Scope) Registration() Registration { return s.registration }

// Reap 只在原 coalition 被内核回收或可信 boot identity 改变后返回成功。
// 调用者应先撤销其 launchd job 的新启动权；空枚举或资源计数绝不表示完成。
func (s *Scope) Reap(ctx context.Context) (Evidence, error) {
	if ctx == nil {
		return Evidence{}, errors.New("process scope cleanup requires context")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return Evidence{}, ctx.Err()
	}
	for {
		if err := ctx.Err(); err != nil {
			return Evidence{}, err
		}
		boot, err := s.kernel.bootID()
		if err != nil {
			return Evidence{}, fmt.Errorf("observe boot: %w", err)
		}
		if !validBootID(boot) {
			return Evidence{}, errors.New("invalid current boot identity")
		}
		if !strings.EqualFold(boot, s.registration.BootID) {
			return Evidence{s.registration, "boot_changed"}, nil
		}
		if err := s.kernel.exists(s.registration.CoalitionID); errors.Is(err, ErrGone) {
			return Evidence{s.registration, "coalition_reaped"}, nil
		} else if err != nil {
			return Evidence{}, fmt.Errorf("observe scope: %w", err)
		}
		members, err := s.kernel.members(s.registration.CoalitionID)
		if err != nil {
			return Evidence{}, fmt.Errorf("observe members: %w", err)
		}
		for _, member := range members {
			if err := ctx.Err(); err != nil {
				return Evidence{}, err
			}
			if member.coalition != s.registration.CoalitionID || member.audit[5] <= 1 || member.audit[5] > 1<<31-1 || member.audit[7] == 0 || member.audit[1] != s.registration.OwnerUID {
				return Evidence{}, errors.New("observed member identity conflicts with registered scope")
			}
			if err := s.kernel.signal(member, int(syscall.SIGKILL)); err != nil && !errors.Is(err, ErrGone) {
				return Evidence{}, fmt.Errorf("terminate exact member: %w", err)
			}
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Evidence{}, ctx.Err()
		case <-timer.C:
		}
	}
}

// validBootID 拒绝缺失和任意字符串，boot 变更不能由墙钟或 PID 推断。
func validBootID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}
