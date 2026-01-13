package agentkit

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/agentkit/providers"
)

func TestNewMockLLM(t *testing.T) {
	mock := NewMockLLM().
		WithResponse("thinking", []ToolCall{
			{Name: "search", Arguments: map[string]any{"query": "test"}},
		}).
		WithFinalResponse("done")

	agent, err := New(Config{
		Model:           "test-model",
		LLMProvider:     mock,
		StreamResponses: false,
		Logging:         &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add a simple tool
	tool := NewTool("search").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"result": "found"}, nil
		}).
		Build()
	agent.AddTool(tool)

	// Run agent
	ctx := context.Background()
	events := agent.Run(ctx, "search for test")

	// Collect events
	var seen int
	for range events {
		seen++
	}

	if seen == 0 {
		t.Fatal("expected events to be emitted")
	}
}

func TestNewMockLLM_WithStream(t *testing.T) {
	mock := NewMockLLM().
		WithStream([]providers.StreamChunk{
			{Content: "hello", IsComplete: false},
			{Content: " world", IsComplete: true, FinishReason: providers.FinishReasonStop},
		})

	agent, err := New(Config{
		Model:           "test-model",
		LLMProvider:     mock,
		StreamResponses: true,
		Logging:         &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()
	events := agent.Run(ctx, "say hello")

	// Collect events
	var seen int
	for event := range events {
		if event.Type == EventTypeResponseChunk {
			seen++
		}
	}

	if seen == 0 {
		t.Fatal("expected streaming chunks to be emitted")
	}
}
