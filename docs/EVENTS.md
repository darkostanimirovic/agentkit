# AgentKit Events Reference

AgentKit emits **15 semantic, actionable events** during agent execution. These events enable real-time streaming via SSE, WebSockets, or any other transport mechanism.

## Philosophy

**Start simple. Add only when you need it.**

These 15 events cover all core agentic operations. They're organized into 5 intuitive categories that map directly to what your agent is doing:

1. **Agent Lifecycle** (2 events) - When agents start and finish
2. **Content Streaming** (2 events) - LLM response chunks and final output
3. **Tool Execution** (2 events) - Tool calls and results
4. **Multi-Agent Coordination** (3 events) - Handoffs and collaboration (unique to AgentKit!)
5. **Human-in-the-Loop** (3 events) - Approval workflows
6. **Progress & Decisions** (2 events) - Internal agent state
7. **Errors** (1 event) - When things go wrong

**Total: 15 semantic, actionable events**

## Quick Start

```go
events := agent.Run(ctx, "What's the weather in SF?")

for event := range events {
    switch event.Type {
    case agentkit.EventTypeAgentStart:
        fmt.Println("Agent started")
    case agentkit.EventTypeThinkingChunk:
        fmt.Print(event.Data["chunk"])
    case agentkit.EventTypeAgentComplete:
        fmt.Printf("Done in %dms\n", event.Data["duration_ms"])
    }
}
```

## All 15 Event Types

### 1. Agent Lifecycle Events

#### `agent.start`

Emitted when an agent begins execution.

**When**: First event emitted when `agent.Run()` is called  
**Frequency**: Once per agent run

**Data Fields**:
- `agent_name` (string): Name of the agent starting

**Example**:
```json
{
  "type": "agent.start",
  "data": {
    "agent_name": "agent"
  },
  "timestamp": "2026-01-11T10:30:00Z"
}
```

**Client Actions**:
- Show "Agent thinking..." indicator
- Start loading animation
- Initialize metrics counters

---

#### `agent.complete`

Emitted when an agent finishes execution (success or error).

**When**: At the end of agent.Run(), after all processing complete

**Data Fields**:
```json
{
  "agent_name": "agent",
  "output": "The final response from the agent",
  "total_tokens": 150,
  "iterations": 2,
  "duration_ms": 2500
}
```

**Client Actions**:
- Display final result
- Show metrics (tokens, duration, iterations)
- Update UI to "complete" state
- Enable "Send" button

**Example**:
```json
{
  "type": "agent.complete",
  "data": {
    "agent_name": "agent",
    "output": "The weather in San Francisco is sunny, 22°C",
    "total_tokens": 0,
    "iterations": 1,
    "duration_ms": 2341
  },
  "timestamp": "2026-01-11T10:30:05Z"
}
```

---

## Content Streaming Events

### thinking_chunk

Emitted when the LLM generates a chunk of content during streaming.

**When**: During streaming LLM responses  
**Frequency**: Multiple times per LLM call  
**Data Fields**:
- `chunk` (string): The content chunk

**Example**:
```json
{
  "type": "thinking_chunk",
  "data": {
    "chunk": "Hello, how can I "
  },
  "timestamp": "2026-01-11T10:30:00Z"
}
```

**Client Actions**:
- Append to chat UI
- Update streaming indicator
- Accumulate for full response

### final_output

Emitted when the agent completes its response.

**Data**:
- `response` (string): The complete output
- `summary` (string): Optional summary

```json
{
  "type": "final_output",
  "data": {
    "response": "The weather in San Francisco is sunny, 22°C",
    "summary": "Retrieved weather information"
  }
}
```

---

## Tool Execution Events

### action_detected

Emitted when the LLM decides to call a tool.

**Data Fields**:
- `description` (string): What the tool does
- `tool_id` (string): The tool identifier

**Example**:
```json
{
  "type": "action_detected",
  "data": {
    "description": "Getting weather for San Francisco",
    "tool_id": "get_weather"
  }
}
```

**Client Actions**:
- Show "Agent is using tool: {tool_name}"
- Display tool indicator/spinner
- Log for debugging

### action_result

Tool execution has completed.

**When**: After tool execution finishes  
**Data**:
- `description` (string): Human-readable description
- `result` (any): Tool execution result

**Example**:
```json
{
  "type": "action_result",
  "data": {
    "description": "Weather lookup completed",
    "result": "The weather in San Francisco is sunny, 22°C"
  },
  "timestamp": "2026-01-11T10:30:02Z"
}
```

**Client Actions**:
- Hide tool loading indicator
- Show tool result in UI
- Update conversation history

---

## Multi-Agent Coordination Events

### handoff.start

**When**: Agent delegates work to another agent

**Data Fields**:
- `from_agent` (string): Name of delegating agent
- `to_agent` (string): Name of receiving agent
- `task` (string): Description of delegated task

**Example**:
```json
{
  "type": "handoff.start",
  "data": {
    "from_agent": "coordinator",
    "to_agent": "research_specialist",
    "task": "Research the latest developments in quantum computing"
  },
  "timestamp": "2026-01-11T10:30:00Z"
}
```

**Client Actions**:
- Show agent transition animation
- Display "Delegating to specialist..." message
- Update UI to show which agent is active

### handoff.complete

Emitted when a delegated agent completes and returns control.

**When**: After delegated agent finishes execution  
**Frequency**: Once per handoff  
**Data**:
- `from_agent` (string): Originating agent name
- `to_agent` (string): Target agent name
- `result` (string): Result from the delegated agent

**Example**:
```json
{
  "type": "handoff.complete",
  "data": {
    "from_agent": "coordinator",
    "to_agent": "specialist",
    "result": "Analysis complete: the data shows..."
  },
  "timestamp": "2026-01-11T10:32:15Z"
}
```

**Client Actions:**
- Hide delegation indicator
- Show result from delegated agent
- Update conversation context

---

### 5. `collaboration.agent.contribution`

**When**: During multi-agent collaboration, each time an agent contributes

**Frequency**: Multiple times per collaboration session (once per agent per round)

**Data Fields**:
- `agent_name` (string): Name of the contributing agent
- `contribution` (string): The agent's contribution to the discussion

**Example**:
```json
{
  "type": "collaboration.agent.contribution",
  "data": {
    "agent_name": "engineer",
    "contribution": "We should use JWT tokens for authentication..."
  },
  "timestamp": "2026-01-11T10:30:05Z"
}
```

**Client Actions**:
- Display contribution in discussion thread
- Show which agent is speaking
- Highlight different perspectives
- Build conversation history UI

---

## Human-in-the-Loop Events

### approval_required

**When**: Tool needs human approval before execution

**Data Fields**:
- `tool_name` (string): Tool requiring approval
- `arguments` (map): Tool arguments
- `description` (string): What the tool will do
- `conversation_id` (string): Conversation identifier
- `call_id` (string): Unique call identifier

**Example**:
```json
{
  "type": "approval_required",
  "data": {
    "tool_name": "delete_database",
    "arguments": {"db": "production"},
    "description": "Delete production database",
    "conversation_id": "conv_123",
    "call_id": "call_456"
  }
}
```

**Client Actions**:
- Show approval dialog
- Present tool details to user
- Provide approve/deny buttons

### approval_granted

Emitted when a human approves a tool execution.

**When**: After user approves
**Frequency**: Once per approval
**Data**:
- `tool_name` (string): Tool being approved
- `call_id` (string): Call identifier

**Example**:
```json
{
  "type": "approval_granted",
  "data": {
    "tool_name": "delete_database",
    "call_id": "call_456"
  }
}
```

**Client Actions**:
- Close approval dialog
- Show "Approved" indicator
- Wait for tool execution

### approval_denied

Emitted when a human denies a tool execution.

**When**: After user denies
**Frequency**: Once per denial
**Data**:
- `tool_name` (string): Tool being denied
- `call_id` (string): Call identifier
- `reason` (string): Why it was denied

**Example**:
```json
{
  "type": "approval_denied",
  "data": {
    "tool_name": "delete_database",
    "call_id": "call_456",
    "reason": "Too dangerous for production"
  }
}
```

**Client Actions**:
- Close approval dialog
- Show "Denied" indicator
- Display reason

---

## Progress & Decision Events

### progress

Emitted to indicate agent progress through iterations.

**When**: During iteration loops
**Frequency**: Multiple times
**Data**:
- `iteration` (int): Current iteration
- `max_iterations` (int): Maximum iterations
- `description` (string): What's happening

**Example**:
```json
{
  "type": "progress",
  "data": {
    "iteration": 2,
    "max_iterations": 5,
    "description": "Processing tool results"
  }
}
```

**Client Actions**:
- Update progress bar
- Show iteration count

### decision

Emitted when agent makes a decision.

**When**: During reasoning
**Frequency**: Variable
**Data**:
- `action` (string): What action was decided
- `confidence` (float): Confidence level (0-1)
- `reasoning` (string): Why this decision

**Example**:
```json
{
  "type": "decision",
  "data": {
    "action": "use_tool",
    "confidence": 0.95,
    "reasoning": "Need current data from API"
  }
}
```

**Client Actions**:
- Show decision in debug view
- Log for analysis

---

## Error Events

### error

Emitted when an error occurs during execution.

**When**: On error
**Frequency**: As needed
**Data**:
- `error` (string): Error message

**Example**:
```json
{
  "type": "error",
  "data": {
    "error": "API rate limit exceeded"
  }
}
```

**Client Actions**:
- Display error to user
- Stop streaming UI
- Log error

---

## Event Flow Patterns

### Pattern 1: Simple Agent Run (Most Common - 80% of use cases)

```
1. agent.start
2. thinking_chunk (streaming response)
3. thinking_chunk
4. thinking_chunk
5. final_output
6. agent.complete
```

### Pattern 2: Agent with Tools

```
1. agent.start
2. thinking_chunk ("I'll check the weather...")
3. action_detected (get_weather)
4. action_result
5. thinking_chunk ("The weather is...")
6. final_output
7. agent.complete
```

### Pattern 3: Handoff (Multi-Agent)

```
1. agent.start (coordinator)
2. handoff.start (→ specialist)
3.   agent.start (specialist)
4.   thinking_chunk
5.   action_detected
6.   action_result
7.   final_output
8.   agent.complete (specialist)
9. handoff.complete
10. thinking_chunk ("The specialist found...")
11. final_output
12. agent.complete (coordinator)
```

### Pattern 4: Collaboration (Multi-Agent)

```
1. agent.start
2. collaboration.agent.contribution (engineer)
3. collaboration.agent.contribution (designer)
4. collaboration.agent.contribution (product)
5. thinking_chunk (synthesis)
6. final_output
7. agent.complete
```

### Pattern 5: Approval Flow

```
1. agent.start
2. thinking_chunk
3. action_detected
4. approval_required
5. ... wait for human ...
6. approval_granted
7. action_result
8. thinking_chunk
9. final_output
10. agent.complete
```

---

## Client Implementation Examples

### JavaScript/TypeScript (Browser)

```typescript
const eventSource = new EventSource('/api/agent/stream?message=' + query);

// Agent lifecycle
eventSource.addEventListener('agent.start', (e) => {
    const data = JSON.parse(e.data);
    console.log('Agent started:', data.agent_name);
    showSpinner();
});

eventSource.addEventListener('agent.complete', (e) => {
    const data = JSON.parse(e.data);
    console.log('Agent completed:', data.output);
    showMetrics(data.duration_ms, data.total_tokens, data.iterations);
    hideSpinner();
});

// Content streaming
eventSource.addEventListener('thinking_chunk', (e) => {
    const data = JSON.parse(e.data);
    appendToChat(data.chunk);
});

eventSource.addEventListener('final_output', (e) => {
    const data = JSON.parse(e.data);
    markFinal(data.response);
});

// Tool execution
eventSource.addEventListener('action_detected', (e) => {
    const data = JSON.parse(e.data);
    showToolIndicator(data.tool_id, data.description);
});

eventSource.addEventListener('action_result', (e) => {
    const data = JSON.parse(e.data);
    hideToolIndicator();
    logToolResult(data.result);
});

// Multi-agent
eventSource.addEventListener('handoff.start', (e) => {
    const data = JSON.parse(e.data);
    showAgentTransition(data.from_agent, data.to_agent, data.task);
});

eventSource.addEventListener('collaboration.agent.contribution', (e) => {
    const data = JSON.parse(e.data);
    addAgentBubble(data.agent_name, data.contribution);
});

// Human-in-the-loop
eventSource.addEventListener('approval_required', (e) => {
    const data = JSON.parse(e.data);
    showApprovalDialog(data.tool_name, data.arguments, data.call_id);
});

// Error handling
eventSource.addEventListener('error', (e) => {
    const data = JSON.parse(e.data);
    showError(data.error);
});

eventSource.onerror = () => {
    eventSource.close();
    console.log('Stream closed');
};
```

### Go (Server to Server)

```go
import (
    "bufio"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

func consumeSSE(url string) error {
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    var eventType string
    
    for scanner.Scan() {
        line := scanner.Text()
        
        if strings.HasPrefix(line, "event: ") {
            eventType = strings.TrimPrefix(line, "event: ")
        } else if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")
            
            var event agentkit.Event
            json.Unmarshal([]byte(data), &event)
            
            // Handle event
            switch event.Type {
            case agentkit.EventTypeAgentStart:
                fmt.Println("Agent started")
            case agentkit.EventTypeThinkingChunk:
                chunk := event.Data["chunk"].(string)
                fmt.Print(chunk)
            case agentkit.EventTypeAgentComplete:
                fmt.Println("\nAgent completed")
            }
        }
    }
    
    return scanner.Err()
}
```

---

## Filtering Events

AgentKit provides built-in event filtering:

```go
// Only stream specific events
interestingEvents := []agentkit.EventType{
    agentkit.EventTypeAgentStart,
    agentkit.EventTypeAgentComplete,
    agentkit.EventTypeActionDetected,
    agentkit.EventTypeActionResult,
}

filtered := agentkit.FilterEvents(agent.Run(ctx, msg), interestingEvents...)
for event := range filtered {
    // Only receives the filtered events
}
```

---

## Best Practices

### 1. Always Handle Core Events

At minimum, handle these 4 events:
- `agent.start` - Show spinner/loading
- `thinking_chunk` - Stream response
- `agent.complete` - Hide spinner, show metrics
- `error` - Display error

### 2. Progressive Enhancement

Start simple, add complexity:
```typescript
// Level 1: Just show output
eventSource.addEventListener('agent.complete', showResult);

// Level 2: Add streaming
eventSource.addEventListener('thinking_chunk', appendText);

// Level 3: Add tool visibility
eventSource.addEventListener('action_detected', showTool);

// Level 4: Add multi-agent visualization
eventSource.addEventListener('handoff.start', showHandoff);
```

### 3. Buffer Thinking Chunks

Don't update UI for every chunk - batch them:
```typescript
let buffer = '';
let timer = null;

eventSource.addEventListener('thinking_chunk', (e) => {
    buffer += JSON.parse(e.data).chunk;
    
    clearTimeout(timer);
    timer = setTimeout(() => {
        appendToUI(buffer);
        buffer = '';
    }, 50); // Flush every 50ms
});
```

### 4. Use Event Data for UI State

```typescript
const state = {
    agentActive: false,
    toolsRunning: [],
    approvalsPending: [],
};

eventSource.addEventListener('agent.start', () => {
    state.agentActive = true;
});

eventSource.addEventListener('action_detected', (e) => {
    const tool = JSON.parse(e.data).tool_id;
    state.toolsRunning.push(tool);
});

eventSource.addEventListener('action_result', (e) => {
    const tool = JSON.parse(e.data).description;
    state.toolsRunning = state.toolsRunning.filter(t => t !== tool);
});
```

### 5. Handle Reconnection

```typescript
let reconnectAttempts = 0;
const maxReconnects = 3;

eventSource.onerror = () => {
    eventSource.close();
    
    if (reconnectAttempts < maxReconnects) {
        reconnectAttempts++;
        setTimeout(() => startStreaming(), 1000 * reconnectAttempts);
    }
};
```

---

## Comparison with Other Frameworks

| Event Type | AgentKit | OpenAI | Anthropic | Pydantic AI |
|------------|----------|--------|-----------|-------------|
| Agent lifecycle | ✅ | ❌ | ❌ | ✅ |
| Content streaming | ✅ | ✅ | ✅ | ✅ |
| Tool execution | ✅ | ✅ | ✅ | ✅ |
| Multi-agent (handoff) | ✅ | ❌ | ❌ | ❌ |
| Multi-agent (collab) | ✅ | ❌ | ❌ | ❌ |
| Human approvals | ✅ | ❌ | ❌ | ❌ |
| Progress tracking | ✅ | ❌ | ❌ | ❌ |
| Total event types | 15 | ~20 | 8 | 6 |

**AgentKit's advantage**: Multi-agent coordination events (handoff, collaboration) and human-in-the-loop approvals that no other framework provides.

---

## FAQ

### Q: Do I need to handle all 15 events?

**A**: No! Start with just 2-3:
- `thinking_chunk` for streaming
- `agent.complete` for completion
- `error` for errors

Add more as needed.

### Q: Can I add custom events?

**A**: Yes, but it's generally better to use the existing events and put custom data in the `data` field. If you really need custom events, extend the `EventType` constants.

### Q: What's the performance impact?

**A**: Minimal. Events are sent over a buffered channel. The bottleneck is usually the LLM, not event streaming.

### Q: Can I replay events?

**A**: Yes! Use `EventRecorder`:
```go
recorder := agentkit.NewEventRecorder()
events := recorder.Record(agent.Run(ctx, msg))

// Later, replay them
for _, event := range recorder.Events() {
    // process event
}
```

### Q: What about WebSockets?

**A**: SSE is simpler for one-way agent→client streaming. Use WebSockets only if you need bi-directional communication (e.g., for approval flows).

---

## See Also

- [SSE Streaming Example](../examples/sse-streaming/) - Working example with web UI
- [Handoff Example](../examples/handoff/) - Multi-agent delegation
- [Collaboration Example](../examples/collaborate/) - Multi-agent discussion
- [Approval Example](../examples/approval/) - Human-in-the-loop patterns

---

## Summary

AgentKit's event system provides **15 semantic, actionable events** optimized for SSE streaming:

✅ **Simple**: Easy to understand, consistent naming
✅ **Practical**: Covers real use cases, not theoretical ones
✅ **Unique**: Multi-agent and approval events no one else has
✅ **Flexible**: Handle as few or as many as you need
✅ **Extensible**: Easy to add more events later

Start with the basics (`thinking_chunk`, `agent.complete`), then progressively enhance your UI with tool visibility, multi-agent visualization, and approval flows.

**The goal**: Make it enjoyable for developers to build rich, real-time agent experiences. 🚀
