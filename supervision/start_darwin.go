//go:build darwin && cgo

// INPUT: 可信 helper 固定摘要、宿主持久回调与显式命令。
// OUTPUT: intent -> job -> kernel identity -> registration -> once-only release -> task。
// POS: 显式监督启动路径；未成为默认 transport 或产品自动恢复。
package supervision

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processbootstrap"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

// Start 失败时也可能返回非 nil Process；它保留共享清理状态，不得据此重发任务。
func Start(ctx context.Context, config Config, command Command) (process *Process, err error) {
	if ctx == nil || config.Host == nil {
		return nil, errors.New("supervision requires context and durable host")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	helper, err := verifyHelper(config.HelperPath, config.HelperSHA256)
	if err != nil {
		return nil, err
	}
	observer, err := processscope.ObserveSelf()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	textID := hex.EncodeToString(id[:])
	intent := Intent{Version: 1, ID: textID, BootID: observer.BootID, OwnerUID: observer.UID, JobLabel: "cn.nexus.runtime." + textID, HelperSHA256: config.HelperSHA256}
	p := &Process{intent: intent, host: config.Host, closed: make(chan struct{})}
	defer func() {
		if err != nil {
			p.startErr = err
			err = errors.Join(err, p.Close(context.Background()))
		}
	}()
	process = p
	// Reserve 响应丢失也视为可能已经落盘，失败收尾必须向同一 Host 核对。
	p.reserved = true
	p.paths, err = p.host.Reserve(ctx, intent)
	if err != nil {
		return p, err
	}
	if !filepath.IsAbs(p.paths.JobFile) || !filepath.IsAbs(p.paths.Socket) || p.paths.JobFile == p.paths.Socket || strings.ContainsRune(p.paths.JobFile+p.paths.Socket, 0) {
		return p, errors.New("invalid private bootstrap paths")
	}
	listener, err := processbootstrap.ListenControlSocket(p.paths.Socket)
	if err != nil {
		return p, err
	}
	// 文件删除由 Host 使用自己的固定目录句柄完成，避免 net 默认按路径清除。
	listener.SetUnlinkOnClose(false)
	defer listener.Close()
	stopAccept := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopAccept()
	deadline, _ := ctx.Deadline()
	if err := listener.SetDeadline(deadline); err != nil {
		return p, err
	}
	if err := p.host.Publish(ctx, intent, jobDefinition(intent, helper, p.paths.Socket, observer.Identity)); err != nil {
		return p, err
	}
	p.submitted = true
	if _, err := launchctl(ctx, "bootstrap", domain(intent), p.paths.JobFile); err != nil {
		return p, fmt.Errorf("start bootstrap job: %w", err)
	}
	conn, err := listener.AcceptUnix()
	if err != nil {
		return p, err
	}
	defer conn.Close()
	stopConnection := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopConnection()
	if err := conn.SetDeadline(deadline); err != nil {
		return p, err
	}
	var ready [1]byte
	if _, err := io.ReadFull(conn, ready[:]); err != nil {
		return p, err
	}
	if ready[0] != 1 {
		return p, errors.New("invalid bootstrap readiness")
	}
	pid, err := jobPID(ctx, intent)
	if err != nil {
		return p, err
	}
	p.scope, err = processscope.CapturePeer(conn, pid)
	if err != nil {
		return p, err
	}
	p.monitor, err = p.scope.WatchRoot(conn, pid)
	if err != nil {
		return p, err
	}
	if _, err := verifyHelper(helper, config.HelperSHA256); err != nil {
		return p, err
	}
	if err := p.host.Register(ctx, intent, p.scope.Registration()); err != nil {
		return p, err
	}
	inputR, inputW, err := os.Pipe()
	if err != nil {
		return p, err
	}
	p.childFiles[0] = inputR
	p.streams.Stdin = inputW
	outputR, outputW, err := os.Pipe()
	if err != nil {
		return p, err
	}
	p.childFiles[1] = outputW
	p.streams.Stdout = outputR
	errorR, errorW, err := os.Pipe()
	if err != nil {
		return p, err
	}
	p.childFiles[2] = errorW
	p.streams.Stderr = errorR
	if err := ctx.Err(); err != nil {
		return p, err
	}
	if err := p.host.ClaimRelease(ctx, intent); err != nil {
		return p, err
	}
	// 不重试发送：任何部分写入/断连都保留 released 的未知结果，收尾后才能恢复。
	if err := processbootstrap.SendLaunch(conn, command, p.childFiles); err != nil {
		return p, err
	}
	for _, file := range p.childFiles {
		if err := file.Close(); err != nil {
			return p, err
		}
	}
	if err := ctx.Err(); err != nil {
		return p, err
	}
	// 与消费者读管道并行观察；主进程先退出且后代仍持有输出管道时也会收口。
	go func() { _, _ = p.monitor.Wait(context.Background()); _ = p.Close(context.Background()) }()
	return p, nil
}

func verifyHelper(path, expected string) (string, error) {
	if !filepath.IsAbs(path) || len(expected) != 64 || strings.ToLower(expected) != expected {
		return "", errors.New("trusted bootstrap path and digest are required")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return "", errors.New("invalid bootstrap digest")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode().Perm()&0022 != 0 || info.Size() > 256<<20 {
		return "", errors.New("invalid trusted bootstrap executable")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, io.LimitReader(file, 256<<20+1)); err != nil {
		return "", err
	}
	if hex.EncodeToString(digest.Sum(nil)) != expected {
		return "", errors.New("bootstrap executable digest mismatch")
	}
	return resolved, nil
}
