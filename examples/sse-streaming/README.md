# SSE Streaming Example

This example demonstrates real-time Server-Sent Events (SSE) streaming with AgentKit's new semantic event system.

## Features

- **Real-time event streaming**: See all 15 event types as they happen
- **Interactive web UI**: Test the agent directly in your browser
- **Simple SSE endpoint**: Just forward events - no complex mapping needed

## Event Types Demonstrated

### Agent Lifecycle
- `agent.start` - Agent begins execution
- `agent.complete` - Agent finishes with metrics

### Content Streaming  
- `thinking_chunk` - Streaming LLM response chunks
- `final_output` - Complete response

### Tool Execution
- `action_detected` - Tool call detected
- `action_result` - Tool execution result

### Multi-Agent (if you extend the example)
- `handoff.start` - Delegation to another agent
- `handoff.complete` - Handoff returns
- `collaboration.agent.contribution` - Agent contributes to discussion

### Human-in-the-Loop (if you add approvals)
- `approval_required` - Tool needs approval
- `approval_granted` - Approval given
- `approval_denied` - Approval denied

## Running the Example

1. Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY=your-key-here
   ```

2. Run the example:
   ```bash
   cd examples/sse-streaming
   go run main.go
   ```

3. Open http://localhost:8080 in your browser

4. Try different queries:
   - "What's the weather in San Francisco?" (uses get_weather tool)
   - "Search for latest AI news" (uses search_web tool)
   - "Tell me a joke" (simple response, no tools)

## Implementation Highlights

### The SSE Endpoint (that's it!)

```go
func streamAgentEvents(w http.ResponseWriter, r *http.Request, agent *agentkit.Agent) {
    // Setup SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    flusher, _ := w.(http.Flusher)
    
    // Run agent and stream events - that's it!
    eventChan := agent.Run(r.Context(), userMessage)
    
    for event := range eventChan {
        eventJSON, _ := json.Marshal(event)
        fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, eventJSON)
        flusher.Flush()
    }
}
```

### Client-Side (TypeScript/JavaScript)

```javascript
const eventSource = new EventSource('/api/agent/stream?message=' + query);

// Listen for specific events
eventSource.addEventListener('agent.start', (e) => {
    const data = JSON.parse(e.data);
    console.log('Agent started:', data.agent_name);
});

eventSource.addEventListener('thinking_chunk', (e) => {
    const data = JSON.parse(e.data);
    appendToUI(data.chunk);
});

eventSource.addEventListener('agent.complete', (e) => {
    const data = JSON.parse(e.data);
    showMetrics(data.duration_ms, data.total_tokens);
});
```

## Filtering Events

Want to only stream specific events? Easy:

```go
// Only stream important events
interestingEvents := []agentkit.EventType{
    agentkit.EventTypeAgentStart,
    agentkit.EventTypeAgentComplete,
    agentkit.EventTypeActionDetected,
    agentkit.EventTypeActionResult,
}

filtered := agentkit.FilterEvents(agent.Run(ctx, msg), interestingEvents...)
for event := range filtered {
    // stream it
}
```

## Why SSE is Perfect for AgentKit

1. **One-way streaming**: Perfect for agent → client communication
2. **Native browser support**: No extra libraries needed
3. **Automatic reconnection**: Built into EventSource API
4. **Typed events**: Each event type can have its own handler
5. **Simple protocol**: text/event-stream format is trivial

## Extending This Example

### Add Handoffs

```go
specialist := agentkit.NewAgent(...)
coordinator.RegisterTool(
    agentkit.NewHandoffConfiguration(coordinator, specialist).
        AsTool("delegate_to_specialist", "Delegate complex tasks"),
)
```

Now you'll see `handoff.start` and `handoff.complete` events!

### Add Collaboration

```go
session := agentkit.NewCollaborationSession(facilitator, engineer, designer)
result, _ := session.Discuss(ctx, "How should we design this feature?")
```

Now you'll see `collaboration.agent.contribution` events for each agent's input!

### Add Approvals

```go
agent, _ := agentkit.NewAgent(agentkit.Config{
    Approval: &agentkit.ApprovalConfig{
        RequireApproval: true,
        Approver: func(ctx context.Context, req agentkit.ApprovalRequest) (bool, string, error) {
            // Your approval logic
            return true, "", nil
        },
    },
})
```

Now you'll see `approval_required`, `approval_granted`, and `approval_denied` events!

## Production Considerations

1. **Authentication**: Add auth middleware to protect your SSE endpoint
2. **Rate limiting**: Prevent abuse with rate limits
3. **Timeouts**: Set reasonable timeouts for long-running agents
4. **Error handling**: Always handle errors gracefully
5. **Monitoring**: Track event metrics for observability

## Next Steps

- Read the [Events Documentation](../../docs/EVENTS.md) for full event reference
- Check out [handoff example](../handoff/) for multi-agent patterns
- Check out [collaborate example](../collaborate/) for collaboration patterns
- See [cost-tracking example](../cost-tracking/) for usage metrics
