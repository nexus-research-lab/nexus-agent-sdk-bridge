//go:build darwin

// INPUT: 内核进程快照及每个进程当前的 Unix session。
// OUTPUT: 目标 session 的进程集合，观察失败必须返回错误。
// POS: macOS runtime session 清理的可见性边界。
package transport

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// processIDsInSession 只忽略快照之后已退出的进程，不把观察错误解释为空集合。
func processIDsInSession(sessionID int) ([]int, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("list process session %d: %w", sessionID, err)
	}

	processIDs := make([]int, 0)
	for _, process := range processes {
		processID := int(process.Proc.P_pid)
		if processID <= 1 {
			continue
		}
		currentSessionID, err := syscall.Getsid(processID)
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect process %d for session %d: %w", processID, sessionID, err)
		}
		if currentSessionID == sessionID {
			processIDs = append(processIDs, processID)
		}
	}
	return processIDs, nil
}
