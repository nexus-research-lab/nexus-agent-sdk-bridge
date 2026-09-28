// INPUT: 显式监督工厂的拒绝结果与本机进程管理器。
// OUTPUT: 失败后不回退、不重新领取，不把监督配置交给普通信号回调。
// POS: 监督 transport 准入与错误合同回归。
package transport

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
)

func TestSupervisedTransportDoesNotRetryOrFallback(t *testing.T) {
	refusal := errors.New("host refused launch")
	calls := 0
	m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", ControlWireDialect: ControlWireDialectNXS, Supervision: func(context.Context, supervision.Purpose) (supervision.Config, error) {
		calls++
		return supervision.Config{}, refusal
	}})
	if err := m.Start(t.Context()); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	if err := m.Start(t.Context()); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	if calls != 1 || m.cmd != nil {
		t.Fatalf("calls=%d cmd=%v", calls, m.cmd)
	}
	if err := m.Wait(); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	for range 2 {
		if err := m.Close(); !errors.Is(err, refusal) {
			t.Fatal(err)
		}
	}
	if err := m.Interrupt(); !errors.Is(err, ErrInterruptUnsupported) {
		t.Fatal(err)
	}
}

func TestSupervisedVersionProbeFailureBlocksMain(t *testing.T) {
	t.Setenv(skipVersionCheckEnv, "")
	refusal := errors.New("probe host refused")
	var purposes []supervision.Purpose
	m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", Supervision: func(_ context.Context, p supervision.Purpose) (supervision.Config, error) {
		purposes = append(purposes, p)
		return supervision.Config{}, refusal
	}})
	if err := m.Start(t.Context()); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	if len(purposes) != 1 || purposes[0] != supervision.VersionProbe || m.cmd != nil {
		t.Fatalf("purposes=%v", purposes)
	}
}

func TestSupervisedTransportRejectsSignalOverride(t *testing.T) {
	calls := 0
	m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", SignalProcess: func(int, ProcessSignal) error { return nil }, Supervision: func(context.Context, supervision.Purpose) (supervision.Config, error) {
		calls++
		return supervision.Config{}, nil
	}})
	if err := m.Start(t.Context()); err == nil {
		t.Fatal("signal override accepted")
	}
	if calls != 0 || m.cmd != nil {
		t.Fatal("override reached launch")
	}
}
