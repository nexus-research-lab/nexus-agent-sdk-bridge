//go:build darwin && cgo

// INPUT: io.Copy 的超限 launchctl 输出。
// OUTPUT: ReaderFrom/WriterTo 优化不能绕过输出上限。
// POS: 外部命令观察的资源上限测试。
package supervision

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestLaunchctlOutputBoundSurvivesCopy(t *testing.T) {
	var output boundedOutput
	if _, err := io.Copy(&output, strings.NewReader(strings.Repeat("x", (1<<20)+1))); err == nil {
		t.Fatal("unbounded command output accepted")
	}
	if len(output.String()) > 1<<20 {
		t.Fatal("command output retained past limit")
	}
}

func TestCanceledStartNeverReservesIntent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	host := &fixtureHost{}
	process, err := Start(ctx, Config{Host: host}, Command{})
	if !errors.Is(err, context.Canceled) || process != nil || len(host.steps) != 0 {
		t.Fatalf("canceled admission: %v, process=%v, steps=%v", err, process, host.steps)
	}
}
