//go:build windows

// INPUT: 原生 Windows helper 在入口立即派生并继承输出管道。
// OUTPUT: Job 准入前无执行，正常退出/取消/宿主崩溃均能回收已准入的后代。
// POS: Bridge Windows 生命周期回归；不把 Job 当作文件或权限沙箱。
package transport

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestWindowsJobAdmissionAndCleanup 故意延长 Start 与 Job 绑定的间隔，复现原先的逃逸窗口。
func TestWindowsJobAdmissionAndCleanup(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := windowsJobTestCommand(ctx, root, "parent")
	configureProcessSession(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	time.Sleep(150 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(root, "child.pid")); !os.IsNotExist(err) {
		t.Fatalf("runtime executed before Job assignment: %v", err)
	}
	session, err := startedProcessSession(command)
	if err != nil {
		t.Fatal(err)
	}
	defer session.cleanup()
	child := openWindowsJobTestProcess(t, root, "child.pid")
	var inJob int32
	result, _, queryErr := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob").Call(
		uintptr(child), uintptr(session.state.job), uintptr(unsafe.Pointer(&inJob)),
	)
	if result == 0 || inJob == 0 {
		t.Fatalf("earliest descendant is outside runtime Job: inJob=%d, err=%v", inJob, queryErr)
	}
	if err := os.WriteFile(filepath.Join(root, "continue"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := session.cleanup(); err != nil {
		t.Fatal(err)
	}
	assertWindowsJobTestProcessExited(t, child)
	if _, err := session.cleanup(); err != nil {
		t.Fatalf("repeated cleanup: %v", err)
	}
}

// TestWindowsProbeCancellationClosesJob 覆盖后代继承管道时的取消，不允许 Wait 卡在输出复制。
func TestWindowsProbeCancellationClosesJob(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command := windowsJobTestCommand(t.Context(), root, "parent")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	done := make(chan error, 1)
	go func() { done <- runProbeProcess(ctx, command) }()
	child := openWindowsJobTestProcess(t, root, "child.pid")
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("probe cancellation lost: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("probe cancellation waited for inherited output handles")
	}
	assertWindowsJobTestProcessExited(t, child)
}

// TestWindowsProbeRejectsLingeringOutput 覆盖父进程成功退出但后代继续持有管道的路径。
func TestWindowsProbeRejectsLingeringOutput(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := windowsJobTestCommand(ctx, root, "parent")
	var output bytes.Buffer
	command.Stdout = &output
	done := make(chan error, 1)
	go func() { done <- runProbeProcess(ctx, command) }()
	child := openWindowsJobTestProcess(t, root, "child.pid")
	if err := os.WriteFile(filepath.Join(root, "continue"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, exec.ErrWaitDelay) {
			t.Fatalf("probe with inherited output must fail admission: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("probe waited for descendant output after parent exit")
	}
	assertWindowsJobTestProcessExited(t, child)
}

// TestWindowsHostCrashClosesRuntimeJob 直接终止持有 Job 的宿主，验证内核收口而非 defer 清理。
func TestWindowsHostCrashClosesRuntimeJob(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	host := windowsJobTestCommand(ctx, root, "host")
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Process.Kill() })
	child := openWindowsJobTestProcess(t, root, "child.pid")
	runtimeProcess := openWindowsJobTestProcess(t, root, "runtime.pid")
	if err := host.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := host.Wait(); err == nil {
		t.Fatal("host was not forcibly terminated")
	}
	assertWindowsJobTestProcessExited(t, runtimeProcess)
	assertWindowsJobTestProcessExited(t, child)
}

// windowsJobTestCommand 使用当前测试二进制，避免 shell 环境或已安装工具影响生命周期证据。
func windowsJobTestCommand(ctx context.Context, root, mode string) *exec.Cmd {
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsJobProcessHelper$")
	command.Env = append(os.Environ(), "NEXUS_WINDOWS_JOB_TEST_MODE="+mode, "NEXUS_WINDOWS_JOB_TEST_ROOT="+root)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command
}

// openWindowsJobTestProcess 等待已创建的 helper，并持有准确进程句柄用于检查及失败清理。
func openWindowsJobTestProcess(t *testing.T, root, name string) windows.Handle {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			pid, parseErr := strconv.ParseUint(strings.TrimSpace(string(body)), 10, 32)
			if parseErr == nil {
				handle, openErr := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, uint32(pid))
				if openErr != nil {
					t.Fatal(openErr)
				}
				t.Cleanup(func() { _ = windows.TerminateProcess(handle, 1); _ = windows.CloseHandle(handle) })
				return handle
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("helper did not publish %s", name)
	return 0
}

// assertWindowsJobTestProcessExited 检查内核退出状态，不把管道关闭或发送 kill 当作完成。
func assertWindowsJobTestProcessExited(t *testing.T, process windows.Handle) {
	t.Helper()
	event, err := windows.WaitForSingleObject(process, 5000)
	if err != nil || event != windows.WAIT_OBJECT_0 {
		t.Fatalf("process remains after Job cleanup: event=%d, err=%v", event, err)
	}
}

// TestWindowsJobProcessHelper 的父模式立即派生，宿主模式走真实 ProcessManager。
func TestWindowsJobProcessHelper(t *testing.T) {
	mode := os.Getenv("NEXUS_WINDOWS_JOB_TEST_MODE")
	root := os.Getenv("NEXUS_WINDOWS_JOB_TEST_ROOT")
	switch mode {
	case "":
		return
	case "leaf":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "host":
		manager := NewProcessManager(ProcessConfig{
			CommandPath: os.Args[0], Args: []string{"-test.run=^TestWindowsJobProcessHelper$"},
			Env: map[string]string{"NEXUS_WINDOWS_JOB_TEST_MODE": "parent", "NEXUS_WINDOWS_JOB_TEST_ROOT": root, skipVersionCheckEnv: "1"},
		})
		if err := manager.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Second)
		_ = manager.Close()
		t.Fatal("host was not terminated")
	case "parent":
		child := windowsJobTestCommand(t.Context(), root, "leaf")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		for name, pid := range map[string]int{"child.pid": child.Process.Pid, "runtime.pid": os.Getpid()} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(strconv.Itoa(pid)), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(root, "continue")); err == nil {
				os.Exit(0)
			}
			time.Sleep(5 * time.Millisecond)
		}
		_ = child.Process.Kill()
		t.Fatal("parent was not terminated")
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}
