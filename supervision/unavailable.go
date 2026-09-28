//go:build !darwin || !cgo

// INPUT: 不支持原生监督的平台构建。
// OUTPUT: 不可用；不得以无监督启动代替。
// POS: 显式 macOS 启动器的平台拒绝入口。
package supervision

import (
	"context"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

func Start(context.Context, Config, Command) (*Process, error) {
	return nil, processscope.ErrUnavailable
}
func removeJob(context.Context, Intent) error { return processscope.ErrUnavailable }
