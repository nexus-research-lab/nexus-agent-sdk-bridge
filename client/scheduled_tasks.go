package client

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/schedule"
)

// CronTask 表示宿主可见的 scheduled_tasks.json 定时任务投影。
type CronTask struct {
	ID        string `json:"id"`
	Cron      string `json:"cron"`
	Prompt    string `json:"prompt"`
	CreatedAt int64  `json:"createdAt"`
	Recurring bool   `json:"recurring,omitempty"`
}

// ScheduledTaskEventType 枚举 scheduled task 事件类型。
type ScheduledTaskEventType string

const (
	// ScheduledTaskFired 表示任务 cron schedule 到期。
	ScheduledTaskFired ScheduledTaskEventType = "fire"
	// ScheduledTaskMissed 表示启动时发现一次性任务已错过触发窗口。
	ScheduledTaskMissed ScheduledTaskEventType = "missed"
)

// ScheduledTaskEvent 表示 ScheduledTasksHandle 事件流里的事件。
// Fire 事件携带单个 Task，missed 事件携带已过期的一次性任务批次。
type ScheduledTaskEvent struct {
	Type  ScheduledTaskEventType
	Task  *CronTask
	Tasks []CronTask
}

// ScheduledTasksHandle 观察某个目录下的 scheduled tasks。
type ScheduledTasksHandle interface {
	// Events 持续输出 fire/missed 事件，直到 watch context 取消。
	// watcher 停止时 channel 会关闭。
	Events() <-chan ScheduledTaskEvent
	// NextFireTime 返回当前已加载任务中最近的触发时间，单位是 epoch milliseconds。
	// 没有已调度任务时 ok=false。
	NextFireTime() (epochMs int64, ok bool)
}

// WatchScheduledTasksOptions 配置 WatchScheduledTasks。
type WatchScheduledTasksOptions struct {
	// Dir 是需要观察 scheduled_tasks.json 的目录。为空时默认当前工作目录。
	Dir string
	// PollInterval 控制重新读取任务文件的间隔。小于等于 0 时默认 30s。
	PollInterval time.Duration
}

const defaultScheduledTasksPollInterval = 30 * time.Second

// WatchScheduledTasks 观察 <dir>/.nexus/scheduled_tasks.json，并在任务触发时输出事件。
// 这是宿主侧轮询 observer；它不会持有 CLI 在交互式 REPL 中使用的跨进程 PID lock。
func WatchScheduledTasks(ctx context.Context, options WatchScheduledTasksOptions) (ScheduledTasksHandle, error) {
	dir := strings.TrimSpace(options.Dir)
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = cwd
	}

	interval := options.PollInterval
	if interval <= 0 {
		interval = defaultScheduledTasksPollInterval
	}

	watcher := &scheduledTasksWatcher{
		dir:      dir,
		interval: interval,
		events:   make(chan ScheduledTaskEvent, 16),
		fired:    map[string]struct{}{},
	}
	go watcher.run(ctx)
	return watcher, nil
}

type scheduledTasksWatcher struct {
	dir      string
	interval time.Duration
	events   chan ScheduledTaskEvent
	fired    map[string]struct{}

	nextFireMu sync.RWMutex
	nextFireMs int64
	nextFireOK bool
}

func (w *scheduledTasksWatcher) Events() <-chan ScheduledTaskEvent {
	return w.events
}

func (w *scheduledTasksWatcher) NextFireTime() (int64, bool) {
	w.nextFireMu.RLock()
	defer w.nextFireMu.RUnlock()
	return w.nextFireMs, w.nextFireOK
}

func (w *scheduledTasksWatcher) run(ctx context.Context) {
	defer close(w.events)

	start := time.Now()
	lastTick := start

	// 启动扫描：暴露目标触发时间已经过去的一次性任务。
	if tasks, err := schedule.ReadTasks(w.dir); err == nil {
		w.refreshNextFire(tasks, start)
		var missed []schedule.Task
		for _, task := range tasks {
			if task.Recurring {
				continue
			}
			target, ok := schedule.NextRunString(task.Cron, time.UnixMilli(task.CreatedAt))
			if ok && target.Before(start) {
				missed = append(missed, task)
				w.fired[task.ID] = struct{}{}
			}
		}
		if len(missed) > 0 {
			if !w.emit(ctx, ScheduledTaskEvent{Type: ScheduledTaskMissed, Tasks: projectCronTasks(missed)}) {
				return
			}
		}
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			tasks, err := schedule.ReadTasks(w.dir)
			if err != nil {
				lastTick = now
				continue
			}
			w.refreshNextFire(tasks, now)

			var remaining []schedule.Task
			fileChanged := false
			for _, task := range tasks {
				if _, done := w.fired[task.ID]; done {
					// 一次性任务已经触发，从持久化文件中移除。
					if !task.Recurring {
						fileChanged = true
						continue
					}
					remaining = append(remaining, task)
					continue
				}

				due, ok := schedule.NextRunString(task.Cron, lastTick)
				if !ok || due.After(now) {
					remaining = append(remaining, task)
					continue
				}

				projected := projectCronTask(task)
				if !w.emit(ctx, ScheduledTaskEvent{Type: ScheduledTaskFired, Task: &projected}) {
					return
				}

				if task.Recurring {
					remaining = append(remaining, task)
				} else {
					w.fired[task.ID] = struct{}{}
					fileChanged = true
				}
			}

			if fileChanged {
				_ = schedule.WriteTasks(w.dir, remaining)
			}
			lastTick = now
		}
	}
}

func (w *scheduledTasksWatcher) emit(ctx context.Context, event ScheduledTaskEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case w.events <- event:
		return true
	}
}

func (w *scheduledTasksWatcher) refreshNextFire(tasks []schedule.Task, from time.Time) {
	var soonest time.Time
	found := false
	for _, task := range tasks {
		if _, done := w.fired[task.ID]; done {
			continue
		}
		next, ok := schedule.NextRunString(task.Cron, from)
		if !ok {
			continue
		}
		if !found || next.Before(soonest) {
			soonest = next
			found = true
		}
	}

	w.nextFireMu.Lock()
	if found {
		w.nextFireMs = soonest.UnixMilli()
		w.nextFireOK = true
	} else {
		w.nextFireMs = 0
		w.nextFireOK = false
	}
	w.nextFireMu.Unlock()
}

func projectCronTasks(tasks []schedule.Task) []CronTask {
	result := make([]CronTask, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, projectCronTask(task))
	}
	return result
}

func projectCronTask(task schedule.Task) CronTask {
	return CronTask{
		ID:        task.ID,
		Cron:      task.Cron,
		Prompt:    task.Prompt,
		CreatedAt: task.CreatedAt,
		Recurring: task.Recurring,
	}
}

// BuildMissedTaskNotification 将错过的一次性任务格式化为提示，要求模型执行前先向用户确认。
func BuildMissedTaskNotification(missed []CronTask) string {
	if len(missed) == 0 {
		return ""
	}

	ordered := append([]CronTask(nil), missed...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].CreatedAt < ordered[j].CreatedAt
	})

	var builder strings.Builder
	if len(ordered) == 1 {
		builder.WriteString("While you were away, 1 scheduled task became due:\n")
	} else {
		builder.WriteString("While you were away, ")
		builder.WriteString(strconv.Itoa(len(ordered)))
		builder.WriteString(" scheduled tasks became due:\n")
	}
	for _, task := range ordered {
		builder.WriteString("- ")
		prompt := strings.TrimSpace(task.Prompt)
		if prompt == "" {
			prompt = "(no prompt)"
		}
		builder.WriteString(prompt)
		builder.WriteString(" [")
		builder.WriteString(task.Cron)
		builder.WriteString("]\n")
	}
	builder.WriteString("\nUse AskUserQuestion to confirm with the user before executing any of them.")
	return builder.String()
}
