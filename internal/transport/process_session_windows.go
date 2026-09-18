//go:build windows

// INPUT: 已启动的 Windows runtime 进程及其 Job Object。
// OUTPUT: 可观测且可回收的进程后代边界。
// POS: Windows runtime 的进程树清理与宿主崩溃回收；不宣称权限或文件沙箱。
package transport

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsProcessCleanupTimeoutMS = 5000

type windowsJobSession struct {
	job          windows.Handle
	cleanupOnce  sync.Once
	terminated   int
	cleanupError error
}

type processSession struct {
	sessionID int
	state     *windowsJobSession
}

// Windows 没有 Unix session；Job Object 在进程启动后绑定，覆盖通过 shell 或
// runtime 自行派生的后代，并在宿主句柄消失时由 KILL_ON_JOB_CLOSE 收口。
func configureProcessSession(_ *exec.Cmd) {}

func startedProcessSession(command *exec.Cmd) (processSession, error) {
	if command == nil || command.Process == nil {
		return processSession{}, nil
	}

	session := processSession{sessionID: command.Process.Pid}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return session, fmt.Errorf("create runtime job object: %w", err)
	}
	state := &windowsJobSession{job: job}
	session.state = state

	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		_ = windows.CloseHandle(job)
		session.state = nil
		return session, fmt.Errorf("configure runtime job object: %w", err)
	}

	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(command.Process.Pid),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		session.state = nil
		return session, fmt.Errorf("open runtime process for job object: %w", err)
	}
	defer windows.CloseHandle(processHandle)

	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		_ = windows.CloseHandle(job)
		session.state = nil
		return session, fmt.Errorf("assign runtime process to job object: %w", err)
	}
	return session, nil
}

func (s processSession) id() int { return s.sessionID }

func (s processSession) hasDirectCleanup() bool {
	return s.state != nil && s.state.job != 0
}

func (s processSession) cleanup() (int, error) {
	if s.state == nil || s.state.job == 0 {
		return 0, nil
	}
	state := s.state
	state.cleanupOnce.Do(func() {
		defer func() {
			if err := windows.CloseHandle(state.job); err != nil {
				state.cleanupError = errors.Join(state.cleanupError, fmt.Errorf("close runtime job object: %w", err))
			}
			state.job = 0
		}()

		// Job Object 在所有成员退出后可等待；先观察一次，避免对已自然
		// 退出的 runtime 把 ERROR_ACCESS_DENIED 误报成清理失败。
		if event, err := windows.WaitForSingleObject(state.job, 0); err == nil && event == windows.WAIT_OBJECT_0 {
			return
		}

		if err := windows.TerminateJobObject(state.job, 1); err != nil {
			// 主进程可能恰好在初次观察后退出；再次等待可证明完整 Job
			// 已收口，不能把竞态误判为失败。
			if event, waitErr := windows.WaitForSingleObject(state.job, windowsProcessCleanupTimeoutMS); waitErr == nil && event == windows.WAIT_OBJECT_0 {
				return
			}
			state.cleanupError = fmt.Errorf("terminate runtime job object: %w", err)
			return
		}
		state.terminated = 1
		event, waitErr := windows.WaitForSingleObject(state.job, windowsProcessCleanupTimeoutMS)
		if waitErr != nil {
			state.cleanupError = fmt.Errorf("wait for runtime job object: %w", waitErr)
			return
		}
		if event != windows.WAIT_OBJECT_0 {
			state.cleanupError = fmt.Errorf("runtime job object did not become empty: wait=%d", event)
		}
	})
	return state.terminated, state.cleanupError
}
