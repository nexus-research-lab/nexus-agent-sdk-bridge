//go:build darwin && cgo

// INPUT: 内核 wait status，包括正常退出、信号和非终态。
// OUTPUT: 退出码与信号不混淆，非终态拒绝。
// POS: 根进程状态解码测试，原生绑定由 bootstrap 与 scope 用例覆盖。
package processscope

import (
	"syscall"
	"testing"
)

func TestDecodeRootExit(t *testing.T) {
	for _, test := range []struct {
		name         string
		data         int64
		code, signal int
		valid        bool
	}{
		{"success", 0, 0, 0, true}, {"failure", 7 << 8, 7, 0, true}, {"signal", int64(syscall.SIGKILL), 0, int(syscall.SIGKILL), true},
		{"negative", -1, 0, 0, false}, {"oversized", 1 << 16, 0, 0, false}, {"stopped", int64(syscall.SIGSTOP)<<8 | 0x7f, 0, 0, false}, {"continued", 0xffff, 0, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := decodeRootExit(202, test.data)
			if (err == nil) != test.valid {
				t.Fatalf("status=%#v %v", result, err)
			}
			if test.valid && (result.PID != 202 || result.Code != test.code || result.Signal != test.signal) {
				t.Fatalf("status=%#v", result)
			}
		})
	}
}
