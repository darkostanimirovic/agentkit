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

	props := schema["properties"].(map[string]any)
	query := props["query"].(map[string]any)
	if query["description"] != "Search query" {
		t.Fatalf("expected description, got %v", query["description"])
	}

	required := schema["required"].([]string)
	// In strict mode, all fields are in required array (optional ones use anyOf with null)
	if len(required) != 2 {
		t.Fatalf("expected 2 fields in required array, got %v", required)
	}
	
	// Check that query is in the required array
	hasQuery := false
	for _, field := range required {
		if field == requiredQueryField {
			hasQuery = true
			break
		}
	}
	if !hasQuery {
		t.Fatalf("expected %s to be in required array", requiredQueryField)
	}
	
	// Check that limit is wrapped in anyOf with null
	limit := props["limit"].(map[string]any)
	if _, hasAnyOf := limit["anyOf"]; !hasAnyOf {
		t.Error("expected optional limit field to have anyOf")
	}
}

func TestSchemaFromStruct_Nested(t *testing.T) {
	schema, err := SchemaFromStruct(nestedStructSchema{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	filters := props["filters"].(map[string]any)
	
	// Since Filters field is not required, it's wrapped in anyOf with null
	anyOf, ok := filters["anyOf"].([]map[string]any)
	if !ok {
		t.Fatalf("expected anyOf for optional nested object, got %v", filters)
	}
	
	// Find the object schema in anyOf
	var objSchema map[string]any
	for _, schema := range anyOf {
		if schema["type"] == paramTypeObject {
			objSchema = schema
			break
		}
	}
	
	if objSchema == nil {
		t.Fatal("expected object schema in anyOf")
	}
	
	// Check that the nested object has additionalProperties: false
	if objSchema["additionalProperties"] != false {
		t.Error("expected nested object to have additionalProperties: false")
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
