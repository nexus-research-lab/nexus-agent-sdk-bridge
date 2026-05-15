package jsonvalue

import "encoding/json"

func AnyMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out, true
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, false
		}
		return out, true
	}
}

func AnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, true
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func MapValue(value any) map[string]any {
	if out, ok := AnyMap(value); ok {
		return out
	}
	return map[string]any{}
}

// CloneMapValue 将动态 JSON map 值解析为保留类型切片的副本。
func CloneMapValue(value any) map[string]any {
	out, ok := AnyMap(value)
	if !ok || len(out) == 0 {
		return nil
	}
	return CloneMapPreserveTypedSlices(out)
}

func SliceValue(value any) []any {
	if out, ok := AnySlice(value); ok {
		return out
	}
	return []any{}
}

func MapSliceValue(value any) []map[string]any {
	items := SliceValue(value)
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := MapValue(item)
		if len(payload) == 0 {
			continue
		}
		results = append(results, payload)
	}
	return results
}

func StringSliceValue(value any) []string {
	items := SliceValue(value)
	results := make([]string, 0, len(items))
	for _, item := range items {
		text := StringValue(item)
		if text == "" {
			continue
		}
		results = append(results, text)
	}
	return results
}
