package transport

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestProcessCloseUnblocksWhenDescendantKeepsStderrOpen(t *testing.T) {
	manager, cleanup := newExitedProcessManagerWithOpenStderr(t)
	defer cleanup()

	done := make(chan error, 1)
	go func() {
		done <- manager.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Close() blocked while stderr pipe was still inherited")
	}
}

func TestProcessWaitUnblocksWhenDescendantKeepsStderrOpen(t *testing.T) {
	manager, cleanup := newExitedProcessManagerWithOpenStderr(t)
	defer cleanup()

	done := make(chan error, 1)
	go func() {
		done <- manager.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait() blocked while stderr pipe was still inherited")
	}
}

func newExitedProcessManagerWithOpenStderr(t *testing.T) (*ProcessManager, func()) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("create stderr pipe: %v", err)
	}
	done := make(chan struct{})
	close(done)

	manager := &ProcessManager{
		cmd:    &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}},
		stdout: stdoutReader,
		stderr: stderrReader,
		done:   done,
	}
	manager.stderrWG.Add(1)
	go manager.readStderr(stderrReader)

	cleanup := func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}
	return manager, cleanup
}
