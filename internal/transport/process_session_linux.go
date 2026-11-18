//go:build linux

// INPUT: 当前可见的 /proc 进程状态和目标 Unix session。
// OUTPUT: 匹配的存活进程集合（跳过僵尸）或明确的读取/解析失败。
// POS: Linux runtime session 清理的进程观察入口；不越过 PID namespace 或 hidepid。
package transport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
		// 进程可在枚举后消失，procfs 的 open 和 read 分别可能返回 ENOENT、ESRCH。
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read process %d for session %d: %w", processID, sessionID, err)
		}
		state, currentSessionID, err := parseProcessStat(data)
		if err != nil {
			return nil, fmt.Errorf("process %d: %w", processID, err)
		}
		// 僵尸进程已不再运行，只等父进程回收；PID 1 不回收孤儿时会一直留在 /proc。
		if state == 'Z' || state == 'X' {
			continue
		}
		if currentSessionID == sessionID {
			processIDs = append(processIDs, processID)
		}
	}
	return processIDs, nil
}

// parseProcessStat 读取 /proc/<pid>/stat 的进程状态与 session；comm 可含空格和括号。
func parseProcessStat(data []byte) (byte, int, error) {
	fieldsStart := strings.LastIndexByte(string(data), ')')
	if fieldsStart < 0 {
		return 0, 0, errors.New("malformed stat")
	}
	fields := strings.Fields(string(data[fieldsStart+1:]))
	if len(fields) < 4 || len(fields[0]) != 1 {
		return 0, 0, errors.New("incomplete stat")
	}
	sessionID, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0, 0, fmt.Errorf("read session: %w", err)
	}
	return fields[0][0], sessionID, nil
}
