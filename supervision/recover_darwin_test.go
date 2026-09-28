//go:build darwin && cgo

// INPUT: 真正退出且未 Close 的宿主子进程、原持久登记和存活的 launchd 任务。
// OUTPUT: 新宿主只按原登记回收集合，并持久确认；不重放原命令。
// POS: 原生断宿主恢复证据；文件仓储是夹具，不替代产品数据库/锁。
package supervision

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoveryNativeHostExit(t *testing.T) {
	helper := os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	if helper == "" {
		t.Skip("explicit native helper required")
	}
	root, err := os.MkdirTemp("/tmp", "nxs-recover-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRecoveryHostFixture$")
	child.Env = append(os.Environ(), "NEXUS_RECOVERY_HOST_FIXTURE="+root)
	child.WaitDelay = time.Second
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("fixture host: %v %s", err, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("records=%v %v", entries, err)
	}
	directory := filepath.Join(root, entries[0].Name())
	data, err := os.ReadFile(filepath.Join(directory, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Intent       Intent
		Phase        string
		Registration Registration
		Evidence     *Evidence
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Phase != "released" || saved.Evidence != nil {
		t.Fatalf("host did not leave unresolved release: %+v", saved)
	}
	host := &fixtureHost{root: root, intent: saved.Intent, registration: saved.Registration, phase: saved.Phase, paths: Paths{JobFile: filepath.Join(directory, "job.plist"), Socket: filepath.Join(directory, "s")}}
	record := Recovery{Intent: saved.Intent, Registration: &saved.Registration}
	defer Recover(context.Background(), record, host)
	if _, err := jobPID(ctx, saved.Intent); err != nil {
		t.Fatalf("runtime not alive after host exit: %v", err)
	}
	if err := Recover(ctx, record, host); err != nil {
		t.Fatal(err)
	}
	if host.phase != "reaped" || host.proof == nil || host.proof.Registration != saved.Registration {
		t.Fatalf("missing original retirement: %+v", host)
	}
	if err := Recover(ctx, record, host); err != nil {
		t.Fatalf("recovery retry: %v", err)
	}
}

func TestRecoveryHostFixture(t *testing.T) {
	root := os.Getenv("NEXUS_RECOVERY_HOST_FIXTURE")
	if root == "" {
		return
	}
	helper := os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	host := &fixtureHost{root: root}
	p, err := Start(t.Context(), Config{HelperPath: helper, HelperSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Host: host}, Command{Version: 1, Command: "/bin/sh", Args: []string{"-c", "printf ready; exec /bin/sleep 60"}, Env: []string{"PATH=/usr/bin:/bin"}, Directory: root})
	if err != nil {
		t.Fatal(err)
	}
	streams := p.Streams()
	if err := streams.Stdout.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var ready [5]byte
	if _, err := io.ReadFull(streams.Stdout, ready[:]); err != nil || string(ready[:]) != "ready" {
		t.Fatal("task not ready", err)
	}
	// 刻意退出宿主而不执行 defer、Close 或 Finish；父测试只读取持久登记来恢复。
	os.Exit(0)
}
