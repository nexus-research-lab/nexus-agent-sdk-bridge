// INPUT: 已构造的无模型 CLI 预检进程与有界 context。
// OUTPUT: 进程退出、超时和后代回收的合并结果。
// POS: 启动前 capability/settings probe 也必须使用正式 runtime 的清理边界。
package transport

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// runProbeProcess starts a parser/help-only probe with the same platform
// process boundary as the real runtime. A successful parent exit is not
// enough: a helper left in the probe session is a failed admission result.
func runProbeProcess(ctx context.Context, command *exec.Cmd) error {
	if command == nil {
		return errors.New("process: probe command is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	configureProcessSession(command)
	// 后代可能继承输出管道；主进程已退出也不能无限等待 io.Copy。
	if command.WaitDelay == 0 {
		command.WaitDelay = defaultStderrDrainTimeout
	}
	if err := command.Start(); err != nil {
		return err
	}
	session, err := startedProcessSession(command)
	if err != nil {
		killErr := command.Process.Kill()
		cleanupErr := cleanupProbeSession(session)
		waitErr := command.Wait()
		return errors.Join(err, ignoreProcessGone(killErr), waitErr, cleanupErr)
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	select {
	case waitErr := <-wait:
		return errors.Join(waitErr, cleanupProbeSession(session))
	case <-ctx.Done():
		killErr := ignoreProcessGone(command.Process.Kill())
		cleanupErr := cleanupProbeSession(session)
		waitErr := <-wait
		return errors.Join(ctx.Err(), killErr, waitErr, cleanupErr)
	}
}

func cleanupProbeSession(session processSession) error {
	terminated, err := session.cleanup()
	_ = terminated
	if err == nil {
		return nil
	}
	return &ProcessCleanupError{SessionID: session.id(), Err: err}
}

func ignoreProcessGone(err error) error {
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
