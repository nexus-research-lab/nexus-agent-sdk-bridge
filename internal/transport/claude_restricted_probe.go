// INPUT: Bridge 已解析的 Claude CLI 命令、受限启动要求和宿主执行环境。
// OUTPUT: 在正式 stream-json 进程启动前确认原生 --restricted 可用。
// POS: Claude 专属的 transport admission；不冒用 nxs 能力，也不证明完整 OS 隔离。
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	claudeRestrictedProbeTimeout = 2 * time.Second
	claudeRestrictedProbeLimit   = 256 * 1024
)

// verifyClaudeRestrictedCommand runs the exact resolved CLI (including a
// Windows PowerShell shim when applicable) with a no-prompt parser probe. The
// probe intentionally receives a scrubbed environment: checking a flag must
// not expose Provider credentials or proxy secrets to a helper process.
func verifyClaudeRestrictedCommand(
	parent context.Context,
	command processCommand,
	config ProcessConfig,
) error {
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, claudeRestrictedProbeTimeout)
	defer cancel()

	cmd := exec.Command(command.executable, command.arguments([]string{"--restricted", "--help"})...)
	cmd.Dir = config.CWD
	cmd.Env = buildClaudeRestrictedProbeEnvironment(config.Env, config.CWD, config.ControlWireDialect)
	if err := applyCommandUser(cmd, config.User); err != nil {
		return fmt.Errorf("process: Claude restricted probe user setup failed: %w", err)
	}
	configureProcessSession(cmd)

	var stdout, stderr limitedProbeBuffer
	stdout.limit = claudeRestrictedProbeLimit
	stderr.limit = claudeRestrictedProbeLimit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := runProbeProcess(ctx, cmd); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("process: Claude restricted probe timed out after %s", claudeRestrictedProbeTimeout)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("process: Claude CLI does not accept --restricted: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	output := strings.ToLower(stdout.String() + "\n" + stderr.String())
	if !strings.Contains(output, "--restricted") {
		return errors.New("process: Claude restricted probe did not advertise --restricted")
	}
	return nil
}

// buildClaudeRestrictedProbeEnvironment keeps path/configuration discovery but
// removes common secret and network-credential variables. Unknown variables
// remain available for CLI launchers; the probe is not a general environment
// sanitizer for the actual Claude runtime.
func buildClaudeRestrictedProbeEnvironment(
	overrides map[string]string,
	cwd string,
	dialect ControlWireDialect,
) []string {
	environment := buildEnvironment(overrides, cwd, dialect)
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		key, _, ok := splitProcessEnvironmentEntry(entry)
		if !ok || claudeRestrictedProbeSecretKey(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func claudeRestrictedProbeSecretKey(key string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if normalized == "" {
		return true
	}
	for _, marker := range []string{
		"_API_KEY", "_AUTH_TOKEN", "_TOKEN", "_SECRET", "_PASSWORD", "_PASS",
		"_CREDENTIAL", "AUTHORIZATION", "COOKIE", "HTTP_PROXY", "HTTPS_PROXY",
		"ALL_PROXY", "NO_PROXY",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

type limitedProbeBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedProbeBuffer) Write(payload []byte) (int, error) {
	if b.limit <= 0 {
		b.limit = claudeRestrictedProbeLimit
	}
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(payload), nil
	}
	if len(payload) > remaining {
		_, _ = b.Buffer.Write(payload[:remaining])
		b.truncated = true
		return len(payload), nil
	}
	return b.Buffer.Write(payload)
}

var _ io.Writer = (*limitedProbeBuffer)(nil)
