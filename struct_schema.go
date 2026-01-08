package agentkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ErrInvalidStructSchema is returned when a schema cannot be built from the provided type.
var ErrInvalidStructSchema = errors.New("agentkit: struct schema requires a struct type")

// SchemaFromStruct builds a JSON schema object from a struct value or pointer.
func SchemaFromStruct(sample any) (map[string]any, error) {
	if sample == nil {
		return nil, ErrInvalidStructSchema
	}

	typeOf := reflect.TypeOf(sample)
	if typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}

	if typeOf.Kind() != reflect.Struct {
		return nil, ErrInvalidStructSchema
	}

	return schemaFromStructType(typeOf, map[reflect.Type]struct{}{})
}

// NewStructTool creates a tool builder using a struct type for schema and decoding.
func NewStructTool[T any](name string, handler func(context.Context, T) (any, error)) (*ToolBuilder, error) {
	var zero T
	schema, err := SchemaFromStruct(zero)
	if err != nil {
		return nil, err
	}

	wrapper := func(ctx context.Context, args map[string]any) (any, error) {
		var typed T
		payload, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to encode tool args: %w", err)
		}
		if err := json.Unmarshal(payload, &typed); err != nil {
			return nil, fmt.Errorf("failed to decode tool args: %w", err)
		}
		return handler(ctx, typed)
	}

	builder := NewTool(name).
		WithRawParameters(schema).
		WithHandler(wrapper)
	return builder, nil
}

func schemaFromStructType(t reflect.Type, visited map[reflect.Type]struct{}) (map[string]any, error) {
	if _, ok := visited[t]; ok {
		return map[string]any{"type": "object"}, nil
	}
	visited[t] = struct{}{}

	properties := make(map[string]any)
	required := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}

		name, omitEmpty, skip := jsonFieldName(field)
		if skip {
			continue
		}

		schema := schemaForType(field.Type, visited)
		if schema == nil {
			continue
		}

		if desc := field.Tag.Get("desc"); desc != "" {
			schema["description"] = desc
		}
		if enum := field.Tag.Get("enum"); enum != "" {
			values := splitCSV(enum)
			if len(values) > 0 {
				schema["enum"] = values
			}
		}
		if def := field.Tag.Get("default"); def != "" {
			schema["default"] = def
		}

		properties[name] = schema

		if isRequired(field, omitEmpty) {
			required = append(required, name)
		}
	}

	result := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		result["required"] = required
	}

	delete(visited, t)
	return result, nil
}

func schemaForType(t reflect.Type, visited map[reflect.Type]struct{}) map[string]any {
	if t.Kind() == reflect.Pointer {
		return schemaForType(t.Elem(), visited)
	}

	if t.PkgPath() == "time" && t.Name() == "Time" {
		return map[string]any{
			"type":   "string",
			"format": "date-time",
		}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		items := schemaForType(t.Elem(), visited)
		if items == nil {
			items = map[string]any{"type": "string"}
		}
		return map[string]any{
			"type":  "array",
			"items": items,
		}
	case reflect.Map:
		return map[string]any{"type": "object"}
	case reflect.Struct:
		schema, err := schemaFromStructType(t, visited)
		if err != nil {
			return map[string]any{"type": "object"}
		}
		return schema
	default:
		return map[string]any{"type": "string"}
	}
}

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}

	parts := strings.Split(tag, ",")
	name := parts[0]
	omitEmpty := false
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
			break
		}
	}

	if name == "" {
		return lowerFirst(field.Name), omitEmpty, false
	}

	return name, omitEmpty, false
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func isRequired(field reflect.StructField, omitEmpty bool) bool {
	requiredTag := strings.ToLower(strings.TrimSpace(field.Tag.Get("required")))
	if requiredTag == "true" || requiredTag == "1" || requiredTag == "yes" {
		return true
	}

	if requiredTag == "false" || requiredTag == "0" || requiredTag == "no" {
		return false
	}

	if omitEmpty {
		return false
	}

	return false
}
