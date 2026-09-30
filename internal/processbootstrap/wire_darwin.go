//go:build darwin && cgo

// INPUT: 已认证的 Unix stream socket，单次启动数据与三个标准管道。
// OUTPUT: 完整校验的 exec 输入；额外 fd 与异常管道拒绝，未知截断要求 helper 退出。
// POS: 不在 argv/plist/普通文件传递任务或凭据的引导协议。
package processbootstrap

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// SendLaunch 不持久化输入；调用方必须先持久确认执行登记。
// 三个文件分别为子进程使用的 stdin/read、stdout/write、stderr/write 端。
// 调用方持有文件且不得并发关闭，并须按启动 context 设置连接写入 deadline。
func SendLaunch(conn *net.UnixConn, launch Launch, files [3]*os.File) error {
	if conn == nil {
		return errors.New("bootstrap control connection is required")
	}
	if err := launch.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(launch)
	if err != nil {
		return err
	}
	if len(data) > maxLaunchBytes {
		return errors.New("bootstrap launch is too large")
	}
	fds := make([]int, 3)
	for i, f := range files {
		if f == nil {
			return errors.New("missing standard pipe")
		}
		fds[i] = int(f.Fd())
	}
	rights := unix.UnixRights(fds...)
	n, oobn, err := conn.WriteMsgUnix([]byte{1}, rights, nil)
	runtime.KeepAlive(files)
	if err != nil {
		return err
	}
	if n != 1 || oobn != len(rights) {
		return io.ErrShortWrite
	}
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	if _, err = io.Copy(conn, io.MultiReader(bytes.NewReader(size[:]), bytes.NewReader(data))); err != nil {
		return err
	}
	return nil
}

func receiveLaunch(conn *net.UnixConn) (launch Launch, files [3]*os.File, err error) {
	var marker [1]byte
	// XNU UIPC_MAX_CMSG_FD 为 512，控制 mbuf 也受 MCLBYTES 约束。
	// 必须容纳内核可能交付的全部 fd；截断后的 cmsg_len 仍可能保留原长度，
	// 通用解析器会丢掉整个消息，且未返回的 fd 无法安全枚举关闭。
	oob := make([]byte, unix.CmsgSpace(512*4))
	n, oobn, flags, _, readErr := conn.ReadMsgUnix(marker[:], oob)
	var fds []int
	defer func() {
		if err != nil {
			for _, fd := range fds {
				_ = unix.Close(fd)
			}
		}
	}()
	messages, parseErr := unix.ParseSocketControlMessage(oob[:oobn])
	for _, message := range messages {
		rights, e := unix.ParseUnixRights(&message)
		if e != nil {
			parseErr = errors.New("invalid bootstrap descriptor message")
			continue
		}
		fds = append(fds, rights...)
	}
	// 先收集内核交付的完整 rights 再检查帧；一旦控制信息仍被截断，
	// helper 必须失败退出，不得复用进程，进程退出关闭无法返回编号的 fd。
	if readErr != nil {
		err = readErr
		return
	}
	if parseErr != nil || flags&(unix.MSG_CTRUNC|unix.MSG_TRUNC) != 0 || n != 1 || marker[0] != 1 || len(fds) != 3 {
		err = errors.New("invalid bootstrap descriptor frame")
		return
	}
	for i, fd := range fds {
		if fd <= 2 {
			err = errors.New("bootstrap standard descriptor collision")
			return
		}
		unix.CloseOnExec(fd)
		var stat unix.Stat_t
		if e := unix.Fstat(fd, &stat); e != nil {
			err = e
			return
		}
		access, e := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
		want := unix.O_WRONLY
		if i == 0 {
			want = unix.O_RDONLY
		}
		if e != nil || stat.Mode&unix.S_IFMT != unix.S_IFIFO || access&unix.O_ACCMODE != want {
			err = errors.New("bootstrap requires directional standard pipes")
			return
		}
	}
	var size [4]byte
	if _, err = io.ReadFull(conn, size[:]); err != nil {
		return
	}
	length := binary.BigEndian.Uint32(size[:])
	if length == 0 || length > maxLaunchBytes {
		err = errors.New("invalid bootstrap launch size")
		return
	}
	data := make([]byte, int(length))
	if _, err = io.ReadFull(conn, data); err != nil {
		return
	}
	launch, err = decodeLaunch(data)
	if err != nil {
		return
	}
	for i, fd := range fds {
		files[i] = os.NewFile(uintptr(fd), "bootstrap-stdio")
	}
	return
}
