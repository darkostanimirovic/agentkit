package agentkit

import (
	"context"
	"testing"
)

func TestAgent_Use(t *testing.T) {
	agent, err := New(Config{
		Model:       "test-model",
		LLMProvider: NewMockLLM().WithFinalResponse("test"),
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Test adding middleware
	middleware := &testMiddleware{}
	agent.Use(middleware)

	// Run the agent to trigger middleware
	ctx := context.Background()
	events := agent.Run(ctx, "test input")
	for range events {
	}

	// Verify middleware was called
	if middleware.agentStartCalled == 0 {
		t.Error("Expected middleware OnAgentStart to be called")
	}
	if middleware.agentCompleteCalled == 0 {
		t.Error("Expected middleware OnAgentComplete to be called")
	}
}

func TestAgent_AsTool(t *testing.T) {
	agent, err := New(Config{
		Model:       "test-model",
		LLMProvider: NewMockLLM().WithFinalResponse("agent response"),
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Create tool from agent
	tool := agent.AsTool("sub_agent", "A helpful sub agent")

	// Tool.Execute expects args as a map
	ctx := context.Background()
	result, err := tool.handler(ctx, map[string]any{"input": "test task"})
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatal("Expected result to be a string")
	}

	if resultStr == "" {
		t.Error("Expected non-empty result")
	}
}

func TestAgent_HandleIterationError(t *testing.T) {
	// Create agent with mock that will fail
	failingProvider := NewMockLLM()
	// Don't add any responses, so it will fail

	agent, err := New(Config{
		Model:       "test-model",
		LLMProvider: failingProvider,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	events := agent.Run(ctx, "test")

	// Collect events and check for error
	var hasError bool
	for event := range events {
		if event.Type == EventTypeError {
			hasError = true
		}
	}

	if !hasError {
		t.Error("Expected error event when provider fails")
	}
}

func TestAgent_GetConversationID(t *testing.T) {
	ctx := context.Background()

	// Without conversation ID
	id, ok := GetConversationID(ctx)
	if ok || id != "" {
		t.Error("Expected no conversation ID")
	}

	// With conversation ID
	ctx = WithConversation(ctx, "test-conv-123")
	id, ok = GetConversationID(ctx)
	if !ok {
		t.Error("Expected conversation ID to be present")
	}
	if id != "test-conv-123" {
		t.Errorf("Expected 'test-conv-123', got '%s'", id)
	}
}

func TestAgent_WithSpanID(t *testing.T) {
	ctx := context.Background()

	// Without span ID
	id, ok := GetSpanID(ctx)
	if ok || id != "" {
		t.Error("Expected no span ID")
	}

	// With span ID
	ctx = WithSpanID(ctx, "span-456")
	id, ok = GetSpanID(ctx)
	if !ok {
		t.Error("Expected span ID to be present")
	}
	if id != "span-456" {
		t.Errorf("Expected 'span-456', got '%s'", id)
	}
}

// testMiddleware for testing middleware functionality
type testMiddleware struct {
	agentStartCalled    int
	agentCompleteCalled int
	toolStartCalled     int
	toolCompleteCalled  int
	llmCallCalled       int
	llmResponseCalled   int
}

func (m *testMiddleware) OnAgentStart(ctx context.Context, input string) context.Context {
	m.agentStartCalled++
	return ctx
}

func (m *testMiddleware) OnAgentComplete(ctx context.Context, output string, err error) {
	m.agentCompleteCalled++
}

func (m *testMiddleware) OnToolStart(ctx context.Context, tool string, args any) context.Context {
	m.toolStartCalled++
	return ctx
}

func (m *testMiddleware) OnToolComplete(ctx context.Context, tool string, result any, err error) {
	m.toolCompleteCalled++
}

func (m *testMiddleware) OnLLMCall(ctx context.Context, req any) context.Context {
	m.llmCallCalled++
	return ctx
}

func (m *testMiddleware) OnLLMResponse(ctx context.Context, resp any, err error) {
	m.llmResponseCalled++
}
