// INPUT: 可控制失败/身份换代的内核观察夹具。
// OUTPUT: 无退出证明时失败关闭，不向错误 scope 或 PID version 发信号。
// POS: 监督组件语义测试；原生覆盖另走显式 macOS 用例。
package processscope

import (
	"context"
	"errors"
	"strconv"
	"syscall"
	"testing"
	"time"
)

const testBoot = "12345678-1234-1234-1234-123456789abc"

type fixtureKernel struct {
	boot         string
	root         process
	availableErr error
	existsFn     func(uint64) error
	membersFn    func(uint64) ([]process, error)
	signalFn     func(process, int) error
}

func fixture() *fixtureKernel {
	return &fixtureKernel{boot: testBoot, root: process{coalition: 91, audit: [8]uint32{501, 501, 20, 501, 20, 202, 1, 4}}}
}
func (k *fixtureKernel) available() error        { return k.availableErr }
func (k *fixtureKernel) bootID() (string, error) { return k.boot, nil }
func (k *fixtureKernel) self() (process, error) {
	return process{coalition: 7, audit: [8]uint32{501, 501, 20, 501, 20, 101, 1, 2}}, nil
}
func (k *fixtureKernel) inspect(int) (process, error) { return k.root, nil }
func (k *fixtureKernel) exists(id uint64) error {
	if k.existsFn != nil {
		return k.existsFn(id)
	}
	return nil
}
func (k *fixtureKernel) members(id uint64) ([]process, error) {
	if k.membersFn != nil {
		return k.membersFn(id)
	}
	return []process{k.root}, nil
}
func (k *fixtureKernel) signal(p process, sig int) error {
	if k.signalFn != nil {
		return k.signalFn(p, sig)
	}
	return errors.New("unexpected signal")
}

func TestCaptureRejectsAmbiguousRegistration(t *testing.T) {
	for _, name := range []string{"unavailable", "observer_scope", "foreign_user", "missing_version", "wrong_pid", "oversized_pid"} {
		t.Run(name, func(t *testing.T) {
			k := fixture()
			pid := 202
			switch name {
			case "unavailable":
				k.availableErr = ErrUnavailable
			case "observer_scope":
				k.root.coalition = 7
			case "foreign_user":
				k.root.audit[1] = 502
			case "missing_version":
				k.root.audit[7] = 0
			case "wrong_pid":
				k.root.audit[5] = 203
			case "oversized_pid":
				if strconv.IntSize < 64 {
					t.Skip("oversized int requires a 64-bit host")
				}
				oversized := int64(1<<32 + 202)
				pid = int(oversized)
			}
			if _, err := capture(k, pid); err == nil {
				t.Fatal("ambiguous registration accepted")
			}
		})
	}
}

func TestRestoreRequiresTrustedRegistrationShape(t *testing.T) {
	k := fixture()
	for _, r := range []Registration{{}, {Version: 1, BootID: "unknown", CoalitionID: 91, OwnerUID: 501}, {Version: 1, BootID: testBoot, CoalitionID: 7, OwnerUID: 501}, {Version: 1, BootID: testBoot, CoalitionID: 91, OwnerUID: 502}} {
		if _, err := restore(k, r); err == nil {
			t.Fatalf("invalid registration accepted: %#v", r)
		}
	}
}

func TestReapRequiresKernelRetirement(t *testing.T) {
	k := fixture()
	s, err := capture(k, 202)
	if err != nil {
		t.Fatal(err)
	}
	signaled := false
	k.existsFn = func(uint64) error {
		if signaled {
			return ErrGone
		}
		return nil
	}
	k.signalFn = func(p process, sig int) error {
		if p != k.root || sig != int(syscall.SIGKILL) {
			t.Fatalf("wrong target: %#v/%d", p, sig)
		}
		signaled = true
		return nil
	}
	evidence, err := s.Reap(context.Background())
	if err != nil || evidence.Reason != "coalition_reaped" || !signaled {
		t.Fatalf("reap = %#v, %v", evidence, err)
	}
}

func TestReapNeverTreatsEmptyObservationAsExit(t *testing.T) {
	k := fixture()
	s, err := capture(k, 202)
	if err != nil {
		t.Fatal(err)
	}
	k.membersFn = func(uint64) ([]process, error) { return nil, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	evidence, err := s.Reap(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || evidence.Reason != "" {
		t.Fatalf("empty observation succeeded: %#v %v", evidence, err)
	}
}

func TestReapPreservesObservationAndSignalErrors(t *testing.T) {
	denied := errors.New("denied")
	for _, stage := range []string{"scope", "members", "signal"} {
		t.Run(stage, func(t *testing.T) {
			k := fixture()
			s, err := capture(k, 202)
			if err != nil {
				t.Fatal(err)
			}
			switch stage {
			case "scope":
				k.existsFn = func(uint64) error { return denied }
			case "members":
				k.membersFn = func(uint64) ([]process, error) { return nil, denied }
			case "signal":
				k.signalFn = func(process, int) error { return denied }
			}
			evidence, err := s.Reap(context.Background())
			if !errors.Is(err, denied) || evidence.Reason != "" {
				t.Fatalf("failure hidden: %#v %v", evidence, err)
			}
		})
	}
}

func TestReapRejectsMemberScopeMismatch(t *testing.T) {
	for _, name := range []string{"coalition", "owner", "pid", "oversized_pid", "version"} {
		t.Run(name, func(t *testing.T) {
			k := fixture()
			s, err := capture(k, 202)
			if err != nil {
				t.Fatal(err)
			}
			wrong := k.root
			switch name {
			case "coalition":
				wrong.coalition++
			case "owner":
				wrong.audit[1]++
			case "pid":
				wrong.audit[5] = 1
			case "oversized_pid":
				wrong.audit[5] = 1 << 31
			case "version":
				wrong.audit[7] = 0
			}
			k.membersFn = func(uint64) ([]process, error) { return []process{wrong}, nil }
			evidence, err := s.Reap(context.Background())
			if err == nil || evidence.Reason != "" {
				t.Fatalf("wrong member accepted: %#v %v", evidence, err)
			}
		})
	}
}

func TestReapRescansAfterStaleAuditIdentity(t *testing.T) {
	k := fixture()
	s, err := capture(k, 202)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	k.existsFn = func(uint64) error {
		if calls == 2 {
			return ErrGone
		}
		return nil
	}
	k.signalFn = func(p process, _ int) error {
		calls++
		if calls == 1 {
			k.root.audit[7]++
			return ErrGone
		}
		if p.audit[7] != 5 {
			t.Fatal("old identity reused")
		}
		return nil
	}
	if _, err := s.Reap(context.Background()); err != nil || calls != 2 {
		t.Fatalf("reap = %v, calls %d", err, calls)
	}
}

func TestReapUsesBootIdentityWithoutSignals(t *testing.T) {
	k := fixture()
	s, err := capture(k, 202)
	if err != nil {
		t.Fatal(err)
	}
	k.boot = "22345678-1234-1234-1234-123456789abc"
	k.existsFn = func(uint64) error { t.Fatal("old boot coalition observed"); return nil }
	evidence, err := s.Reap(context.Background())
	if err != nil || evidence.Reason != "boot_changed" {
		t.Fatalf("reboot evidence: %#v %v", evidence, err)
	}
}

func TestReapCanceledWaiterDoesNotInterruptOwner(t *testing.T) {
	k := fixture()
	s, err := capture(k, 202)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	k.membersFn = func(uint64) ([]process, error) {
		close(entered)
		<-release
		return nil, errors.New("owner observation failure")
	}
	done := make(chan error, 1)
	go func() { _, err := s.Reap(context.Background()); done <- err }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Reap(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("queued cancellation: %v", err)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("owner failure lost")
	}
}
