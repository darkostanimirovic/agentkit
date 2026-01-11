package agentkit


import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

const (
	testToolName2 = "test_tool"
	typeString    = "string"
	typeArray     = "array"
)

func TestNewTool(t *testing.T) {
	tool := NewTool(testToolName2).Build()

	if tool.name != testToolName2 {
		t.Errorf("expected name test_tool, got %s", tool.name)
	}

	if tool.description != "" {
		t.Errorf("expected empty description, got %s", tool.description)
	}

	if tool.handler != nil {
		t.Error("expected nil handler")
	}
}

func TestToolBuilder_WithDescription(t *testing.T) {
	desc := "A test tool for testing"
	tool := NewTool("test_tool").
		WithDescription(desc).
		Build()

	if tool.description != desc {
		t.Errorf("expected description %s, got %s", desc, tool.description)
	}
}

func TestToolBuilder_WithParameter_String(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required().WithDescription("User name")).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	nameParam := props["name"].(map[string]any)

	if nameParam["type"] != typeString {
		t.Errorf("expected type string, got %v", nameParam["type"])
	}

	if nameParam["description"] != "User name" {
		t.Errorf("expected description 'User name', got %v", nameParam["description"])
	}

	required := tool.parameters["required"].([]string)
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("expected required=['name'], got %v", required)
	}
}

func TestToolBuilder_WithParameter_Optional(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("optional_field", String().Optional()).
		Build()

	// In strict mode, all fields are in required array (optional ones use anyOf with null)
	required := tool.parameters["required"].([]string)
	if len(required) != 1 {
		t.Errorf("expected 1 param in required array, got %v", required)
	}
	
	// Check that the optional field uses anyOf with null
	props := tool.parameters["properties"].(map[string]any)
	optField := props["optional_field"].(map[string]any)
	if _, hasAnyOf := optField["anyOf"]; !hasAnyOf {
		t.Error("expected optional field to have anyOf")
	}
}

func TestToolBuilder_WithParameter_Array(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("tags", Array("string").Required().WithDescription("List of tags")).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	tagsParam := props["tags"].(map[string]any)

	if tagsParam["type"] != typeArray {
		t.Errorf("expected type array, got %v", tagsParam["type"])
	}

	items := tagsParam["items"].(map[string]any)
	itemType := items["type"].(string)

	if itemType != typeString {
		t.Errorf("expected items type string, got %v", itemType)
	}

	if tagsParam["description"] != "List of tags" {
		t.Errorf("expected description 'List of tags', got %v", tagsParam["description"])
	}
}

func TestToolBuilder_MultipleParameters(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("required_field", String().Required()).
		WithParameter("optional_field", String().Optional()).
		WithParameter("array_field", Array("string").Required()).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	if len(props) != 3 {
		t.Errorf("expected 3 properties, got %d", len(props))
	}

	// In strict mode, all fields are in required array
	required := tool.parameters["required"].([]string)
	if len(required) != 3 {
		t.Errorf("expected 3 params in required array, got %d", len(required))
	}

	// Check all fields are present in required
	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}

	if !requiredMap["required_field"] || !requiredMap["array_field"] || !requiredMap["optional_field"] {
		t.Error("expected all fields to be in required array")
	}

	// Check that optional field uses anyOf with null
	optField := props["optional_field"].(map[string]any)
	if _, hasAnyOf := optField["anyOf"]; !hasAnyOf {
		t.Error("expected optional field to have anyOf")
	}
}

func TestToolBuilder_WithHandler(t *testing.T) {
	called := false
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		called = true
		return "result", nil
	}

	tool := NewTool("test_tool").
		WithHandler(handler).
		Build()

	if tool.handler == nil {
		t.Fatal("expected handler to be set")
	}

	// Test handler
	result, err := tool.handler(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected handler to be called")
	}

	if result != "result" {
		t.Errorf("expected result 'result', got %v", result)
	}
}

func TestToolBuilder_WithConcurrency(t *testing.T) {
	tool := NewTool("serial_tool").
		WithConcurrency(ConcurrencySerial).
		Build()

	if tool.concurrency != ConcurrencySerial {
		t.Fatalf("expected concurrency serial, got %v", tool.concurrency)
	}
}

func TestToolBuilder_Chaining(t *testing.T) {
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return map[string]any{"success": true}, nil
	}

	tool := NewTool("test_tool").
		WithDescription("Test tool").
		WithParameter("param1", String().Required()).
		WithParameter("param2", String().Optional()).
		WithHandler(handler).
		Build()

	if tool.name != "test_tool" {
		t.Error("builder chaining failed for name")
	}

	if tool.description != "Test tool" {
		t.Error("builder chaining failed for description")
	}

	if tool.handler == nil {
		t.Error("builder chaining failed for handler")
	}

	props := tool.parameters["properties"].(map[string]any)
	if len(props) != 2 {
		t.Error("builder chaining failed for parameters")
	}
}

func TestTool_ToOpenAI(t *testing.T) {
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return nil, nil
	}

	tool := NewTool("assign_team").
		WithDescription("Assign work item to team").
		WithParameter("team_slug", String().Required().WithDescription("Team slug")).
		WithParameter("reasoning", String().Optional().WithDescription("Reasoning")).
		WithHandler(handler).
		Build()

	// Test new provider-agnostic method
	toolDef := tool.ToToolDefinition()

	if toolDef.Name != "assign_team" {
		t.Errorf("expected name assign_team, got %s", toolDef.Name)
	}

	if toolDef.Description != "Assign work item to team" {
		t.Errorf("expected description, got %s", toolDef.Description)
	}

	// Verify parameters are JSON-encodable
	paramsJSON, err := json.Marshal(toolDef.Parameters)
	if err != nil {
		t.Fatalf("parameters not JSON-encodable: %v", err)
	}

	var params map[string]any
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		t.Fatalf("failed to unmarshal parameters: %v", err)
	}

	if params["type"] != paramTypeObject {
		t.Errorf("expected type object, got %v", params["type"])
	}

	props := params["properties"].(map[string]any)
	if len(props) != 2 {
		t.Errorf("expected 2 properties, got %d", len(props))
	}
}

func TestParameterSchema_String(t *testing.T) {
	schema := String()

	if schema.paramType != typeString {
		t.Errorf("expected type string, got %s", schema.paramType)
	}

	m := schema.ToMap()
	if m["type"] != typeString {
		t.Errorf("expected type string in map, got %v", m["type"])
	}
}

func TestParameterSchema_Array(t *testing.T) {
	schema := Array(typeString)

	if schema.paramType != typeArray {
		t.Errorf("expected type array, got %s", schema.paramType)
	}

	m := schema.ToMap()
	if m["type"] != typeArray {
		t.Errorf("expected type array in map, got %v", m["type"])
	}

	items := m["items"].(map[string]any)
	itemType := items["type"].(string)

	if itemType != typeString {
		t.Errorf("expected items type string, got %v", itemType)
	}
}

func TestParameterSchema_Required(t *testing.T) {
	schema := String().Required()

	if !schema.required {
		t.Error("expected required to be true")
	}
}

func TestParameterSchema_Optional(t *testing.T) {
	schema := String().Optional()

	if schema.required {
		t.Error("expected required to be false")
	}
}

func TestParameterSchema_WithDescription(t *testing.T) {
	desc := "A test description"
	schema := String().WithDescription(desc)

	if schema.description != desc {
		t.Errorf("expected description %s, got %s", desc, schema.description)
	}

	m := schema.ToMap()
	if m["description"] != desc {
		t.Errorf("expected description in map, got %v", m["description"])
	}
}

func TestParameterSchema_ToMap(t *testing.T) {
	schema := String().Required().WithDescription("Test param")
	m := schema.ToMap()

	if m["type"] != typeString {
		t.Error("ToMap missing type")
	}

	if m["description"] != "Test param" {
		t.Error("ToMap missing description")
	}

	// Required is handled at tool level, not in parameter map
}

func TestParameterSchema_WithEnum(t *testing.T) {
	schema := String().Required().WithEnum("option1", "option2", "option3")

	m := schema.ToMap()
	if m["type"] != typeString {
		t.Error("expected type string")
	}

	enum, ok := m["enum"].([]string)
	if !ok {
		t.Fatal("expected enum to be []string")
	}

	if len(enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(enum))
	}

	expected := []string{"option1", "option2", "option3"}
	for i, v := range expected {
		if enum[i] != v {
			t.Errorf("expected enum[%d] = %s, got %s", i, v, enum[i])
		}
	}
}

func TestTool_Execute(t *testing.T) {
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		name := args["name"].(string)
		return map[string]any{"greeting": "Hello, " + name}, nil
	}

	tool := NewTool("greet").
		WithHandler(handler).
		Build()

	ctx := context.Background()
	result, err := tool.Execute(ctx, `{"name":"Alice"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["greeting"] != "Hello, Alice" {
		t.Errorf("unexpected result: %v", resultMap)
	}
}

func TestTool_Execute_InvalidJSON(t *testing.T) {
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return nil, nil
	}

	tool := NewTool("test").WithHandler(handler).Build()

	ctx := context.Background()
	_, err := tool.Execute(ctx, `{invalid json}`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestTool_Execute_HandlerError(t *testing.T) {
	expectedErr := errors.New("handler failed")
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return nil, expectedErr
	}

	tool := NewTool("failing").WithHandler(handler).Build()

	ctx := context.Background()
	_, err := tool.Execute(ctx, `{}`)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestParameterSchema_ArrayToMap(t *testing.T) {
	schema := Array("number").WithDescription("Array of numbers")
	m := schema.ToMap()

	if m["type"] != typeArray {
		t.Error("ToMap missing type")
	}

	if m["description"] != "Array of numbers" {
		t.Error("ToMap missing description")
	}

	items := m["items"].(map[string]any)
	itemType := items["type"].(string)

	if itemType != "number" {
		t.Error("ToMap missing items type")
	}
}

func TestParameterSchema_Object(t *testing.T) {
	schema := Object().
		WithProperty("query", String().Required()).
		WithProperty("limit", String().Optional())

	m := schema.ToMap()
	if m["type"] != paramTypeObject {
		t.Fatalf("expected type object, got %v", m["type"])
	}

	props := m["properties"].(map[string]any)
	if _, ok := props["query"]; !ok {
		t.Fatal("expected query property")
	}
	if _, ok := props["limit"]; !ok {
		t.Fatal("expected limit property")
	}

	required := m["required"].([]string)
	if len(required) != 1 || required[0] != "query" {
		found := false
		for _, name := range required {
			if name == "query" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected required to include query, got %v", required)
		}
	}
}

func TestParameterSchema_ArrayOf(t *testing.T) {
	schema := ArrayOf(Object().WithProperty("id", String().Required()))
	m := schema.ToMap()

	items := m["items"].(map[string]any)
	if items["type"] != paramTypeObject {
		t.Fatalf("expected items type object, got %v", items["type"])
	}
}

func TestToolBuilder_WithJSONSchema(t *testing.T) {
	rawSchema := map[string]any{
		"type": paramTypeObject,
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
		"required": []string{"query"},
	}

	tool := NewTool("search").WithJSONSchema(rawSchema).Build()
	if tool.parameters["type"] != paramTypeObject {
		t.Fatalf("expected object schema, got %v", tool.parameters["type"])
	}
}
