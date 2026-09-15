// INPUT: 宿主指定的版本化写入范围与私有 scratch 目录。
// OUTPUT: 独立于普通 settings 的 initialize 资源对象。
// POS: Bridge 资源线格式；runtime 负责路径、平台与实际执行校验。
package protocol

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SandboxWriteScope 表达工作区默认写入范围，独立于批准方式。
type SandboxWriteScope string

const (
	SandboxWriteScopeReadOnly       SandboxWriteScope = "read-only"
	SandboxWriteScopeWorkspaceWrite SandboxWriteScope = "workspace-write"
)

// SandboxResourcePolicy 由宿主准备 scratch；Bridge 不创建、清理或承诺全进程隔离。
type SandboxResourcePolicy struct {
	Version     int               `json:"version"`
	WriteScope  SandboxWriteScope `json:"write_scope"`
	ScratchRoot string            `json:"scratch_root"`
}

// Validate 只校验公开语法；目标目录与平台事实由 nxs 验证。
func (p SandboxResourcePolicy) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("unsupported sandbox resource policy version %d", p.Version)
	}
	if p.WriteScope != SandboxWriteScopeReadOnly && p.WriteScope != SandboxWriteScopeWorkspaceWrite {
		return fmt.Errorf("unsupported sandbox write scope %q", p.WriteScope)
	}
	if !filepath.IsAbs(p.ScratchRoot) || filepath.Clean(p.ScratchRoot) != p.ScratchRoot || strings.ContainsAny(p.ScratchRoot, "\x00*?[]") || filepath.Dir(p.ScratchRoot) == p.ScratchRoot {
		return fmt.Errorf("sandbox scratch root must be an explicit absolute directory")
	}
	return nil
}
