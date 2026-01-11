package agentkit

import (
	"context"
	"testing"
	"time"
)

func TestParallelToolExecution(t *testing.T) {
	started := make(chan string, 2)
	gate := make(chan struct{})

	makeHandler := func(name string) ToolHandler {
		return func(ctx context.Context, args map[string]any) (any, error) {
			started <- name
			<-gate
			return map[string]any{"ok": true}, nil
		}
	}

	tool1 := NewTool("tool1").WithHandler(makeHandler("tool1")).Build()
	tool2 := NewTool("tool2").WithHandler(makeHandler("tool2")).Build()

	mock := NewMockLLM().
		WithResponse("calling tools", []MockToolCall{{Name: "tool1", Args: map[string]any{}}, {Name: "tool2", Args: map[string]any{}}}).
		WithFinalResponse("done")

	agent, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     mock,
		StreamResponses: false,
		ParallelToolExecution: &ParallelConfig{
			Enabled:       true,
			MaxConcurrent: 2,
		},
		Logging: &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	agent.AddTool(tool1)
	agent.AddTool(tool2)

	done := make(chan struct{})
	go func() {
		for range agent.Run(context.Background(), "run") {
		}
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected tools to start in parallel")
		}
	}

	close(gate)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("agent did not finish")
	}
}

func TestParallelToolExecution_SerialBarrier(t *testing.T) {
	started := make(chan string, 2)
	gate := make(chan struct{})

	makeHandler := func(name string) ToolHandler {
		return func(ctx context.Context, args map[string]any) (any, error) {
			started <- name
			<-gate
			return map[string]any{"ok": true}, nil
		}
	}

	tool1 := NewTool("tool1").WithConcurrency(ConcurrencySerial).WithHandler(makeHandler("tool1")).Build()
	tool2 := NewTool("tool2").WithHandler(makeHandler("tool2")).Build()

	mock := NewMockLLM().
		WithResponse("calling tools", []MockToolCall{{Name: "tool1", Args: map[string]any{}}, {Name: "tool2", Args: map[string]any{}}}).
		WithFinalResponse("done")

	agent, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     mock,
		StreamResponses: false,
		ParallelToolExecution: &ParallelConfig{
			Enabled:       true,
			MaxConcurrent: 2,
		},
		Logging: &LoggingConfig{LogPrompts: false},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	agent.AddTool(tool1)
	agent.AddTool(tool2)

	done := make(chan struct{})
	go func() {
		for range agent.Run(context.Background(), "run") {
		}
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected first tool to start")
	}

	select {
	case <-started:
		t.Fatal("expected serial tool to block parallel execution")
	case <-time.After(100 * time.Millisecond):
	}

	close(gate)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("agent did not finish")
	}
}
