# Agent Coordination in AgentKit

AgentKit provides intuitive, natural patterns for agent coordination that mirror how real people work together. There are no "sub-agents" or "sub-people" - just agents working together through **handoffs** and **collaboration**.

## Philosophy

Real people coordinate in two fundamental ways:

1. **Handoffs** - "Go figure this out and report back"
   - One person delegates work to another
   - The delegate works independently with isolated context
   - They return with results
   - Think: sending someone to do research

2. **Collaboration** - "Let's hash this out together"
   - Multiple people brainstorm as equals
   - Everyone contributes to a shared conversation
   - No hierarchy, just peers building on each other's ideas
   - Think: breakout rooms, design sessions, whiteboarding

AgentKit makes these patterns natural and elegant in code.

## Handoffs

### Basic Handoff

```go
// Create agents
researchAgent, _ := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Model:  "claude-3-5-sonnet-20241022",
    SystemPrompt: func(ctx context.Context) string {
        return "You are a research specialist..."
    },
})

coordinatorAgent, _ := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Model:  "claude-3-5-sonnet-20241022",
    SystemPrompt: func(ctx context.Context) string {
        return "You are a project coordinator..."
    },
})

// Delegate work
result, err := coordinatorAgent.Handoff(
    ctx,
    researchAgent,
    "Research the top 3 Go web frameworks in 2026",
)

fmt.Printf("Research findings: %s\n", result.Response)
```

### Handoff with Trace

Enable tracing to see how the delegated agent approached the task:

```go
result, err := coordinatorAgent.Handoff(
    ctx,
    researchAgent,
    "What are JWT authentication best practices?",
    agentkit.WithIncludeTrace(true),
)

// Examine execution steps
for _, step := range result.Trace {
    fmt.Printf("[%s] %s\n", step.Type, step.Content)
}
```

### Handoff with Context

Provide background information to help the delegated agent:

```go
result, err := coordinatorAgent.Handoff(
    ctx,
    researchAgent,
    "Should we use GraphQL or REST?",
    agentkit.WithContext(agentkit.HandoffContext{
        Background: "Building mobile app. 3 engineers who know REST. 6 month timeline.",
    }),
    agentkit.WithMaxTurns(5),
)
```

### Agent as Tool

Convert any agent into a tool that can be called by another agent:

```go
// Quick way - directly from agent
coordinatorAgent.AddTool(
    researchAgent.AsHandoffTool(
        "research_agent",
        "Delegate research tasks to a specialized research agent",
    ),
)

// Or create a reusable configuration
handoffConfig := agentkit.NewHandoffConfiguration(
    coordinatorAgent,
    researchAgent,
    agentkit.WithIncludeTrace(true),
    agentkit.WithMaxTurns(10),
)

tool := handoffConfig.AsTool(
    "research_agent",
    "Delegate research tasks to a specialized research agent",
)

coordinatorAgent.AddTool(tool)

// Now the LLM decides when to delegate and what to ask
result, _ := coordinatorAgent.Run(ctx, "We need to research microservices patterns")
```

// Now the coordinator can decide when to delegate
events := coordinatorAgent.Run(ctx, "We need to choose a database...")

for event := range events {
    // Coordinator will call research_agent tool when needed
}
```

## Collaboration

### Basic Collaboration

Multiple agents discuss a topic as peers:

```go
// Create specialized agents
engineerAgent, _ := agentkit.New(/* engineer config */)
designerAgent, _ := agentkit.New(/* designer config */)
productAgent, _ := agentkit.New(/* product config */)
facilitatorAgent, _ := agentkit.New(/* facilitator config */)

// Create a collaboration session
session := agentkit.NewCollaborationSession(
    facilitatorAgent,  // Runs the discussion
    engineerAgent,     // Peer contributor
    designerAgent,     // Peer contributor
    productAgent,      // Peer contributor
)

// Discuss together
result, err := session.Discuss(
    ctx,
    "How should we design the authentication API?",
)

fmt.Printf("Final Decision: %s\n", result.FinalResponse)
fmt.Printf("Summary: %s\n", result.Summary)
```

### Collaboration with Options

Customize the discussion:

```go
session := agentkit.NewCollaborationSession(
    facilitatorAgent,
    engineerAgent,
    designerAgent,
).Configure(
    agentkit.WithMaxRounds(5),         // More rounds for complex topics
    agentkit.WithRoundTimeout(3*time.Minute),  // Longer timeouts
    agentkit.WithCaptureHistory(true), // Full conversation history
)

result, err := session.Discuss(
    ctx,
    "WebSockets vs Server-Sent Events vs polling?",
)
```

### Inspecting Discussion Flow

See how the conversation evolved:

```go
for _, round := range result.Rounds {
    fmt.Printf("\nRound %d:\n", round.Number)
    
    // Each agent's contribution
    for _, contrib := range round.Contributions {
        fmt.Printf("[%s at %s]:\n%s\n\n", 
            contrib.Agent, 
            contrib.Time, 
            contrib.Content,
        )
    }
    
    // Facilitator's synthesis
    if round.Synthesis != "" {
        fmt.Printf("[Facilitator Synthesis]:\n%s\n", round.Synthesis)
    }
}
```

## When to Use What

### Use Handoffs When:
- ✅ One agent needs specialized expertise from another
- ✅ The work can be done independently
- ✅ You want isolation (delegate's work doesn't pollute main conversation)
- ✅ You need to track what the delegate did
- ✅ Pattern: Research, specialized analysis, focused subtasks

### Use Collaboration When:
- ✅ Multiple perspectives are needed
- ✅ Brainstorming or consensus-building
- ✅ Everyone contributes as equals (no hierarchy)
- ✅ Iterative refinement of ideas
- ✅ Pattern: Design sessions, technical debates, strategic planning

### Converting Collaborations to Tools

Just like handoffs, collaborations can be converted to tools for LLM-driven coordination:

```go
// Create a standing collaboration session
designSession := agentkit.NewCollaborationSession(
    facilitatorAgent,
    engineerAgent,
    designerAgent,
    productAgent,
)

// Convert it to a tool
designTool := designSession.AsTool(
    "design_discussion",
    "Form a collaborative design discussion on a specific topic",
)

// Now the coordinator agent can decide WHEN and WHAT to discuss
coordinator := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Model: "claude-3-5-sonnet-20241022",
    Tools: []agentkit.Tool{designTool},
    Instructions: "You coordinate product development. When design decisions are needed, use the design_discussion tool.",
})

// LLM will autonomously call the tool with appropriate topics:
// - "Should we use WebSockets or SSE for real-time updates?"
// - "How should we handle authentication tokens?"
// - "What should the error handling UX be?"
result, _ := coordinator.Run(ctx, "We need to add real-time notifications")
```

The LLM provides the `topic` parameter at runtime based on context, rather than you hardcoding what to discuss.

## Comparison with Traditional "Subagents"

Traditional subagent pattern:
```go
// ❌ Old way - hierarchical, rigid
subAgentTool := NewSubAgentTool(config, subAgent)
parentAgent.AddTool(subAgentTool)
```

New coordination patterns:
```go
// ✅ Handoff - natural delegation
result := agentA.Handoff(ctx, agentB, "task")

// ✅ Collaboration - peer discussion
session := NewCollaborationSession(facilitator, agentA, agentB, agentC)
result := session.Discuss(ctx, "topic")

// ✅ As tools for LLM-driven coordination
handoffTool := agentB.AsHandoffTool("delegate_work", "description")
collaborateTool := session.AsTool("discuss_topic", "description")
```

## Tracing

Both patterns integrate seamlessly with AgentKit's tracing system:

- **Handoffs** create isolated spans - perfect for seeing delegate's work separately
- **Collaborations** create nested spans for each round and contribution
- All LLM calls are traced automatically
- Use Langfuse or any compatible tracer

```go
tracer := agentkit.NewLangfuseTracer(/* config */)

agent, _ := agentkit.New(agentkit.Config{
    APIKey: apiKey,
    Tracer: tracer,
})

// Traces will show:
// - handoff
//   - handoff.llm_call
//   - handoff.tool_execution
// - collaboration
//   - collaboration_round_1
//     - peer_1_contribution
//     - peer_2_contribution
//   - collaboration_round_2
//     ...
```

## Examples

See complete working examples:

- [`examples/handoff/`](examples/handoff/) - Comprehensive handoff patterns
- [`examples/collaborate/`](examples/collaborate/) - Team collaboration scenarios

## Design Principles

1. **Natural Metaphors** - Code reads like human coordination
2. **No Hierarchies** - Agents are peers, not parent/child
3. **Context Isolation** - Handoffs keep work separate
4. **Shared Context** - Collaborations build on each contribution
5. **Flexible** - Use what fits your use case
6. **Go-Native** - Idiomatic Go, no magic

## Migration from Subagents

If you're using the old subagent pattern:

```go
// Old
subTool, _ := NewSubAgentTool(SubAgentConfig{
    Name: "researcher",
    Description: "Research specialist",
}, researchAgent)
agent.AddTool(subTool)

// New - Handoff
result, _ := agent.Handoff(ctx, researchAgent, "task")

// Or as a tool
agent.AddTool(researchAgent.AsHandoffTool("researcher", "Research specialist"))
```

The subagent API remains for backward compatibility but is considered deprecated.
