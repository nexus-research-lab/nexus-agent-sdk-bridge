//go:build darwin && cgo

// INPUT: 超过 sockaddr_un 限制的目录、并发普通文件读取和原生 IPC。
// OUTPUT: 原目录 socket 通讯、拒绝覆盖/目录链接，进程 cwd 不受影响。
// POS: 本机 socket 原语验证，不声明其他 macOS 版本支持。
package processbootstrap

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestControlSocketLongPathPreservesWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), strings.Repeat("long-directory-", 12))
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "control.socket")
	if len(path) <= 103 {
		t.Fatal("fixture is not long")
	}
	listener, err := ListenControlSocket(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if _, err := ListenControlSocket(path); err == nil {
		t.Fatal("overwrote an existing socket")
	}
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for range 200 {
			current, err := os.Getwd()
			if err != nil || current != cwd {
				t.Errorf("process cwd changed: %s %v", current, err)
				return
			}
			if _, err := os.ReadFile("launch.go"); err != nil {
				t.Errorf("ambient relative read changed: %v", err)
				return
			}
		}
	}()
	defer group.Wait()
	conn, err := dialControlSocket(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	peer, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 4)
	if _, err := io.ReadFull(peer, data); err != nil || string(data) != "ping" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	listener.Close()
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("listener removed host-owned path: %v", err)
	}
	if current, err := os.Getwd(); err != nil || current != cwd {
		t.Fatalf("cwd=%s err=%v", current, err)
	}
}

func TestControlSocketRejectsInvalidPaths(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative", "/", filepath.Join(root, strings.Repeat("x", 104)), filepath.Join(alias, "s"), root + "/../s", root + "/s\x00"} {
		if listener, err := ListenControlSocket(path); err == nil {
			listener.Close()
			t.Errorf("accepted %q", path)
		}
	}
	if conn, err := dialControlSocket(filepath.Join(target, "absent")); err == nil {
		conn.Close()
		t.Fatal("connected to missing endpoint")
	}
}
