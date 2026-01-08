# AgentKit

A Go framework for building LLM-powered agents with tool calling, streaming, and elegant DX.

[![Go Reference](https://pkg.go.dev/badge/github.com/darkostanimirovic/agentkit.svg)](https://pkg.go.dev/github.com/darkostanimirovic/agentkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkostanimirovic/agentkit)](https://goreportcard.com/report/github.com/darkostanimirovic/agentkit)

## Installation

```bash
go get github.com/darkostanimirovic/agentkit@latest
```

## Philosophy

AgentKit is inspired by Pydantic AI's design but adapted for Go best practices:

- **Explicit over implicit**: No magic decorators, clear function calls
- **Type-safe**: Leverages Go generics for context dependencies
- **Composable**: Builder pattern for tools, functional options for configuration
- **Channel-based**: Native Go channels for streaming events
- **Framework-agnostic**: Can be used with any database, web framework, or LLM provider
- **Modern API**: Uses OpenAI's Responses API for stateful conversations and advanced features

## Architecture

AgentKit uses OpenAI's **Responses API** (not the older Chat Completions API) for:

- **Stateful conversations**: Automatic conversation management with `previous_response_id`
- **Built-in tools**: Support for web search, file search, and other OpenAI-provided tools
- **Better streaming**: More robust streaming with server-sent events
- **Future-proof**: Access to the latest OpenAI features like reasoning models and structured outputs

The framework handles the complexity of:
- Converting between OpenAI tool formats and Response API formats
- Managing conversation state across multiple turns
- Tool execution and result handling
- Streaming response parsing and event emission

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/darkostanimirovic/agentkit"
)

func main() {
    // Create agent
    agent, err := agentkit.New(agentkit.Config{
        APIKey:       os.Getenv("OPENAI_API_KEY"),
        Model:        "gpt-4o-mini",
        SystemPrompt: buildSystemPrompt,
        MaxIterations: 5,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Register tools
    agent.AddTool(
        agentkit.NewTool("search").
            WithDescription("Search for information").
            WithParameter("query", agentkit.String().Required().WithDescription("Search query")).
            WithHandler(searchHandler).
            Build(),
    )

    // Run agent with streaming
    ctx := agentkit.WithDeps(context.Background(), myDeps)
    events := agent.Run(ctx, "Find information about Go best practices")

    for event := range events {
        switch event.Type {
        case agentkit.EventTypeThinkingChunk:
            fmt.Print(event.Data["chunk"])
        case agentkit.EventTypeActionDetected:
            fmt.Printf("Tool: %s\n", event.Data["description"])
        case agentkit.EventTypeFinalOutput:
            fmt.Printf("Done: %s\n", event.Data["response"])
        }
    }
}

func buildSystemPrompt(ctx context.Context) string {
    deps, err := agentkit.GetDeps[MyDeps](ctx)
    if err != nil {
        return "You are a helpful assistant."
    }
    return fmt.Sprintf("You are an assistant for %s", deps.UserName)
}

func searchHandler(ctx context.Context, args map[string]any) (any, error) {
    query := args["query"].(string)
    // Perform search...
    return map[string]any{
        "results": []string{"result1", "result2"},
    }, nil
}
```

## Core Concepts

### Agent

The orchestrator that manages LLM interactions, tool calling, and streaming.

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey:          os.Getenv("OPENAI_API_KEY"),
    Model:           "gpt-4o-mini",
    SystemPrompt:    buildPrompt,
    MaxIterations:   5,
    Temperature:     0.7,
    StreamResponses: true,
})
if err != nil {
    log.Fatal(err)
}
```

### Configuration

Key `Config` fields (all optional unless noted):

- `APIKey` (required unless `LLMProvider` is set)
- `Model` (validated; unknown models log a warning)
- `SystemPrompt` (func that builds instructions from context)
- `MaxIterations`, `Temperature`
- `StreamResponses` (stream SSE events vs. single response)
- `Retry`, `Timeout` (see sections below)
- `ConversationStore`, `Approval`
- `LLMProvider` (custom provider or `MockLLM`)
- `Logging`, `EventBuffer`
- `ParallelToolExecution`

### Tools

Tools are functions the LLM can call. Build them with a fluent API:

```go
tool := agentkit.NewTool("assign_team").
    WithDescription("Assign work item to a team").
    WithParameter("team_slug", agentkit.String().Required()).
    WithParameter("reasoning", agentkit.String().Optional()).
    WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
        teamSlug := args["team_slug"].(string)
        // Execute tool logic...
        return map[string]any{"success": true}, nil
    }).
    Build()

agent.AddTool(tool)
```

Defaults are sensible: if you don't supply formatters, AgentKit renders a pending message and a success/error summary based on the tool name or `error`/`success` fields.

```go
tool := agentkit.NewTool("assign_team").
    WithHandler(assignTeamHandler).
    WithPendingFormatter(func(_ string, args map[string]any) string {
        return fmt.Sprintf("Assigning to %s...", args["team_slug"])
    }).
    WithResultFormatter(func(_ string, result any) string {
        return fmt.Sprintf("✓ Assigned to %v", result)
    }).
    Build()
```

### Struct-Based Tools

Generate tool schemas from Go structs and get typed handler input.

```go
type SearchParams struct {
    Query  string   `json:"query" required:"true" desc:"Search query"`
    Labels []string `json:"labels"`
    Limit  int      `json:"limit" default:"10"`
}

toolBuilder, err := agentkit.NewStructTool("search", func(ctx context.Context, args SearchParams) (any, error) {
    return map[string]any{"hits": 3}, nil
})
if err != nil {
    log.Fatal(err)
}
tool := toolBuilder.Build()
agent.AddTool(tool)
```

### Complex Schemas

```go
tool := agentkit.NewTool("complex_search").
    WithParameter("filters", agentkit.Object().
        WithProperty("status", agentkit.String().WithEnum("open", "closed")).
        WithProperty("labels", agentkit.Array("string")).
        WithProperty("assignee", agentkit.Object().
            WithProperty("id", agentkit.String().Required()).
            WithProperty("name", agentkit.String().Optional()),
        ).
        Required(),
    ).
    Build()

tool = agentkit.NewTool("advanced").
    WithJSONSchema(myJSONSchema).
    Build()
```

### Approval Flows

Require human approval for sensitive tools:

```go
agent, _ := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Approval: &agentkit.ApprovalConfig{
        Tools: []string{"assign_team", "deploy"},
        Handler: func(ctx context.Context, req agentkit.ApprovalRequest) (bool, error) {
            // Persist request + wait for approval response.
            return true, nil
        },
    },
})
```

### Agents as Tools (Composition)

Agents can be composed by using one agent as a tool for another. This is done using the `AsTool` method, which automatically handles event bubbling so the parent agent (and its caller) receives events from the child agent seamlessly.

```go
researchAgent, _ := agentkit.New(researchConfig)

mainAgent, _ := agentkit.New(mainConfig)
mainAgent.AddTool(researchAgent.AsTool("researcher", "Can perform deep research on a topic"))
```

### Parallel Tool Execution

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4o-mini",
    ParallelToolExecution: &agentkit.ParallelConfig{
        Enabled:       true,
        MaxConcurrent: 3,
        SafetyMode:    agentkit.SafetyModeOptimistic, // Pessimistic disables parallel execution
    },
})
if err != nil {
    log.Fatal(err)
}

tool := agentkit.NewTool("serial_tool").
    WithConcurrency(agentkit.ConcurrencySerial).
    WithHandler(handler).
    Build()
```

### Observability & Logging

AgentKit provides middleware hooks and configurable logging.

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4o-mini",
    Logging: &agentkit.LoggingConfig{
        Level:           slog.LevelInfo,
        LogPrompts:      true,
        LogResponses:    true,
        LogToolCalls:    true,
        RedactSensitive: true,
        PromptLogPath:   "/var/log/agentkit/prompts.log",
    },
})
if err != nil {
    log.Fatal(err)
}

agent.Use(myMiddleware)
```

### Timeouts & Retries

Configure overall run time, per-LLM call, per-tool, and stream read timeouts. Add retry backoff for transient API errors.

```go
agent, _ := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Retry: &agentkit.RetryConfig{
        MaxRetries:   3,
        InitialDelay: time.Second,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
    },
    Timeout: &agentkit.TimeoutConfig{
        AgentExecution: 2 * time.Minute,
        LLMCall:        30 * time.Second,
        ToolExecution:  15 * time.Second,
        StreamChunk:    5 * time.Second,
    },
})
```

### Testing With Mock LLM

```go
mock := agentkit.NewMockLLM().
    WithResponse("Searching...", []agentkit.ToolCall{
        {Name: "search", Args: map[string]any{"query": "timeout"}},
    }).
    WithFinalResponse("Done")

agent, err := agentkit.New(agentkit.Config{
    Model:           "gpt-4o-mini",
    LLMProvider:     mock,
    StreamResponses: false,
    Logging: &agentkit.LoggingConfig{
        LogPrompts: false,
    },
})
if err != nil {
    log.Fatal(err)
}
```

### Trace IDs

```go
ctx := agentkit.WithTraceID(context.Background(), "trace-123")
ctx = agentkit.WithSpanID(ctx, "span-456")
events := agent.Run(ctx, "triage issue")
```

### Event Utilities

```go
events := agent.Run(ctx, "triage issue")
filtered := agentkit.FilterEvents(events, agentkit.EventTypeActionDetected, agentkit.EventTypeFinalOutput)

recorder := agentkit.NewEventRecorder()
recorded := recorder.Record(filtered)

for range recorded {
    // consume filtered events
}

_ = recorder.Events() // replay later
```

### Context & Dependencies

Pass dependencies through context with type safety:

```go
type MyDeps struct {
    DB     *database.DB
    UserID string
}

// Add to context
ctx := agentkit.WithDeps(context.Background(), MyDeps{
    DB:     db,
    UserID: "123",
})

// Retrieve in tools
func myHandler(ctx context.Context, args map[string]any) (any, error) {
    deps, err := agentkit.GetDeps[MyDeps](ctx)
    if err != nil {
        return nil, err
    }
    // Use deps.DB, deps.UserID...
}
```

### Events

Stream events during agent execution:

```go
events := agent.Run(ctx, "user message")

for event := range events {
    switch event.Type {
    case agentkit.EventTypeThinkingChunk:
        // LLM thinking process
    case agentkit.EventTypeActionDetected:
        // Tool about to be called
    case agentkit.EventTypeActionResult:
        // Tool execution result
    case agentkit.EventTypeFinalOutput:
        // Agent finished
    case agentkit.EventTypeError:
        // Error occurred
    }
}
```

### Conversation Store

Persist multi-turn conversations and resume later:

```go
store := agentkit.NewMemoryConversationStore()
agent, _ := agentkit.New(agentkit.Config{
    APIKey:            os.Getenv("OPENAI_API_KEY"),
    ConversationStore: store,
})

ctx := agentkit.WithConversation(context.Background(), "conv-123")
events := agent.Run(ctx, "continue where we left off")
```

## Real-World Examples

### Multi-Turn Conversation (Persistence)

```go
store := agentkit.NewMemoryConversationStore()
agent, _ := agentkit.New(agentkit.Config{
    APIKey:            os.Getenv("OPENAI_API_KEY"),
    Model:             "gpt-4o-mini",
    ConversationStore: store,
})

ctx := agentkit.WithConversation(context.Background(), "conv-123")
events := agent.Run(ctx, "continue where we left off")
```

### RAG With Vector DB

```go
tool := agentkit.NewTool("retrieve_context").
    WithParameter("query", agentkit.String().Required()).
    WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
        hits := vectorDB.Search(args["query"].(string))
        return map[string]any{"chunks": hits}, nil
    }).
    Build()
```

### Multi-Agent Collaboration

```go
triageAgent, _ := agentkit.New(triageConfig)
assignAgent, _ := agentkit.New(assignConfig)

mainAgent, _ := agentkit.New(mainConfig)
_ = mainAgent.AddSubAgent("triage", triageAgent)
_ = mainAgent.AddSubAgent("assign", assignAgent)
```

### Production Deployment Tips

```go
agent, _ := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Retry: &agentkit.RetryConfig{MaxRetries: 3},
    Timeout: &agentkit.TimeoutConfig{
        AgentExecution: 2 * time.Minute,
        LLMCall:        30 * time.Second,
        ToolExecution:  15 * time.Second,
    },
})
```

### Error Handling Patterns

```go
for event := range agent.Run(ctx, "do work") {
    if event.Type == agentkit.EventTypeError {
        log.Printf("agent error: %v", event.Data["error"])
    }
}
```

### Performance Optimization

```go
agent, _ := agentkit.New(agentkit.Config{
    ParallelToolExecution: &agentkit.ParallelConfig{Enabled: true, MaxConcurrent: 4},
    EventBuffer:           100,
})
```

### Security Best Practices

```go
agent, _ := agentkit.New(agentkit.Config{
    Logging: &agentkit.LoggingConfig{
        RedactSensitive: true,
        LogPrompts:      false,
    },
    Approval: &agentkit.ApprovalConfig{
        Tools: []string{"deploy", "close_issue"},
        Handler: approvalHandler,
    },
})
```

## API Reference

### Agent Methods

- `New(cfg Config) (*Agent, error)` - Create new agent
- `AddTool(tool Tool)` - Register a tool
- `AddSubAgent(name string, sub *Agent)` - Register a sub-agent tool
- `Use(m Middleware)` - Register middleware hooks
- `Run(ctx context.Context, userMessage string) <-chan Event` - Execute agent

### Config & Context

- `Config` - Agent configuration (model, retries, timeouts, logging, etc.)
- `DefaultConfig()` - Default configuration values
- `WithDeps(ctx, deps)` / `GetDeps[T](ctx)` - Type-safe dependency injection
- `WithConversation(ctx, id)` / `GetConversationID(ctx)` - Conversation IDs
- `WithTraceID(ctx, id)` / `WithSpanID(ctx, id)` - Trace correlation

### Approvals

- `ApprovalConfig` - Tool approval settings
- `ApprovalHandler` / `ApprovalRequest` - Approval callback types

### Retry & Timeout

- `RetryConfig`, `DefaultRetryConfig()`, `WithRetry(...)`
- `TimeoutConfig`, `DefaultTimeoutConfig()`, `NoTimeouts()`

### Conversation Store

- `ConversationStore` - Persistence interface
- `NewMemoryConversationStore()` - In-memory store for tests/dev

### Tool Builder

- `NewTool(name string) *ToolBuilder` - Start building a tool
- `NewStructTool(name string, handler)` - Build from struct tags
- `SchemaFromStruct(sample any)` - Generate JSON schema from struct tags
- `WithDescription(desc string)` - Set tool description
- `WithParameter(name string, schema ParameterSchema)` - Add parameter
- `WithJSONSchema(schema map[string]any)` - Set raw JSON schema
- `WithConcurrency(mode ConcurrencyMode)` - Control parallel execution
- `WithHandler(handler ToolHandler)` - Set execution handler
- `Build() Tool` - Construct the tool

### Parameter Schemas

- `String()` - String parameter
- `Array(itemType string)` - Array parameter
- `ArrayOf(itemSchema *ParameterSchema)` - Array of complex items
- `Object()` - Object schema builder
- `WithDescription(desc string)` - Add description
- `Required()` - Mark as required
- `Optional()` - Mark as optional
- `WithEnum(values ...string)` - Restrict to enum values

### Parallel Tool Execution

- `ParallelConfig` - Tool execution configuration
- `ConcurrencySerial` - Tool runs exclusively
- `ConcurrencyParallel` - Tool can run in parallel

### Event Helpers

- `ThinkingChunk(chunk string) Event`
- `ActionDetected(toolName, toolID string) Event`
- `ActionResult(toolName string, result any) Event`
- `FinalOutput(summary, response string) Event`
- `Error(err error) Event`

### Event Utilities

- `FilterEvents(input <-chan Event, types ...EventType) <-chan Event`
- `NewEventRecorder() *EventRecorder`

### Testing Utilities

- `LLMProvider` - Provider abstraction
- `NewMockLLM()` - Deterministic LLM for tests

## Design Principles

1. **Explicit Configuration**: No hidden magic, everything is configured explicitly
2. **Type Safety**: Generics for dependency injection, strong typing throughout
3. **Composability**: Tools are independent units that compose together
4. **Streaming First**: Built for real-time SSE responses
5. **Error Handling**: Errors are events, gracefully handled
6. **Go Idioms**: Follows Go best practices (builders, options, interfaces)

## Comparison to Other Frameworks

| Feature | AgentKit (Go) | Pydantic AI (Python) | LangChain (Python) | OpenAI SDK |
|---------|---------------|----------------------|--------------------|------------|
| Tool registration | Builder API | Decorators | Chains/Tools | Functions/Tools |
| Streaming | Channel events | async iter | callbacks | stream events |
| Typed deps | `WithDeps[T]` | RunContext | custom | manual |
| Mocking | `MockLLM` + `LLMProvider` | test clients | mocks | stub client |
| Parallel tools | Config + per-tool concurrency | custom | limited | model-driven |

### When to Use AgentKit

- You want Go-native APIs with explicit configuration and no magic decorators.
- You need streaming events and tool execution in a single agent loop.
- You want easy testability without calling real LLMs.

### Performance Characteristics

- Streaming-first design keeps UI responsive with minimal buffering.
- Parallel tool execution is configurable with per-tool concurrency gates.
- Prompt logging is optional and can be disabled for high-throughput systems.

### Limitations & Gotchas

- Struct-tag schemas are best-effort; complex validation is still manual.
- Tool outputs are returned as JSON-compatible values; custom types should be mapped.
- The underlying LLM provider still controls which tools are called.

## Testing

AgentKit has comprehensive test coverage:

```bash
# Run all tests
go test ./pkg/agentkit/...

# Run with coverage
go test ./pkg/agentkit/... -cover

# Generate coverage report
go test ./pkg/agentkit/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Coverage Strategy:**

- **100% coverage** for all public APIs (events, tools, context, builders)
- **Integration tests** verify end-to-end tool execution and event streaming
- **Agent orchestration** (Run method) is tested via the MockLLM and LLMProvider hooks
- Target: 85%+ for framework APIs (achieved), full integration testing for LLM orchestration

**Test Categories:**

1. **Unit tests**: Event helpers, tool builders, context management, parameter schemas
2. **Integration tests**: Tool registration → execution, multi-tool scenarios, context flow
3. **Real-world usage**: Inbox agent implementation serves as integration test

## Project Structure

```
agentkit/
├── *.go              # Core library (public API)
├── *_test.go         # Tests
├── examples/         # Example applications
│   ├── basic/        # Simple agent example
│   ├── multi-agent/  # Multi-agent orchestration
│   └── rag/          # RAG implementation
├── internal/         # Private packages
│   └── testutil/     # Test utilities
└── docs/             # Documentation
```

See [docs/PROJECT_STRUCTURE.md](docs/PROJECT_STRUCTURE.md) for detailed information.

## Examples

Check out the [examples/](examples/) directory for complete working examples:
- **Basic Agent** - Simple tool usage and event handling
- **Multi-Agent** - Agent composition and orchestration
- **RAG** - Retrieval augmented generation

## Documentation

- [Usage Guide](docs/USAGE.md) - Installation and usage
- [Migration Guide](docs/MIGRATION.md) - Upgrading between versions
- [Project Structure](docs/PROJECT_STRUCTURE.md) - Code organization
- [Community Feedback](docs/COMMUNITY_FEEDBACK.md) - Feature requests and feedback

## Future Enhancements

- [ ] Tool result validation
- [ ] Multi-agent orchestration
- [ ] Struct-tag schema generation
- [ ] Parallel tool execution control
- [ ] More provider adapters (Anthropic, etc.)

## License

MIT License - see [LICENSE](LICENSE) for details
