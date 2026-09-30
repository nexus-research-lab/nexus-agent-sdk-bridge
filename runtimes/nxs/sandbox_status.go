// INPUT: 明确配置的 nxs 可执行文件及调用方取消上下文。
// OUTPUT: 版本化后端诊断，或不改变运行时可用性的查询错误。
// POS: 独立本地诊断进程；不下载、不回退、不启动会话，不代替隔离验收。
package nxs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"time"
)

// SandboxBackendStatus 报告原生后端实现及默认依赖；不表示当前策略已经生效。
type SandboxBackendStatus struct {
	Version               int    `json:"version"`
	Platform              string `json:"platform"`
	BackendSupported      bool   `json:"backend_supported"`
	DependenciesAvailable bool   `json:"dependencies_available"`
	UnavailableReason     string `json:"unavailable_reason,omitempty"`
}

// SandboxStatus 只运行配置路径的 --sandbox-status，最多等待五秒、接收 16 KiB。
// 旧 runtime、无效响应及执行失败返回错误，调用方必须展示未知而非推断可用。
func (i *RuntimeInspector) SandboxStatus(ctx context.Context) (*SandboxBackendStatus, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	status, err := i.Ensure()
	if err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	path, err := filepath.Abs(status.Path)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(bounded, path, "--sandbox-status")
	command.WaitDelay = 250 * time.Millisecond
	output := &sandboxStatusOutput{}
	command.Stdout = output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if bounded.Err() != nil {
			return nil, bounded.Err()
		}
		return nil, errors.New("nxs sandbox diagnosis unavailable")
	}
	if output.exceeded {
		return nil, errors.New("nxs sandbox diagnosis exceeds output limit")
	}
	return decodeSandboxStatus(output.buffer.Bytes())
}

// sandboxStatusOutput 限制捕获内存；超限后交由命令截止与 WaitDelay 收口。
type sandboxStatusOutput struct {
	buffer   bytes.Buffer
	exceeded bool
}

func (b *sandboxStatusOutput) Write(data []byte) (int, error) {
	if len(data) > 16384-b.buffer.Len() {
		b.exceeded = true
		return 0, errors.New("sandbox diagnostic output too large")
	}
	return b.buffer.Write(data)
}

// decodeSandboxStatus 拒绝缺字段、未知版本及自相矛盾的可用性声明。
func decodeSandboxStatus(data []byte) (*SandboxBackendStatus, error) {
	var wire struct {
		Version      int    `json:"version"`
		Platform     string `json:"platform"`
		Supported    *bool  `json:"backend_supported"`
		Dependencies *bool  `json:"dependencies_available"`
		Reason       string `json:"unavailable_reason"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || wire.Version != 1 || wire.Platform == "" || wire.Supported == nil || wire.Dependencies == nil {
		return nil, errors.New("invalid nxs sandbox diagnosis")
	}
	if (*wire.Dependencies && (!*wire.Supported || wire.Reason != "")) || (!*wire.Dependencies && wire.Reason == "") {
		return nil, errors.New("inconsistent nxs sandbox diagnosis")
	}
	return &SandboxBackendStatus{Version: wire.Version, Platform: wire.Platform, BackendSupported: *wire.Supported, DependenciesAvailable: *wire.Dependencies, UnavailableReason: wire.Reason}, nil
}
