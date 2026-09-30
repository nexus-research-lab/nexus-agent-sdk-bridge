//go:build windows

// INPUT: 已持久Reserve的机器helper启动意图和固定受保护映像。
// OUTPUT: 实际helper进程、四条认证管道及不伪造清理证明的owner。
// POS: 不向helper传任务环境或凭据，放行业务任务由上层协议单独控制。
package windowssandbox

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"
)

type nativeMachine struct {
	tokenMu       sync.Mutex
	tokens        []windows.Token
	configuration registry.Key
	image         *nativeImage
	process       windows.ProcessInformation
	created       uint64
	hostSID       string
	pipes         [4]*nativePipe
	ctx           context.Context
	cancel        context.CancelFunc
	resumed       bool
	job           windows.Handle
}

func (m *nativeMachine) prepare(config Config) error {
	var err error
	m.hostSID, err = m.installedHost()
	if err != nil {
		return err
	}
	if err = m.verifyAccount(windows.CurrentProcess()); err != nil {
		return err
	}
	m.image, err = openNativeImage(config)
	if err != nil {
		return err
	}
	for i := range m.pipes {
		m.pipes[i], err = newNativePipe(m)
		if err != nil {
			return err
		}
	}
	m.pipes[1].stdin = true
	return nil
}

// launch 只能在Reserve成功后调用；悬挂时完成实际映像/token核验，之后才恢复可信helper。
func (m *nativeMachine) launch(ctx context.Context) error {
	created, err := nativeCreated(windows.CurrentProcess())
	if err != nil {
		return err
	}
	args := []string{m.image.path, "--internal-windows-machine-host-v1", "--control-pipe", m.pipes[0].name, "--stdin-pipe", m.pipes[1].name, "--stdout-pipe", m.pipes[2].name, "--stderr-pipe", m.pipes[3].name, "--host-pid", strconv.FormatUint(uint64(windows.GetCurrentProcessId()), 10), "--host-created", strconv.FormatUint(created, 10)}
	for i := range args {
		args[i] = windows.EscapeArg(args[i])
	}
	command, err := windows.UTF16PtrFromString(strings.Join(args, " "))
	if err != nil {
		return err
	}
	application, _ := windows.UTF16PtrFromString(m.image.path)
	directory, _ := windows.UTF16PtrFromString(filepath.Dir(m.image.path))
	system, err := windows.GetWindowsDirectory()
	if err != nil {
		return err
	}
	// UTF16FromString拒绝嵌入NUL；显式编码双NUL环境，绝不继承产品秘密。
	environment := append(utf16.Encode([]rune("PATH="+filepath.Join(system, "System32")+"\x00SystemRoot="+system+"\x00")), 0)
	if err = m.createHelper(application, command, directory, environment); err != nil {
		return err
	}
	runtime.KeepAlive(environment)
	m.created, err = nativeCreated(m.process.Process)
	if err != nil {
		return err
	}
	if err = m.image.verifyProcess(m.process.Process); err != nil {
		return err
	}
	if err = m.verifyAccount(m.process.Process); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	previous, err := windows.ResumeThread(m.process.Thread)
	m.resumed = true
	if err != nil || previous != 1 {
		return errors.Join(errors.New("helper initial resume result is unknown"), err)
	}
	for _, pipe := range m.pipes {
		if err = pipe.connect(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (m *nativeMachine) wait(ctx context.Context) (uint32, error) {
	if m.process.Process == 0 {
		return 0, errors.New("helper was not created")
	}
	for {
		state, err := windows.WaitForSingleObject(m.process.Process, 20)
		if err != nil {
			return 0, err
		}
		if state == windows.WAIT_OBJECT_0 {
			var code uint32
			err = windows.GetExitCodeProcess(m.process.Process, &code)
			return code, err
		}
		if err = ctx.Err(); err != nil {
			return 0, err
		}
	}
}

// close 只允许实际退出或从未恢复的helper收口；已运行未知helper不强杀、不丢owner。
func (m *nativeMachine) close(ctx context.Context) error {
	if m.process.Process != 0 {
		state, err := windows.WaitForSingleObject(m.process.Process, 0)
		if err != nil {
			return err
		}
		if state != windows.WAIT_OBJECT_0 {
			if m.resumed {
				return errors.New("helper cleanup is unknown; live process owner retained")
			}
			if err = windows.TerminateProcess(m.process.Process, 1); err != nil {
				return err
			}
			if _, err = m.wait(ctx); err != nil {
				return err
			}
		}
	}
	m.cancel()
	if err := m.verifyJobEmpty(); err != nil {
		return err
	}
	for _, pipe := range m.pipes {
		if pipe != nil {
			if err := pipe.release(); err != nil {
				return err
			}
		}
	}
	m.tokenMu.Lock()
	for len(m.tokens) > 0 {
		if err := m.tokens[0].Close(); err != nil {
			m.tokenMu.Unlock()
			return err
		}
		m.tokens = m.tokens[1:]
	}
	m.tokenMu.Unlock()
	if m.configuration != 0 {
		if err := m.configuration.Close(); err != nil {
			return err
		}
		m.configuration = 0
	}
	for _, slot := range []*windows.Handle{&m.process.Thread, &m.process.Process} {
		if *slot != 0 {
			if err := windows.CloseHandle(*slot); err != nil {
				return err
			}
			*slot = 0
		}
	}
	if m.image != nil {
		if err := m.image.Close(); err != nil {
			return err
		}
		m.image = nil
	}
	if m.job != 0 {
		if err := windows.CloseHandle(m.job); err != nil {
			return err
		}
		m.job = 0
	}
	return nil
}
