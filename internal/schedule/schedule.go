// Package schedule 实现宿主 scheduled_tasks.json observer 需要的最小 cron 与任务文件逻辑。
package schedule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Task 是项目内 .nexus/scheduled_tasks.json 的任务文件形状。
type Task struct {
	CreatedAt int64
	Cron      string
	ID        string
	Prompt    string
	Recurring bool
}

type taskFileShape struct {
	Tasks []map[string]any `json:"tasks"`
}

// TaskFilePath 返回项目内调度任务文件路径。
func TaskFilePath(dir string) string {
	return filepath.Join(dir, ".nexus", "scheduled_tasks.json")
}

// ReadTasks 读取并返回合法的磁盘任务；缺失、不可读或格式错误都按空任务处理。
func ReadTasks(dir string) ([]Task, error) {
	content, err := os.ReadFile(TaskFilePath(dir))
	if err != nil {
		return nil, nil
	}
	var root taskFileShape
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, nil
	}
	if root.Tasks == nil {
		return nil, nil
	}
	result := make([]Task, 0, len(root.Tasks))
	for _, item := range root.Tasks {
		task, ok := taskFromMap(item)
		if !ok {
			continue
		}
		result = append(result, task)
	}
	return result, nil
}

// WriteTasks 将调度任务写回项目任务文件。
func WriteTasks(dir string, tasks []Task) error {
	if err := os.MkdirAll(filepath.Dir(TaskFilePath(dir)), 0o755); err != nil {
		return err
	}
	root := taskFileShape{Tasks: make([]map[string]any, 0, len(tasks))}
	for _, task := range tasks {
		root.Tasks = append(root.Tasks, taskToMap(task))
	}
	content, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(TaskFilePath(dir), content, 0o644)
}

func taskFromMap(input map[string]any) (Task, bool) {
	id, ok := stringField(input, "id")
	if !ok || strings.TrimSpace(id) == "" {
		return Task{}, false
	}
	cron, ok := stringField(input, "cron")
	if !ok || strings.TrimSpace(cron) == "" {
		return Task{}, false
	}
	if _, ok := parseCron(cron); !ok {
		return Task{}, false
	}
	prompt, ok := stringField(input, "prompt")
	if !ok {
		return Task{}, false
	}
	createdAt, ok := numericMilliseconds(input["created_at"])
	if !ok {
		return Task{}, false
	}
	task := Task{
		CreatedAt: createdAt,
		Cron:      cron,
		ID:        id,
		Prompt:    prompt,
	}
	if recurring, ok := boolField(input, "recurring"); ok && recurring {
		task.Recurring = true
	}
	return task, true
}

func taskToMap(task Task) map[string]any {
	result := map[string]any{
		"id":         task.ID,
		"cron":       task.Cron,
		"prompt":     task.Prompt,
		"created_at": task.CreatedAt,
	}
	if task.Recurring {
		result["recurring"] = true
	}
	return result
}

func stringField(input map[string]any, key string) (string, bool) {
	value, ok := input[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func boolField(input map[string]any, key string) (bool, bool) {
	value, ok := input[key]
	if !ok {
		return false, false
	}
	typed, ok := value.(bool)
	return typed, ok
}

func numericMilliseconds(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		number, err := typed.Int64()
		return number, err == nil
	default:
		return 0, false
	}
}

type cronFields struct {
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int
}

func parseCron(expr string) (cronFields, bool) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return cronFields{}, false
	}
	minute, ok := expandCronField(parts[0], 0, 59, false)
	if !ok {
		return cronFields{}, false
	}
	hour, ok := expandCronField(parts[1], 0, 23, false)
	if !ok {
		return cronFields{}, false
	}
	dayOfMonth, ok := expandCronField(parts[2], 1, 31, false)
	if !ok {
		return cronFields{}, false
	}
	month, ok := expandCronField(parts[3], 1, 12, false)
	if !ok {
		return cronFields{}, false
	}
	dayOfWeek, ok := expandCronField(parts[4], 0, 6, true)
	if !ok {
		return cronFields{}, false
	}
	return cronFields{
		Minute:     minute,
		Hour:       hour,
		DayOfMonth: dayOfMonth,
		Month:      month,
		DayOfWeek:  dayOfWeek,
	}, true
}

func expandCronField(field string, min int, max int, dayOfWeek bool) ([]int, bool) {
	field = strings.TrimSpace(field)
	if field == "*" {
		result := make([]int, 0, max-min+1)
		for value := min; value <= max; value++ {
			result = append(result, value)
		}
		return result, true
	}

	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 {
			return nil, false
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return nil, false
		}
		if parts[0] == "*" {
			result := make([]int, 0, (max-min)/step+1)
			for value := min; value <= max; value += step {
				result = append(result, value)
			}
			return result, true
		}
		if strings.Contains(parts[0], "-") {
			rangeParts := strings.Split(parts[0], "-")
			if len(rangeParts) != 2 {
				return nil, false
			}
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, false
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, false
			}
			limit := max
			if dayOfWeek {
				limit = 7
			}
			if start < min || start > limit || end < min || end > limit || start > end {
				return nil, false
			}
			seen := map[int]struct{}{}
			result := []int{}
			for value := start; value <= end; value += step {
				normalized := value
				if dayOfWeek && normalized == 7 {
					normalized = 0
				}
				if _, exists := seen[normalized]; exists {
					continue
				}
				seen[normalized] = struct{}{}
				result = append(result, normalized)
			}
			return result, len(result) > 0
		}
		return nil, false
	}

	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		result := make([]int, 0, len(parts))
		seen := map[int]struct{}{}
		for _, part := range parts {
			values, ok := expandCronField(part, min, max, dayOfWeek)
			if !ok {
				return nil, false
			}
			for _, value := range values {
				if _, exists := seen[value]; exists {
					continue
				}
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
		return result, len(result) > 0
	}

	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return nil, false
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, false
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, false
		}
		limit := max
		if dayOfWeek {
			limit = 7
		}
		if start < min || start > limit || end < min || end > limit || start > end {
			return nil, false
		}
		seen := map[int]struct{}{}
		result := make([]int, 0, end-start+1)
		for value := start; value <= end; value++ {
			normalized := value
			if dayOfWeek && normalized == 7 {
				normalized = 0
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
		return result, true
	}

	value, err := strconv.Atoi(field)
	if err != nil {
		return nil, false
	}
	if dayOfWeek && value == 7 {
		value = 0
	}
	if value < min || value > max {
		return nil, false
	}
	return []int{value}, true
}

// NextRunString 计算 cron 表达式从指定时间之后的下一次运行时间。
func NextRunString(expr string, from time.Time) (time.Time, bool) {
	fields, ok := parseCron(expr)
	if !ok {
		return time.Time{}, false
	}
	return nextRun(fields, from)
}

func nextRun(fields cronFields, from time.Time) (time.Time, bool) {
	start := from.Truncate(time.Minute).Add(time.Minute)
	minutes := intSet(fields.Minute)
	hours := intSet(fields.Hour)
	dayOfMonths := intSet(fields.DayOfMonth)
	months := intSet(fields.Month)
	dayOfWeeks := intSet(fields.DayOfWeek)
	dayOfMonthWildcard := len(fields.DayOfMonth) == 31
	dayOfWeekWildcard := len(fields.DayOfWeek) == 7

	for step := 0; step < 366*24*60; step++ {
		candidate := start.Add(time.Duration(step) * time.Minute)
		if _, ok := months[int(candidate.Month())]; !ok {
			continue
		}
		_, dayOfMonthOK := dayOfMonths[candidate.Day()]
		_, dayOfWeekOK := dayOfWeeks[int(candidate.Weekday())]
		dayMatches := false
		switch {
		case dayOfMonthWildcard && dayOfWeekWildcard:
			dayMatches = true
		case dayOfMonthWildcard:
			dayMatches = dayOfWeekOK
		case dayOfWeekWildcard:
			dayMatches = dayOfMonthOK
		default:
			dayMatches = dayOfMonthOK || dayOfWeekOK
		}
		if !dayMatches {
			continue
		}
		if _, ok := hours[candidate.Hour()]; !ok {
			continue
		}
		if _, ok := minutes[candidate.Minute()]; !ok {
			continue
		}
		return candidate, true
	}
	return time.Time{}, false
}

func intSet(values []int) map[int]struct{} {
	result := make(map[int]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
