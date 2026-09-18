package transport

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClaudeRestrictedProbeRequiresAdvertisedFlagAndScrubsSecrets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows CLI probe uses a PowerShell shim and is covered by the Windows acceptance job")
	}
	command := writeProbeScript(t, `#!/bin/sh
if [ "${NEXUS_PROBE_SECRET_TOKEN:-}" = "probe-secret" ]; then
  echo leaked-secret >&2
  exit 91
fi
if [ "$1" = "--restricted" ] && [ "$2" = "--help" ]; then
  echo 'Claude Code options: --restricted'
  exit 0
fi
exit 92
`)

	err := verifyClaudeRestrictedCommand(context.Background(), processCommand{path: command, executable: command}, ProcessConfig{
		Env: map[string]string{
			"NEXUS_PROBE_SECRET_TOKEN": "probe-secret",
		},
		ControlWireDialect: ControlWireDialectClaude,
	})
	if err != nil {
		t.Fatalf("restricted probe rejected a compatible CLI: %v", err)
	}
}

func TestClaudeRestrictedProbeFailsClosedWhenFlagIsMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows CLI probe uses a PowerShell shim and is covered by the Windows acceptance job")
	}
	command := writeProbeScript(t, `#!/bin/sh
echo 'Claude Code options: --print'
exit 0
`)
	err := verifyClaudeRestrictedCommand(context.Background(), processCommand{path: command, executable: command}, ProcessConfig{
		ControlWireDialect: ControlWireDialectClaude,
	})
	if err == nil || !strings.Contains(err.Error(), "did not advertise --restricted") {
		t.Fatalf("missing restricted flag was accepted: %v", err)
	}
}

func writeProbeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
