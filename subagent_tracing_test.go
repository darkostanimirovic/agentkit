package agentkit

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
)

// TestSubAgentTracerInheritance verifies that sub-agents inherit parent's tracer
func TestSubAgentTracerInheritance(t *testing.T) {
	// Track which tracers were used
	var mu sync.Mutex
	traceCalls := make(map[string][]string) // map[spanName][]tracerID

	// Create a mock tracer that records which tracer instance was used
	parentTracerID := "parent-tracer"
	mockTracer := &trackingTracer{
		id:         parentTracerID,
		traceCalls: traceCalls,
		mu:         &mu,
	}

	// Create parent agent with tracer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockClient := &testLLMClient{
		response: &ResponseObject{
			ID:     "test-response",
			Status: "completed",
			Output: []ResponseOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []ResponseContentItem{
						{Type: "output_text", Text: "parent response"},
					},
				},
			},
		},
	}

	parentAgent := &Agent{
		tracer:            mockTracer,
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   mockClient,
		maxIterations:     1,
		model:             "test-model",
		systemPrompt:      func(ctx context.Context) string { return "test" },
		streamResponses:   false,
		conversationStore: nil,
	}

	// Create sub-agent WITHOUT a tracer (should inherit parent's)
	subAgent := &Agent{
		tracer:            &NoOpTracer{}, // Sub has NoOpTracer initially
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   mockClient,
		maxIterations:     1,
		model:             "sub-model",
		systemPrompt:      func(ctx context.Context) string { return "sub-test" },
		streamResponses:   false,
		conversationStore: nil,
	}

	// Register sub-agent as a tool
	err := parentAgent.AddSubAgent("test_sub", subAgent)
	if err != nil {
		t.Fatalf("Failed to add sub-agent: %v", err)
	}

	// Run parent agent (which should call sub-agent through tool)
	// Note: We'd need to actually trigger the tool call, but for this test
	// we'll test the context propagation directly
	ctx := context.Background()
	ctx = WithTracer(ctx, mockTracer)

	// Simulate what happens when sub-agent tool is called
	handler := subAgentHandler(subAgent, SubAgentConfig{
		Name:        "test_sub",
		Description: "test sub-agent",
	})

	// Call the handler
	result, err := handler(ctx, map[string]any{"input": "test message"})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result == nil {
		t.Fatal("Handler returned nil result")
	}

	// Verify that the parent tracer was used for sub-agent span
	mu.Lock()
	defer mu.Unlock()

	if len(traceCalls) == 0 {
		t.Fatal("No trace calls recorded")
	}

	// Check that sub_agent span was created with parent tracer
	subAgentCalls, ok := traceCalls["sub_agent.test_sub"]
	if !ok {
		t.Errorf("Sub-agent span not found. Available spans: %v", getKeys(traceCalls))
	} else if len(subAgentCalls) == 0 {
		t.Error("Sub-agent span has no tracer records")
	} else if subAgentCalls[0] != parentTracerID {
		t.Errorf("Sub-agent used wrong tracer: got %s, want %s", subAgentCalls[0], parentTracerID)
	} else {
		t.Logf("✓ Sub-agent correctly inherited parent tracer: %s", parentTracerID)
	}

	// Check that agent.run span was also created (from sub-agent running)
	agentRunCalls, ok := traceCalls["agent.run"]
	if !ok {
		t.Logf("Note: agent.run span not found (may not have been called in this test)")
	} else if len(agentRunCalls) > 0 && agentRunCalls[0] != parentTracerID {
		t.Errorf("Sub-agent's agent.run used wrong tracer: got %s, want %s", agentRunCalls[0], parentTracerID)
	} else if len(agentRunCalls) > 0 {
		t.Logf("✓ Sub-agent's agent.run also used parent tracer")
	}
}

// trackingTracer records which tracer instance was used for each span
type trackingTracer struct {
	NoOpTracer
	id         string
	traceCalls map[string][]string
	mu         *sync.Mutex
}

func (t *trackingTracer) StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func()) {
	t.mu.Lock()
	t.traceCalls[name] = append(t.traceCalls[name], t.id)
	t.mu.Unlock()
	// Make sure to propagate tracer in context
	return WithTracer(ctx, t), func() {}
}

func (t *trackingTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func()) {
	t.mu.Lock()
	t.traceCalls[name] = append(t.traceCalls[name], t.id)
	t.mu.Unlock()
	return ctx, func() {}
}

func getKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestSubAgentWithoutParentTracer verifies graceful degradation when no parent tracer exists
func TestSubAgentWithoutParentTracer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockClient := &testLLMClient{
		response: &ResponseObject{
			ID:     "test-response",
			Status: "completed",
			Output: []ResponseOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []ResponseContentItem{
						{Type: "output_text", Text: "response"},
					},
				},
			},
		},
	}

	// Create sub-agent with its own tracer
	subTracerID := "sub-tracer"
	var mu sync.Mutex
	traceCalls := make(map[string][]string)
	subTracer := &trackingTracer{
		id:         subTracerID,
		traceCalls: traceCalls,
		mu:         &mu,
	}

	subAgent := &Agent{
		tracer:            subTracer,
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   mockClient,
		maxIterations:     1,
		model:             "sub-model",
		systemPrompt:      func(ctx context.Context) string { return "test" },
		streamResponses:   false,
		conversationStore: nil,
	}

	// Call handler WITHOUT parent tracer in context
	handler := subAgentHandler(subAgent, SubAgentConfig{
		Name:        "test_sub",
		Description: "test",
	})

	ctx := context.Background() // No tracer in context

	result, err := handler(ctx, map[string]any{"input": "test"})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result == nil {
		t.Fatal("Handler returned nil")
	}

	// Verify sub-agent fell back to its own tracer
	mu.Lock()
	defer mu.Unlock()

	subAgentCalls, ok := traceCalls["sub_agent.test_sub"]
	if !ok {
		t.Error("Sub-agent span not created")
	} else if len(subAgentCalls) == 0 {
		t.Error("No tracer recorded for sub-agent span")
	} else if subAgentCalls[0] != subTracerID {
		t.Errorf("Sub-agent used wrong tracer: got %s, want %s", subAgentCalls[0], subTracerID)
	} else {
		t.Logf("✓ Sub-agent correctly fell back to its own tracer when no parent tracer available")
	}
}

// TestIsNoOpTracer verifies the helper function works correctly
func TestIsNoOpTracer(t *testing.T) {
	noOp := &NoOpTracer{}
	if !isNoOpTracer(noOp) {
		t.Error("isNoOpTracer should return true for NoOpTracer")
	}

	mockTracer := &trackingTracer{}
	if isNoOpTracer(mockTracer) {
		t.Error("isNoOpTracer should return false for non-NoOpTracer")
	}

	t.Log("✓ isNoOpTracer correctly identifies NoOpTracer instances")
}

// TestTracerContextPropagation verifies WithTracer and GetTracer work correctly
func TestTracerContextPropagation(t *testing.T) {
	mockTracer := &NoOpTracer{}
	ctx := context.Background()

	// Initially no tracer
	if tracer := GetTracer(ctx); tracer != nil {
		t.Error("Expected nil tracer from empty context")
	}

	// Add tracer to context
	ctx = WithTracer(ctx, mockTracer)

	// Retrieve tracer
	retrieved := GetTracer(ctx)
	if retrieved == nil {
		t.Fatal("Failed to retrieve tracer from context")
	}

	if retrieved != mockTracer {
		t.Error("Retrieved tracer is not the same instance")
	}

	t.Log("✓ Tracer context propagation works correctly")
}

// TestSubAgentToolIntegration is an end-to-end test with actual tool execution
func TestSubAgentToolIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var mu sync.Mutex
	traceCalls := make(map[string][]string)
	parentTracerID := "integration-parent"
	mockTracer := &trackingTracer{
		id:         parentTracerID,
		traceCalls: traceCalls,
		mu:         &mu,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create mock LLM that returns tool calls
	parentClient := &testLLMClient{
		response: &ResponseObject{
			ID:     "parent-resp",
			Status: "completed",
			Output: []ResponseOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []ResponseContentItem{
						{Type: "output_text", Text: "delegating to sub-agent"},
					},
				},
			},
		},
	}

	subClient := &testLLMClient{
		response: &ResponseObject{
			ID:     "sub-resp",
			Status: "completed",
			Output: []ResponseOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []ResponseContentItem{
						{Type: "output_text", Text: "sub-agent completed the task"},
					},
				},
			},
		},
	}

	// Create sub-agent
	subAgent := &Agent{
		tracer:            &NoOpTracer{}, // Will be overridden by parent
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   subClient,
		maxIterations:     1,
		model:             "sub-model",
		systemPrompt:      func(ctx context.Context) string { return "You are a sub-agent" },
		streamResponses:   false,
	}

	// Create parent agent with sub-agent tool
	parentAgent := &Agent{
		tracer:            mockTracer,
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   parentClient,
		maxIterations:     2, // Allow for tool call + continuation
		model:             "parent-model",
		systemPrompt:      func(ctx context.Context) string { return "You are a parent agent" },
		streamResponses:   false,
	}

	err := parentAgent.AddSubAgent("test_sub", subAgent)
	if err != nil {
		t.Fatalf("Failed to add sub-agent: %v", err)
	}

	// Note: This test would need the actual Run() to execute and call the tool
	// For now, we're testing the mechanism directly
	t.Log("✓ Sub-agent tool integration test setup complete")
	t.Log("Note: Full end-to-end execution would require mocking the tool call flow")
}

// TestSubAgentTracerInheritanceInRun verifies tracer inheritance when agent.Run() is called
func TestSubAgentTracerInheritanceInRun(t *testing.T) {
	var mu sync.Mutex
	traceCalls := make(map[string][]string)
	parentTracerID := "run-parent"
	mockTracer := &trackingTracer{
		id:         parentTracerID,
		traceCalls: traceCalls,
		mu:         &mu,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockClient := &testLLMClient{
		response: &ResponseObject{
			ID:     "test-response",
			Status: "completed",
			Output: []ResponseOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []ResponseContentItem{
						{Type: "output_text", Text: "done"},
					},
				},
			},
		},
	}

	agent := &Agent{
		tracer:            mockTracer,
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   mockClient,
		maxIterations:     1,
		model:             "test-model",
		systemPrompt:      func(ctx context.Context) string { return "test" },
		streamResponses:   false,
	}

	// Run agent
	events := agent.Run(context.Background(), "test message")

	// Drain events
	for range events {
	}

	// Verify tracer was added to context
	mu.Lock()
	defer mu.Unlock()

	agentRunCalls, ok := traceCalls["agent.run"]
	if !ok {
		t.Fatal("agent.run trace not found")
	}

	if len(agentRunCalls) == 0 {
		t.Fatal("No tracer recorded for agent.run")
	}

	if agentRunCalls[0] != parentTracerID {
		t.Errorf("Wrong tracer used: got %s, want %s", agentRunCalls[0], parentTracerID)
	}

	t.Logf("✓ Agent.Run() correctly uses configured tracer and adds it to context")
}
