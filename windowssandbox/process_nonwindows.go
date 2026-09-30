//go:build !windows

// INPUT: 非Windows机器沙箱请求。
// OUTPUT: 明确不可用，不退回普通exec。
// POS: 保留跨平台公开类型和失败关闭合同。
package windowssandbox

import (
	"context"
	"errors"
)

type Process struct{}

func Start(context.Context, Config, Command) (*Process, error) {
	return nil, errors.New("Windows machine sandbox is unavailable on this platform")
}
func (*Process) Intent() Intent   { return Intent{} }
func (*Process) Streams() Streams { return Streams{} }
func (*Process) Wait(context.Context) (Outcome, error) {
	return Outcome{}, errors.New("Windows machine sandbox unavailable")
}
func (*Process) Close(context.Context) error {
	return errors.New("Windows machine sandbox unavailable")
}
