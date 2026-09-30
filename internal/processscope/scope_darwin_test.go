//go:build darwin && cgo

// INPUT: 显式启用的本机 launchd 测试 job、Unix 控制连接与独立状态目录。
// OUTPUT: 连接身份登记后放行、脱离及恢复、拒绝错误 token、内核回收与对照存活证据。
// POS: 原生组件验收；不启动产品、模型或用户已有会话。
package processscope

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestMacOSScopeWorker 只在测试启动的唯一 job 内运行，异常情况下自主退出。
func TestMacOSScopeWorker(t *testing.T) {
	mode := os.Getenv("NEXUS_SCOPE_HELPER")
	if mode == "" {
		t.Skip("test process only")
	}
	root := os.Getenv("NEXUS_SCOPE_FIXTURE")
	if root == "" {
		t.Fatal("missing fixture root")
	}
	if mode == "child" {
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(filepath.Join(root, "child"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Second)
		return
	}
	if mode != "root" {
		t.Fatal("unknown fixture mode")
	}
	if err := os.WriteFile(filepath.Join(root, "root"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: os.Getenv("NEXUS_SCOPE_SOCKET"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var start [1]byte
	if _, err := io.ReadFull(conn, start[:]); err != nil {
		return
	}
	if start[0] != 1 {
		t.Fatal("invalid admission")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.run=^TestMacOSScopeWorker$")
	child.Env = []string{"PATH=/usr/bin:/bin", "NEXUS_SCOPE_HELPER=child", "NEXUS_SCOPE_FIXTURE=" + root}
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	// 不等待后代，故意形成另建 session 的孤儿；它有自己的退出期限。
}

func TestMacOSScopeReapsDetached(t *testing.T) {
	if os.Getenv("NEXUS_NATIVE_SCOPE_TEST") != "1" {
		t.Skip("explicit native process scope acceptance")
	}
	root := t.TempDir()
	// 使用短路径，避免 sockaddr_un 的 104 字节上限；目录仍为独立 0700。
	socketRoot, err := os.MkdirTemp("/tmp", "nxs-scope-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketRoot) })
	socketPath := filepath.Join(socketRoot, "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	label := fmt.Sprintf("cn.nexus.scope-test.%d.%d", os.Getpid(), time.Now().UnixNano())
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
	quote := func(value string) string {
		var b bytes.Buffer
		if err := xml.EscapeText(&b, []byte(value)); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	plist := fmt.Sprintf(`<?xml version="1.0"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>%s</string><string>-test.run=^TestMacOSScopeWorker$</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><false/><key>AbandonProcessGroup</key><false/>
<key>EnvironmentVariables</key><dict><key>NEXUS_SCOPE_HELPER</key><string>root</string><key>NEXUS_SCOPE_FIXTURE</key><string>%s</string><key>NEXUS_SCOPE_SOCKET</key><string>%s</string></dict>
<key>StandardOutPath</key><string>%s</string><key>StandardErrorPath</key><string>%s</string></dict></plist>`, quote(label), quote(exe), quote(root), quote(socketPath), quote(filepath.Join(root, "stdout")), quote(filepath.Join(root, "stderr")))
	path := filepath.Join(root, "job.plist")
	if err := os.WriteFile(path, []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	launch := func(args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "/bin/launchctl", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("launchctl %v: %w (%s)", args, err, out)
		}
		return nil
	}
	var scope *Scope
	t.Cleanup(func() {
		_ = launch("bootout", service)
		if scope != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := scope.Reap(ctx); err != nil {
				t.Errorf("fixture cleanup: %v", err)
			}
		}
	})
	if err := launch("bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path); err != nil {
		t.Fatal(err)
	}
	waitPID := func(name string) int {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				if pid, err := strconv.Atoi(string(data)); err == nil && pid > 1 {
					return pid
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		stderr, _ := os.ReadFile(filepath.Join(root, "stderr"))
		t.Fatalf("missing %s identity: %s", name, stderr)
		return 0
	}
	rootPID := waitPID("root")
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := CapturePeer(conn, rootPID+1); err == nil {
		t.Fatal("wrong expected launcher PID accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("child executed before registration")
	}
	scope, err = CapturePeer(conn, rootPID)
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := scope.WatchRoot(conn, rootPID)
	if err != nil {
		t.Fatal(err)
	}
	defer monitor.Close()
	encoded, err := json.Marshal(scope.Registration())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "registration.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	childPID := waitPID("child")
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	rootExit, err := monitor.Wait(waitCtx)
	if err != nil || rootExit.PID != rootPID || rootExit.Code != 0 || rootExit.Signal != 0 {
		t.Fatalf("root exit evidence: %#v %v", rootExit, err)
	}
	// 根退出事实不能代表后代退出；下面必须仍能观察到脱离后代并独立回收。
	child, err := newKernel().inspect(childPID)
	if err != nil {
		t.Fatal(err)
	}
	if child.coalition != scope.Registration().CoalitionID {
		t.Fatal("detached child left coalition")
	}
	sid, err := syscall.Getsid(childPID)
	if err != nil || sid != childPID {
		t.Fatalf("child not detached: %d %v", sid, err)
	}
	wrong := child
	wrong.audit[7] ^= 0x40000000
	if err := newKernel().signal(wrong, int(syscall.SIGKILL)); !errors.Is(err, ErrGone) {
		t.Fatalf("wrong PID version accepted: %v", err)
	}
	control := exec.Command("/bin/sleep", "20")
	if err := control.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = control.Process.Kill(); _ = control.Wait() })
	controlIdentity, err := newKernel().inspect(control.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := launch("bootout", service); err != nil {
		t.Fatal(err)
	}
	// bootout 成功也必须继续观察；该后代忽略 SIGTERM，不能冒充自然退出。
	if _, err := newKernel().inspect(childPID); err != nil {
		t.Fatalf("detached fixture vanished before component cleanup: %v", err)
	}
	var restored Registration
	stored, err := os.ReadFile(filepath.Join(root, "registration.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stored, &restored); err != nil {
		t.Fatal(err)
	}
	scope, err = Restore(restored)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	evidence, err := scope.Reap(ctx)
	if err != nil || evidence.Reason != "coalition_reaped" {
		t.Fatalf("native cleanup: %#v %v", evidence, err)
	}
	if err := newKernel().exists(restored.CoalitionID); !errors.Is(err, ErrGone) {
		t.Fatalf("coalition retained: %v", err)
	}
	after, err := newKernel().inspect(control.Process.Pid)
	if err != nil || after != controlIdentity {
		t.Fatalf("control affected: %#v %v", after, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := CapturePeer(conn, rootPID); err == nil {
		t.Fatal("closed control connection accepted")
	}
	if err := launch("print", service); err == nil {
		t.Fatal("test job still registered")
	}
}
