//go:build darwin && cgo

// INPUT: 真正 Unix socket 与标准管道，注入截断、超限、错误 fd 和关闭。
// OUTPUT: 正常传递保持三条管道；拒绝路径不保留写端或执行输入。
// POS: SCM_RIGHTS 资源生命周期测试，不运行用户命令。
package processbootstrap

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func connectionPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "nxs-wire-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(root, "s"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	sender, err := net.DialUnix("unix", nil, listener.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := listener.AcceptUnix()
	if err != nil {
		sender.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { sender.Close(); receiver.Close() })
	for _, conn := range []*net.UnixConn{sender, receiver} {
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	return sender, receiver
}
func pipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return r, w
}

func TestLaunchWirePreservesPipes(t *testing.T) {
	sender, receiver := connectionPair(t)
	inputR, inputW := pipe(t)
	outputR, outputW := pipe(t)
	errorR, errorW := pipe(t)
	want := Launch{Version: 1, Command: "/bin/sh", Directory: "/tmp", Env: []string{"TOKEN=not-a-real-secret"}}
	if err := SendLaunch(sender, want, [3]*os.File{inputR, outputW, errorW}); err != nil {
		t.Fatal(err)
	}
	got, files, err := receiveLaunch(receiver)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()
	if got.Command != want.Command || got.Env[0] != want.Env[0] {
		t.Fatal("launch input changed")
	}
	if _, err := inputW.WriteString("in"); err != nil {
		t.Fatal(err)
	}
	inputW.Close()
	data, err := io.ReadAll(files[0])
	if err != nil || string(data) != "in" {
		t.Fatalf("input: %q %v", data, err)
	}
	for i, pair := range [][2]*os.File{{files[1], outputR}, {files[2], errorR}} {
		if _, err := pair[0].WriteString("x"); err != nil {
			t.Fatal(err)
		}
		var b [1]byte
		if _, err := io.ReadFull(pair[1], b[:]); err != nil || b[0] != 'x' {
			t.Fatalf("output %d: %v", i, err)
		}
	}
}

func TestLaunchWireRejectsAndClosesTransferredDescriptors(t *testing.T) {
	for _, mode := range []string{"extra_fd", "many_rights", "wrong_direction", "wrong_type", "bad_marker", "missing_body", "oversized", "invalid_json", "unknown_field", "truncated_body"} {
		t.Run(mode, func(t *testing.T) {
			sender, receiver := connectionPair(t)
			inputR, inputW := pipe(t)
			outputR, outputW := pipe(t)
			errorR, errorW := pipe(t)
			fds := []int{int(inputR.Fd()), int(outputW.Fd()), int(errorW.Fd())}
			marker := byte(1)
			switch mode {
			case "extra_fd":
				fds = append(fds, int(errorW.Fd()))
			case "many_rights":
				for len(fds) < 200 {
					fds = append(fds, int(errorW.Fd()))
				}
			case "wrong_direction":
				fds[0] = int(inputW.Fd())
			case "wrong_type":
				file, err := os.Open(os.DevNull)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				fds[0] = int(file.Fd())
			case "bad_marker":
				marker = 2
			}
			if _, _, err := sender.WriteMsgUnix([]byte{marker}, unix.UnixRights(fds...), nil); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(Launch{Version: 1, Command: "/bin/sh", Directory: "/tmp"})
			switch mode {
			case "invalid_json":
				data = []byte("{")
			case "unknown_field":
				data = []byte(`{"version":1,"command":"/bin/sh","directory":"/tmp","grant":true}`)
			}
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(len(data)))
			if mode == "oversized" {
				binary.BigEndian.PutUint32(size[:], maxLaunchBytes+1)
			}
			if mode != "missing_body" {
				if _, err := sender.Write(size[:]); err != nil {
					t.Fatal(err)
				}
				if mode == "truncated_body" {
					data = data[:len(data)-1]
				}
				if _, err := sender.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			sender.Close()
			_, files, err := receiveLaunch(receiver)
			if err == nil {
				for _, f := range files {
					f.Close()
				}
				t.Fatal("invalid frame accepted")
			}
			for _, f := range files {
				if f != nil {
					t.Fatal("failure exposed received descriptor")
				}
			}
			// 接收端若漏掉任何 stdout 写端，此处不会得到 EOF。
			outputW.Close()
			errorW.Close()
			for _, reader := range []*os.File{outputR, errorR} {
				if err := reader.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				data, err = io.ReadAll(reader)
				if err != nil || len(data) != 0 {
					t.Fatalf("descriptor retained: %v", err)
				}
			}
		})
	}
}
