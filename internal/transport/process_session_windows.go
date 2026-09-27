//go:build windows

// INPUT: 挂起创建的 Windows runtime 进程及其 Job Object。
// OUTPUT: 可观测且可回收的进程后代边界。
// POS: Windows runtime 的进程树清理与宿主崩溃回收；不宣称权限或文件沙箱。
package transport

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
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

// configureProcessSession 禁止入口代码先于 Job 绑定执行，否则早期后代不会补入 Job。
func configureProcessSession(command *exec.Cmd) {
	if command == nil {
		return
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
}

// startedProcessSession 先建立后代边界，再恢复初始线程；任何失败由调用方终止挂起进程。
func startedProcessSession(command *exec.Cmd) (processSession, error) {
	if command == nil || command.Process == nil {
		return processSession{}, nil
	}

	session := processSession{sessionID: command.Process.Pid}
	if command.SysProcAttr == nil || command.SysProcAttr.CreationFlags&windows.CREATE_SUSPENDED == 0 {
		return session, errors.New("runtime process was not created suspended")
	}
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
	// os/exec 已关闭创建时的线程句柄；只恢复此仍挂起进程唯一的初始线程。
	// 绑定后失败保留 Job，由统一 abort/cleanup 终止所有成员。
	if err := resumeInitialRuntimeThread(uint32(command.Process.Pid)); err != nil {
		return session, fmt.Errorf("resume runtime after job assignment: %w", err)
	}
	return session, nil
}

// resumeInitialRuntimeThread 不按快照中的线程 ID 盲目恢复；开句柄后再次验证所属进程。
func resumeInitialRuntimeThread(processID uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot initial runtime thread: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	var threadID uint32
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != processID {
			continue
		}
		if threadID != 0 {
			return errors.New("suspended runtime has more than one initial thread")
		}
		threadID = entry.ThreadID
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("enumerate initial runtime thread: %w", err)
	}
	if threadID == 0 {
		return errors.New("suspended runtime initial thread is missing")
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME|windows.THREAD_QUERY_LIMITED_INFORMATION, false, threadID)
	if err != nil {
		return fmt.Errorf("open initial runtime thread: %w", err)
	}
	defer windows.CloseHandle(thread)
	owner, _, ownerErr := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessIdOfThread").Call(uintptr(thread))
	if owner == 0 {
		return fmt.Errorf("query initial runtime thread owner: %w", ownerErr)
	}
	if uint32(owner) != processID {
		return errors.New("initial runtime thread owner changed")
	}
	previous, err := windows.ResumeThread(thread)
	if err != nil {
		return fmt.Errorf("resume initial runtime thread: %w", err)
	}
	if previous != 1 {
		return fmt.Errorf("initial runtime thread suspend count is %d, want 1", previous)
	}
	return nil
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
