//go:build darwin || linux

// INPUT: 成功退出的本地进程与失败的宿主后代清理回调。
// OUTPUT: Wait 与重复 Close 均保留清理失败，不能只写诊断日志。
// POS: 资源回收前的进程清理错误传播回归。
package transport

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestProcessCleanupFailureReachesWaitAndRepeatedClose 保留精确清理错误而非主进程 exit 0。
func TestProcessCleanupFailureReachesWaitAndRepeatedClose(t *testing.T) {
	if os.Getenv("NEXUS_BRIDGE_CLEANUP_ERROR_HELPER") == "1" {
		os.Exit(0)
	}
	want := errors.New("test descendant cleanup rejected")
	manager := NewProcessManager(ProcessConfig{
		CommandPath: os.Args[0], CWD: t.TempDir(), Args: []string{"-test.run=^TestProcessCleanupFailureReachesWaitAndRepeatedClose$"},
		Env: map[string]string{"NEXUS_BRIDGE_CLEANUP_ERROR_HELPER": "1"}, ControlWireDialect: ControlWireDialectNXS,
		SignalProcess: func(int, ProcessSignal) error { return want },
	})
	if err := manager.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Wait(); !errors.Is(err, want) {
		t.Errorf("Wait lost cleanup failure: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := manager.Close(); !errors.Is(err, want) {
			t.Errorf("Close %d lost cleanup failure: %v", i, err)
		}
	}
}

// TestForcedProcessClosePreservesCleanupFailure 主动 TERM 后仍必须返回清理失败。
func TestForcedProcessClosePreservesCleanupFailure(t *testing.T) {
	if os.Getenv("NEXUS_BRIDGE_FORCED_CLEANUP_HELPER") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	want := errors.New("test cleanup after forced exit failed")
	manager := NewProcessManager(ProcessConfig{
		CommandPath: os.Args[0], CWD: t.TempDir(), Args: []string{"-test.run=^TestForcedProcessClosePreservesCleanupFailure$"},
		Env: map[string]string{"NEXUS_BRIDGE_FORCED_CLEANUP_HELPER": "1"}, ControlWireDialect: ControlWireDialectNXS,
		SignalProcess: func(pid int, signal ProcessSignal) error {
			if signal == ProcessSignalKill {
				return want
			}
			return syscall.Kill(pid, syscall.SIGTERM)
		},
	})
	if err := manager.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.cmd.Process.Kill() })
	for i := 0; i < 2; i++ {
		if err := manager.Close(); !errors.Is(err, want) {
			t.Fatalf("forced Close %d lost cleanup failure: %v", i, err)
		}
	}
	if err := manager.Wait(); !errors.Is(err, want) {
		t.Fatalf("Wait after forced Close lost cleanup failure: %v", err)
	}
}

// TestProcessSessionRequiresEmptyFinalObservation 信号成功、观察失败或仍有后代都不能变成清理成功。
func TestProcessSessionRequiresEmptyFinalObservation(t *testing.T) {
	for _, name := range []string{"gone", "still_running", "observation_failed", "signal_failed"} {
		t.Run(name, func(t *testing.T) {
			calls, signals := 0, 0
			want := errors.New("test cleanup boundary failure")
			list := func(int) ([]int, error) {
				calls++
				if name == "observation_failed" && calls > 1 {
					return nil, want
				}
				if name == "gone" && calls > 1 {
					return nil, nil
				}
				return []int{9001}, nil
			}
			_, err := (processSession{sessionID: 9000}).cleanupWith(list, func(int) error {
				signals++
				if name == "signal_failed" {
					return want
				}
				return nil
			}, func(time.Duration) {})
			if (err == nil) != (name == "gone") {
				t.Fatalf("%s cleanup result: %v", name, err)
			}
			if signals == 0 || calls < 2 {
				t.Fatal("cleanup did not observe state after signaling")
			}
		})
	}
}
