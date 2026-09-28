//go:build darwin && cgo

// INPUT: 可信 launcher 指定的控制 socket 和宿主 audit identity。
// OUTPUT: 认证后等待单次启动输入，最后原地 exec 到业务 runtime。
// POS: 引导阶段不运行任务、不读取用户设置、不创建任务后代。
package processbootstrap

import (
	"errors"
	"net"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

// Run 由独立 helper 入口调用，成功时 exec 不返回；失败不得回退普通启动。
func Run(socketPath, observerIdentity string) error {
	if !filepath.IsAbs(socketPath) {
		return errors.New("bootstrap requires an absolute control socket")
	}
	connection, err := net.DialTimeout("unix", socketPath, 10*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	conn, ok := connection.(*net.UnixConn)
	if !ok {
		return errors.New("invalid bootstrap control transport")
	}
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	if err := processscope.VerifyObserver(conn, observerIdentity); err != nil {
		return err
	}
	// 仅确认宿主身份；宿主仍须核验本 helper 并持久登记后才能发送启动帧。
	if _, err := conn.Write([]byte{1}); err != nil {
		return err
	}
	launch, files, err := receiveLaunch(conn)
	if err != nil {
		return err
	}
	defer func() {
		for _, file := range files {
			_ = file.Close()
		}
	}()
	if err := syscall.Chdir(launch.Directory); err != nil {
		return err
	}
	for target, file := range files {
		if err := syscall.Dup2(int(file.Fd()), target); err != nil {
			return err
		}
	}
	// 控制 socket 和收到的原始管道均为 CLOEXEC，只保留 0/1/2；
	// 使用显式环境，不把 launchd/helper 环境或控制身份借给业务进程。
	return syscall.Exec(launch.Command, append([]string{launch.Command}, launch.Args...), launch.Env)
}
