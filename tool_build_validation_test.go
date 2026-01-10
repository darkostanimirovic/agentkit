package agentkit

import (
	"testing"
)

func TestBuild_AddsAdditionalPropertiesFalse(t *testing.T) {
	tests := []struct {
		name                 string
		builder              func() *ToolBuilder
		expectAdditionalProp bool
	}{
		{
			name: "WithParameter automatically adds additionalProperties",
			builder: func() *ToolBuilder {
				return NewTool("test").
					WithParameter("name", String().Required())
			},
			expectAdditionalProp: true,
		},
		{
			name: "WithRawParameters without additionalProperties gets it added",
			builder: func() *ToolBuilder {
				return NewTool("test").
					WithRawParameters(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
					})
			},
			expectAdditionalProp: true,
		},
		{
			name: "WithJSONSchema without additionalProperties gets it added",
			builder: func() *ToolBuilder {
				return NewTool("test").
					WithJSONSchema(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{"type": "string"},
						},
					})
			},
			expectAdditionalProp: true,
		},
		{
			name: "Existing additionalProperties is preserved",
			builder: func() *ToolBuilder {
				return NewTool("test").
					WithRawParameters(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
						"additionalProperties": true, // Explicitly set to true
					})
			},
			expectAdditionalProp: false, // Should preserve user's choice
		},
		{
			name: "Strict mode disabled does not add additionalProperties",
			builder: func() *ToolBuilder {
				return NewTool("test").
					WithRawParameters(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
						},
					}).
					WithStrictMode(false)
			},
			expectAdditionalProp: false,
		},
		{
			name: "Schema with properties but no type gets additionalProperties",
			builder: func() *ToolBuilder {
				return NewTool("test").
					WithRawParameters(map[string]any{
						"properties": map[string]any{
							"field": map[string]any{"type": "string"},
						},
					})
			},
			expectAdditionalProp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.builder().Build()

			additionalProps, hasKey := tool.parameters["additionalProperties"]
			if tt.expectAdditionalProp {
				if !hasKey {
					t.Error("Expected additionalProperties key to be present")
				} else if additionalProps != false {
					t.Errorf("Expected additionalProperties to be false, got %v", additionalProps)
				}
			}

			// For the case where we explicitly set additionalProperties to true
			if tt.name == "Existing additionalProperties is preserved" {
				if additionalProps != true {
					t.Errorf("Expected additionalProperties to be true (preserved), got %v", additionalProps)
				}
			}

			// For strict mode disabled, additionalProperties should not be added
			if tt.name == "Strict mode disabled does not add additionalProperties" && hasKey && additionalProps == false {
				t.Error("Should not add additionalProperties when strict mode is disabled")
			}
		})
	}
}

func TestBuild_ContextSizeToolExample(t *testing.T) {
	// Simulate a tool like "check_context_size" that might use raw parameters
	tool := NewTool("check_context_size").
		WithDescription("Check context size").
		WithRawParameters(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "Input text to check",
				},
			},
			"required": []string{"input"},
		}).
		Build()

	// Verify additionalProperties was added
	if additionalProps, ok := tool.parameters["additionalProperties"]; !ok {
		t.Error("Expected additionalProperties to be set for strict mode tool")
	} else if additionalProps != false {
		t.Errorf("Expected additionalProperties to be false, got %v", additionalProps)
	}

	// Verify the schema is valid for OpenAI
	if tool.parameters["type"] != "object" {
		t.Error("Expected type to be object")
	}
	if _, ok := tool.parameters["properties"]; !ok {
		t.Error("Expected properties to be set")
	}
}

func TestBuild_EmptyParametersNoOp(t *testing.T) {
	// Tools with no parameters should get proper strict mode schema
	tool := NewTool("no_params").
		WithDescription("A tool with no parameters").
		Build()

	// Should have proper empty object schema for strict mode
	if tool.parameters["type"] != "object" {
		t.Errorf("Expected type to be 'object', got %v", tool.parameters["type"])
	}
	
	if props, ok := tool.parameters["properties"]; !ok {
		t.Error("Expected properties field to be present")
	} else if propsMap, ok := props.(map[string]any); !ok || len(propsMap) != 0 {
		t.Errorf("Expected empty properties map, got %v", props)
	}
	
	if additionalProps, ok := tool.parameters["additionalProperties"]; !ok {
		t.Error("Expected additionalProperties to be set")
	} else if additionalProps != false {
		t.Errorf("Expected additionalProperties to be false, got %v", additionalProps)
	}
}

func TestBuild_MinimalSchema(t *testing.T) {
	// Tool with minimal schema should get type and properties added
	tool := NewTool("minimal").
		WithRawParameters(map[string]any{
			"required": []string{},
		}).
		Build()

	if tool.parameters["type"] != "object" {
		t.Errorf("Expected type to be 'object', got %v", tool.parameters["type"])
	}
	
	if _, ok := tool.parameters["properties"]; !ok {
		t.Error("Expected properties to be added")
	}
	
	if additionalProps := tool.parameters["additionalProperties"]; additionalProps != false {
		t.Errorf("Expected additionalProperties to be false, got %v", additionalProps)
	}
}
