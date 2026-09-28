//go:build !darwin || !cgo

// INPUT: 不具备原生引导能力的构建。
// OUTPUT: 不可用错误，不回退到无监督执行。
// POS: helper 的平台拒绝入口。
package processbootstrap

import "github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"

func Run(string, string) error { return processscope.ErrUnavailable }
