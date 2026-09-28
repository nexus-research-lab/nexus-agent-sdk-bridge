//go:build darwin && cgo

// INPUT: 宿主固定的绝对 socket 路径；父目录必须受任务文件策略保护。
// OUTPUT: 在原目录中绑定/连接的 Unix socket，不受完整路径 103 字节限制。
// POS: 专用原生线程只改变自身 cwd；不改进程 cwd，不创建短路径别名或旁路目录。
package processbootstrap

/*
#include <errno.h>
#include <fcntl.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

// macOS 导出的线程 cwd 接口。弱链接缺失时失败，不退回进程级 chdir。
extern int pthread_fchdir_np(int) __attribute__((weak_import));
struct nx_socket_call { int directory; int socket; int connecting; const char *name; int error; };
static void *nx_socket_worker(void *argument) {
 struct nx_socket_call *call = argument;
 if (!pthread_fchdir_np) { call->error = ENOTSUP; return NULL; }
 if (pthread_fchdir_np(call->directory) != 0) { call->error = errno ? errno : EIO; return NULL; }
 struct sockaddr_un address;
 memset(&address, 0, sizeof(address));
 address.sun_family = AF_UNIX;
 memcpy(address.sun_path, call->name, strlen(call->name) + 1);
 address.sun_len = sizeof(address);
 int result = call->connecting ? connect(call->socket, (struct sockaddr *)&address, sizeof(address)) : bind(call->socket, (struct sockaddr *)&address, sizeof(address));
 call->error = result == 0 ? 0 : errno;
 // 线程退出销毁 thread-local cwd；从不把该线程放回 Go 调度池。
 return NULL;
}
static int nx_socket_at(int directory, int socket, const char *name, int connecting) {
 struct nx_socket_call call = {directory, socket, connecting, name, 0};
 pthread_t worker;
 int error = pthread_create(&worker, NULL, nx_socket_worker, &call);
 if (error) return error;
 error = pthread_join(worker, NULL);
 return error ? error : call.error;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// socketAt 只在专用原生线程内使用相对名称；完整父目录由打开的句柄固定。
// 路径来自可信 Host，父目录的任务隔离仍由宿主负责。
func socketAt(path string, connecting bool) (*os.File, error) {
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) || filepath.Clean(path) != path || len(filepath.Base(path)) > 103 || filepath.Base(path) == "." || path == "/" {
		return nil, errors.New("invalid bootstrap socket path")
	}
	directoryFD, err := unix.Open(filepath.Dir(path), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	directory := os.NewFile(uintptr(directoryFD), "bootstrap-socket-directory")
	defer directory.Close()
	info, err := directory.Stat()
	if err != nil || !info.IsDir() {
		return nil, errors.New("bootstrap socket parent is not a directory")
	}
	syscall.ForkLock.RLock()
	fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err == nil {
		unix.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "bootstrap-control")
	success := false
	defer func() {
		if !success {
			file.Close()
		}
	}()
	if err := unix.SetNonblock(fd, true); err != nil {
		return nil, err
	}
	name := C.CString(filepath.Base(path))
	defer C.free(unsafe.Pointer(name))
	mode := C.int(0)
	if connecting {
		mode = 1
	}
	code := syscall.Errno(C.nx_socket_at(C.int(directory.Fd()), C.int(fd), name, mode))
	if code != 0 {
		if !connecting || code != unix.EINPROGRESS {
			return nil, fmt.Errorf("bootstrap socket operation: %w", code)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return nil, os.ErrDeadlineExceeded
			}
			events := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
			n, err := unix.Poll(events, int(remaining.Milliseconds())+1)
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, os.ErrDeadlineExceeded
			}
			pending, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
			if err != nil {
				return nil, err
			}
			if pending != 0 {
				return nil, syscall.Errno(pending)
			}
			break
		}
	}
	if !connecting {
		if err := unix.Listen(fd, 128); err != nil {
			return nil, err
		}
	}
	success = true
	return file, nil
}

// ListenControlSocket 不自动删除 socket；清理由 Host 的原目录句柄负责。
func ListenControlSocket(path string) (*net.UnixListener, error) {
	file, err := socketAt(path, false)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	listener, err := net.FileListener(file)
	if err != nil {
		return nil, err
	}
	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		listener.Close()
		return nil, errors.New("invalid bootstrap listener")
	}
	unixListener.SetUnlinkOnClose(false)
	return unixListener, nil
}

func dialControlSocket(path string) (*net.UnixConn, error) {
	file, err := socketAt(path, true)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	connection, err := net.FileConn(file)
	if err != nil {
		return nil, err
	}
	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		connection.Close()
		return nil, errors.New("invalid bootstrap connection")
	}
	return unixConnection, nil
}
