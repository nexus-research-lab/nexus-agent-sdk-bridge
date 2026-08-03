//go:build darwin

package transport

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

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
		if err == nil && currentSessionID == sessionID {
			processIDs = append(processIDs, processID)
		}
	}
	return processIDs, nil
}
