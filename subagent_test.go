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

	res := result.(map[string]any)
	if res["response"] != "sub done" {
		t.Fatalf("expected response 'sub done', got %v", res["response"])
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
