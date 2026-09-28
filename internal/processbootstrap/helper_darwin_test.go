//go:build darwin && cgo

// INPUT: 独立 launchd job、真实 helper 二进制及限界测试管道。
// OUTPUT: 启动身份绑定、登记前禁止执行、原地 exec、标准流与显式环境证据。
// POS: 引导组件原生测试，不启动产品或模型，不替代宿主持久恢复。
package processbootstrap

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
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

func TestMacOSBootstrapExec(t *testing.T) {
	if os.Getenv("NEXUS_NATIVE_SCOPE_TEST") != "1" {
		t.Skip("explicit native bootstrap acceptance")
	}
	root, err := os.MkdirTemp("/tmp", "nxs-bootstrap-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	binary := filepath.Join(root, "bootstrap")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/nexus-runtime-bootstrap")
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	identity, err := processscope.ObserverIdentity()
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(root, "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	for _, mode := range []string{"wrong_observer", "disconnect_before_admission"} {
		t.Run(mode, func(t *testing.T) {
			expected := identity
			if mode == "wrong_observer" {
				last := "0"
				if expected[len(expected)-1] == '0' {
					last = "1"
				}
				expected = expected[:len(expected)-1] + last
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, socket, expected)
			command.Env = []string{"PATH=/usr/bin:/bin"}
			var output bytes.Buffer
			command.Stdout = &output
			command.Stderr = &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if command.ProcessState == nil {
					_ = command.Process.Kill()
					_ = command.Wait()
				}
			}()
			if err := listener.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			conn, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			var ack [1]byte
			_, readErr := io.ReadFull(conn, ack[:])
			if mode == "wrong_observer" {
				if readErr != io.EOF {
					t.Fatalf("wrong observer admitted: %v", readErr)
				}
			} else {
				if readErr != nil || ack[0] != 1 {
					t.Fatalf("ready: %v %v", ack, readErr)
				}
			}
			conn.Close()
			var exitErr *exec.ExitError
			if err := command.Wait(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("rejected bootstrap exit: %v", err)
			}
			if output.String() != "runtime bootstrap admission failed\n" {
				t.Fatalf("unexpected diagnostic: %q", output.String())
			}
		})
	}
	quote := func(s string) string {
		var b bytes.Buffer
		if err := xml.EscapeText(&b, []byte(s)); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	label := fmt.Sprintf("cn.nexus.bootstrap-test.%d.%d", os.Getpid(), time.Now().UnixNano())
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	service := domain + "/" + label
	plist := fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict><key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>%s</string><string>%s</string><string>%s</string></array><key>RunAtLoad</key><true/><key>KeepAlive</key><false/><key>EnvironmentVariables</key><dict><key>BOOTSTRAP_HOST_ONLY</key><string>must-not-inherit</string></dict></dict></plist>`, quote(label), quote(binary), quote(socket), quote(identity))
	plistPath := filepath.Join(root, "job.plist")
	if err := os.WriteFile(plistPath, []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	launch := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "/bin/launchctl", args...).CombinedOutput()
		return string(out), err
	}
	var scope *processscope.Scope
	t.Cleanup(func() {
		_, _ = launch("bootout", service)
		if scope != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := scope.Reap(ctx); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
	if out, err := launch("bootstrap", domain, plistPath); err != nil {
		t.Fatalf("bootstrap: %v %s", err, out)
	}
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var ready [1]byte
	if _, err := io.ReadFull(conn, ready[:]); err != nil || ready[0] != 1 {
		t.Fatalf("ready: %v %v", ready, err)
	}
	out, err := launch("print", service)
	if err != nil {
		t.Fatal(err)
	}
	pid := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "\tpid = ") {
			value, e := strconv.Atoi(strings.TrimPrefix(line, "\tpid = "))
			if e != nil || pid != 0 {
				t.Fatal("ambiguous job PID")
			}
			pid = value
		}
	}
	if pid <= 1 {
		t.Fatalf("missing launchd PID: %s", out)
	}
	scope, err = processscope.CapturePeer(conn, pid)
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := scope.WatchRoot(conn, pid)
	if err != nil {
		t.Fatal(err)
	}
	defer monitor.Close()
	canceled, cancelWait := context.WithCancel(context.Background())
	cancelWait()
	if _, err := monitor.Wait(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled observer wait: %v", err)
	}
	stopped, err := scope.WatchRoot(conn, pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := stopped.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stopped.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := stopped.Wait(context.Background()); !errors.Is(err, processscope.ErrObservationStopped) {
		t.Fatalf("stopped observation became exit evidence: %v", err)
	}
	marker := filepath.Join(root, "result")
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("task ran before admission")
	}
	registration, _ := json.Marshal(scope.Registration())
	file, err := os.OpenFile(filepath.Join(root, "registration.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(registration); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dir.Sync(); err != nil {
		t.Fatal(err)
	}
	_ = dir.Close()
	inputR, inputW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outputR, outputW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errorR, errorW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []*os.File{inputR, inputW, outputR, outputW, errorR, errorW} {
		defer f.Close()
	}
	task := Launch{Version: 1, Command: "/bin/sh", Directory: root, Args: []string{"-c", `printf 'out:'; /bin/cat; printf 'err' >&2; test -z "$BOOTSTRAP_HOST_ONLY" || exit 42; printf '%s:%s' "$BOOTSTRAP_TASK_VALUE" "$$" > "$1"; exit 7`, "bootstrap-task", marker}, Env: []string{"PATH=/usr/bin:/bin", "BOOTSTRAP_TASK_VALUE=fixture-value"}}
	if err := SendLaunch(conn, task, [3]*os.File{inputR, outputW, errorW}); err != nil {
		t.Fatal(err)
	}
	_ = inputR.Close()
	_ = outputW.Close()
	_ = errorW.Close()
	// 任务仍阻塞在 cat，必须在交付 stdin/EOF 之前确认控制 fd 已在 exec 关闭。
	// 若只在任务退出后读取 EOF，会把普通进程退出误当作 CLOEXEC 证据。
	if _, err := conn.Read(ready[:]); err != io.EOF {
		t.Fatalf("control fd survived exec: %v", err)
	}
	if _, err := inputW.WriteString("input"); err != nil {
		t.Fatal(err)
	}
	_ = inputW.Close()
	for _, f := range []*os.File{outputR, errorR} {
		if err := f.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	stdout, err := io.ReadAll(outputR)
	if err != nil || string(stdout) != "out:input" {
		t.Fatalf("stdout=%q %v", stdout, err)
	}
	stderr, err := io.ReadAll(errorR)
	if err != nil || string(stderr) != "err" {
		t.Fatalf("stderr=%q %v", stderr, err)
	}
	result, err := os.ReadFile(marker)
	if err != nil || string(result) != fmt.Sprintf("fixture-value:%d", pid) {
		t.Fatalf("exec identity/env=%q %v", result, err)
	}
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	status, err := monitor.Wait(waitCtx)
	if err != nil || status.PID != pid || status.Code != 7 || status.Signal != 0 {
		t.Fatalf("kernel root exit: %#v %v", status, err)
	}
	if strings.Contains(plist, "fixture-value") || strings.Contains(plist, "bootstrap-task") {
		t.Fatal("task leaked into job definition")
	}
	if out, err := launch("bootout", service); err != nil {
		t.Fatalf("bootout: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := scope.Reap(ctx); err != nil {
		t.Fatal(err)
	}
}
