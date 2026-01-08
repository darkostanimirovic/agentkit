package agentkit

import (
	"context"
	"testing"
)

type testStructSchema struct {
	Query string `json:"query" desc:"Search query" required:"true"`
	Limit int    `json:"limit"`
}

const requiredQueryField = "query"

type nestedStructSchema struct {
	Filters testStructSchema `json:"filters"`
	Tags    []string         `json:"tags"`
}

func TestSchemaFromStruct(t *testing.T) {
	schema, err := SchemaFromStruct(testStructSchema{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema["type"] != paramTypeObject {
		t.Fatalf("expected type object, got %v", schema["type"])
	}

	props := schema["properties"].(map[string]interface{})
	query := props["query"].(map[string]interface{})
	if query["description"] != "Search query" {
		t.Fatalf("expected description, got %v", query["description"])
	}

	required := schema["required"].([]string)
	if len(required) != 1 || required[0] != requiredQueryField {
		t.Fatalf("expected required [%s], got %v", requiredQueryField, required)
	}
}

func TestSchemaFromStruct_Nested(t *testing.T) {
	schema, err := SchemaFromStruct(nestedStructSchema{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]interface{})
	filters := props["filters"].(map[string]interface{})
	if filters["type"] != paramTypeObject {
		t.Fatalf("expected nested object, got %v", filters["type"])
	}
}

func TestNewStructTool(t *testing.T) {
	builder, err := NewStructTool("search", func(ctx context.Context, args testStructSchema) (any, error) {
		if args.Query == "" {
			return nil, nil
		}
		return map[string]any{"query": args.Query, "limit": args.Limit}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := builder.Build()
	result, err := tool.Execute(context.Background(), `{"query":"hello","limit":2}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res := result.(map[string]any)
	if res["query"] != "hello" {
		t.Fatalf("expected query hello, got %v", res["query"])
	}
}
