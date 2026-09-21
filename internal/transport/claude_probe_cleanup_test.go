//go:build darwin || linux

package transport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestClaudeProbesCleanDescendantsBeforeReturning(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		run  func(context.Context, processCommand, ProcessConfig) error
	}{
		{
			name: "restricted",
			args: []string{"--restricted", "--help"},
			run:  verifyClaudeRestrictedCommand,
		},
		{
			name: "native settings",
			args: []string{"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}}`, "--help"},
			run:  verifyClaudeNativeSandboxCommand,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "child.pid")
			commandPath := writeProbeScript(t, `#!/bin/sh
sleep 60 >/dev/null 2>&1 &
printf '%s' "$!" > "$NEXUS_PROBE_CHILD_MARKER"
echo 'Claude Code options: --restricted --settings <file-or-json>'
exit 0
`)
			config := ProcessConfig{
				Args:               test.args,
				Env:                map[string]string{"NEXUS_PROBE_CHILD_MARKER": marker},
				ControlWireDialect: ControlWireDialectClaude,
			}
			if strings.HasPrefix(test.name, "native") {
				config.RequireClaudeNativeSandbox = true
			}
			command := processCommand{
				path:       commandPath,
				executable: commandPath,
			}
			if err := test.run(context.Background(), command, config); err != nil {
				t.Fatalf("probe error = %v", err)
			}
			data, err := os.ReadFile(marker)
			if err != nil {
				t.Fatalf("read child marker: %v", err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil || pid <= 1 {
				t.Fatalf("child pid = %q, err=%v", data, err)
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				err = syscall.Kill(pid, 0)
				if errors.Is(err, syscall.ESRCH) {
					return
				}
				if err != nil {
					t.Fatalf("inspect child %d: %v", pid, err)
				}
				if time.Now().After(deadline) {
					t.Fatalf("probe descendant %d survived cleanup", pid)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
