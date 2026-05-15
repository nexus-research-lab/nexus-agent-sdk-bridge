package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// NewTypedTool creates an MCP tool with a JSON Schema inferred from T.
func NewTypedTool[T any](
	name string,
	description string,
	handler func(context.Context, T) (ToolResult, error),
	annotations *ToolAnnotations,
) (Tool, error) {
	schema, err := JSONSchemaFor[T]()
	if err != nil {
		return Tool{}, err
	}

	return Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Annotations: annotations,
		Handler: func(ctx context.Context, input map[string]any) (ToolResult, error) {
			var decoded T
			payload, err := json.Marshal(input)
			if err != nil {
				return ToolResult{}, fmt.Errorf("client: marshal typed MCP tool input failed: %w", err)
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				return ToolResult{}, fmt.Errorf("client: decode typed MCP tool input failed: %w", err)
			}
			return handler(ctx, decoded)
		},
	}, nil
}

// JSONSchemaFor infers a JSON Schema from a Go type.
func JSONSchemaFor[T any]() (map[string]any, error) {
	var zero T
	return schemaForType(reflect.TypeOf(zero))
}

func schemaForType(valueType reflect.Type) (map[string]any, error) {
	if valueType == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}, nil
	}

	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}

	switch valueType.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.Slice, reflect.Array:
		itemSchema, err := schemaForType(valueType.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type":  "array",
			"items": itemSchema,
		}, nil
	case reflect.Map:
		return map[string]any{"type": "object"}, nil
	case reflect.Interface:
		return map[string]any{}, nil
	case reflect.Struct:
		properties := map[string]any{}
		required := make([]string, 0, valueType.NumField())
		for index := 0; index < valueType.NumField(); index++ {
			field := valueType.Field(index)
			if !field.IsExported() {
				continue
			}

			jsonTag := field.Tag.Get("json")
			name, omitEmpty, skip := parseJSONFieldTag(field.Name, jsonTag)
			if skip {
				continue
			}

			fieldSchema, err := schemaForType(field.Type)
			if err != nil {
				return nil, err
			}
			if description := strings.TrimSpace(field.Tag.Get("description")); description != "" {
				fieldSchema["description"] = description
			}
			applySchemaTags(field, fieldSchema)
			properties[name] = fieldSchema

			if !omitEmpty && field.Type.Kind() != reflect.Pointer {
				required = append(required, name)
			}
		}

		schema := map[string]any{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema, nil
	default:
		return nil, fmt.Errorf("client: unsupported schema type %s", valueType.Kind())
	}
}

func applySchemaTags(field reflect.StructField, schema map[string]any) {
	for _, key := range []string{"title", "format", "default"} {
		if value := strings.TrimSpace(field.Tag.Get(key)); value != "" {
			schema[key] = value
		}
	}

	if enumTag := strings.TrimSpace(field.Tag.Get("enum")); enumTag != "" {
		schema["enum"] = parseEnumValues(enumTag, field.Type)
	}

	for _, key := range []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf"} {
		if value := strings.TrimSpace(field.Tag.Get(key)); value != "" {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil {
				schema[key] = parsed
			}
		}
	}

	for _, key := range []string{"minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties"} {
		if value := strings.TrimSpace(field.Tag.Get(key)); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				schema[key] = parsed
			}
		}
	}
}

func parseEnumValues(enumTag string, valueType reflect.Type) []any {
	valueType = derefType(valueType)
	parts := strings.Split(enumTag, ",")
	values := make([]any, 0, len(parts))
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		values = append(values, parseEnumValue(raw, valueType))
	}
	return values
}

func parseEnumValue(raw string, valueType reflect.Type) any {
	switch valueType.Kind() {
	case reflect.Bool:
		if parsed, err := strconv.ParseBool(raw); err == nil {
			return parsed
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return parsed
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return parsed
		}
	case reflect.Float32, reflect.Float64:
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			return parsed
		}
	}
	return raw
}

func derefType(valueType reflect.Type) reflect.Type {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	return valueType
}

func parseJSONFieldTag(fieldName string, jsonTag string) (name string, omitEmpty bool, skip bool) {
	if jsonTag == "-" {
		return "", false, true
	}
	if jsonTag == "" {
		return fieldName, false, false
	}

	parts := strings.Split(jsonTag, ",")
	name = strings.TrimSpace(parts[0])
	if name == "" {
		name = fieldName
	}
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}
