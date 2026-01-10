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

**For reasoning models, use `ReasoningEffort` instead of `Temperature`:**

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey:          os.Getenv("OPENAI_API_KEY"),
    Model:           "o1-mini",
    SystemPrompt:    buildPrompt,
    MaxIterations:   5,
    ReasoningEffort: agentkit.ReasoningEffortHigh, // none, minimal, low, medium, high, or xhigh
    StreamResponses: true,
})
```

> **Note**: If you specify `ReasoningEffort`, it will be used instead of `Temperature`. Only set one or the other based on your model's capabilities.

### Configuration

Key `Config` fields (all optional unless noted):

- `APIKey` (required unless `LLMProvider` is set)
- `Model` (any OpenAI model name)
- `SystemPrompt` (func that builds instructions from context)
- `MaxIterations`, `Temperature` (for GPT models)
- `ReasoningEffort` (for reasoning models: use constants `ReasoningEffortNone`, `ReasoningEffortMinimal`, `ReasoningEffortLow`, `ReasoningEffortMedium`, `ReasoningEffortHigh`, or `ReasoningEffortXHigh`; if set, `Temperature` is ignored)
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

Generate tool schemas from Go structs and get typed handler input. Structured Outputs are enabled by default.

```go
type SearchParams struct {
    Query  string   `json:"query" required:"true" desc:"Search query"`
    Labels []string `json:"labels" desc:"Optional filter labels"`
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

### OpenAI Structured Outputs

AgentKit automatically enables **OpenAI Structured Outputs** for all tools by default. This ensures the model's output always matches your schema exactly, with guaranteed type-safety and no hallucinated fields.

**Key features:**
- ✅ All tools use `strict: true` by default
- ✅ Automatic `additionalProperties: false` for all object schemas (added at Build time)
- ✅ Automatic `type: "object"` and `properties: {}` for tools with empty/minimal schemas
- ✅ Optional fields use `anyOf` with `null` type
- ✅ All parameter names are in the `required` array (with null unions for optional)
- ✅ Works with `WithParameter()`, `WithRawParameters()`, `WithJSONSchema()`, and even tools with no parameters

**Two ways to define schemas:**

#### 1. StructToSchema (Recommended for complex schemas)

Use Go structs with tags to automatically generate schemas. Supports nested objects, enums, descriptions, and more.

```go
type SearchFilters struct {
    EmailDomain string `json:"email_domain" desc:"Filter by email domain"`
    Status      string `json:"status" required:"true" enum:"active,inactive" desc:"User status"`
    AgeRange    struct {
        Min int `json:"min" desc:"Minimum age"`
        Max int `json:"max" desc:"Maximum age"`
    } `json:"age_range"`
}

filtersSchema, _ := agentkit.StructToSchema[SearchFilters]()
tool := agentkit.NewTool("search_users").
    WithParameter("filters", filtersSchema).
    Build()
```

**Supported struct tags:**
- `json`: Field name (use `"-"` to skip field)
- `required:"true"`: Mark field as required (omit for optional fields)
- `desc`: Field description
- `enum`: Comma-separated allowed values
- `default`: Default value

#### 2. Fluent API (Good for simple inline schemas)

```go
// Structured Outputs enabled by default
tool := agentkit.NewTool("create_user").
    WithParameter("name", agentkit.String().Required()).
    WithParameter("email", agentkit.String().Required()).
    WithParameter("nickname", agentkit.String().Optional()). // Uses anyOf with null
    Build()

// Disable strict mode only if needed (not recommended)
tool := agentkit.NewTool("legacy_tool").
    WithParameter("data", agentkit.String()).
    WithStrictMode(false). // Disables Structured Outputs
    Build()
```

### Complex Schemas

```go
tool := agentkit.NewTool("complex_search").
    WithParameter("filters", agentkit.Object().
        WithProperty("status", agentkit.String().WithEnum("open", "closed")).
        WithProperty("labels", agentkit.Array("string")).
        WithProperty("assignee", agentkit.Object().
            WithProperty("id", agentkit.String().Required()).
            WithProperty("name", agentkit.String().Optional()), // Nested optional field
        ).
        Required(),
    ).
    Build()

// Array of complex objects
tool := agentkit.NewTool("batch_update").
    WithParameter("users", agentkit.ArrayOf(
        agentkit.Object().
            WithProperty("id", agentkit.String().Required()).
            WithProperty("name", agentkit.String().Required()),
    ).Required()).
    Build()

// Raw JSON Schema for maximum control
// Note: additionalProperties: false is automatically added when strict mode is enabled
tool := agentkit.NewTool("advanced").
    WithJSONSchema(map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{"type": "string"},
        },
        "required": []string{"query"},
        // additionalProperties: false added automatically
    }).
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

### Multi-Agent Coordination

AgentKit provides two natural patterns for agent coordination, mimicking how real people work together:

#### 1. Handoffs - Delegation

When one agent delegates work to another who works independently:

```go
// Create specialized agents
researchAgent, _ := agentkit.New(researchConfig)
coordinatorAgent, _ := agentkit.New(coordinatorConfig)

// Direct handoff
result, err := coordinatorAgent.Handoff(
    ctx,
    researchAgent,
    "Research the top 3 Go web frameworks",
    agentkit.WithIncludeTrace(true), // Optional: see how they worked
    agentkit.WithMaxTurns(10),
)

fmt.Printf("Research: %s\n", result.Response)
```

**As a Tool** (LLM decides when to delegate):

```go
// The description tells the LLM WHEN to use this tool.
// The LLM will provide the actual task when it calls the tool.
coordinatorAgent.AddTool(
    researchAgent.AsHandoffTool(
        "research_agent",                           // Tool name
        "Delegate to research specialist when you need deep research on technical topics", // When to use
    ),
)

// When coordinator runs, the LLM autonomously decides:
// Tool call: { "tool": "research_agent", "parameters": { "task": "Research top 3 Go web frameworks" } }
//                                                          ^^^^^ LLM provides this dynamically

// Or create reusable configuration
handoffConfig := agentkit.NewHandoffConfiguration(
    coordinatorAgent,
    researchAgent,
    agentkit.WithIncludeTrace(true),
)

coordinatorAgent.AddTool(
    handoffConfig.AsTool(
        "research_agent",
        "Delegate to research specialist when you need deep research on technical topics",
    ),
)
```

#### 2. Collaboration - Peer Discussion

When multiple agents need to discuss a topic as equals:

```go
// Create a collaborative session
session := agentkit.NewCollaborationSession(
    facilitatorAgent,  // Runs the discussion
    engineerAgent,     // Peers who contribute
    designerAgent,
    productAgent,
)

// Discuss a topic
result, err := session.Discuss(
    ctx,
    "Should we use WebSockets or Server-Sent Events?",
)

fmt.Printf("Decision: %s\n", result.FinalResponse)
```

**As a Tool** (LLM decides when to collaborate and what to discuss):

```go
designSession := agentkit.NewCollaborationSession(
    facilitatorAgent,
    engineerAgent,
    designerAgent,
    productAgent,
)

coordinatorAgent.AddTool(
    designSession.AsTool(
        "design_discussion",
        "Form a collaborative design discussion on a specific topic",
    ),
)

// LLM will call with: {"topic": "authentication flow design"}
```

**When to use what:**
- **Handoff**: One agent needs focused work done independently ("Go research this and report back")
- **Collaboration**: Multiple perspectives needed on a topic ("Let's all discuss this together")

See [`docs/COORDINATION.md`](docs/COORDINATION.md) for comprehensive examples and patterns.

### Agents as Tools (Composition)

> **Note**: The handoff and collaboration patterns above are preferred for new code. The methods below are maintained for backward compatibility.

Agents can be composed by using one agent as a tool for another. There are two approaches:

#### 1. Using `AsTool` (Simple)

The quickest way to add an agent as a tool:

```go
researchAgent, _ := agentkit.New(researchConfig)

mainAgent, _ := agentkit.New(mainConfig)
mainAgent.AddTool(researchAgent.AsTool("researcher", "Can perform deep research on a topic"))
```

#### 2. Using `NewSubAgentTool` (Advanced)

For more control, including optional execution trace visibility:

```go
// Basic: sub-agent returns just its response
tool, _ := agentkit.NewSubAgentTool(
    agentkit.SubAgentConfig{
        Name:        "researcher",
        Description: "Performs deep research on a topic",
    },
    researchAgent,
)
mainAgent.AddTool(tool)

// Advanced: include sub-agent's reasoning trace in parent's context
// (useful for debugging or when parent needs to learn from sub-agent's approach)
tool, _ := agentkit.NewSubAgentTool(
    agentkit.SubAgentConfig{
        Name:         "researcher",
        Description:  "Performs deep research on a topic",
        IncludeTrace: true,  // Parent agent sees sub-agent's execution steps
    },
    researchAgent,
)
```

**Note:** `IncludeTrace` controls whether the sub-agent's execution trace (reasoning, tool calls, decisions) is included in the result sent to the parent agent. This consumes additional tokens in the parent's context window, so enable only when needed for debugging or hierarchical learning scenarios.

Both approaches automatically handle event bubbling so the parent agent receives events from child agents.

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

AgentKit separates **agent events** from **internal logs**:

- **Events** (streamed via channel): What the agent is doing - thinking chunks, tool calls, results, final output. This is the primary output for CLI applications and UIs.
- **Logs** (via slog): Internal diagnostics for debugging - iteration counts, chunk metadata, errors. These go to stderr by default (following Unix conventions).

#### Clean CLI Output (Recommended for most CLI apps)

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey:  os.Getenv("OPENAI_API_KEY"),
    Model:   "gpt-4o-mini",
    Logging: agentkit.LoggingConfig{}.Silent(), // Disable internal logs
})

// Handle events only - no log pollution
for event := range agent.Run(ctx, "do something") {
    switch event.Type {
    case agentkit.EventTypeThinkingChunk:
        fmt.Print(event.Data["chunk"]) // Clean stdout
    case agentkit.EventTypeFinalOutput:
        fmt.Printf("\n%s\n", event.Data["response"])
    }
}
```

#### Development/Debugging

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey:  os.Getenv("OPENAI_API_KEY"),
    Model:   "gpt-4o-mini"),
    Logging: agentkit.LoggingConfig{}.Verbose(), // Debug-level logs to stderr
})
```

#### Custom Logging

```go
agent, err := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4o-mini",
    Logging: &agentkit.LoggingConfig{
        Level:           slog.LevelInfo,
        Handler:         customHandler,        // Use your own handler
        LogPrompts:      true,                 // Log prompts to file
        LogResponses:    true,
        LogToolCalls:    true,
        RedactSensitive: true,
        PromptLogPath:   "/var/log/agentkit/prompts.log",
    },
})
```

**Default Behavior:**
- Logs go to stderr (not stdout) following Unix conventions
- Use `.Silent()` for CLI apps where you only want events
- Use `.Verbose()` for development debugging

**Migration Note:** Prior versions logged to stdout by default. This has been changed to stderr to prevent log pollution in CLI applications. To restore the old behavior:
```go
Logging: &agentkit.LoggingConfig{
    Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
}
```

AgentKit also provides middleware hooks for custom observability:

```go
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
- `Use(m Middleware)` - Register middleware hooks
- `Run(ctx context.Context, userMessage string) <-chan Event` - Execute agent

### Coordination

**Handoffs:**
- `agent.Handoff(ctx, to, task, ...opts)` - Delegate task to another agent
- `agent.AsHandoffTool(name, desc, ...opts)` - Convert agent to handoff tool
- `NewHandoffConfiguration(from, to, ...opts)` - Create reusable handoff config
- `config.AsTool(name, desc)` - Convert handoff config to tool
- `WithIncludeTrace(bool)`, `WithMaxTurns(int)`, `WithContext(HandoffContext)` - Handoff options

**Collaborations:**
- `NewCollaborationSession(facilitator, ...peers)` - Create collaboration session
- `session.Discuss(ctx, topic)` - Execute collaborative discussion
- `session.Configure(...opts)` - Add options to session
- `session.AsTool(name, desc)` - Convert session to tool
- `WithMaxRounds(int)`, `WithRoundTimeout(duration)`, `WithCaptureHistory(bool)` - Collaboration options

### Legacy Agent Composition

- `AddSubAgent(name string, sub *Agent)` - Register a sub-agent tool (legacy)
- `NewSubAgentTool(config, agent)` - Create sub-agent tool with options (legacy)

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
- `StructToSchema[T any]() (*ParameterSchema, error)` - Convert struct type to ParameterSchema (recommended)
- `WithDescription(desc string)` - Set tool description
- `WithParameter(name string, schema ParameterSchema)` - Add parameter
- `WithJSONSchema(schema map[string]any)` - Set raw JSON schema
- `WithConcurrency(mode ConcurrencyMode)` - Control parallel execution
- `WithStrictMode(strict bool)` - Enable/disable OpenAI Structured Outputs (default: true)
- `WithHandler(handler ToolHandler)` - Set execution handler
- `Build() Tool` - Construct the tool

### Parameter Schemas

- `String()` - String parameter
- `Array(itemType string)` - Array parameter
- `ArrayOf(itemSchema *ParameterSchema)` - Array of complex items
- `Object()` - Object schema builder
- `StructToSchema[T any]()` - Generate schema from Go struct with tags
- `WithProperty(name string, schema *ParameterSchema)` - Add object property
- `WithDescription(desc string)` - Add description
- `Required()` - Mark as required
- `Optional()` - Mark as optional (uses anyOf with null in strict mode)
- `WithEnum(values ...string)` - Restrict to enum values
- `ToMap()` - Convert to map for OpenAI (no strict mode wrapping)
- `ToMapStrict()` - Convert with strict mode (anyOf for optional fields)

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
- [Tracing Guide](docs/TRACING.md) - LLM observability with Langfuse
- [Migration Guide](docs/MIGRATION.md) - Upgrading between versions
- [Project Structure](docs/PROJECT_STRUCTURE.md) - Code organization
- [Community Feedback](docs/COMMUNITY_FEEDBACK.md) - Feature requests and feedback

## LLM Tracing

AgentKit includes built-in support for LLM observability through an extensible tracing interface. Currently supports Langfuse via OpenTelemetry.

```go
// Create Langfuse tracer
tracer, err := agentkit.NewLangfuseTracer(agentkit.LangfuseConfig{
    PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
    SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
    BaseURL:   "https://cloud.langfuse.com",
    Enabled:   true,
})
if err != nil {
    log.Fatal(err)
}
defer tracer.Shutdown(context.Background())

// Create agent with tracing
agent, err := agentkit.New(agentkit.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Tracer: tracer,  // Enable tracing
})
```

Traces automatically include:
- Agent execution flows
- LLM generations with token usage and costs
- Tool executions with inputs and outputs
- Error details and timing information

See [docs/TRACING.md](docs/TRACING.md) for complete setup instructions.

## Future Enhancements

- [ ] Tool result validation
- [ ] Multi-agent orchestration
- [ ] Struct-tag schema generation
- [ ] Parallel tool execution control
- [ ] More provider adapters (Anthropic, etc.)

## License

MIT License - see [LICENSE](LICENSE) for details
