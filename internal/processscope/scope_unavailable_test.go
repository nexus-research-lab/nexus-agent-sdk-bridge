//go:build !darwin || !cgo

// INPUT: 缺少原生监督能力的当前构建。
// OUTPUT: 新登记与旧登记恢复均拒绝，不退回裸 PID 或进程组清理。
// POS: 不可用平台的入口合同测试。
package processscope

import (
	"errors"
	"testing"
)

func TestNativeUnavailableRejectsCaptureAndRestore(t *testing.T) {
	if _, err := Capture(202); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("capture: %v", err)
	}
	r := Registration{Version: 1, BootID: testBoot, CoalitionID: 91, OwnerUID: 501}
	if _, err := Restore(r); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("restore: %v", err)
	}
}
