//go:build darwin || linux

package transport

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCleanupProcessSessionUsesHostSignalHandler(t *testing.T) {
	var processID int
	var received ProcessSignal
	manager := NewProcessManager(ProcessConfig{
		SignalProcess: func(pid int, signal ProcessSignal) error {
			processID = pid
			received = signal
			return nil
		},
	})

	manager.cleanupProcessSession(processSession{sessionID: 321})
	if processID != 321 || received != ProcessSignalKill {
		t.Fatalf("cleanup signal = pid:%d signal:%q", processID, received)
	}
}

func TestKillStartedProcessReportsHostAndDirectSignalFailures(t *testing.T) {
	process, err := os.FindProcess(1 << 30)
	if err != nil {
		t.Fatalf("FindProcess() error = %v", err)
	}
	hostErr := errors.New("host signal denied")
	manager := NewProcessManager(ProcessConfig{
		SignalProcess: func(int, ProcessSignal) error { return hostErr },
	})

	err = manager.killStartedProcess(process)
	if !errors.Is(err, hostErr) {
		t.Fatalf("killStartedProcess() error = %v, want host error", err)
	}
}

func TestProcessWaitTerminatesRuntimeDescendants(t *testing.T) {
	if os.Getenv("NEXUS_BRIDGE_TEST_RUNTIME_DESCENDANT") == "1" {
		child := exec.Command("/bin/sh", "-c", "exec sleep 60")
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := child.Start(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "start descendant: %v\n", err)
			os.Exit(2)
		}
		markerPath := os.Getenv("NEXUS_BRIDGE_TEST_RUNTIME_DESCENDANT_PID")
		if err := os.WriteFile(markerPath, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "write descendant marker: %v\n", err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	markerPath := filepath.Join(t.TempDir(), "descendant.pid")
	var diagnostics []ProcessDiagnosticEvent
	manager := NewProcessManager(ProcessConfig{
		CommandPath: os.Args[0],
		CWD:         t.TempDir(),
		Args:        []string{"-test.run=^TestProcessWaitTerminatesRuntimeDescendants$"},
		Env: map[string]string{
			"NEXUS_BRIDGE_TEST_RUNTIME_DESCENDANT":     "1",
			"NEXUS_BRIDGE_TEST_RUNTIME_DESCENDANT_PID": markerPath,
		},
		ControlWireDialect: ControlWireDialectNXS,
		Diagnostics: func(event ProcessDiagnosticEvent) {
			diagnostics = append(diagnostics, event)
		},
	})
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := manager.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read descendant marker: %v", err)
	}
	descendantPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse descendant PID: %v", err)
	}
	defer func() {
		_ = syscall.Kill(-descendantPID, syscall.SIGKILL)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		err := syscall.Kill(descendantPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil {
			t.Fatalf("inspect descendant %d: %v", descendantPID, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant process %d survived runtime exit", descendantPID)
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, event := range diagnostics {
		if event.Event == "process_descendants_terminated" {
			return
		}
	}
	t.Fatalf("missing descendant cleanup diagnostics: %#v", diagnostics)
}
