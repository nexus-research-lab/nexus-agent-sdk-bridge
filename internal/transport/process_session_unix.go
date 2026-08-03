//go:build darwin || linux

package transport

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"time"
)

const processSessionCleanupAttempts = 3
const processSessionCleanupInterval = 10 * time.Millisecond

// processSession 隔离一棵 runtime 进程树；工具可以创建自己的进程组，
// 但不能脱离 runtime 的 session 生命周期。
type processSession struct {
	sessionID int
}

func configureProcessSession(command *exec.Cmd) {
	if command == nil {
		return
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setsid = true
}

func startedProcessSession(command *exec.Cmd) processSession {
	if command == nil || command.Process == nil {
		return processSession{}
	}
	return processSession{sessionID: command.Process.Pid}
}

func (s processSession) id() int {
	return s.sessionID
}

func (s processSession) cleanup() (int, error) {
	if s.sessionID <= 1 || s.sessionID == os.Getpid() {
		return 0, nil
	}

	terminated := make(map[int]struct{})
	var cleanupErr error
	for attempt := 0; attempt < processSessionCleanupAttempts; attempt++ {
		processIDs, err := processIDsInSession(s.sessionID)
		if err != nil {
			return len(terminated), errors.Join(cleanupErr, err)
		}
		sort.Ints(processIDs)

		found := false
		for _, processID := range processIDs {
			if processID <= 1 || processID == os.Getpid() || processID == s.sessionID {
				continue
			}
			found = true
			if err := syscall.Kill(processID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("kill process %d: %w", processID, err))
				continue
			}
			terminated[processID] = struct{}{}
		}
		if !found {
			break
		}
		time.Sleep(processSessionCleanupInterval)
	}
	return len(terminated), cleanupErr
}
