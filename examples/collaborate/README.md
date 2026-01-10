# Collaboration Example

This example demonstrates the **Collaboration** pattern - multiple agents working together as peers in a shared conversation, like a breakout room.

## Scenario

Designing an API requires input from engineering, design, and product perspectives. These agents brainstorm together, building on each other's ideas, with a facilitator synthesizing the discussion.

## Running

```bash
export ANTHROPIC_API_KEY=your_key_here
go run main.go
```

## What it Demonstrates

### 1. Basic Collaboration
Three specialized agents (engineer, designer, product) discuss an API design with a facilitator.

### 2. Extended Collaboration
More rounds for complex topics that need deeper exploration.

### 3. Quick Brainstorm
Fast 2-round discussion for simpler decisions.

### 4. Technical Deep-Dive
Two engineers with different specializations collaborate on a technical problem.

## Key Concepts

- **Peer-to-Peer**: No hierarchy - all agents contribute equally
- **Shared Context**: Each contribution builds on previous ones
- **Facilitation**: One agent synthesizes and guides the discussion
- **Rounds**: Structured turns ensure everyone contributes
- **Synthesis**: The facilitator creates a cohesive final answer

## Real-World Uses

- Design sessions with cross-functional teams
- Technical architecture discussions
- Strategic planning
- Brainstorming and ideation
- Consensus-building on complex decisions
- Code review discussions
- Requirements gathering

## Discussion Flow

Each round:
1. Every peer agent contributes their perspective
2. Facilitator synthesizes the round's insights
3. History grows - agents see previous contributions
4. Facilitator decides whether to continue or conclude

Final synthesis integrates all insights into a coherent decision or recommendation.
