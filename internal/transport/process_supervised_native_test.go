//go:build darwin && cgo

// INPUT: 显式测试 helper、真实 launchd 与独立 Host 文件夹具。
// OUTPUT: JSON 双向管道、退出码、脱离后代 EOF、强制关闭、预检与错误保留。
// POS: transport 原生集成；不替代 Nexus 默认装配、可信目录或发布验收。
package transport

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
)

type transportHostFixture struct {
	root         string
	intent       supervision.Intent
	registration supervision.Registration
	phase        string
	failFinish   bool
	mu           sync.Mutex
}

var transportFinishFailure = errors.New("fixture finish failed")

func (h *transportHostFixture) Reserve(_ context.Context, i supervision.Intent) (supervision.Paths, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.phase != "" {
		return supervision.Paths{}, errors.New("host reused")
	}
	h.intent = i
	h.phase = "prepared"
	dir := filepath.Join(h.root, i.ID)
	if err := os.Mkdir(dir, 0700); err != nil {
		return supervision.Paths{}, err
	}
	return supervision.Paths{JobFile: filepath.Join(dir, "job.plist"), Socket: filepath.Join(dir, "s")}, nil
}
func (h *transportHostFixture) Publish(_ context.Context, i supervision.Intent, data []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.intent != i || h.phase != "prepared" {
		return errors.New("invalid publish")
	}
	return os.WriteFile(filepath.Join(h.root, i.ID, "job.plist"), data, 0600)
}
func (h *transportHostFixture) Register(_ context.Context, i supervision.Intent, r supervision.Registration) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.intent != i || h.phase != "prepared" {
		return errors.New("invalid register")
	}
	h.registration = r
	h.phase = "registered"
	return nil
}
func (h *transportHostFixture) ClaimRelease(_ context.Context, i supervision.Intent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.intent != i || h.phase != "registered" {
		return errors.New("release replay")
	}
	h.phase = "released"
	return nil
}
func (h *transportHostFixture) Finish(_ context.Context, i supervision.Intent, proof *supervision.Evidence) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failFinish {
		return transportFinishFailure
	}
	if h.intent != i {
		return errors.New("wrong finish intent")
	}
	if h.phase != "prepared" && (proof == nil || proof.Registration != h.registration) {
		return errors.New("missing finish evidence")
	}
	h.phase = "finished"
	return os.RemoveAll(filepath.Join(h.root, i.ID))
}
func (h *transportHostFixture) state() string { h.mu.Lock(); defer h.mu.Unlock(); return h.phase }

func TestSupervisedTransportNative(t *testing.T) {
	helper := os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	if helper == "" {
		t.Skip("explicit native helper required")
	}
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	t.Setenv(skipVersionCheckEnv, "")
	factory := func(t *testing.T, fail bool) (supervision.Factory, *[]*transportHostFixture, *[]supervision.Purpose) {
		t.Helper()
		root, err := os.MkdirTemp("/tmp", "nst-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(root) })
		hosts := new([]*transportHostFixture)
		purposes := new([]supervision.Purpose)
		return func(_ context.Context, purpose supervision.Purpose) (supervision.Config, error) {
			h := &transportHostFixture{root: root, failFinish: fail}
			*hosts = append(*hosts, h)
			*purposes = append(*purposes, purpose)
			return supervision.Config{HelperPath: helper, HelperSHA256: digest, Host: h}, nil
		}, hosts, purposes
	}
	t.Run("stdio", func(t *testing.T) {
		f, hosts, _ := factory(t, false)
		m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", CWD: "/", ControlWireDialect: ControlWireDialectNXS, Supervision: f, Args: []string{"-c", `IFS= read -r line; printf '%s\n' "$line"; printf fixture-stderr >&2`}})
		if err := m.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		if err := m.WriteJSON(map[string]any{"type": "fixture", "value": "hello"}); err != nil {
			t.Fatal(err)
		}
		got, err := m.ReadJSON()
		if err != nil || got["value"] != "hello" {
			t.Fatalf("message=%v err=%v", got, err)
		}
		if err := m.Wait(); err != nil {
			t.Fatal(err)
		}
		if m.StderrTail() != "fixture-stderr" || (*hosts)[0].state() != "finished" {
			t.Fatal("streams or cleanup missing")
		}
		if err := m.Interrupt(); !errors.Is(err, ErrInterruptUnsupported) {
			t.Fatal(err)
		}
	})
	t.Run("exit_code", func(t *testing.T) {
		f, _, _ := factory(t, false)
		m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", ControlWireDialect: ControlWireDialectNXS, Supervision: f, Args: []string{"-c", "exit 7"}})
		if err := m.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		var exit interface{ ExitCode() int }
		if err := m.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
			t.Fatalf("exit=%v", err)
		}
		for range 2 {
			if err := m.Close(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
				t.Fatalf("close=%v", err)
			}
		}
	})
	t.Run("detached_output", func(t *testing.T) {
		f, hosts, _ := factory(t, false)
		// 独立 Go 测试进程在下面的 helper case setsid 后持有 stdout；根退出必须自动清理。
		m := NewProcessManager(ProcessConfig{CommandPath: os.Args[0], ControlWireDialect: ControlWireDialectNXS, Supervision: f, Args: []string{"-test.run=^TestSupervisedDetachedFixture$"}, Env: map[string]string{"NEXUS_SUPERVISED_DETACHED_FIXTURE": "parent"}})
		if err := m.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		if _, err := m.ReadJSON(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { _, err := m.ReadJSON(); done <- err }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("detached writer kept pipe open")
		}
		if err := m.Wait(); err != nil {
			t.Fatal(err)
		}
		if (*hosts)[0].state() != "finished" {
			t.Fatal("collection not finished")
		}
	})
	t.Run("forced_close", func(t *testing.T) {
		f, hosts, _ := factory(t, false)
		m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", ControlWireDialect: ControlWireDialectNXS, Supervision: f, Args: []string{"-c", `printf '{"ready":true}\n'; exec /bin/sleep 30`}})
		if err := m.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := m.ReadJSON(); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if (*hosts)[0].state() != "finished" {
			t.Fatal("forced close failed")
		}
	})
	t.Run("finish_failure", func(t *testing.T) {
		f, hosts, _ := factory(t, true)
		m := NewProcessManager(ProcessConfig{CommandPath: "/bin/sh", ControlWireDialect: ControlWireDialectNXS, Supervision: f, Args: []string{"-c", "exit 0"}})
		if err := m.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		var cleanup *ProcessCleanupError
		if err := m.Wait(); !errors.As(err, &cleanup) || !errors.Is(err, transportFinishFailure) {
			t.Fatalf("wait=%v", err)
		}
		for range 2 {
			if err := m.Close(); !errors.As(err, &cleanup) || !errors.Is(err, transportFinishFailure) {
				t.Fatalf("close=%v", err)
			}
		}
		if (*hosts)[0].state() != "released" {
			t.Fatal("lost unresolved receipt")
		}
	})
	t.Run("all_probes", func(t *testing.T) {
		f, hosts, purposes := factory(t, false)
		cli := filepath.Join(t.TempDir(), "cli")
		script := "#!/bin/sh\ncase \"$1\" in\n-v) echo 2.2.0; exit 0;;\n--restricted|--settings) echo '--restricted --settings'; exit 0;;\nesac\n"
		if err := os.WriteFile(cli, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		m := NewProcessManager(ProcessConfig{CommandPath: cli, Supervision: f, RequireClaudeRestricted: true, RequireClaudeNativeSandbox: true, Args: []string{"--settings", `{"sandbox":{"enabled":true,"failIfUnavailable":true,"allowUnsandboxedCommands":false}}`}})
		if err := m.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		if err := m.Wait(); err != nil {
			t.Fatal(err)
		}
		want := []supervision.Purpose{supervision.ClaudeSandboxProbe, supervision.ClaudeRestrictedProbe, supervision.VersionProbe, supervision.Runtime}
		if !reflect.DeepEqual(*purposes, want) {
			t.Fatalf("purposes=%v", *purposes)
		}
		for _, h := range *hosts {
			if h.state() != "finished" {
				t.Fatal("probe not retired")
			}
		}
	})
	t.Run("probe_cancel", func(t *testing.T) {
		f, hosts, _ := factory(t, false)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		cmd := exec.Command("/bin/sleep", "30")
		cmd.Dir = "/"
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		if err := runConfiguredProbe(ctx, cmd, ProcessConfig{Supervision: f}, supervision.VersionProbe); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		for _, h := range *hosts {
			if h.state() != "finished" {
				t.Fatal("canceled probe not retired")
			}
		}
	})
}

func TestSupervisedDetachedFixture(t *testing.T) {
	switch os.Getenv("NEXUS_SUPERVISED_DETACHED_FIXTURE") {
	case "parent":
		cmd := exec.Command(os.Args[0], "-test.run=^TestSupervisedDetachedFixture$")
		cmd.Env = append(os.Environ(), "NEXUS_SUPERVISED_DETACHED_FIXTURE=child")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			os.Exit(41)
		}
		fmt.Println(`{"ready":true}`)
		os.Exit(0)
	case "child":
		signal.Ignore(syscall.SIGTERM)
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
}
