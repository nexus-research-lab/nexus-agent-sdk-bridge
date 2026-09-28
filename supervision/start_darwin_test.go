//go:build darwin && cgo

// INPUT: 显式启用的真实 helper/launchd 与独立持久回调夹具。
// OUTPUT: 正确阶段顺序、标准流/退出码与各阶段失败后的非重放收口。
// POS: 启动器原生验收，不替代 Nexus 数据库适配或 App 发布验收。
package supervision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

var fixtureFailure = errors.New("injected durable stage failure")

type fixtureHost struct {
	root         string
	failure      string
	steps        []string
	intent       Intent
	registration Registration
	proof        *Evidence
	paths        Paths
	phase        string
}

func (h *fixtureHost) Reserve(ctx context.Context, i Intent) (Paths, error) {
	h.steps = append(h.steps, "reserve")
	h.intent = i
	h.phase = "prepared"
	directory := filepath.Join(h.root, i.ID)
	if err := os.Mkdir(directory, 0700); err != nil {
		return Paths{}, err
	}
	h.paths = Paths{JobFile: filepath.Join(directory, "job.plist"), Socket: filepath.Join(directory, "s")}
	if err := h.persist(); err != nil {
		return Paths{}, err
	}
	if h.failure == "reserve" {
		return Paths{}, fixtureFailure
	}
	return h.paths, nil
}
func (h *fixtureHost) Publish(ctx context.Context, i Intent, data []byte) error {
	h.steps = append(h.steps, "publish")
	if i != h.intent {
		return errors.New("wrong intent")
	}
	if strings.Contains(string(data), "TASK_FIXTURE_TOKEN") {
		return errors.New("task data leaked into plist")
	}
	if h.failure == "publish" {
		return fixtureFailure
	}
	file, err := os.OpenFile(h.paths.JobFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}
func (h *fixtureHost) Register(ctx context.Context, i Intent, r Registration) error {
	h.steps = append(h.steps, "register")
	if i != h.intent || h.phase != "prepared" {
		return errors.New("invalid registration order")
	}
	h.registration = r
	h.phase = "registered"
	if err := h.persist(); err != nil {
		return err
	}
	if h.failure == "register" {
		return fixtureFailure
	}
	return nil
}
func (h *fixtureHost) ClaimRelease(ctx context.Context, i Intent) error {
	h.steps = append(h.steps, "release")
	if i != h.intent || h.phase != "registered" {
		return errors.New("duplicate or invalid release")
	}
	h.phase = "released"
	if err := h.persist(); err != nil {
		return err
	}
	// 模拟事务成功但响应丢失；调用方不能据此重新发送。
	if h.failure == "release" {
		return fixtureFailure
	}
	return nil
}
func (h *fixtureHost) Finish(ctx context.Context, i Intent, e *Evidence) error {
	h.steps = append(h.steps, "finish")
	if h.failure == "finish" {
		return fixtureFailure
	}
	if i != h.intent {
		return errors.New("wrong finish intent")
	}
	if h.phase == "prepared" {
		if e != nil && e.Registration.BootID != i.BootID {
			return errors.New("wrong proof")
		}
		h.phase = "aborted"
	} else {
		if e == nil || e.Registration != h.registration {
			return errors.New("missing exact proof")
		}
		if e.Reason != "coalition_reaped" && e.Reason != "boot_changed" {
			return errors.New("invalid proof")
		}
		h.phase = "reaped"
	}
	h.proof = e
	if err := h.persist(); err != nil {
		return err
	}
	_ = os.Remove(h.paths.JobFile)
	_ = os.Remove(h.paths.Socket)
	return nil
}
func (h *fixtureHost) persist() error {
	data, err := json.Marshal(struct {
		Intent       Intent
		Phase        string
		Registration Registration
		Evidence     *Evidence
	}{h.intent, h.phase, h.registration, h.proof})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(h.root, h.intent.ID, "record.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func TestMacOSSupervisedStart(t *testing.T) {
	if os.Getenv("NEXUS_NATIVE_SCOPE_TEST") != "1" {
		t.Skip("explicit native supervised launch acceptance")
	}
	root, err := os.MkdirTemp("/tmp", "nxs-launch-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	binary := filepath.Join(root, "bootstrap")
	build := exec.Command("go", "build", "-o", binary, "../cmd/nexus-runtime-bootstrap")
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	t.Run("detached_output", func(t *testing.T) {
		host := &fixtureHost{root: root}
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := Command{Version: 1, Command: executable, Directory: root, Args: []string{"-test.run=^TestSupervisionDetachedWorker$"}, Env: []string{"PATH=/usr/bin:/bin", "NEXUS_SUPERVISION_WORKER=root", "NEXUS_SUPERVISION_MARKER=" + filepath.Join(root, "detached-ready")}}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		process, err := Start(ctx, Config{HelperPath: binary, HelperSHA256: digest, Host: host}, command)
		if err != nil {
			t.Fatal(err)
		}
		defer process.Close(context.Background())
		streams := process.Streams()
		defer streams.Stdout.Close()
		defer streams.Stderr.Close()
		streams.Stdin.Close()
		// 在显式 Wait 之前读 EOF，证明主进程退出会主动收口持有输出管道的脱离后代。
		if err := streams.Stdout.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(streams.Stdout); err != nil {
			t.Fatalf("detached output retained: %v", err)
		}
		status, err := process.Wait(ctx)
		if err != nil || status.Code != 0 || status.Signal != 0 {
			t.Fatalf("detached root=%#v %v", status, err)
		}
		if host.phase != "reaped" || host.proof == nil {
			t.Fatal("descendant cleanup lacks durable proof")
		}
	})
	t.Run("cancel_close_waiter", func(t *testing.T) {
		host := &fixtureHost{root: root}
		command := Command{Version: 1, Command: "/bin/sh", Directory: root, Args: []string{"-c", "printf ready; /bin/sleep 20"}, Env: []string{"PATH=/usr/bin:/bin"}}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		process, err := Start(ctx, Config{HelperPath: binary, HelperSHA256: digest, Host: host}, command)
		if err != nil {
			t.Fatal(err)
		}
		defer process.Close(context.Background())
		streams := process.Streams()
		defer streams.Stdout.Close()
		defer streams.Stderr.Close()
		if err := streams.Stdout.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var ready [5]byte
		if _, err := io.ReadFull(streams.Stdout, ready[:]); err != nil || string(ready[:]) != "ready" {
			t.Fatalf("ready=%q %v", ready, err)
		}
		canceled, stop := context.WithCancel(context.Background())
		stop()
		if err := process.Close(canceled); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if err := process.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if host.phase != "reaped" || host.proof == nil {
			t.Fatal("canceled waiter interrupted shared cleanup")
		}
	})
	for _, stage := range []string{"success", "reserve", "publish", "register", "release", "finish"} {
		t.Run(stage, func(t *testing.T) {
			host := &fixtureHost{root: root, failure: stage}
			marker := filepath.Join(root, stage+"-task")
			command := Command{Version: 1, Command: "/bin/sh", Directory: root, Args: []string{"-c", `/bin/cat; printf 'error-stream' >&2; printf '%s' "$TASK_FIXTURE_TOKEN" > "$1"; exit 7`, "task", marker}, Env: []string{"PATH=/usr/bin:/bin", "TASK_FIXTURE_TOKEN=fixture"}}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			process, err := Start(ctx, Config{HelperPath: binary, HelperSHA256: digest, Host: host}, command)
			if process != nil {
				t.Cleanup(func() {
					_ = process.Close(context.Background())
					streams := process.Streams()
					if streams.Stdout != nil {
						streams.Stdout.Close()
					}
					if streams.Stderr != nil {
						streams.Stderr.Close()
					}
				})
			}
			if stage != "success" && stage != "finish" {
				if !errors.Is(err, fixtureFailure) {
					t.Fatalf("stage error: %v", err)
				}
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("task executed after admission failure")
				}
				count := 0
				for _, step := range host.steps {
					if step == "release" {
						count++
					}
				}
				if count > 1 {
					t.Fatal("release replayed")
				}
				if host.phase != "aborted" && host.phase != "reaped" {
					t.Fatalf("unclosed failure: %s", host.phase)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			streams := process.Streams()
			if _, err := streams.Stdin.WriteString("output-stream"); err != nil {
				t.Fatal(err)
			}
			streams.Stdin.Close()
			for _, file := range []*os.File{streams.Stdout, streams.Stderr} {
				if err := file.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			stdout, err := io.ReadAll(streams.Stdout)
			if err != nil || string(stdout) != "output-stream" {
				t.Fatalf("stdout=%q %v", stdout, err)
			}
			stderr, err := io.ReadAll(streams.Stderr)
			if err != nil || string(stderr) != "error-stream" {
				t.Fatalf("stderr=%q %v", stderr, err)
			}
			status, err := process.Wait(ctx)
			if status.Code != 7 || status.Signal != 0 {
				t.Fatalf("exit=%#v", status)
			}
			if stage == "finish" {
				if !errors.Is(err, fixtureFailure) {
					t.Fatalf("lost cleanup failure: %v", err)
				}
				if !errors.Is(process.Close(ctx), fixtureFailure) {
					t.Fatal("repeated Close hid failure")
				}
				if host.phase != "released" {
					t.Fatal("failed store changed durable phase")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if host.phase != "reaped" || host.proof == nil {
					t.Fatal("missing retirement proof")
				}
			}
			if !reflect.DeepEqual(host.steps, []string{"reserve", "publish", "register", "release", "finish"}) {
				t.Fatalf("steps=%v", host.steps)
			}
			value, err := os.ReadFile(marker)
			if err != nil || string(value) != "fixture" {
				t.Fatalf("task env=%q %v", value, err)
			}
		})
	}
}

// TestSupervisionDetachedWorker 是限时夹具，任务后代在自身 session 中忽略 TERM。
func TestSupervisionDetachedWorker(t *testing.T) {
	mode := os.Getenv("NEXUS_SUPERVISION_WORKER")
	if mode == "" {
		t.Skip("native fixture worker only")
	}
	marker := os.Getenv("NEXUS_SUPERVISION_MARKER")
	if mode == "child" {
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(marker, []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Second)
		return
	}
	if mode != "root" {
		t.Fatal("invalid fixture mode")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestSupervisionDetachedWorker$")
	child.Env = []string{"PATH=/usr/bin:/bin", "NEXUS_SUPERVISION_WORKER=child", "NEXUS_SUPERVISION_MARKER=" + marker}
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached fixture did not start")
}
