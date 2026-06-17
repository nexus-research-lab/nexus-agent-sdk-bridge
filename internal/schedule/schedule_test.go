package schedule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadTasksMatchesTaskFileShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(TaskFilePath(dir)), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	payload := `{"tasks":[
		{"id":"task-1","cron":"0 * * * *","prompt":"","created_at":0,"last_fired_at":1710000000000,"recurring":true,"permanent":true,"durable":false,"agent_id":"agent-a","extra":"ignored"},
		{"id":"bad-cron","cron":"nope","prompt":"skip","created_at":1710000000000},
		{"id":123,"cron":"0 * * * *","prompt":"skip","created_at":1710000000000}
	]}`
	if err := os.WriteFile(TaskFilePath(dir), []byte(payload), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	tasks, err := ReadTasks(dir)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks = %#v, want one valid task", tasks)
	}
	if task := tasks[0]; task.ID != "task-1" || task.Prompt != "" || task.CreatedAt != 0 || !task.Recurring {
		t.Fatalf("task = %#v, want normalized task fields including empty prompt", task)
	}
}

func TestWriteTasksStripsRuntimeOnlyFieldsByConstruction(t *testing.T) {
	dir := t.TempDir()
	if err := WriteTasks(dir, []Task{{
		CreatedAt: 1710000000000,
		Cron:      "0 9 * * *",
		ID:        "task-1",
		Prompt:    "run check",
		Recurring: true,
	}}); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	content, err := os.ReadFile(TaskFilePath(dir))
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}
	text := string(content)
	if containsAny(text, "durable", "agent_id", "last_fired_at") {
		t.Fatalf("stored task file contains runtime-only fields: %s", text)
	}
}

func TestParseCronMatchesTSCronSubset(t *testing.T) {
	fields, ok := parseCron("0 9 * * 5-7/1")
	if !ok {
		t.Fatalf("parse range-step day-of-week expression failed")
	}
	want := []int{5, 6, 0}
	if len(fields.DayOfWeek) != len(want) {
		t.Fatalf("DayOfWeek = %#v, want %#v", fields.DayOfWeek, want)
	}
	for index := range want {
		if fields.DayOfWeek[index] != want[index] {
			t.Fatalf("DayOfWeek = %#v, want %#v", fields.DayOfWeek, want)
		}
	}
	if _, ok := parseCron("0 9 * * 5-7/0"); ok {
		t.Fatalf("parse accepted zero step")
	}
	if _, ok := parseCron("0 9 * * 7"); !ok {
		t.Fatalf("parse rejected day-of-week 7 Sunday alias")
	}
}

func TestNextRunUsesStandardCronDayOrSemantics(t *testing.T) {
	fields, ok := parseCron("0 9 15 * 1")
	if !ok {
		t.Fatalf("parse cron failed")
	}
	next, ok := nextRun(fields, time.Date(2026, time.April, 14, 10, 0, 0, 0, time.Local))
	if !ok {
		t.Fatalf("next run failed")
	}
	if next.Day() != 15 || next.Hour() != 9 || next.Minute() != 0 {
		t.Fatalf("next run = %s, want day-of-month match before next Monday", next)
	}
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
