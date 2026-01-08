package agentkit

import (
	"context"
	"sync"
	"testing"
)

type recordingMiddleware struct {
	BaseMiddleware
	mu             sync.Mutex
	agentStarts    int
	agentCompletes int
	llmCalls       int
	llmResponses   int
	toolStarts     int
	toolCompletes  int
}

func (m *recordingMiddleware) OnAgentStart(ctx context.Context, _ string) context.Context {
	m.mu.Lock()
	m.agentStarts++
	m.mu.Unlock()
	return ctx
}

func (m *recordingMiddleware) OnAgentComplete(context.Context, string, error) {
	m.mu.Lock()
	m.agentCompletes++
	m.mu.Unlock()
}

func (m *recordingMiddleware) OnLLMCall(ctx context.Context, _ any) context.Context {
	m.mu.Lock()
	m.llmCalls++
	m.mu.Unlock()
	return ctx
}

func (m *recordingMiddleware) OnLLMResponse(context.Context, any, error) {
	m.mu.Lock()
	m.llmResponses++
	m.mu.Unlock()
}

func (m *recordingMiddleware) OnToolStart(ctx context.Context, _ string, _ any) context.Context {
	m.mu.Lock()
	m.toolStarts++
	m.mu.Unlock()
	return ctx
}

func (m *recordingMiddleware) OnToolComplete(context.Context, string, any, error) {
	m.mu.Lock()
	m.toolCompletes++
	m.mu.Unlock()
}

func TestMiddlewareHooks(t *testing.T) {
	mock := NewMockLLM().
		WithResponse("calling tool", []ToolCall{{Name: "echo", Args: map[string]any{"message": "hi"}}}).
		WithFinalResponse("done")

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

	tool := NewTool("echo").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"echo": args["message"]}, nil
		}).
		Build()
	agent.AddTool(tool)

	mw := &recordingMiddleware{}
	agent.Use(mw)

	for range agent.Run(context.Background(), "hello") {
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	if mw.agentStarts != 1 || mw.agentCompletes != 1 {
		t.Fatalf("expected agent start/complete 1/1, got %d/%d", mw.agentStarts, mw.agentCompletes)
	}
	if mw.llmCalls != 2 || mw.llmResponses != 2 {
		t.Fatalf("expected llm call/response 2/2, got %d/%d", mw.llmCalls, mw.llmResponses)
	}
	if mw.toolStarts != 1 || mw.toolCompletes != 1 {
		t.Fatalf("expected tool start/complete 1/1, got %d/%d", mw.toolStarts, mw.toolCompletes)
	}
}
