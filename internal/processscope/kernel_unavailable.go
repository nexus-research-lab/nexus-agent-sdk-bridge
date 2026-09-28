//go:build !darwin || !cgo

// INPUT: 不支持原生观察接口的平台或无 cgo 的构建。
// OUTPUT: 明确不可用，不退回 PID/session 扫描。
// POS: 未接入组件的平台拒绝边界。
package processscope

type unavailableKernel struct{}

func newKernel() kernel                                     { return unavailableKernel{} }
func (unavailableKernel) available() error                  { return ErrUnavailable }
func (unavailableKernel) bootID() (string, error)           { return "", ErrUnavailable }
func (unavailableKernel) self() (process, error)            { return process{}, ErrUnavailable }
func (unavailableKernel) inspect(int) (process, error)      { return process{}, ErrUnavailable }
func (unavailableKernel) exists(uint64) error               { return ErrUnavailable }
func (unavailableKernel) members(uint64) ([]process, error) { return nil, ErrUnavailable }
func (unavailableKernel) signal(process, int) error         { return ErrUnavailable }
