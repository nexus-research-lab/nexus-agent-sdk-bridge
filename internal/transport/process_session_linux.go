//go:build linux

// INPUT: 当前可见的 /proc 进程状态和目标 Unix session。
// OUTPUT: 匹配的进程集合或明确的读取/解析失败。
// POS: Linux runtime session 清理的进程观察入口；不越过 PID namespace 或 hidepid。
package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// processIDsInSession 只忽略已经消失的进程，读取或格式未知不能当作清理完成。
func processIDsInSession(sessionID int) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("list process session %d: %w", sessionID, err)
	}

	processIDs := make([]int, 0)
	for _, entry := range entries {
		processID, err := strconv.Atoi(entry.Name())
		if err != nil || processID <= 1 {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read process %d for session %d: %w", processID, sessionID, err)
		}
		fieldsStart := strings.LastIndexByte(string(data), ')')
		if fieldsStart < 0 {
			return nil, fmt.Errorf("process %d has malformed stat", processID)
		}
		fields := strings.Fields(string(data[fieldsStart+1:]))
		if len(fields) < 4 {
			return nil, fmt.Errorf("process %d has incomplete stat", processID)
		}
		currentSessionID, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("read process %d session: %w", processID, err)
		}
		if currentSessionID == sessionID {
			processIDs = append(processIDs, processID)
		}
	}
	return processIDs, nil
}
