# Handoff Example

This example demonstrates the **Handoff** pattern - delegating work to another agent who works in isolation and reports back.

## Scenario

A project manager agent delegates research tasks to a specialized research agent. The researcher works independently, then returns findings that the manager uses for decision-making.

## Running

```bash
export ANTHROPIC_API_KEY=your_key_here
go run main.go
```

## What it Demonstrates

### 1. Basic Handoff
Simple delegation of a research task.

### 2. Handoff with Trace
Enable trace to see the delegated agent's execution steps - useful for debugging or learning from the agent's approach.

### 3. Handoff with Context
Provide background information to help the delegated agent make better decisions.

### 4. Agent as Tool
Register an agent as a tool so the LLM can decide when to delegate.

### 5. Chain of Handoffs
Manager delegates research, then uses those findings to make a decision.

## Key Concepts

- **Isolation**: The delegated agent's work doesn't pollute the main conversation
- **Traceability**: Optional trace shows exactly how the work was done
- **Context**: Background information helps the delegate make informed decisions
- **Flexibility**: Can be called directly or registered as a tool

## Real-World Uses

- Research and fact-finding
- Specialized analysis (legal, technical, financial)
- Data gathering and synthesis
- Subtask delegation in complex workflows
