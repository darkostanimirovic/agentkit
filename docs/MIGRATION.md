# Migration to OpenAI Responses API

## Overview

AgentKit has been migrated from OpenAI's Chat Completions API to the new **Responses API**. This migration provides better conversation management, improved streaming, and access to newer OpenAI features.

## What Changed

### API Endpoint
- **Before**: `/v1/chat/completions`
- **After**: `/v1/responses`

### Conversation State Management
- **Before**: Manually managed message arrays passed on each request
- **After**: Automatic state management using `previous_response_id` parameter

### Request Structure
```go
// Before (Chat Completions)
openai.ChatCompletionRequest{
    Model:    "gpt-4o-mini",
    Messages: []ChatCompletionMessage{...},
    Tools:    []Tool{...},
}

// After (Responses API)
ResponseRequest{
    Model:              "gpt-4o-mini",
    Input:              []ResponseInput{...},
    Instructions:       "system prompt",
    PreviousResponseID: "resp_xxx",
    Tools:              []ResponseTool{...},
}
```

### Response Structure
```go
// Before (Chat Completions)
resp.Choices[0].Message.Content
resp.Choices[0].Message.ToolCalls

// After (Responses API)
resp.Output[0].Content[0].Text
resp.Output[0].ToolCalls
```

## Benefits

### 1. Stateful Conversations
The Responses API automatically manages conversation history:
```go
// First turn
req := ResponseRequest{
    Model: "gpt-4o-mini",
    Input: userMessage,
}
resp, _ := client.CreateResponse(ctx, req)

// Second turn - just reference the previous response
req2 := ResponseRequest{
    Model:              "gpt-4o-mini",
    PreviousResponseID: resp.ID,  // Automatically includes history
}
resp2, _ := client.CreateResponse(ctx, req2)
```

### 2. Built-in Tools
Access OpenAI's built-in tools like web search and file search:
```go
req := ResponseRequest{
    Model: "gpt-4o-mini",
    Tools: []ResponseTool{
        {Type: "web_search"},
        {Type: "file_search"},
    },
}
```

### 3. Improved Streaming
Better streaming support with server-sent events:
```go
stream, _ := client.CreateResponseStream(ctx, req)
for {
    chunk, err := stream.Recv()
    if err == io.EOF {
        break
    }
    // Process delta content
    if chunk.Delta != nil {
        fmt.Print(chunk.Delta.Content[0].Text)
    }
}
```

### 4. Future Features
Access to newer OpenAI capabilities:
- Reasoning models (o-series)
- Structured outputs with JSON schema
- Conversation compaction
- Background processing
- Extended prompt caching

## Implementation Details

### Custom HTTP Client
Since the go-openai library doesn't support the Responses API yet, we implemented a custom client:

```go
type ResponsesClient struct {
    apiKey     string
    httpClient *http.Client
}

func (c *ResponsesClient) CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error) {
    // Direct HTTP calls to /v1/responses
}
```

### Tool Execution
Tool execution now works with the Responses API format:

```go
func (a *Agent) handleResponseToolCalls(ctx context.Context, responseID string, toolCalls []ResponseToolCall, events chan<- Event) bool {
    for _, toolCall := range toolCalls {
        // Execute tool
        result, err := tool.Execute(ctx, toolCall.Function.Arguments)

        // Result is automatically included in next request via previous_response_id
    }
    return true
}
```

### Streaming Events
The event system remains unchanged for backward compatibility:

```go
for event := range agent.Run(ctx, userMessage) {
    switch event.Type {
    case EventTypeThinkingChunk:
        // Assistant thinking
    case EventTypeActionDetected:
        // Tool call detected
    case EventTypeActionResult:
        // Tool execution result
    case EventTypeFinalOutput:
        // Final response
    }
}
```

## Migration Checklist

If you're upgrading existing code:

- ✅ No changes needed to tool definitions
- ✅ No changes needed to event handling
- ✅ No changes needed to system prompts
- ✅ Agent configuration remains the same
- ✅ All existing tests pass

The migration is **backward compatible** at the API level. Your existing AgentKit code will work without changes.

## Testing

All existing tests pass with the new implementation:

```bash
cd go-backend
go test ./pkg/agentkit/... -v
```

## Resources

- [OpenAI Responses API Documentation](https://platform.openai.com/docs/api-reference/responses)
- [Conversation State Guide](https://platform.openai.com/docs/guides/conversation-state)
- [Responses API Quickstart](https://platform.openai.com/docs/quickstart?api-mode=responses)

## Future Work

Potential enhancements:
- Support for conversation objects (alternative to previous_response_id)
- Background response processing
- Conversation compaction for long conversations
- Integration with OpenAI's built-in tools (web search, file search)
- Structured output with JSON schema validation
