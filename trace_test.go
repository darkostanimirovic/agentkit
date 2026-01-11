package agentkit


import (
	"context"
	"testing"
)

func TestTraceIDPropagation(t *testing.T) {
	mock := NewMockLLM().WithFinalResponse("done")

	agent, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     mock,
		StreamResponses: false,
		Logging: &LoggingConfig{
			LogPrompts: false,
		},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := WithTraceID(context.Background(), "trace-123")
	events := agent.Run(ctx, "hello")

	seen := 0
	for event := range events {
		seen++
		if event.TraceID != "trace-123" {
			t.Fatalf("expected trace_id trace-123, got %q", event.TraceID)
		}
	}

	if seen == 0 {
		t.Fatal("expected events to be emitted")
	}
}
