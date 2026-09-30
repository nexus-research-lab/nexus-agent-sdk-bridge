//go:build darwin && cgo

// INPUT: 当前 boot 与 macOS 原生集合/进程接口。
// OUTPUT: 有类型的观察错误；所有 Mach/C 资源由本次调用释放。
// POS: 实验监督组件的原生适配，不更改进程启动或默认清理。
package processscope

/*
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type darwinKernel struct{}

func newKernel() kernel                       { return darwinKernel{} }
func (darwinKernel) available() error         { return nativeError(C.nx_scope_available()) }
func (darwinKernel) bootID() (string, error)  { return unix.Sysctl("kern.bootsessionuuid") }
func (k darwinKernel) self() (process, error) { return k.inspect(os.Getpid()) }
func (darwinKernel) inspect(pid int) (process, error) {
	var raw C.struct_nx_scope_process
	if err := nativeError(C.nx_scope_inspect(C.int(pid), &raw)); err != nil {
		return process{}, err
	}
	return fromNative(raw), nil
}
func (darwinKernel) exists(id uint64) error { return nativeError(C.nx_scope_exists(C.uint64_t(id))) }
func (darwinKernel) members(id uint64) ([]process, error) {
	var raw *C.struct_nx_scope_process
	var count C.int
	if err := nativeError(C.nx_scope_members(C.uint64_t(id), &raw, &count)); err != nil {
		return nil, err
	}
	defer C.free(unsafe.Pointer(raw))
	items := make([]process, 0, int(count))
	for _, item := range unsafe.Slice(raw, int(count)) {
		items = append(items, fromNative(item))
	}
	return items, nil
}
func (darwinKernel) signal(item process, signal int) error {
	raw := C.struct_nx_scope_process{coalition: C.uint64_t(item.coalition)}
	for i, v := range item.audit {
		raw.audit[i] = C.uint32_t(v)
	}
	return nativeError(C.nx_scope_signal(&raw, C.int(signal)))
}
func fromNative(raw C.struct_nx_scope_process) process {
	item := process{coalition: uint64(raw.coalition)}
	for i, v := range raw.audit {
		item.audit[i] = uint32(v)
	}
	return item
}
func nativeError(code C.int) error {
	switch syscall.Errno(code) {
	case 0:
		return nil
	case syscall.ESRCH:
		return ErrGone
	case syscall.ENOSYS:
		return ErrUnavailable
	default:
		return fmt.Errorf("native process observation: %w", syscall.Errno(code))
	}
}

func (darwinKernel) peer(fd int) ([8]uint32, error) {
	var raw [8]C.uint32_t
	var result [8]uint32
	if err := nativeError(C.nx_scope_peer(C.int(fd), &raw[0])); err != nil {
		return result, err
	}
	for i, value := range raw {
		result[i] = uint32(value)
	}
	return result, nil
}
