// INPUT: 取消的等待者和随后到达的共享退出事实。
// OUTPUT: 取消不清除观察结果，重复等待仍取得同一退出状态。
// POS: 等待生命周期测试，不冒充内核事件验证。
package processscope

import (
	"context"
	"errors"
	"testing"
)

func TestExitWaitCancellationPreservesFutureEvidence(t *testing.T) {
	monitor := &ExitMonitor{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := monitor.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	monitor.result = RootExit{PID: 202, Code: 7}
	close(monitor.done)
	for i := 0; i < 2; i++ {
		got, err := monitor.Wait(context.Background())
		if err != nil || got != monitor.result {
			t.Fatalf("wait=%#v %v", got, err)
		}
	}
	if _, err := monitor.Wait(nil); err == nil {
		t.Fatal("nil wait context accepted")
	}
}
