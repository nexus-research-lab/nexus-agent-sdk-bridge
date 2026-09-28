// INPUT: 宿主可信数据库中的原启动意图、可选原登记及受生命周期锁保护的恢复回调。
// OUTPUT: 原 job 撤销、原集合回收后精确终态；不启动 helper 或重放任务。
// POS: 崩溃恢复的原生执行入口；不能从用户文件、PID 或 job 枚举重建原登记。
package supervision

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

// Recovery 只能来自可信宿主持久记录。Registration=nil 仅适用于尚未登记/放行的
// prepared 意图；不能由调用方省略未知登记来请求较弱的回收。
type Recovery struct {
	Intent       Intent
	Registration *Registration
}

// RecoveryHost 只收口原记录。实现仍须核对数据库当前阶段，不得顺带清除其他代次。
type RecoveryHost interface {
	Finish(context.Context, Intent, *Evidence) error
}

type recoveryOps struct {
	observe func() (processscope.Observer, error)
	restore func(Registration) (func(context.Context) (processscope.Evidence, error), error)
	revoke  func(context.Context, Intent) error
	boot    func() (string, error)
}

// Recover 不重建历史退出码或任务结果。宿主必须先取得排他的生命周期所有权，
// 确认原宿主不再持有活动启动，再调用本入口；不能同时恢复一个仍被正常使用的进程。
// 失败保留原持久记录，重试只重复撤销/回收，不重复执行任务。
func Recover(ctx context.Context, record Recovery, host RecoveryHost) error {
	return recoverLaunch(ctx, record, host, recoveryOps{
		observe: processscope.ObserveSelf,
		restore: func(r Registration) (func(context.Context) (processscope.Evidence, error), error) {
			scope, err := processscope.Restore(r)
			if err != nil {
				return nil, err
			}
			return scope.Reap, nil
		},
		revoke: removeJob,
		boot:   processscope.CurrentBootID,
	})
}

func validRecoveryHex(s string, n int) bool {
	if len(s) != n || strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func validRecoveryBoot(s string) bool {
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' && validRecoveryHex(strings.ReplaceAll(s, "-", ""), 32)
}
func recoverLaunch(ctx context.Context, record Recovery, host RecoveryHost, ops recoveryOps) error {
	if ctx == nil || host == nil {
		return errors.New("recovery requires context and durable host")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	i := record.Intent
	if i.Version != 1 || !validRecoveryHex(i.ID, 32) || i.JobLabel != "cn.nexus.runtime."+i.ID || !validRecoveryBoot(i.BootID) || !validRecoveryHex(i.HelperSHA256, 64) {
		return errors.New("invalid persisted recovery intent")
	}
	// 在任何 job 撤销之前验证完整绑定和原生观察者，避免错误身份产生副作用。
	if r := record.Registration; r != nil && (r.Version != 1 || r.BootID != i.BootID || r.OwnerUID != i.OwnerUID || r.CoalitionID == 0) {
		return errors.New("recovery registration does not match original intent")
	}
	observer, err := ops.observe()
	if err != nil {
		return err
	}
	if observer.UID != i.OwnerUID || !validRecoveryBoot(observer.BootID) {
		return errors.New("recovery observer does not match persisted owner")
	}
	var reap func(context.Context) (processscope.Evidence, error)
	if record.Registration != nil {
		reap, err = ops.restore(*record.Registration)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// 新 boot 的同名 job 不属于原启动，不按旧身份撤销它。
	if observer.BootID == i.BootID {
		if err := ops.revoke(ctx, i); err != nil {
			return err
		}
	}
	var evidence *Evidence
	if reap != nil {
		proof, err := reap(ctx)
		if err != nil {
			return err
		}
		boot, err := ops.boot()
		if err != nil {
			return err
		}
		if proof.Registration != *record.Registration || !validRecoveryBoot(boot) || !((proof.Reason == "coalition_reaped" && boot == i.BootID) || (proof.Reason == "boot_changed" && boot != i.BootID)) {
			return errors.New("recovery lacks exact collection retirement evidence")
		}
		evidence = &Evidence{Registration: proof.Registration, Reason: proof.Reason, ObservedBootID: boot}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return host.Finish(ctx, i, evidence)
}
