# AgentKit Community Feedback & Improvement Roadmap

Feedback from developers experienced with OpenAI SDK, LangChain, Pydantic AI, and similar frameworks.

---

## Recent Updates (Jan 1, 2026)

- **Phase 4**: Refactored streaming tool-call assembly and sub-agent tooling for clarity, lower complexity, and lint compliance.
- **Phase 5**: Hardened SSE parsing/tests and added prompt log path sanitization to improve reliability and safety.

## Phase 1: Critical Developer Experience Issues 🔥

**Priority**: Immediate - These issues cause daily friction

### 1.1 Tool Definition is Too Verbose

**Current Problem**:
```go
// Need 3 separate functions for one tool!
func AssignTeamTool() agentkit.Tool {
    return agentkit.NewTool("assign_team").
        WithHandler(assignTeamHandler).
        WithPendingFormatter(assignTeamPendingFormatter).  // ❌ Annoying
        WithResultFormatter(assignTeamResultFormatter).     // ❌ Annoying
        Build()
}

func assignTeamHandler(ctx context.Context, args map[string]any) (any, error) { ... }
func assignTeamPendingFormatter(_ string, args map[string]any) string { ... }
func assignTeamResultFormatter(_ string, result any) string { ... }
```

**Community Feedback**:
> "Why do I need 3 functions? This is way more verbose than Pydantic AI decorators."

**Solution**:
- Provide sensible default formatters
- Make formatters optional
- Support inline formatters with lambda-like syntax
- Add struct tags for automatic formatting

**Proposed API**:
```go
// Option 1: Minimal (use defaults)
tool := agentkit.NewTool("assign_team").
    WithHandler(assignTeamHandler).
    Build()  // Auto-formats as "Running assign_team..." and "✓ assign_team completed"

// Option 2: Inline formatters
tool := agentkit.NewTool("assign_team").
    WithHandler(assignTeamHandler).
    WithFormatters(
        func(args map[string]any) string {
            return fmt.Sprintf("Assigning to %s...", args["team_slug"])
        },
        func(result any) string {
            return fmt.Sprintf("✓ Assigned to %s", result.(AssignTeamResult).TeamSlug)
        },
    ).
    Build()

// Option 3: Struct-based with tags (future)
type AssignTeamTool struct {
    TeamSlug  string `json:"team_slug" desc:"Team to assign to" format:"Assigning to {team_slug}"`
    Reasoning string `json:"reasoning" desc:"Assignment reasoning" optional:"true"`
}
```

### 1.2 Type Assertions Everywhere

**Current Problem**:
```go
func assignTeamResultFormatter(_ string, result any) string {
    // ❌ Manual type assertions every time
    if typed, ok := result.(AssignTeamResult); ok {
        if typed.Error != "" {
            return fmt.Sprintf("✗ %s", typed.Error)
        }
        return fmt.Sprintf("✓ Assigned to %s", typed.AssignedTeam)
    }
    // ❌ Fallback for map[string]any
    if resultMap, ok := result.(map[string]any); ok { ... }
    return "✓ Assigned team"
}
```

**Community Feedback**:
> "The `any` return type kills type safety. Pydantic AI has typed tools - why doesn't this?"

**Solution**: Use generics for type-safe tools

**Proposed API**:
```go
// Define typed tool handler
type AssignTeamHandler = agentkit.TypedHandler[AssignTeamArgs, AssignTeamResult]

func assignTeamHandler(ctx context.Context, args AssignTeamArgs) (AssignTeamResult, error) {
    deps := agentkit.MustGetDeps[Deps](ctx)
    // No type assertions needed!
    team, err := deps.DB.Queries.GetTeamBySlug(ctx, args.TeamSlug)
    // ...
    return AssignTeamResult{
        Success: true,
        AssignedTeam: team.Name,
    }, nil
}

// Register with full type safety
tool := agentkit.NewTypedTool("assign_team", assignTeamHandler).
    WithDescription("Assign work item to team").
    Build()
```

### 1.3 Panic Instead of Errors

**Current Problem**:
```go
func MustGetDeps[T any](ctx context.Context) T {
    deps, ok := GetDeps[T](ctx)
    if !ok {
        panic("agentkit: dependencies not found in context")  // ❌ Not idiomatic Go!
    }
    return deps
}
```

**Community Feedback**:
> "Go developers don't use panics for control flow. This forces me to use defer/recover."

**Solution**: Provide both versions, encourage error returns

**Proposed API**:
```go
// Preferred: Return error
func toolHandler(ctx context.Context, args map[string]any) (any, error) {
    deps, err := agentkit.GetDeps[Deps](ctx)
    if err != nil {
        return nil, fmt.Errorf("missing deps: %w", err)
    }
    // Use deps safely
}

// Keep MustGetDeps for convenience but document when to use it
// (only in controlled environments where deps are guaranteed)
```

### 1.4 Silent Config Validation Issues

**Current Problem**:
```go
func New(cfg Config) *Agent {
    if cfg.Model == "" {
        cfg.Model = "gpt-4o-mini"  // ❌ Silent override
    }
    // No validation if model exists or is typo: "gpt-5.2" ← from your actual code!
}
```

**Community Feedback**:
> "I had a typo in my model name and it just failed at runtime. No validation!"

**Solution**: Validate config and return errors

**Proposed API**:
```go
func New(cfg Config) (*Agent, error) {
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }
    // Validate model name against known models
    if !isValidModel(cfg.Model) {
        return nil, fmt.Errorf("unknown model: %s", cfg.Model)
    }
    return &Agent{...}, nil
}

func (c Config) Validate() error {
    if c.APIKey == "" {
        return errors.New("APIKey is required")
    }
    if c.MaxIterations < 1 || c.MaxIterations > 100 {
        return errors.New("MaxIterations must be between 1 and 100")
    }
    return nil
}
```

---

## Phase 2: Core Missing Features 🎯

**Priority**: High - Needed for production use cases

### 2.1 Approval Flows for Tool Execution

**Current Problem**: No way to pause and request human approval before executing tools

**Community Feedback**:
> "For sensitive operations (assign team, close issue, deploy), I need human-in-the-loop. How do I implement this?"

**Use Case**:
- User wants to review tool calls before execution
- Some tools require approval (database changes, external API calls)
- Need to pause agent, show user the plan, wait for approval/rejection

**Proposed Solution**:

```go
// 1. Configure approval requirements
agent := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Model: "gpt-4o",
    ApprovalRequired: agentkit.ApprovalConfig{
        Tools: []string{"assign_team", "update_status"},  // These tools require approval
        Handler: approvalHandler,  // Callback for approval requests
    },
})

// 2. Approval handler interface
type ApprovalHandler func(ctx context.Context, request ApprovalRequest) (bool, error)

type ApprovalRequest struct {
    ToolName    string
    Arguments   map[string]any
    Reasoning   string  // Why agent wants to call this
    ConversationID string
}

// 3. Implementation
func approvalHandler(ctx context.Context, req ApprovalRequest) (bool, error) {
    // Store in DB for async approval
    approvalID := uuid.New()
    err := db.CreateApprovalRequest(ctx, approvalID, req)

    // Wait for user response (webhook, polling, channel, etc.)
    approved := waitForApproval(ctx, approvalID)
    return approved, nil
}

// 4. Event system integration
for event := range events {
    switch event.Type {
    case agentkit.EventTypeApprovalRequired:
        // Notify user, pause execution
        approvalID := event.Data["approval_id"].(string)
        toolName := event.Data["tool_name"].(string)
        // Show approval UI to user

    case agentkit.EventTypeApprovalGranted:
        // Resume execution

    case agentkit.EventTypeApprovalDenied:
        // Agent should re-plan
    }
}
```

### 2.2 Persisted Conversations & Memory Management

**Current Problem**: `PreviousResponseID` is internal, no conversation persistence

**Community Feedback**:
> "How do I save conversations to DB and resume them later? Every agent framework has this."

**Use Cases**:
- Multi-turn conversations across HTTP requests
- Resume interrupted agent runs
- Conversation history for audit/debugging
- Context window management

**Proposed Solution**:

```go
// 1. Conversation interface
type ConversationStore interface {
    Save(ctx context.Context, conv Conversation) error
    Load(ctx context.Context, id string) (Conversation, error)
    Append(ctx context.Context, id string, turn ConversationTurn) error
}

type Conversation struct {
    ID        string
    AgentID   string
    Turns     []ConversationTurn
    Metadata  map[string]any
    CreatedAt time.Time
    UpdatedAt time.Time
}

type ConversationTurn struct {
    Role           string  // "user", "assistant", "tool"
    Content        string
    ToolCalls      []ToolCall
    ToolResults    []ToolResult
    ResponseID     string  // OpenAI Response ID
    Timestamp      time.Time
}

// 2. Agent with conversation support
agent := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    ConversationStore: postgresConvStore,  // or memoryStore, redisStore
})

// 3. Resume conversation
ctx = agentkit.WithConversation(ctx, "conv-123")
events := agent.Run(ctx, "continue where we left off")

// 4. Built-in stores
convStore := agentkit.NewPostgresConversationStore(db)
convStore := agentkit.NewRedisConversationStore(redisClient)
convStore := agentkit.NewMemoryConversationStore()  // For testing

// 5. Context window management
agent := agentkit.New(agentkit.Config{
    ConversationStore: store,
    MaxContextTokens: 8000,
    ContextStrategy: agentkit.TruncateOldest,  // or SummarizeOldest, KeepRecent
})
```

### 2.3 Conversation History API

**Current Problem**: No way to inspect or manipulate conversation history

**Proposed Solution**:

```go
// Get conversation history
conv, err := agent.GetConversation(ctx, "conv-123")

// Inspect turns
for _, turn := range conv.Turns {
    fmt.Printf("Role: %s, Content: %s\n", turn.Role, turn.Content)
}

// Add context manually
agent.AddContext(ctx, "conv-123", "Additional context: Project deadline is Friday")

// Clear conversation
agent.ClearConversation(ctx, "conv-123")

// Fork conversation
newConvID, err := agent.ForkConversation(ctx, "conv-123", "What if we took a different approach?")
```

### 2.4 Retry Logic & Error Recovery

**Current Problem**: No built-in retry for transient API failures

**Community Feedback**:
> "Rate limits, network timeouts, 500 errors - I have to handle all retries myself."

**Proposed Solution**:

```go
agent := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Retry: agentkit.RetryConfig{
        MaxRetries:    3,
        InitialDelay:  time.Second,
        MaxDelay:      30 * time.Second,
        Multiplier:    2.0,
        RetryableErrors: []error{
            agentkit.ErrRateLimited,
            agentkit.ErrTimeout,
            agentkit.ErrServerError,
        },
    },
})

// Tool-level retries
tool := agentkit.NewTool("flaky_api").
    WithHandler(flakyHandler).
    WithRetry(3, time.Second).  // Tool-specific retry
    Build()
```

### 2.5 Timeout Configuration

**Current Problem**: No timeout management

**Proposed Solution**:

```go
agent := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Timeouts: agentkit.TimeoutConfig{
        AgentExecution: 5 * time.Minute,    // Total agent run timeout
        LLMCall:        30 * time.Second,   // Per LLM call
        ToolExecution:  10 * time.Second,   // Per tool
        StreamChunk:    5 * time.Second,    // Stream read timeout
    },
})

// Override per run
ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
defer cancel()
events := agent.Run(ctx, "quick task")
```

---

## Phase 3: Production Readiness 🏗️

**Priority**: Medium-High - Required for production deployment

### 3.1 Observability & Middleware Hooks

**Current Problem**: Hardcoded file logging, no instrumentation hooks

**Community Feedback**:
> "I need to integrate with DataDog/NewRelic. Where are the middleware hooks?"

**Status**: ✅ Implemented

```go
// Middleware interface
type Middleware interface {
    OnAgentStart(ctx context.Context, input string) context.Context
    OnAgentComplete(ctx context.Context, output string, err error)
    OnToolStart(ctx context.Context, tool string, args any) context.Context
    OnToolComplete(ctx context.Context, tool string, result any, err error)
    OnLLMCall(ctx context.Context, req any) context.Context
    OnLLMResponse(ctx context.Context, resp any, err error)
}

// Usage
agent := agentkit.New(cfg)
agent.Use(loggingMiddleware)
agent.Use(metricsMiddleware)
agent.Use(tracingMiddleware)

// Built-in middleware
agent.Use(agentkit.OpenTelemetryMiddleware(tracer))
agent.Use(agentkit.PrometheusMiddleware(registry))
agent.Use(agentkit.StructuredLoggingMiddleware(logger))
```

**Example Metrics Middleware**:
```go
type metricsMiddleware struct {
    registry *prometheus.Registry
}

func (m *metricsMiddleware) OnToolStart(ctx context.Context, tool string, args any) context.Context {
    toolCallsCounter.WithLabelValues(tool).Inc()
    return context.WithValue(ctx, "tool_start_time", time.Now())
}

func (m *metricsMiddleware) OnToolComplete(ctx context.Context, tool string, result any, err error) {
    startTime := ctx.Value("tool_start_time").(time.Time)
    duration := time.Since(startTime)
    toolDurationHistogram.WithLabelValues(tool).Observe(duration.Seconds())
    if err != nil {
        toolErrorsCounter.WithLabelValues(tool).Inc()
    }
}
```

### 3.2 Configurable Logging

**Current Problem**:
```go
const promptLogPath = "agent-prompts.log"  // ❌ Hardcoded
```

**Status**: ✅ Implemented

```go
agent, _ := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Logging: &agentkit.LoggingConfig{
        Level:           slog.LevelInfo,
        Handler:         customHandler,  // Or use default
        LogPrompts:      true,
        LogResponses:    true,
        LogToolCalls:    true,
        RedactSensitive: true,  // Redact API keys, PII
        PromptLogPath:   "/var/log/agentkit/prompts.log",
    },
})

// Or disable prompt logging
agent, _ := agentkit.New(agentkit.Config{
    Logging: &agentkit.LoggingConfig{
        LogPrompts: false,  // Don't log to file
    },
})
```

### 3.3 Testing Utilities

**Current Problem**: Can't test without hitting OpenAI API

**Status**: ✅ Implemented

```go
// Mock LLM for testing
mockLLM := agentkit.NewMockLLM().
    WithResponse("I'll search for similar issues", []ToolCall{
        {Name: "search_issues", Arguments: map[string]any{"query": "timeout"}},
    }).
    WithResponse("Found 3 issues, assigning to Infrastructure", []ToolCall{
        {Name: "assign_team", Arguments: map[string]any{"team_slug": "infrastructure"}},
    }).
    WithFinalResponse("Assigned to Infrastructure team")

agent, _ := agentkit.New(agentkit.Config{
    LLMProvider: mockLLM,  // Use mock instead of real API
    StreamResponses: false,
})

// Test
events := agent.Run(ctx, "triage this issue")
// Assertions...

// For streaming tests, you can also configure mock streams.
```

### 3.4 Trace IDs & Request Correlation

**Current Problem**: Can't correlate agent runs across services

**Status**: ✅ Implemented

```go
// Inject trace ID
ctx = agentkit.WithTraceID(ctx, "trace-123")
ctx = agentkit.WithSpanID(ctx, "span-456")
events := agent.Run(ctx, "triage issue")

// All events include trace_id/span_id
// Event structure includes it automatically
event := Event{
    Type: EventTypeToolCall,
    TraceID: "trace-123",
    SpanID: "span-456",
    Data: {...},
}
```

---

## Phase 4: Advanced Capabilities 🚀

**Priority**: Medium - Nice to have, enables advanced use cases

### 4.1 Complex Schema Support

**Current Problem**: Limited parameter types (`String()`, `Array()`), no nested objects

**Status**: ✅ Implemented (Object/ArrayOf/JSON Schema/struct tags).

```go
// Object parameters
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

// Or use JSON Schema directly
tool := agentkit.NewTool("advanced").
    WithJSONSchema(myJSONSchema).
    Build()

// Struct tags (Go-native)
type SearchParams struct {
    Query   string   `json:"query" required:"true" desc:"Search query"`
    Labels  []string `json:"labels"`
    Limit   int      `json:"limit" default:"10"`
}

toolBuilder, _ := agentkit.NewStructTool("search", func(ctx context.Context, args SearchParams) (any, error) {
    // ...
})
tool := toolBuilder.Build()
```

### 4.2 Event System Improvements

**Current Problem**: Must consume all events, no selective subscription

**Status**: ✅ Implemented (filtering + recording). Buffered strategies still planned.

```go
// Filtered event subscription
events := agent.Run(ctx, "triage issue")
filtered := agentkit.FilterEvents(events, agentkit.EventTypeActionDetected, agentkit.EventTypeFinalOutput)

// Configurable buffer size
agent, _ := agentkit.New(agentkit.Config{
    EventBuffer: 100,  // Buffer size
})

// Event replay
recorder := agentkit.NewEventRecorder()
events := recorder.Record(agent.Run(ctx, "message"))

// Replay later
for _, event := range recorder.Events() {
    fmt.Printf("%+v\n", event)
}
```

### 4.3 Agent Composition & Sub-Agents

**Current Problem**: No way to compose agents or delegate to specialized agents

**Status**: ✅ Implemented

```go
// Sub-agent as a tool
triageAgent := agentkit.New(triageConfig)
assignAgent := agentkit.New(assignConfig)

// Compose
mainAgent := agentkit.New(mainConfig)
_ = mainAgent.AddSubAgent("triage", triageAgent)
_ = mainAgent.AddSubAgent("assign", assignAgent)

// Agent can delegate
// User: "Triage this issue"
// Main agent: "I'll delegate to triage specialist"
// [calls triage sub-agent]
// Triage agent: "This is a platform issue"
// Main agent: "Now assigning..."
```

### 4.4 Parallel Tool Execution Control

**Current Problem**: Parallel execution controlled by OpenAI, not by framework

**Status**: ✅ Implemented

```go
agent, _ := agentkit.New(agentkit.Config{
    ParallelToolExecution: &agentkit.ParallelConfig{
        Enabled: true,
        MaxConcurrent: 3,  // Run max 3 tools in parallel
        SafetyMode: agentkit.SafetyModeOptimistic,  // or Pessimistic
    },
})

// Tool-level concurrency control
tool := agentkit.NewTool("serial_tool").
    WithConcurrency(agentkit.ConcurrencySerial).  // Never run in parallel
    WithHandler(handler).
    Build()
```

---

## Phase 5: Polish & Documentation 📚

**Priority**: Low-Medium - Improves adoption and usability

### 5.1 Real-World Examples

**Status**: ✅ Implemented (examples added to README)
- [x] Multi-turn conversation example
- [x] RAG implementation with vector DB
- [x] Multi-agent collaboration
- [x] Production deployment guide
- [x] Error handling patterns
- [x] Performance optimization guide
- [x] Security best practices

### 5.2 Improved README

**Status**: ✅ Implemented
- Comparison table vs LangChain, Pydantic AI, OpenAI SDK
- When to use AgentKit vs alternatives
- Performance characteristics
- Limitations and gotchas

### 5.3 API Reference Documentation

**Status**: ✅ Implemented (expanded README API reference + recipes)
- GoDoc-style documentation
- Interactive examples
- Common recipes
- Troubleshooting guide

---

## Implementation Priority Summary

| Phase | Estimated Effort | Impact | Business Value |
|-------|-----------------|--------|----------------|
| **Phase 1** | 2-3 weeks | 🔥 Critical | Fixes daily frustrations, improves adoption |
| **Phase 2** | 4-6 weeks | 🎯 High | Enables production features (approval, persistence) |
| **Phase 3** | 3-4 weeks | 🏗️ Medium-High | Required for reliable production deployment |
| **Phase 4** | 6-8 weeks | 🚀 Medium | Enables advanced use cases, competitive parity |
| **Phase 5** | 2-3 weeks | 📚 Low-Medium | Improves adoption, reduces support burden |

## Quick Wins (Do First)

1. ✅ Make formatters optional with sensible defaults (1 day)
2. ✅ Add `GetDeps` with error return, deprecate `MustGetDeps` (1 day)
3. ✅ Add config validation (1 day)
4. ✅ Configurable log output (remove hardcoded file) (1 day)
5. ✅ Basic retry logic for API calls (2 days)

These 5 changes would immediately improve developer experience and could be shipped in a week.

---

## Community Quotes

> "The builder pattern is nice, but 3 functions per tool kills productivity. Just give me good defaults." - Senior Go Engineer

> "I need approval flows for production. Can't auto-assign teams without PM review." - Engineering Manager

> "Where's the conversation persistence? I can't make a Slack bot without saving state." - Bot Developer

> "Love that it uses Responses API, but the streaming internals leak everywhere in my logs." - DevOps Engineer

> "Type assertions on every result formatter is not Go-like. Use generics!" - Go Advocate

> "No testing utilities? I can't unit test my agent logic without mocking OpenAI." - QA Engineer
