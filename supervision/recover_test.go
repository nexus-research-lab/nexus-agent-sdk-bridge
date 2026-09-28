// INPUT: 固定原登记与可控的原生恢复结果。
// OUTPUT: 身份核验先于撤销、旧 boot 不碰新 job、失败不提交终态。
// POS: 恢复编排的故障顺序验证，不替代原生 OS 回收证据。
package supervision

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

type recoveryHostFunc func(context.Context, Intent, *Evidence) error

func (f recoveryHostFunc) Finish(c context.Context, i Intent, e *Evidence) error { return f(c, i, e) }
func TestRecoveryOrderingAndFailures(t *testing.T) {
	for _, stage := range []string{"success", "prepared", "boot_changed", "observe", "restore", "revoke", "reap", "boot", "finish", "wrong_owner", "wrong_scope", "bad_proof", "canceled"} {
		t.Run(stage, func(t *testing.T) {
			boot := "12345678-1234-1234-1234-123456789abc"
			observed := boot
			if stage == "boot_changed" {
				observed = "87654321-1234-1234-1234-123456789abc"
			}
			id := strings.Repeat("a", 32)
			i := Intent{Version: 1, ID: id, BootID: boot, OwnerUID: 501, JobLabel: "cn.nexus.runtime." + id, HelperSHA256: strings.Repeat("b", 64)}
			r := Registration{Version: 1, BootID: boot, OwnerUID: 501, CoalitionID: 42}
			record := Recovery{Intent: i, Registration: &r}
			if stage == "prepared" {
				record.Registration = nil
			}
			if stage == "wrong_scope" {
				r.OwnerUID++
			}
			failure := errors.New("injected failure")
			var steps []string
			step := func(name string) error {
				steps = append(steps, name)
				if stage == name {
					return failure
				}
				return nil
			}
			ops := recoveryOps{
				observe: func() (processscope.Observer, error) {
					uid := uint32(501)
					if stage == "wrong_owner" {
						uid++
					}
					return processscope.Observer{UID: uid, BootID: observed}, step("observe")
				},
				restore: func(reg Registration) (func(context.Context) (processscope.Evidence, error), error) {
					if err := step("restore"); err != nil {
						return nil, err
					}
					return func(context.Context) (processscope.Evidence, error) {
						reason := "coalition_reaped"
						if stage == "boot_changed" {
							reason = "boot_changed"
						}
						if stage == "bad_proof" {
							reg.CoalitionID++
						}
						return processscope.Evidence{Registration: reg, Reason: reason}, step("reap")
					}, nil
				},
				revoke: func(context.Context, Intent) error { return step("revoke") },
				boot:   func() (string, error) { return observed, step("boot") },
			}
			host := recoveryHostFunc(func(_ context.Context, got Intent, proof *Evidence) error {
				if got != i || (stage == "prepared") != (proof == nil) {
					t.Fatal("finish binding changed")
				}
				return step("finish")
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if stage == "canceled" {
				cancel()
			}
			err := recoverLaunch(ctx, record, host, ops)
			success := stage == "success" || stage == "prepared" || stage == "boot_changed"
			if (err == nil) != success {
				t.Fatalf("err=%v steps=%v", err, steps)
			}
			var want []string
			switch stage {
			case "success", "finish":
				want = []string{"observe", "restore", "revoke", "reap", "boot", "finish"}
			case "prepared":
				want = []string{"observe", "revoke", "finish"}
			case "boot_changed":
				want = []string{"observe", "restore", "reap", "boot", "finish"}
			case "observe", "wrong_owner":
				want = []string{"observe"}
			case "restore":
				want = []string{"observe", "restore"}
			case "revoke":
				want = []string{"observe", "restore", "revoke"}
			case "reap":
				want = []string{"observe", "restore", "revoke", "reap"}
			case "boot", "bad_proof":
				want = []string{"observe", "restore", "revoke", "reap", "boot"}
			}
			if !reflect.DeepEqual(steps, want) {
				t.Fatalf("steps=%v want=%v", steps, want)
			}
		})
	}
}
