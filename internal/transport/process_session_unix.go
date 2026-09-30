//go:build darwin || linux

// INPUT: runtime 创建的 Unix session 及平台可见进程。
// OUTPUT: 有界清理与最后一次观察结果，保留信号和枚举错误。
// POS: 同 session 后代的回收步骤，不覆盖另建 session 的后代。
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

const processSessionCleanupAttempts = 100
const processSessionCleanupInterval = 10 * time.Millisecond

// processSession 回收仍在 runtime Unix session 内的后代，包含独立进程组。
// setsid 可创建新 session；本结构不能证明所有脱离后代都已退出。
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

func startedProcessSession(command *exec.Cmd) (processSession, error) {
	if command == nil || command.Process == nil {
		return processSession{}, nil
	}
	return processSession{sessionID: command.Process.Pid}, nil
}

func (s processSession) id() int {
	return s.sessionID
}

func (s processSession) hasDirectCleanup() bool { return false }

func (s processSession) cleanup() (int, error) {
	return s.cleanupWith(processIDsInSession, func(pid int) error {
		return syscall.Kill(pid, syscall.SIGKILL)
	}, time.Sleep)
}

// cleanupWith 对平台枚举与信号结果执行同一有界终态检查。
func (s processSession) cleanupWith(list func(int) ([]int, error), kill func(int) error, pause func(time.Duration)) (int, error) {
	if s.sessionID <= 1 || s.sessionID == os.Getpid() {
		return 0, nil
	}

	terminated := make(map[int]struct{})
	var cleanupErr error
	for attempt := 0; attempt < processSessionCleanupAttempts; attempt++ {
		processIDs, err := list(s.sessionID)
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
			if err := kill(processID); err != nil && !errors.Is(err, syscall.ESRCH) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("kill process %d: %w", processID, err))
				continue
			}
			terminated[processID] = struct{}{}
		}
		if !found {
			break
		}
		pause(processSessionCleanupInterval)
	}
	remaining, err := list(s.sessionID)
	if err != nil {
		return len(terminated), errors.Join(cleanupErr, err)
	}
	for _, pid := range remaining {
		if pid > 1 && pid != os.Getpid() && pid != s.sessionID {
			return len(terminated), errors.Join(cleanupErr, fmt.Errorf("process session %d still has descendants after cleanup", s.sessionID))
		}
	}
	return len(terminated), cleanupErr
}
