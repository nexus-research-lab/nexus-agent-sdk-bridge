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
