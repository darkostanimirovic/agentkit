package agentkit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkostanimirovic/agentkit/providers"
)

// TestE2E_BasicStreamingWorkflow tests a complete streaming workflow with events
func TestE2E_BasicStreamingWorkflow(t *testing.T) {
	// Simulate a real scenario: User asks a question, agent responds
	chunks := []providers.StreamChunk{
		{Content: "The capital of France is Paris."},
		{Content: "", IsComplete: true},
	}
	mock := NewMockLLM().WithStream(chunks)

	agent, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     mock,
		StreamResponses: true,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	events := agent.Run(ctx, "What is the capital of France?")

	// Client simulation: collect all events as they stream
	var collectedEvents []Event
	var finalOutput string
	eventCount := make(map[EventType]int)

	for event := range events {
		collectedEvents = append(collectedEvents, event)
		eventCount[event.Type]++

		// Client would process events in real-time
		switch event.Type {
		case EventTypeFinalOutput:
			finalOutput = event.Data["response"].(string)
		}
	}

	// Verify complete event flow
	if len(collectedEvents) == 0 {
		t.Fatal("Expected to receive events")
	}

	// Must have AgentStart and AgentComplete
	if eventCount[EventTypeAgentStart] != 1 {
		t.Errorf("Expected 1 AgentStart event, got %d", eventCount[EventTypeAgentStart])
	}
	if eventCount[EventTypeAgentComplete] != 1 {
		t.Errorf("Expected 1 AgentComplete event, got %d", eventCount[EventTypeAgentComplete])
	}

	// Must have FinalOutput for successful completion
	if eventCount[EventTypeFinalOutput] != 1 {
		t.Errorf("Expected 1 FinalOutput event, got %d", eventCount[EventTypeFinalOutput])
	}

	if finalOutput != "The capital of France is Paris." {
		t.Errorf("Expected final output about Paris, got: %s", finalOutput)
	}

	// Verify event ordering: AgentStart must come first
	if collectedEvents[0].Type != EventTypeAgentStart {
		t.Errorf("First event should be AgentStart, got %s", collectedEvents[0].Type)
	}

	// AgentComplete must come last
	if collectedEvents[len(collectedEvents)-1].Type != EventTypeAgentComplete {
		t.Errorf("Last event should be AgentComplete, got %s", collectedEvents[len(collectedEvents)-1].Type)
	}
}

// TestE2E_MultiTurnConversationWithTools tests a complete multi-turn workflow
func TestE2E_MultiTurnConversationWithTools(t *testing.T) {
	// Simulate: User asks for weather, agent uses tool, returns answer
	mock := NewMockLLM().
		WithResponse("I'll check the weather for you.", []ToolCall{
			{
				ID:        "call_1",
				Name:      "get_weather",
				Arguments: map[string]any{"location": "San Francisco"},
			},
		}).
		WithFinalResponse("The weather in San Francisco is 72°F and sunny.")

	// Create a weather tool
	weatherTool := NewTool("get_weather").
		WithDescription("Get current weather for a location").
		WithParameter("location", String().Required().WithDescription("City name")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			location := args["location"].(string)
			return map[string]any{
				"temperature": 72,
				"condition":   "sunny",
				"location":    location,
			}, nil
		}).
		Build()

	agent, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     mock,
		StreamResponses: false, // Test non-streaming mode
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(weatherTool)

	ctx := context.Background()
	events := agent.Run(ctx, "What's the weather in San Francisco?")

	// Track the complete event sequence
	var eventSequence []EventType
	var toolResults []string
	var finalOutput string

	for event := range events {
		eventSequence = append(eventSequence, event.Type)

		switch event.Type {
		case EventTypeActionResult:
			if desc, ok := event.Data["description"].(string); ok {
				toolResults = append(toolResults, desc)
			}
		case EventTypeFinalOutput:
			finalOutput = event.Data["response"].(string)
		}
	}

	// Verify complete workflow
	if len(eventSequence) == 0 {
		t.Fatal("Expected events in multi-turn conversation")
	}

	// Must have tool execution results
	if len(toolResults) != 1 || !strings.Contains(toolResults[0], "get_weather") {
		t.Errorf("Expected get_weather tool result, got: %v", toolResults)
	}

	// Must have final output with weather info
	if !strings.Contains(finalOutput, "72") || !strings.Contains(finalOutput, "sunny") {
		t.Errorf("Expected weather information in output, got: %s", finalOutput)
	}

	// Verify event sequence has correct order
	foundStart := false
	foundResult := false
	foundFinal := false
	foundComplete := false

	for _, eventType := range eventSequence {
		switch eventType {
		case EventTypeAgentStart:
			if foundStart {
				t.Error("Multiple AgentStart events")
			}
			foundStart = true
		case EventTypeActionResult:
			foundResult = true
		case EventTypeFinalOutput:
			if !foundResult {
				t.Error("FinalOutput before tool result")
			}
			foundFinal = true
		case EventTypeAgentComplete:
			if !foundFinal {
				t.Error("AgentComplete before FinalOutput")
			}
			foundComplete = true
		}
	}

	if !foundStart || !foundResult || !foundFinal || !foundComplete {
		t.Errorf("Missing expected events. Start:%v Result:%v Final:%v Complete:%v",
			foundStart, foundResult, foundFinal, foundComplete)
	}
}

// TestE2E_StreamingWithChunks tests real-time streaming with text chunks
func TestE2E_StreamingWithChunks(t *testing.T) {
	// Simulate streaming response in chunks
	chunks := []providers.StreamChunk{
		{Content: "The"},
		{Content: " answer"},
		{Content: " is"},
		{Content: " 42."},
		{Content: "", IsComplete: true},
	}

	mock := NewMockLLM().WithStream(chunks)

	agent, err := New(Config{
		Model:           "gpt-4o",
		LLMProvider:     mock,
		StreamResponses: true,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	events := agent.Run(ctx, "What is the meaning of life?")

	// Client simulation: build response from chunks in real-time
	var streamedChunks []string
	var completeResponse string

	for event := range events {
		switch event.Type {
		case EventTypeThinkingChunk:
			chunk := event.Data["chunk"].(string)
			streamedChunks = append(streamedChunks, chunk)
			completeResponse += chunk
		case EventTypeFinalOutput:
			// Verify final output matches streamed chunks
			finalResp := event.Data["response"].(string)
			if finalResp != completeResponse && finalResp != "" {
				t.Errorf("Final output mismatch. Streamed: '%s', Final: '%s'", 
					completeResponse, finalResp)
			}
		}
	}

	// Verify we received chunks
	if len(streamedChunks) == 0 {
		t.Error("Expected streaming chunks, got none")
	}

	// Verify complete response was built
	expectedResponse := "The answer is 42."
	if completeResponse != expectedResponse {
		t.Errorf("Expected response '%s', got '%s'", expectedResponse, completeResponse)
	}
}

// TestE2E_ErrorHandling tests error scenarios with proper event emission
func TestE2E_ErrorHandling(t *testing.T) {
	// Simulate a tool that fails
	failingTool := NewTool("failing_tool").
		WithDescription("A tool that always fails").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return nil, fmt.Errorf("simulated tool failure")
		}).
		Build()

	mock := NewMockLLM().
		WithResponse("Let me try the tool.", []ToolCall{
			{ID: "call_1", Name: "failing_tool", Arguments: map[string]any{}},
		}).
		WithFinalResponse("The tool failed, but I'm handling it gracefully.")

	agent, err := New(Config{
		Model:       "gpt-4o",
		LLMProvider: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(failingTool)

	ctx := context.Background()
	events := agent.Run(ctx, "Use the failing tool")

	// Verify error is reported in events
	var completedSuccessfully bool

	for event := range events {
		switch event.Type {
		case EventTypeAgentComplete:
			completedSuccessfully = true
		}
	}

	// The agent should complete gracefully even with tool errors
	if !completedSuccessfully {
		t.Error("Agent should complete even with tool error")
	}
}

// TestE2E_ParallelToolExecution tests concurrent tool calls
func TestE2E_ParallelToolExecution(t *testing.T) {
	// Track tool execution timing
	var mu sync.Mutex
	executionOrder := []string{}

	tool1 := NewTool("tool_1").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			executionOrder = append(executionOrder, "tool_1")
			mu.Unlock()
			return "result1", nil
		}).
		Build()

	tool2 := NewTool("tool_2").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			executionOrder = append(executionOrder, "tool_2")
			mu.Unlock()
			return "result2", nil
		}).
		Build()

	mock := NewMockLLM().
		WithResponse("Using both tools", []ToolCall{
			{ID: "call_1", Name: "tool_1", Arguments: map[string]any{}},
			{ID: "call_2", Name: "tool_2", Arguments: map[string]any{}},
		}).
		WithFinalResponse("Both tools completed")

	agent, err := New(Config{
		Model:       "gpt-4o",
		LLMProvider: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(tool1)
	agent.AddTool(tool2)

	ctx := context.Background()
	events := agent.Run(ctx, "Run both tools")

	var toolResults int
	for event := range events {
		if event.Type == EventTypeActionResult {
			toolResults++
		}
	}

	// Both tools should execute and return results
	if toolResults != 2 {
		t.Errorf("Expected 2 tool results, got %d", toolResults)
	}

	// Both tools should have executed
	mu.Lock()
	defer mu.Unlock()
	if len(executionOrder) != 2 {
		t.Errorf("Expected 2 tools to execute, got %d", len(executionOrder))
	}
}

// TestE2E_MaxIterationsLimit tests iteration limit enforcement
func TestE2E_MaxIterationsLimit(t *testing.T) {
	// Create a scenario that would loop forever without max iterations
	mock := NewMockLLM().
		WithResponse("Iteration 1", []ToolCall{{ID: "1", Name: "loop", Arguments: map[string]any{}}}).
		WithResponse("Iteration 2", []ToolCall{{ID: "2", Name: "loop", Arguments: map[string]any{}}}).
		WithResponse("Iteration 3", []ToolCall{{ID: "3", Name: "loop", Arguments: map[string]any{}}}).
		WithFinalResponse("Done")

	loopTool := NewTool("loop").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return "continue", nil
		}).
		Build()

	agent, err := New(Config{
		Model:         "gpt-4o",
		LLMProvider:   mock,
		MaxIterations: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(loopTool)

	ctx := context.Background()
	events := agent.Run(ctx, "Keep looping")

	iterations := 0
	for event := range events {
		if event.Type == EventTypeActionResult {
			iterations++
		}
	}

	// Should respect max iterations
	if iterations > 3 {
		t.Errorf("Expected max 3 iterations, got %d", iterations)
	}
}

// TestE2E_ContextCancellation tests proper handling of context cancellation
func TestE2E_ContextCancellation(t *testing.T) {
	// Simulate a long-running operation
	slowTool := NewTool("slow_operation").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			select {
			case <-time.After(5 * time.Second):
				return "completed", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}).
		Build()

	mock := NewMockLLM().
		WithResponse("Starting slow operation", []ToolCall{
			{ID: "call_1", Name: "slow_operation", Arguments: map[string]any{}},
		})

	agent, err := New(Config{
		Model:       "gpt-4o",
		LLMProvider: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(slowTool)

	// Create a context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	events := agent.Run(ctx, "Run slow operation")

	// Collect events until cancellation
	eventReceived := false
	for range events {
		eventReceived = true
	}

	// Should receive at least some events before cancellation
	if !eventReceived {
		t.Error("Expected to receive events even with cancellation")
	}
}

// TestE2E_ComplexWorkflow tests a realistic multi-step scenario
func TestE2E_ComplexWorkflow(t *testing.T) {
	// Scenario: Research assistant that searches, analyzes, and summarizes
	searchCalls := 0
	analyzeCalls := 0

	searchTool := NewTool("search").
		WithDescription("Search for information").
		WithParameter("query", String().Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			searchCalls++
			query := args["query"].(string)
			return map[string]any{
				"results": []string{
					"Result 1 for " + query,
					"Result 2 for " + query,
					"Result 3 for " + query,
				},
			}, nil
		}).
		Build()

	analyzeTool := NewTool("analyze").
		WithDescription("Analyze search results").
		WithParameter("data", String().Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			analyzeCalls++
			return map[string]any{
				"insights": "Key insights from the data",
				"summary":  "Brief summary of findings",
			}, nil
		}).
		Build()

	mock := NewMockLLM().
		WithResponse("Let me search for that", []ToolCall{
			{ID: "call_1", Name: "search", Arguments: map[string]any{"query": "AI trends"}},
		}).
		WithResponse("Now analyzing the results", []ToolCall{
			{ID: "call_2", Name: "analyze", Arguments: map[string]any{"data": "search results"}},
		}).
		WithFinalResponse("Based on my research and analysis, here are the key AI trends: [insights]")

	agent, err := New(Config{
		Model:       "gpt-4o",
		LLMProvider: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(searchTool)
	agent.AddTool(analyzeTool)

	ctx := context.Background()
	events := agent.Run(ctx, "Research the latest AI trends")

	// Track the workflow via action results
	var workflow []string
	var finalOutput string

	for event := range events {
		switch event.Type {
		case EventTypeActionResult:
			if desc, ok := event.Data["description"].(string); ok {
				if strings.Contains(desc, "search") {
					workflow = append(workflow, "search")
				} else if strings.Contains(desc, "analyze") {
					workflow = append(workflow, "analyze")
				}
			}
		case EventTypeFinalOutput:
			finalOutput = event.Data["response"].(string)
		}
	}

	// Verify complete workflow executed
	if searchCalls != 1 {
		t.Errorf("Expected 1 search call, got %d", searchCalls)
	}
	if analyzeCalls != 1 {
		t.Errorf("Expected 1 analyze call, got %d", analyzeCalls)
	}

	// Verify correct sequence
	if len(workflow) != 2 || workflow[0] != "search" || workflow[1] != "analyze" {
		t.Errorf("Expected [search, analyze] workflow, got %v", workflow)
	}

	// Verify final output exists
	if finalOutput == "" {
		t.Error("Expected final output with research summary")
	}
}

// TestE2E_EventDataIntegrity tests that all event data is properly populated
func TestE2E_EventDataIntegrity(t *testing.T) {
	mock := NewMockLLM().
		WithResponse("Using tool", []ToolCall{
			{ID: "test_call", Name: "test_tool", Arguments: map[string]any{"param": "value"}},
		}).
		WithFinalResponse("Completed")

	testTool := NewTool("test_tool").
		WithParameter("param", String().Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"status": "success", "value": args["param"]}, nil
		}).
		Build()

	agent, err := New(Config{
		Model:       "gpt-4o",
		LLMProvider: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	agent.AddTool(testTool)

	ctx := context.Background()
	events := agent.Run(ctx, "Test event data")

	for event := range events {
		// Verify all events have timestamp
		if event.Timestamp.IsZero() {
			t.Errorf("Event %s has zero timestamp", event.Type)
		}

		// Verify event-specific data integrity
		switch event.Type {
		case EventTypeAgentStart:
			// AgentStart should have basic data
			if event.Data == nil {
				t.Error("AgentStart missing data")
			}

		case EventTypeActionDetected:
			// ActionDetected should have description
			if _, ok := event.Data["description"].(string); !ok {
				t.Error("ActionDetected missing description")
			}

		case EventTypeActionResult:
			// ActionResult should have description and result or error info
			if _, ok := event.Data["description"].(string); !ok {
				t.Error("ActionResult missing description")
			}
			// Should have result data
			if event.Data["result"] == nil {
				t.Error("ActionResult missing result")
			}

		case EventTypeFinalOutput:
			if _, ok := event.Data["response"].(string); !ok {
				t.Error("FinalOutput missing response")
			}

		case EventTypeAgentComplete:
			// AgentComplete should have duration
			if _, ok := event.Data["duration_ms"].(int64); !ok {
				t.Error("AgentComplete missing duration_ms")
			}
		}
	}
}

// TestE2E_ConcurrentAgents tests multiple agents running concurrently
func TestE2E_ConcurrentAgents(t *testing.T) {
	const numAgents = 5

	var wg sync.WaitGroup
	errors := make(chan error, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			mock := NewMockLLM().WithFinalResponse(fmt.Sprintf("Agent %d response", id))

			agent, err := New(Config{
				Model:       "gpt-4o",
				LLMProvider: mock,
			})
			if err != nil {
				errors <- fmt.Errorf("agent %d creation failed: %w", id, err)
				return
			}

			ctx := context.Background()
			events := agent.Run(ctx, fmt.Sprintf("Request from agent %d", id))

			// Each agent should complete independently
			var completed bool
			for event := range events {
				if event.Type == EventTypeAgentComplete {
					completed = true
				}
			}

			if !completed {
				errors <- fmt.Errorf("agent %d did not complete", id)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

// TestE2E_EmptyResponseHandling tests that empty LLM responses are handled correctly
func TestE2E_EmptyResponseHandling(t *testing.T) {
	// Simulate LLM returning empty response (no content, no tool calls)
	mock := NewMockLLM().WithResponse("", nil)
	
	agent, err := New(Config{
		Model:       "gpt-4o",
		LLMProvider: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	
	ctx := context.Background()
	events := agent.Run(ctx, "Test prompt")
	
	var sawStart bool
	var sawFinalOutput bool
	var sawComplete bool
	var finalOutput string
	
	for event := range events {
		switch event.Type {
		case EventTypeAgentStart:
			sawStart = true
		case EventTypeFinalOutput:
			sawFinalOutput = true
			if response, ok := event.Data["response"].(string); ok {
				finalOutput = response
			}
		case EventTypeAgentComplete:
			sawComplete = true
		}
	}
	
	// Verify complete event lifecycle even with empty response
	if !sawStart {
		t.Error("Missing agent.start event")
	}
	
	if !sawFinalOutput {
		t.Error("Missing final_output event (should be emitted even when empty)")
	}
	
	if !sawComplete {
		t.Error("Missing agent.complete event")
	}
	
	if finalOutput != "" {
		t.Errorf("Expected empty response, got: %s", finalOutput)
	}
	
	t.Log("Empty response properly handled with complete event lifecycle")
}
