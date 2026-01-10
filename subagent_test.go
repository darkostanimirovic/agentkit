package agentkit

import (
	"context"
	"testing"
)

func TestNewSubAgentTool(t *testing.T) {
	sub, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     NewMockLLM().WithFinalResponse("sub done"),
		StreamResponses: false,
		Logging:         &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create sub-agent: %v", err)
	}

	tool, err := NewSubAgentTool(SubAgentConfig{Name: "sub"}, sub)
	if err != nil {
		t.Fatalf("failed to create sub-agent tool: %v", err)
	}

	result, err := tool.Execute(context.Background(), `{"input":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result is now returned as a string directly (unless IncludeTrace is true)
	res, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	if res != "sub done" {
		t.Fatalf("expected response 'sub done', got %v", res)
	}
}

func TestNewSubAgentTool_MissingInput(t *testing.T) {
	sub, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     NewMockLLM().WithFinalResponse("sub done"),
		StreamResponses: false,
		Logging:         &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create sub-agent: %v", err)
	}

	tool, err := NewSubAgentTool(SubAgentConfig{Name: "sub"}, sub)
	if err != nil {
		t.Fatalf("failed to create sub-agent tool: %v", err)
	}

	_, err = tool.Execute(context.Background(), `{}`)
	if err == nil {
		t.Fatal("expected error for missing input")
	}
}

func TestNewSubAgentTool_WithTrace(t *testing.T) {
	sub, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     NewMockLLM().WithFinalResponse("traced response"),
		StreamResponses: false,
		Logging:         &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create sub-agent: %v", err)
	}

	tool, err := NewSubAgentTool(SubAgentConfig{
		Name:         "sub_with_trace",
		IncludeTrace: true,
	}, sub)
	if err != nil {
		t.Fatalf("failed to create sub-agent tool: %v", err)
	}

	result, err := tool.Execute(context.Background(), `{"input":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With IncludeTrace=true, result is a map
	res, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result with trace, got %T", result)
	}
	if res["response"] != "traced response" {
		t.Fatalf("expected response 'traced response', got %v", res["response"])
	}
	if _, hasTrace := res["trace"]; !hasTrace {
		t.Fatal("expected trace field in result")
	}
}
