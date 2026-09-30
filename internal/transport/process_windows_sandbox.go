// INPUT: 可信Windows机器监督工厂、最终命令及明确的probe用途。
// OUTPUT: 既有stream-json传输与独立的完整清理结果；未知结果不降级普通exec。
// POS: ProcessManager的机器沙箱适配，非Windows平台由后端明确拒绝。
package transport

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/windowssandbox"
)

// startWindowsSandbox 在工厂和后端成功前不交出stdio，也不创建普通runtime进程。
func (m *ProcessManager) startWindowsSandbox(ctx context.Context, command processCommand) error {
	config, err := m.config.WindowsSandbox(ctx, supervision.Runtime)
	if err != nil {
		return err
	}
	directory, err := supervisedDirectory(m.config.CWD)
	if err != nil {
		return err
	}
	p, err := windowssandbox.Start(ctx, config, windowssandbox.Command{Program: command.executable, Arguments: command.arguments(m.config.Args),
		Directory: directory, Environment: buildEnvironment(m.config.Env, m.config.CWD, m.config.ControlWireDialect)})
	m.windowsSandbox = p
	if err != nil {
		if p != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err = errors.Join(err, supervisedCleanupError(p.Close(cleanup)))
		}
		return err
	}
	streams := p.Streams()
	m.stdin, m.stdout, m.stderr = streams.Stdin, streams.Stdout, streams.Stderr
	m.reader = bufio.NewReader(streams.Stdout)
	m.maxBufferSize = m.config.MaxBufferSize
	if m.maxBufferSize <= 0 {
		m.maxBufferSize = defaultMaxBufferSize
	}
	m.stderrWG.Add(1)
	go m.readStderr(streams.Stderr)
	m.emitDiagnostic("process_start", map[string]any{"command_path": command.path, "windows_sandbox": true, "launch_id": p.Intent().LaunchID})
	go func() {
		defer close(m.done)
		outcome, err := p.Wait(context.Background())
		err = windowsSandboxOutcomeError(outcome, err)
		m.setWaitError(err)
		m.waitForStderrReader()
		attributes := map[string]any{"windows_sandbox": true, "launch_id": p.Intent().LaunchID, "cleaned": outcome.Cleaned}
		if err != nil {
			attributes["error"] = err.Error()
		}
		m.emitDiagnostic("process_exit", attributes)
	}()
	return nil
}

// windowsSandboxOutcomeError 保留真实退出状态与独立清理失败，不以退出零证明资源已撤销。
func windowsSandboxOutcomeError(outcome windowssandbox.Outcome, err error) error {
	if !outcome.Cleaned {
		err = errors.Join(err, errors.New("Windows sandbox cleanup remains unconfirmed"))
	}
	var exitErr error
	if outcome.Cleaned && outcome.ExitCode != 0 {
		exitErr = &supervisedProcessExitError{code: int(outcome.ExitCode)}
	}
	return errors.Join(exitErr, supervisedCleanupError(err))
}

// closeWindowsSandbox 有序EOF后请求原owner关闭；超时保留类型化unknown，不发送裸PID信号。
func (m *ProcessManager) closeWindowsSandbox() error {
	cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if m.supervisedStartErr != nil {
		return errors.Join(m.supervisedStartErr, supervisedCleanupError(m.windowsSandbox.Close(cleanup)))
	}
	forced := !m.waitForDone(defaultCloseTimeout)
	closeErr := supervisedCleanupError(m.windowsSandbox.Close(cleanup))
	select {
	case <-m.done:
	case <-cleanup.Done():
		closeErr = errors.Join(closeErr, supervisedCleanupError(cleanup.Err()))
	}
	m.closeOutputPipes()
	m.waitForStderrReader()
	if forced {
		return closeErr
	}
	return errors.Join(closeErr, normalizeExitErrorWithStderr(m.waitError(), m.stderrTail.String()))
}

// runWindowsSandboxProbe 每次probe拥有独立授权与登记，不沿用主runtime的执行身份。
func runWindowsSandboxProbe(ctx context.Context, cmd *exec.Cmd, config ProcessConfig, purpose supervision.Purpose) error {
	if config.Supervision != nil || strings.TrimSpace(config.User) != "" || config.SignalProcess != nil {
		return errors.New("Windows sandbox probe cannot combine supervision or identity overrides")
	}
	launch, err := config.WindowsSandbox(ctx, purpose)
	if err != nil {
		return err
	}
	directory, err := supervisedDirectory(cmd.Dir)
	if err != nil {
		return err
	}
	p, err := windowssandbox.Start(ctx, launch, windowssandbox.Command{Program: cmd.Path, Arguments: cmd.Args[1:], Environment: cmd.Env, Directory: directory})
	if err != nil {
		if p != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err = errors.Join(err, supervisedCleanupError(p.Close(cleanup)))
		}
		return err
	}
	streams := p.Streams()
	inputErr := streams.Stdin.Close()
	defer streams.Stdout.Close()
	defer streams.Stderr.Close()
	copies := make(chan error, 2)
	copyTo := func(destination io.Writer, source io.Reader) {
		if destination == nil {
			destination = io.Discard
		}
		_, err := io.Copy(destination, source)
		copies <- err
	}
	go copyTo(cmd.Stdout, streams.Stdout)
	go copyTo(cmd.Stderr, streams.Stderr)
	var outcome windowssandbox.Outcome
	var waitErr error
	if inputErr == nil {
		outcome, waitErr = p.Wait(ctx)
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	closeErr := p.Close(cleanup)
	if inputErr != nil || waitErr != nil || closeErr != nil || !outcome.Cleaned {
		_ = streams.Stdout.Close()
		_ = streams.Stderr.Close()
	}
	copyErr := errors.Join(<-copies, <-copies)
	return errors.Join(inputErr, windowsSandboxOutcomeError(outcome, waitErr), supervisedCleanupError(closeErr), copyErr)
}
