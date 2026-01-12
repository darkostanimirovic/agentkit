# End-to-End Testing Guide

## Overview

This document describes the end-to-end (E2E) testing approach for the agentkit framework. E2E tests validate real-world usage patterns and ensure that clients using the framework see all proper events in the correct order.

## Testing Philosophy

### First Principles Approach

E2E tests are written **from first principles** - they simulate how real clients would use the framework:

1. **No mocking internal components** - Only mock external dependencies (LLM providers)
2. **Complete event stream validation** - Verify every event from start to finish
3. **Real-world scenarios** - Test actual use cases, not isolated code paths
4. **Event-driven verification** - Track events as they would appear to clients

### What E2E Tests Validate

✅ Complete agent lifecycle (start → execution → completion)  
✅ Event emission and ordering  
✅ Tool execution and results  
✅ Multi-turn conversations  
✅ Error handling and recovery  
✅ Streaming responses  
✅ Context cancellation  
✅ Concurrent execution  
✅ Event data integrity  

## Event Flow Architecture

### Standard Event Sequence

```
agent.start
  └─> action_result (for each tool execution)
       └─> final_output (agent's response)
            └─> agent.complete (with metrics)
```

### Event Types

- **agent.start**: Agent begins execution
  - Data: `agent_name`
  
- **action_result**: Tool execution completed
  - Data: `description` (tool name), `result` (tool output)
  
- **final_output**: Agent's final response
  - Data: `response` (text output), `summary` (optional)
  
- **agent.complete**: Agent finished execution
  - Data: `agent_name`, `duration_ms`, `iterations`, `output`, `total_tokens`

### Important Notes

⚠️ **ActionDetected events** are NOT emitted in standard (non-streaming) tool execution  
⚠️ Tool names appear in the `"description"` field, not a `"tool"` field  
⚠️ Event ordering is guaranteed: start → actions → final → complete  

## Test Patterns

### Pattern 1: Basic Workflow Validation

```go
func TestE2E_BasicStreamingWorkflow(t *testing.T) {
    // Setup mock LLM with expected responses
    mock := NewMockLLM().
        WithResponse("Using tool", []ToolCall{...}).
        WithFinalResponse("Done")
    
    // Create agent with tools
    agent := setupAgent(mock)
    
    // Run agent and collect events
    events := agent.Run(ctx, "task")
    
    // Validate event sequence and data
    for event := range events {
        // Track events, verify data
    }
    
    // Assert expectations
}
```

### Pattern 2: Multi-Turn Conversations

```go
func TestE2E_MultiTurnConversation(t *testing.T) {
    // Mock returns multiple responses (conversation turns)
    mock := NewMockLLM().
        WithResponse("First response", toolCalls).
        WithFinalResponse("Final answer")
    
    // Track complete conversation flow
    var toolResults []string
    var finalOutput string
    
    for event := range agent.Run(ctx, "question") {
        // Track action results
        // Capture final output
    }
    
    // Verify conversation completed correctly
}
```

### Pattern 3: Streaming with Chunks

```go
func TestE2E_StreamingWithChunks(t *testing.T) {
    // Simulate real streaming with chunks
    chunks := []providers.StreamChunk{
        {Content: "The ", IsComplete: false},
        {Content: "answer ", IsComplete: false},
        {Content: "is 42.", IsComplete: true},
    }
    
    mock := NewMockLLM().WithStream(chunks)
    
    // Track streaming reconstruction
    var streamedChunks []string
    var completeResponse string
    
    for event := range agent.Run(ctx, "question") {
        if event.Type == EventTypeStreamChunk {
            // Collect chunks
        }
        if event.Type == EventTypeFinalOutput {
            // Verify complete response
        }
    }
}
```

### Pattern 4: Error Handling

```go
func TestE2E_ErrorHandling(t *testing.T) {
    // Create a tool that fails
    failingTool := NewTool("fail_tool").
        WithHandler(func(ctx, args) (any, error) {
            return nil, fmt.Errorf("intentional failure")
        }).
        Build()
    
    agent.AddTool(failingTool)
    
    // Verify agent handles errors gracefully
    var completedSuccessfully bool
    for event := range agent.Run(ctx, "use failing tool") {
        if event.Type == EventTypeAgentComplete {
            completedSuccessfully = true
        }
    }
    
    // Agent should complete despite tool failure
    assert.True(t, completedSuccessfully)
}
```

### Pattern 5: Parallel Execution

```go
func TestE2E_ParallelToolExecution(t *testing.T) {
    // Track concurrent tool execution
    var mu sync.Mutex
    executionOrder := []string{}
    
    tool1 := NewTool("tool_1").
        WithHandler(func(ctx, args) (any, error) {
            mu.Lock()
            executionOrder = append(executionOrder, "tool_1")
            mu.Unlock()
            return "result1", nil
        }).
        Build()
    
    // Multiple tools can execute concurrently
    // Verify they complete independently
}
```

### Pattern 6: Context Cancellation

```go
func TestE2E_ContextCancellation(t *testing.T) {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    // Tool that takes too long
    slowTool := NewTool("slow").
        WithHandler(func(ctx, args) (any, error) {
            time.Sleep(200 * time.Millisecond)
            return "result", nil
        }).
        Build()
    
    // Verify graceful cancellation
    events := agent.Run(ctx, "use slow tool")
    for event := range events {
        // Events should complete gracefully
    }
}
```

### Pattern 7: Complex Workflows

```go
func TestE2E_ComplexWorkflow(t *testing.T) {
    // Multi-step workflow: search → analyze → summarize
    searchCalls := 0
    analyzeCalls := 0
    
    searchTool := NewTool("search").
        WithHandler(func(ctx, args) (any, error) {
            searchCalls++
            return "search results", nil
        }).
        Build()
    
    analyzeTool := NewTool("analyze").
        WithHandler(func(ctx, args) (any, error) {
            analyzeCalls++
            return "analysis", nil
        }).
        Build()
    
    // Track workflow execution order via events
    var workflow []string
    for event := range agent.Run(ctx, "research topic") {
        if event.Type == EventTypeActionResult {
            // Track which tools executed in order
        }
    }
    
    // Verify correct execution sequence
    assert.Equal(t, []string{"search", "analyze"}, workflow)
}
```

## Event Data Validation

### Essential Checks

```go
// 1. Event Type Verification
assert.Equal(t, EventTypeActionResult, event.Type)

// 2. Required Fields Present
_, hasDescription := event.Data["description"]
assert.True(t, hasDescription)

// 3. Data Type Correctness
desc, ok := event.Data["description"].(string)
assert.True(t, ok)

// 4. Value Validation
assert.Contains(t, desc, "expected_tool_name")

// 5. Timestamp Presence
assert.NotNil(t, event.Timestamp)
```

### Common Assertions

```go
// Agent completed successfully
assert.Equal(t, EventTypeAgentComplete, event.Type)

// Tool executed with correct name
desc := event.Data["description"].(string)
assert.Contains(t, desc, "tool_name")

// Final output contains expected content
response := event.Data["response"].(string)
assert.Contains(t, response, "expected text")

// Metrics are populated
duration := event.Data["duration_ms"].(float64)
assert.Greater(t, duration, 0.0)

iterations := event.Data["iterations"].(int)
assert.Greater(t, iterations, 0)
```

## Test Data Setup

### Mock LLM Responses

```go
// Single response with tool call
mock := NewMockLLM().
    WithResponse("I'll use the tool", []ToolCall{
        {ID: "1", Name: "tool_name", Arguments: map[string]any{"arg": "value"}},
    }).
    WithFinalResponse("Here's the result")

// Multi-turn conversation
mock := NewMockLLM().
    WithResponse("First, let me check", []ToolCall{...}).
    WithResponse("Now analyzing", []ToolCall{...}).
    WithFinalResponse("Final conclusion")

// Streaming response
chunks := []providers.StreamChunk{
    {Content: "chunk1", IsComplete: false},
    {Content: "chunk2", IsComplete: false},
    {Content: "chunk3", IsComplete: true},
}
mock := NewMockLLM().WithStream(chunks)
```

### Tool Creation

```go
// Simple tool
tool := NewTool("tool_name").
    WithDescription("What the tool does").
    WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
        return "result", nil
    }).
    Build()

// Tool with parameters
tool := NewTool("search").
    WithDescription("Search for information").
    WithParameter("query", "string", "Search query").
    WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
        query := args["query"].(string)
        return fmt.Sprintf("Results for: %s", query), nil
    }).
    Build()

// Tool that tracks calls
callCount := 0
tool := NewTool("counter").
    WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
        callCount++
        return fmt.Sprintf("Call %d", callCount), nil
    }).
    Build()
```

## Running E2E Tests

### Run All E2E Tests
```bash
go test -v -run TestE2E_
```

### Run Specific E2E Test
```bash
go test -v -run TestE2E_BasicStreamingWorkflow
```

### Run with Race Detection
```bash
go test -race -v -run TestE2E_
```

### Run with Coverage
```bash
go test -v -run TestE2E_ -coverprofile=e2e_coverage.out
go tool cover -html=e2e_coverage.out
```

## Best Practices

### DO ✅

- **Test from the client's perspective** - What would a real user see?
- **Validate complete event streams** - Check every event type
- **Use real tool execution** - Don't mock tool handlers unless testing errors
- **Test error scenarios** - Verify graceful degradation
- **Verify event ordering** - Ensure predictable sequence
- **Check event data integrity** - Validate all fields are populated
- **Test concurrent scenarios** - Multiple agents, parallel tools
- **Use meaningful assertions** - Clear error messages on failure

### DON'T ❌

- **Don't mock internal agent logic** - Only mock LLM provider
- **Don't skip event validation** - Every event matters
- **Don't assume event structure** - Verify actual fields
- **Don't ignore error cases** - Test failure paths
- **Don't test in isolation** - E2E means end-to-end
- **Don't forget cleanup** - Close channels, cancel contexts
- **Don't use magic numbers** - Make test data obvious

## Debugging E2E Tests

### Enable Verbose Logging
```go
agent, _ := New(Config{
    Model: "gpt-4o",
    LLMProvider: mock,
    // Enable verbose logging for debugging
})
```

### Print Event Stream
```go
for event := range events {
    t.Logf("Event: %s, Data: %+v", event.Type, event.Data)
}
```

### Inspect Event Data
```go
func dumpEvent(t *testing.T, event Event) {
    t.Logf("=== EVENT ===")
    t.Logf("Type: %s", event.Type)
    t.Logf("Timestamp: %v", event.Timestamp)
    for k, v := range event.Data {
        t.Logf("  %s: %v", k, v)
    }
}
```

## Coverage Goals

- **E2E Tests**: Cover all major workflows
- **Event Types**: Every event type tested
- **Error Paths**: All error scenarios validated
- **Integration Points**: LLM, tools, middleware tested together

## Continuous Improvement

As the framework evolves:

1. Add E2E tests for new features
2. Validate event stream changes
3. Update this guide with new patterns
4. Maintain test documentation
5. Keep tests fast and reliable

## Resources

- [TEST_SUMMARY.md](../TEST_SUMMARY.md) - Overall test coverage
- [USAGE.md](USAGE.md) - Framework usage guide
- [EVENTS.md](EVENTS.md) - Event system documentation
- [agent_test.go](../agent_test.go) - Unit test examples
- [e2e_test.go](../e2e_test.go) - Complete E2E test suite
