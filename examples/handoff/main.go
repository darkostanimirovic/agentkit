package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/darkostanimirovic/agentkit"
)

// This example demonstrates the Handoff pattern - delegating work to another agent
// who works in isolation and reports back.
//
// Scenario: A project manager agent delegates specific research to a research agent,
// then uses the findings to make a decision.

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Create a research specialist agent
	researchAgent, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.3,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a research specialist. When given a research task:
- Be thorough and cite specific facts
- Focus on recent developments
- Provide actionable insights
- Keep responses concise but comprehensive`
		},
		MaxIterations: 5,
	})
	if err != nil {
		log.Fatalf("Failed to create research agent: %v", err)
	}

	// Create a project manager agent
	managerAgent, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.7,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a project manager who makes strategic decisions.
You can delegate research to specialists when needed.`
		},
		MaxIterations: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create manager agent: %v", err)
	}

	fmt.Println("=== Handoff Example: Project Manager → Research Specialist ===\n")

	// Example 1: Simple handoff
	fmt.Println("Example 1: Basic Handoff")
	fmt.Println("-------------------------")
	result, err := managerAgent.Handoff(
		ctx,
		researchAgent,
		"Research the top 3 Go web frameworks in 2026. Focus on performance, adoption, and ecosystem.",
	)
	if err != nil {
		log.Fatalf("Handoff failed: %v", err)
	}

	fmt.Printf("Research findings:\n%s\n\n", result.Response)
	if result.Summary != "" {
		fmt.Printf("Summary: %s\n\n", result.Summary)
	}

	// Example 2: Handoff with full context
	fmt.Println("Example 2: Handoff with Full Context (for debugging)")
	fmt.Println("----------------------------------------------")
	resultWithTrace, err := managerAgent.Handoff(
		ctx,
		researchAgent,
		"What are the security best practices for JWT authentication in Go?",
		agentkit.WithFullContext(true), // Include full thinking trace in result
	)
	if err != nil {
		log.Fatalf("Handoff failed: %v", err)
	}

	fmt.Printf("Research findings:\n%s\n\n", resultWithTrace.Response)
	
	if len(resultWithTrace.Trace) > 0 {
		fmt.Printf("Execution trace (%d steps):\n", len(resultWithTrace.Trace))
		for i, item := range resultWithTrace.Trace {
			fmt.Printf("  %d. [%s] %s\n", i+1, item.Type, truncate(item.Content, 80))
		}
		fmt.Println()
	}

	// Example 3: Handoff with context
	fmt.Println("Example 3: Handoff with Background Context")
	fmt.Println("-------------------------------------------")
	resultWithContext, err := managerAgent.Handoff(
		ctx,
		researchAgent,
		"Should we adopt GraphQL or continue with REST?",
		agentkit.WithContext(agentkit.HandoffContext{
			Background: "Our team is building a mobile app with complex data requirements. We have 3 backend engineers familiar with REST, none with GraphQL. Timeline is 6 months.",
		}),
		agentkit.WithMaxTurns(3),
	)
	if err != nil {
		log.Fatalf("Handoff failed: %v", err)
	}

	fmt.Printf("Recommendation:\n%s\n\n", resultWithContext.Response)

	// Example 4: Register as a tool (LLM-driven handoff)
	fmt.Println("Example 4: Research Agent as a Tool")
	fmt.Println("------------------------------------")
	
	// Register research agent as a tool
	managerWithTool, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.7,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a project manager. When you need research, use the research_agent tool.`
		},
		MaxIterations: 5,
	})
	if err != nil {
		log.Fatalf("Failed to create manager with tool: %v", err)
	}

	managerWithTool.AddTool(
		researchAgent.AsHandoffTool(
			"research_agent",
			"Delegate research tasks to a specialized research agent",
		),
	)

	events := managerWithTool.Run(
		ctx,
		"We're deciding between PostgreSQL and MongoDB for our new project. Can you research the trade-offs and make a recommendation?",
	)
	
	var response string
	for event := range events {
		if event.Type == agentkit.EventTypeFinalOutput {
			if resp, ok := event.Data["response"].(string); ok {
				response = resp
			}
		}
	}

	fmt.Printf("Manager's decision (after delegating research):\n%s\n\n", response)

	// Example 5: Chain of handoffs
	fmt.Println("Example 5: Chain of Handoffs")
	fmt.Println("-----------------------------")
	
	// Manager → Researcher → Manager makes decision
	techResearch, err := managerAgent.Handoff(
		ctx,
		researchAgent,
		"Research the current state of AI code generation tools",
		agentkit.WithFullContext(false), // Just get result, not full trace
	)
	if err != nil {
		log.Fatalf("First handoff failed: %v", err)
	}

	// Now manager uses research to make a decision
	events2 := managerAgent.Run(
		ctx,
		fmt.Sprintf("Based on this research:\n\n%s\n\nShould our team invest in building AI-assisted coding features?", techResearch.Response),
	)
	
	var decision string
	for event := range events2 {
		if event.Type == agentkit.EventTypeFinalOutput {
			if resp, ok := event.Data["response"].(string); ok {
				decision = resp
			}
		}
	}

	fmt.Printf("Final decision:\n%s\n", decision)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
