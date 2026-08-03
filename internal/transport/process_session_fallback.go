//go:build !darwin && !linux

package transport

import "os/exec"

// processSession 在未提供 OS 级 session 枚举的平台保持空实现；原有的
// 直接子进程终止路径继续生效，不改变现有平台行为。
type processSession struct{}

func configureProcessSession(_ *exec.Cmd) {}

func startedProcessSession(_ *exec.Cmd) processSession {
	return processSession{}
}

func (processSession) id() int {
	return 0
}

func (processSession) cleanup() (int, error) {
	return 0, nil
}
