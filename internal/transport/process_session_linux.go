//go:build linux

package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
		if err != nil {
			continue
		}
		fieldsStart := strings.LastIndexByte(string(data), ')')
		if fieldsStart < 0 {
			continue
		}
		fields := strings.Fields(string(data[fieldsStart+1:]))
		if len(fields) < 4 {
			continue
		}
		currentSessionID, err := strconv.Atoi(fields[3])
		if err == nil && currentSessionID == sessionID {
			processIDs = append(processIDs, processID)
		}
	}
	return processIDs, nil
}
