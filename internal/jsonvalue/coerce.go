package jsonvalue

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

func StringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func BoolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func BoolValueStrict(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func IntValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
		if parsed, err := strconv.Atoi(typed.String()); err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

// IntValueStrict 将动态 JSON 值解析为整数，拒绝带小数的数值。
func IntValueStrict(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		floatValue := float64(typed)
		if math.Trunc(floatValue) != floatValue {
			return 0, false
		}
		return int(typed), true
	case float64:
		if math.Trunc(typed) != typed {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func Int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func FloatValue(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case float32:
		return float64(typed)
	case float64:
		return typed
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func FirstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func FirstNonEmptyString(values ...any) string {
	for _, value := range values {
		if text := StringValue(value); text != "" {
			return text
		}
	}
	return ""
}

func TrimmedStringValue(value any) string {
	return strings.TrimSpace(StringValue(value))
}

func FirstNonEmptyTrimmedString(values ...any) string {
	for _, value := range values {
		if text := TrimmedStringValue(value); text != "" {
			return text
		}
	}
	return ""
}

func StringPointer(value any) *string {
	text := StringValue(value)
	if text == "" {
		return nil
	}
	return &text
}

// BoolPointer 将动态 JSON 值解析为 bool 指针，nil 表示字段不存在。
func BoolPointer(value any) *bool {
	if value == nil {
		return nil
	}
	parsed := BoolValue(value)
	return &parsed
}

func IntPointer(value any) *int {
	if value == nil {
		return nil
	}
	parsed := IntValue(value)
	return &parsed
}

func FirstNonZeroInt(values ...any) int {
	for _, value := range values {
		if parsed := IntValue(value); parsed != 0 {
			return parsed
		}
	}
	return 0
}

func FirstNonNilInt(values ...any) int {
	for _, value := range values {
		if value == nil {
			continue
		}
		return IntValue(value)
	}
	return 0
}

func FirstNonNilIntPointer(values ...any) *int {
	for _, value := range values {
		if value == nil {
			continue
		}
		parsed := IntValue(value)
		return &parsed
	}
	return nil
}
