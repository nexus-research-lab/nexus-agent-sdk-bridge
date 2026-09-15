// INPUT: 宿主资源选项与普通沙箱设置。
// OUTPUT: 公开资源类型和启动前的矛盾配置拒绝。
// POS: Bridge 资源准入；平台及文件访问事实归 runtime。
package client

import (
	"errors"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// SandboxResourcePolicy 与 initialize 线格式共用唯一类型定义。
type SandboxResourcePolicy = protocol.SandboxResourcePolicy
type SandboxWriteScope = protocol.SandboxWriteScope

const (
	SandboxWriteScopeReadOnly       = protocol.SandboxWriteScopeReadOnly
	SandboxWriteScopeWorkspaceWrite = protocol.SandboxWriteScopeWorkspaceWrite
)

// validateSandboxResources 不让错误合同通过旧字段或普通 settings 降级。
func validateSandboxResources(settings *SandboxSettings) error {
	if settings.Resources == nil {
		return nil
	}
	if !settings.RequireSandbox || !settings.RequireFileTools {
		return errors.New("sandbox resources require command and file sandbox requirements")
	}
	if err := settings.Resources.Validate(); err != nil {
		return err
	}
	if settings.AllowUnsandboxedCommands != nil && *settings.AllowUnsandboxedCommands {
		return errors.New("sandbox resources do not allow unsandboxed commands")
	}
	if settings.Resources.WriteScope == SandboxWriteScopeReadOnly && settings.Filesystem != nil && len(settings.Filesystem.AllowWrite) != 0 {
		return errors.New("read-only sandbox resources cannot include write grants")
	}
	return nil
}
