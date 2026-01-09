package agentkit

import (
	"context"
	"encoding/json"
	"testing"
)

// TestStructuredOutputs_StrictModeEnabled tests that strict mode is enabled by default
func TestStructuredOutputs_StrictModeEnabled(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		Build()

	if !tool.strict {
		t.Error("expected strict mode to be enabled by default")
	}
}

// TestStructuredOutputs_StrictModeDisabled tests disabling strict mode
func TestStructuredOutputs_StrictModeDisabled(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		WithStrictMode(false).
		Build()

	if tool.strict {
		t.Error("expected strict mode to be disabled")
	}
}

// TestStructuredOutputs_AdditionalPropertiesFalse tests that additionalProperties: false is set
func TestStructuredOutputs_AdditionalPropertiesFalse(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		WithParameter("age", String().Optional()).
		Build()

	params := tool.parameters
	if params["additionalProperties"] != false {
		t.Errorf("expected additionalProperties to be false, got %v", params["additionalProperties"])
	}
}

// TestStructuredOutputs_AllFieldsRequired tests that all fields are in required array in strict mode
func TestStructuredOutputs_AllFieldsRequired(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("required_field", String().Required()).
		WithParameter("optional_field", String().Optional()).
		Build()

	params := tool.parameters
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("expected required to be []string")
	}

	// In strict mode, all fields must be in required array
	if len(required) != 2 {
		t.Errorf("expected 2 fields in required array, got %d", len(required))
	}

	// Check that both fields are present
	requiredMap := make(map[string]bool)
	for _, name := range required {
		requiredMap[name] = true
	}

	if !requiredMap["required_field"] {
		t.Error("expected required_field to be in required array")
	}
	if !requiredMap["optional_field"] {
		t.Error("expected optional_field to be in required array (with anyOf union)")
	}
}

// TestStructuredOutputs_OptionalFieldsAnyOf tests that optional fields use anyOf with null
func TestStructuredOutputs_OptionalFieldsAnyOf(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		WithParameter("nickname", String().Optional()).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	
	// Required field should be a simple string
	nameSchema := props["name"].(map[string]any)
	if nameSchema["type"] != "string" {
		t.Errorf("expected name type to be string, got %v", nameSchema["type"])
	}
	if _, hasAnyOf := nameSchema["anyOf"]; hasAnyOf {
		t.Error("required field should not have anyOf")
	}

	// Optional field should have anyOf with null
	nicknameSchema := props["nickname"].(map[string]any)
	anyOf, ok := nicknameSchema["anyOf"].([]map[string]any)
	if !ok {
		t.Fatal("expected optional field to have anyOf array")
	}
	if len(anyOf) != 2 {
		t.Errorf("expected anyOf to have 2 items, got %d", len(anyOf))
	}

	// Check that one is string and one is null
	hasString := false
	hasNull := false
	for _, schema := range anyOf {
		if schema["type"] == "string" {
			hasString = true
		}
		if schema["type"] == "null" {
			hasNull = true
		}
	}

	if !hasString {
		t.Error("expected anyOf to include string type")
	}
	if !hasNull {
		t.Error("expected anyOf to include null type")
	}
}

// TestStructuredOutputs_NestedObjects tests strict mode with nested objects
func TestStructuredOutputs_NestedObjects(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("user", Object().
			WithProperty("id", String().Required()).
			WithProperty("name", String().Required()).
			WithProperty("email", String().Optional()).
			Required(), // Mark the object itself as required
		).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	userSchema := props["user"].(map[string]any)

	// Debug: print the schema
	t.Logf("userSchema: %+v", userSchema)

	// Check nested object has additionalProperties: false
	if userSchema["additionalProperties"] != false {
		t.Errorf("expected nested object to have additionalProperties: false, got %v", userSchema["additionalProperties"])
	}

	// Check nested required array includes all fields
	userProps := userSchema["properties"].(map[string]any)
	userRequired := userSchema["required"].([]string)
	
	if len(userRequired) != 3 {
		t.Errorf("expected 3 fields in nested required array, got %d", len(userRequired))
	}

	// Check optional email has anyOf with null
	emailSchema := userProps["email"].(map[string]any)
	if _, hasAnyOf := emailSchema["anyOf"]; !hasAnyOf {
		t.Error("expected optional nested field to have anyOf")
	}
}

// TestStructuredOutputs_ArrayOfObjects tests arrays with object items
func TestStructuredOutputs_ArrayOfObjects(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("users", ArrayOf(
			Object().
				WithProperty("id", String().Required()).
				WithProperty("name", String().Required()),
		).Required()).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	usersSchema := props["users"].(map[string]any)

	t.Logf("usersSchema: %+v", usersSchema)

	items := usersSchema["items"].(map[string]any)
	if items["type"] != "object" {
		t.Errorf("expected items type to be object, got %v", items["type"])
	}
	if items["additionalProperties"] != false {
		t.Error("expected array items object to have additionalProperties: false")
	}
}

// TestStructuredOutputs_ResponseToolStrict tests that ResponseTool gets strict flag
func TestStructuredOutputs_ResponseToolStrict(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		Build()

	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	agent.AddTool(tool)

	responseTools := agent.buildResponseTools()
	if len(responseTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(responseTools))
	}

	if !responseTools[0].Strict {
		t.Error("expected ResponseTool to have Strict: true")
	}
}

// TestStructuredOutputs_ResponseToolStrictDisabled tests disabling strict mode
func TestStructuredOutputs_ResponseToolStrictDisabled(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		WithStrictMode(false).
		Build()

	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	agent.AddTool(tool)

	responseTools := agent.buildResponseTools()
	if len(responseTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(responseTools))
	}

	if responseTools[0].Strict {
		t.Error("expected ResponseTool to have Strict: false")
	}
}

// TestStructuredOutputs_StructSchema tests struct-based schemas with strict mode
func TestStructuredOutputs_StructSchema(t *testing.T) {
	type TestStruct struct {
		Name     string `json:"name" required:"true" desc:"User name"`
		Age      int    `json:"age" required:"true"`
		Email    string `json:"email" desc:"Optional email"`
		Nickname string `json:"nickname,omitempty"`
	}

	schema, err := SchemaFromStruct(TestStruct{})
	if err != nil {
		t.Fatalf("failed to generate schema: %v", err)
	}

	// Check additionalProperties: false
	if schema["additionalProperties"] != false {
		t.Error("expected struct schema to have additionalProperties: false")
	}

	// Check all fields are in required (even optional ones)
	required := schema["required"].([]string)
	if len(required) != 4 {
		t.Errorf("expected 4 fields in required array, got %d", len(required))
	}

	// Check optional fields have anyOf with null
	props := schema["properties"].(map[string]any)
	emailSchema := props["email"].(map[string]any)
	if _, hasAnyOf := emailSchema["anyOf"]; !hasAnyOf {
		t.Error("expected optional email field to have anyOf")
	}

	nicknameSchema := props["nickname"].(map[string]any)
	if _, hasAnyOf := nicknameSchema["anyOf"]; !hasAnyOf {
		t.Error("expected optional nickname field to have anyOf")
	}
}

// TestStructuredOutputs_JSONValidation tests that the schema is valid JSON
func TestStructuredOutputs_JSONValidation(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("name", String().Required()).
		WithParameter("tags", Array("string").Optional()).
		WithParameter("metadata", Object().
			WithProperty("key", String().Required()).
			WithProperty("value", String().Optional()),
		).
		Build()

	// Marshal and unmarshal to ensure valid JSON
	data, err := json.Marshal(tool.parameters)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal schema: %v", err)
	}

	// Verify key fields are present
	if result["type"] != "object" {
		t.Error("expected type to be object")
	}
	if result["additionalProperties"] != false {
		t.Error("expected additionalProperties to be false")
	}
}

// TestStructuredOutputs_EnumValues tests enum handling in strict mode
func TestStructuredOutputs_EnumValues(t *testing.T) {
	tool := NewTool("test_tool").
		WithParameter("status", String().
			Required().
			WithEnum("open", "closed", "pending")).
		Build()

	props := tool.parameters["properties"].(map[string]any)
	statusSchema := props["status"].(map[string]any)

	enum, ok := statusSchema["enum"].([]string)
	if !ok {
		t.Fatal("expected enum to be []string")
	}

	if len(enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(enum))
	}

	// Check enum values are present
	enumMap := make(map[string]bool)
	for _, val := range enum {
		enumMap[val] = true
	}

	if !enumMap["open"] || !enumMap["closed"] || !enumMap["pending"] {
		t.Error("expected all enum values to be present")
	}
}

// TestStructuredOutputs_NewStructTool tests NewStructTool with strict mode
func TestStructuredOutputs_NewStructTool(t *testing.T) {
	type SearchArgs struct {
		Query string   `json:"query" required:"true" desc:"Search query"`
		Tags  []string `json:"tags" desc:"Filter tags"`
	}

	handler := func(ctx context.Context, args SearchArgs) (any, error) {
		return map[string]any{"results": 5}, nil
	}

	toolBuilder, err := NewStructTool("search", handler)
	if err != nil {
		t.Fatalf("failed to create struct tool: %v", err)
	}

	tool := toolBuilder.Build()

	if !tool.strict {
		t.Error("expected struct tool to have strict mode enabled")
	}

	// Check schema has required fields
	if tool.parameters["additionalProperties"] != false {
		t.Error("expected struct tool schema to have additionalProperties: false")
	}
}
