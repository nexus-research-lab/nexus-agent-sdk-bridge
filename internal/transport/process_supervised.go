// INPUT: 显式宿主监督工厂、解析后的本机命令及过滤后的环境。
// OUTPUT: 同一 JSON 管道协议、精确集合生命周期及不可吞掉的清理错误。
// POS: ProcessManager 的监督执行分支；准入失败不回退普通 exec。
package transport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
)

func supervisedDirectory(cwd string) (string, error) {
	if cwd != "" {
		return cwd, nil
	}
	return os.Getwd()
}

func (m *ProcessManager) startSupervised(ctx context.Context, command processCommand) error {
	config, err := m.config.Supervision(ctx, supervision.Runtime)
	if err != nil {
		return err
	}
	dir, err := supervisedDirectory(m.config.CWD)
	if err != nil {
		return err
	}
	p, err := supervision.Start(ctx, config, supervision.Command{Version: 1, Command: command.executable, Args: command.arguments(m.config.Args), Env: buildEnvironment(m.config.Env, m.config.CWD, m.config.ControlWireDialect), Directory: dir})
	m.supervised = p
	if err != nil {
		if p != nil {
			err = errors.Join(err, supervisedCleanupError(p.Close(context.Background())))
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
	m.emitDiagnostic("process_start", map[string]any{"command_path": command.path, "supervised": true, "launch_id": p.Intent().ID})
	go func() {
		defer close(m.done)
		exit, err := p.Wait(context.Background())
		err = errors.Join(supervisedExitError(exit), supervisedCleanupError(err))
		m.setWaitError(err)
		m.waitForStderrReader()
		attrs := map[string]any{"supervised": true, "launch_id": p.Intent().ID}
		if err != nil {
			attrs["error"] = err.Error()
		}
		m.emitDiagnostic("process_exit", attrs)
	}()
	return nil
}

func supervisedCleanupError(err error) error {
	if err == nil {
		return nil
	}
	return &ProcessCleanupError{Err: err}
}

// supervisedExitError 保留公开 ExitCode 语义，不伪造 os.ProcessState 或历史 PID。
type supervisedProcessExitError struct{ code, signal int }

func (e *supervisedProcessExitError) Error() string {
	if e.signal != 0 {
		return fmt.Sprintf("process: terminated by signal %d", e.signal)
	}
	return fmt.Sprintf("process: exit status %d", e.code)
}
func (e *supervisedProcessExitError) ExitCode() int {
	if e.signal != 0 {
		return -1
	}
	return e.code
}
func supervisedExitError(exit supervision.Exit) error {
	if exit.Signal != 0 || exit.Code != 0 {
		return &supervisedProcessExitError{code: exit.Code, signal: exit.Signal}
	}
	return nil
}

func (m *ProcessManager) closeSupervised() error {
	if m.supervisedStartErr != nil {
		return errors.Join(m.supervisedStartErr, supervisedCleanupError(m.supervised.Close(context.Background())))
	}
	// stdin EOF 先允许 runtime 正常退出；超时后通过原集合撤销，不发裸 PID 信号。
	forced := !m.waitForDone(defaultCloseTimeout)
	cleanupErr := supervisedCleanupError(m.supervised.Close(context.Background()))
	<-m.done
	m.closeOutputPipes()
	m.waitForStderrReader()
	if forced {
		return cleanupErr
	}
	return errors.Join(cleanupErr, normalizeExitErrorWithStderr(m.waitError(), m.stderrTail.String()))
}

func runConfiguredProbe(ctx context.Context, cmd *exec.Cmd, config ProcessConfig, purpose supervision.Purpose) error {
	if config.Supervision == nil {
		return runProbeProcess(ctx, cmd)
	}
	if config.User != "" || config.SignalProcess != nil {
		return errors.New("supervised probe cannot use identity overrides")
	}
	launchConfig, err := config.Supervision(ctx, purpose)
	if err != nil {
		return err
	}
	dir, err := supervisedDirectory(cmd.Dir)
	if err != nil {
		return err
	}
	p, err := supervision.Start(ctx, launchConfig, supervision.Command{Version: 1, Command: cmd.Path, Args: cmd.Args[1:], Env: cmd.Env, Directory: dir})
	if err != nil {
		if p != nil {
			err = errors.Join(err, supervisedCleanupError(p.Close(context.Background())))
		}
		return err
	}
	streams := p.Streams()
	_ = streams.Stdin.Close()
	defer streams.Stdout.Close()
	defer streams.Stderr.Close()
	// probe 的 writer 是调用方有界内存缓冲区；后代持管道时由集合回收释放 EOF。
	copies := make(chan error, 2)
	copyTo := func(dst io.Writer, src io.Reader) {
		if dst == nil {
			dst = io.Discard
		}
		_, err := io.Copy(dst, src)
		copies <- err
	}
	go copyTo(cmd.Stdout, streams.Stdout)
	go copyTo(cmd.Stderr, streams.Stderr)
	exit, waitErr := p.Wait(ctx)
	cleanupErr := p.Close(context.Background())
	if waitErr != nil || cleanupErr != nil {
		_ = streams.Stdout.Close()
		_ = streams.Stderr.Close()
	}
	copyErr := errors.Join(<-copies, <-copies)
	return errors.Join(waitErr, supervisedExitError(exit), supervisedCleanupError(cleanupErr), copyErr)
}
