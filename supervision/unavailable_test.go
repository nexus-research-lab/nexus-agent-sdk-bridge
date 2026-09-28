//go:build !darwin || !cgo

// INPUT: 不支持原生监督的本机构建。
// OUTPUT: typed unavailable，不创建进程或调用持久阶段。
// POS: 平台拒绝合同，不运行跨平台二进制。
package supervision

import (
	"context"
	"errors"
	"testing"
)

func TestUnavailableStart(t *testing.T) {
	process, err := Start(context.Background(), Config{}, Command{})
	if process != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsupported start: %v %v", process, err)
	}
}

func TestUnavailableRecovery(t *testing.T) {
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	record := Recovery{Intent: Intent{Version: 1, ID: id, BootID: "12345678-1234-1234-1234-123456789abc", OwnerUID: 501, JobLabel: "cn.nexus.runtime." + id, HelperSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}
	called := false
	err := Recover(t.Context(), record, recoveryHostFunc(func(context.Context, Intent, *Evidence) error { called = true; return nil }))
	if !errors.Is(err, ErrUnavailable) || called {
		t.Fatalf("unsupported recovery: %v finish=%v", err, called)
	}
}
