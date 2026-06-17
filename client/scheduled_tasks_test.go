package client

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/schedule"
)

func TestWatchScheduledTasksEmitsMissedOneShotAtStartup(t *testing.T) {
	dir := t.TempDir()
	// 创建一个很久以前的一次性任务，使其单次触发窗口已经过去。
	created := time.Now().Add(-72 * time.Hour)
	if err := schedule.WriteTasks(dir, []schedule.Task{{
		ID:        "missed-1",
		Cron:      "0 9 * * *",
		Prompt:    "run the daily report",
		CreatedAt: created.UnixMilli(),
		Recurring: false,
	}}); err != nil {
		t.Fatalf("write tasks: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	handle, err := WatchScheduledTasks(ctx, WatchScheduledTasksOptions{Dir: dir, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	select {
	case event := <-handle.Events():
		if event.Type != ScheduledTaskMissed {
			t.Fatalf("expected missed event, got %q", event.Type)
		}
		if len(event.Tasks) != 1 || event.Tasks[0].ID != "missed-1" {
			t.Fatalf("unexpected missed payload: %+v", event.Tasks)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for missed event")
	}
}

func TestWatchScheduledTasksReportsNextFireForRecurring(t *testing.T) {
	dir := t.TempDir()
	if err := schedule.WriteTasks(dir, []schedule.Task{{
		ID:        "every-minute",
		Cron:      "* * * * *",
		Prompt:    "tick",
		CreatedAt: time.Now().Add(-time.Hour).UnixMilli(),
		Recurring: true,
	}}); err != nil {
		t.Fatalf("write tasks: %v", err)
	}

	// Cron 粒度是一分钟，直接断言真实触发需要等到下一分钟边界。
	// NextFireTime 与触发路径共享同一调度计算，并且可以立即观测。
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	handle, err := WatchScheduledTasks(ctx, WatchScheduledTasksOptions{Dir: dir, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if next, ok := handle.NextFireTime(); ok {
			if next < time.Now().UnixMilli() {
				t.Fatalf("next fire time should be in the future, got %d", next)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected a next fire time for a recurring task")
}

func TestBuildMissedTaskNotification(t *testing.T) {
	if got := BuildMissedTaskNotification(nil); got != "" {
		t.Fatalf("expected empty notification for no tasks, got %q", got)
	}

	notification := BuildMissedTaskNotification([]CronTask{
		{ID: "b", Cron: "0 9 * * *", Prompt: "later task", CreatedAt: 200},
		{ID: "a", Cron: "30 8 * * *", Prompt: "earlier task", CreatedAt: 100},
	})
	if !strings.Contains(notification, "2 scheduled tasks") {
		t.Fatalf("expected count in notification: %q", notification)
	}
	// 按 CreatedAt 排序：更早的任务在前。
	if strings.Index(notification, "earlier task") > strings.Index(notification, "later task") {
		t.Fatalf("expected tasks ordered by CreatedAt: %q", notification)
	}
	if !strings.Contains(notification, "AskUserQuestion") {
		t.Fatalf("expected AskUserQuestion guidance: %q", notification)
	}
}
