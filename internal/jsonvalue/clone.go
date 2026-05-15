package jsonvalue

func CloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = CloneValue(value)
	}
	return output
}

func CloneMapOrEmpty(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	return CloneMap(input)
}

func CloneAnySlice(input []any) []any {
	if input == nil {
		return nil
	}
	output := make([]any, 0, len(input))
	for _, item := range input {
		output = append(output, CloneValue(item))
	}
	return output
}

func CloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func CloneStringSlice(input []string) []string {
	if input == nil {
		return nil
	}
	return append([]string(nil), input...)
}

func CloneMapPreserveTypedSlices(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = CloneValuePreserveTypedSlices(value)
	}
	return output
}

func CloneAnySlicePreserveTypedSlices(input []any) []any {
	if input == nil {
		return nil
	}
	output := make([]any, 0, len(input))
	for _, item := range input {
		output = append(output, CloneValuePreserveTypedSlices(item))
	}
	return output
}

func CloneMapSlice(input []map[string]any) []map[string]any {
	if input == nil {
		return nil
	}
	output := make([]map[string]any, 0, len(input))
	for _, item := range input {
		output = append(output, CloneMapPreserveTypedSlices(item))
	}
	return output
}

func CloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneMap(typed)
	case map[string]string:
		return CloneStringMap(typed)
	case []any:
		return CloneAnySlice(typed)
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, CloneMap(item))
		}
		return out
	case []string:
		return CloneStringSlice(typed)
	default:
		return typed
	}
}

func CloneValuePreserveTypedSlices(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneMapPreserveTypedSlices(typed)
	case map[string]string:
		return CloneStringMap(typed)
	case []any:
		return CloneAnySlicePreserveTypedSlices(typed)
	case []map[string]any:
		return CloneMapSlice(typed)
	case []string:
		return CloneStringSlice(typed)
	default:
		return typed
	}
}
