package agentkit

import (
	"context"
	"testing"
)

const testUserID = "user123"

// Integration tests for the agentkit framework
// These tests verify end-to-end scenarios

func TestToolIntegration(t *testing.T) {
	// Test complete tool lifecycle: build -> register -> execute
	callCount := 0
	handler := func(ctx context.Context, args map[string]any) (any, error) {
		callCount++
		name := args["name"].(string)
		return map[string]any{
			"success": true,
			"message": "Hello, " + name,
		}, nil
	}

	tool := NewTool("greet").
		WithDescription("Greets a user").
		WithParameter("name", String().Required().WithDescription("User name")).
		WithParameter("title", String().Optional().WithDescription("User title")).
		WithHandler(handler).
		Build()

	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	agent.AddTool(tool)

	// Verify tool is registered
	if len(agent.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(agent.tools))
	}

	// Execute tool
	result, err := tool.Execute(context.Background(), `{"name":"Alice","title":"Dr."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected handler to be called once, got %d", callCount)
	}

	resultMap := result.(map[string]any)
	if resultMap["message"] != "Hello, Alice" {
		t.Errorf("unexpected message: %v", resultMap["message"])
	}
}

func TestContextWithToolExecution(t *testing.T) {
	// Test that context deps are passed to tool handlers
	type TestDeps struct {
		UserID string
	}

	handler := func(ctx context.Context, args map[string]any) (any, error) {
		deps := MustGetDeps[TestDeps](ctx)
		return map[string]any{
			"user_id": deps.UserID,
			"message": args["message"],
		}, nil
	}

	tool := NewTool("process").
		WithParameter("message", String().Required()).
		WithHandler(handler).
		Build()

	ctx := WithDeps(context.Background(), TestDeps{UserID: testUserID})

	result, err := tool.Execute(ctx, `{"message":"test"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["user_id"] != testUserID {
		t.Errorf("expected user_id user123, got %v", resultMap["user_id"])
	}
}

func TestMultipleToolsIntegration(t *testing.T) {
	// Test agent with multiple tools
	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add multiple tools
	tool1 := NewTool("add").
		WithDescription("Add two numbers").
		WithParameter("a", String().Required()).
		WithParameter("b", String().Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"result": "sum"}, nil
		}).
		Build()

	tool2 := NewTool("multiply").
		WithDescription("Multiply two numbers").
		WithParameter("a", String().Required()).
		WithParameter("b", String().Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"result": "product"}, nil
		}).
		Build()

	agent.AddTool(tool1)
	agent.AddTool(tool2)

	if len(agent.tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(agent.tools))
	}

	// Verify both tools exist and have correct names
	if _, exists := agent.tools["add"]; !exists {
		t.Error("expected 'add' tool to be registered")
	}

	if _, exists := agent.tools["multiply"]; !exists {
		t.Error("expected 'multiply' tool to be registered")
	}

	// Test tool execution
	result1, err := tool1.Execute(context.Background(), `{"a":"1","b":"2"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result2, err := tool2.Execute(context.Background(), `{"a":"3","b":"4"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1.(map[string]any)["result"] != "sum" {
		t.Error("tool1 returned wrong result")
	}

	if result2.(map[string]any)["result"] != "product" {
		t.Error("tool2 returned wrong result")
	}
}

func TestEventStreamingSimulation(t *testing.T) {
	// Simulate event streaming without OpenAI
	events := make(chan Event, 10)

	go func() {
		events <- ThinkingChunk("Analyzing request...")
		events <- ThinkingChunk("Planning actions...")
		events <- ActionDetected("assign_team", "call_123")
		events <- ActionResult("assign_team", map[string]any{"success": true})
		events <- FinalOutput("Complete", "Task assigned successfully")
		close(events)
	}()

	var received = make([]Event, 0, 5)
	for event := range events {
		received = append(received, event)
	}

	if len(received) != 5 {
		t.Errorf("expected 5 events, got %d", len(received))
	}

	// Verify event types in order
	expectedTypes := []EventType{
		EventTypeThinkingChunk,
		EventTypeThinkingChunk,
		EventTypeActionDetected,
		EventTypeActionResult,
		EventTypeFinalOutput,
	}

	for i, event := range received {
		if event.Type != expectedTypes[i] {
			t.Errorf("event %d: expected type %s, got %s", i, expectedTypes[i], event.Type)
		}
	}
}

func TestSystemPromptIntegration(t *testing.T) {
	// Test system prompt with context
	type AppContext struct {
		AppName string
		Version string
	}

	promptCalled := false
	systemPrompt := func(ctx context.Context) string {
		promptCalled = true
		deps := MustGetDeps[AppContext](ctx)
		return "You are " + deps.AppName + " v" + deps.Version
	}

	agent, err := New(Config{
		APIKey:       "test-key",
		Model:        "gpt-4",
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := WithDeps(context.Background(), AppContext{
		AppName: "InboxAgent",
		Version: "1.0",
	})

	prompt := agent.systemPrompt(ctx)
	if !promptCalled {
		t.Error("expected system prompt to be called")
	}

	if prompt != "You are InboxAgent v1.0" {
		t.Errorf("unexpected prompt: %s", prompt)
	}
}
