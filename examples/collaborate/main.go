package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/darkostanimirovic/agentkit"
)

// This example demonstrates the Collaboration pattern - multiple agents
// working together as peers in a shared conversation, like a breakout room.
//
// Scenario: Designing an API requires input from engineering, design, and product.
// They brainstorm together in real-time to reach a consensus.

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Create specialized agents for different roles
	engineerAgent, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.5,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a backend engineer focused on:
- Technical feasibility
- Performance and scalability
- Security considerations
- Implementation complexity
Keep responses concise and technical.`
		},
		MaxIterations: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create engineer agent: %v", err)
	}

	designerAgent, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.7,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a UX designer focused on:
- User experience
- API ergonomics
- Developer experience
- Consistency and intuitiveness
Keep responses concise and user-focused.`
		},
		MaxIterations: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create designer agent: %v", err)
	}

	productAgent, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.6,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a product manager focused on:
- Business value
- Time to market
- Customer needs
- Competitive positioning
Keep responses concise and strategic.`
		},
		MaxIterations: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create product agent: %v", err)
	}

	// Create a facilitator who runs the discussion
	facilitatorAgent, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.5,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a facilitator who synthesizes input from different team members.
Your job is to find common ground and build consensus.
Be concise and action-oriented.`
		},
		MaxIterations: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create facilitator agent: %v", err)
	}

	fmt.Println("=== Collaboration Example: API Design Discussion ===\n")

	// Example 1: Basic collaboration
	fmt.Println("Example 1: Basic Collaboration")
	fmt.Println("-------------------------------")
	
	session := agentkit.NewCollaborationSession(
		facilitatorAgent,
		engineerAgent,
		designerAgent,
		productAgent,
	)

	result, err := session.Discuss(
		ctx,
		"How should we design the authentication API for our new mobile app?",
	)
	if err != nil {
		log.Fatalf("Collaboration failed: %v", err)
	}

	fmt.Printf("Final Decision:\n%s\n\n", result.FinalResponse)
	fmt.Printf("Summary: %s\n\n", result.Summary)
	
	// Show the discussion flow
	fmt.Println("Discussion Flow:")
	for _, round := range result.Rounds {
		fmt.Printf("\nRound %d:\n", round.Number)
		for _, contrib := range round.Contributions {
			fmt.Printf("  [%s]: %s\n", contrib.Agent, truncate(contrib.Content, 100))
		}
		if round.Synthesis != "" {
			fmt.Printf("  [Synthesis]: %s\n", truncate(round.Synthesis, 100))
		}
	}
	fmt.Println()

	// Example 2: Collaboration with custom options
	fmt.Println("\nExample 2: Extended Collaboration (5 rounds)")
	fmt.Println("---------------------------------------------")
	
	session2 := agentkit.NewCollaborationSession(
		facilitatorAgent,
		engineerAgent,
		designerAgent,
	).Configure(
		agentkit.WithMaxRounds(5),
		agentkit.WithCaptureHistory(true),
	)

	result2, err := session2.Discuss(
		ctx,
		"What's the best strategy for handling real-time updates in our app: WebSockets, Server-Sent Events, or polling?",
	)
	if err != nil {
		log.Fatalf("Collaboration failed: %v", err)
	}

	fmt.Printf("Final Recommendation:\n%s\n\n", result2.FinalResponse)
	fmt.Printf("Rounds completed: %d\n", len(result2.Rounds))
	fmt.Printf("Participants: %v\n\n", result2.Participants)

	// Example 3: Quick brainstorm (2 rounds, fast)
	fmt.Println("\nExample 3: Quick Brainstorm")
	fmt.Println("---------------------------")
	
	quickSession := agentkit.NewCollaborationSession(
		facilitatorAgent,
		engineerAgent,
		productAgent,
	)

	quickResult, err := quickSession.Discuss(
		ctx,
		"Should we support dark mode in our MVP?",
		agentkit.WithMaxRounds(2),
	)
	if err != nil {
		log.Fatalf("Collaboration failed: %v", err)
	}

	fmt.Printf("Quick Decision:\n%s\n\n", quickResult.FinalResponse)

	// Example 4: Technical deep-dive with two engineers
	fmt.Println("\nExample 4: Technical Deep-Dive")
	fmt.Println("-------------------------------")
	
	engineer2, err := agentkit.New(agentkit.Config{
		APIKey:      apiKey,
		Model:       "claude-3-5-sonnet-20241022",
		Temperature: 0.4,
		SystemPrompt: func(ctx context.Context) string {
			return `You are a senior backend engineer specializing in distributed systems.
Focus on: reliability, consistency, fault tolerance.`
		},
		MaxIterations: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create engineer2 agent: %v", err)
	}

	techSession := agentkit.NewCollaborationSession(
		facilitatorAgent,
		engineerAgent,
		engineer2,
	)

	techResult, err := techSession.Discuss(
		ctx,
		"How should we handle distributed transactions across our microservices?",
		agentkit.WithMaxRounds(4),
	)
	if err != nil {
		log.Fatalf("Collaboration failed: %v", err)
	}

	fmt.Printf("Technical Solution:\n%s\n\n", techResult.FinalResponse)

	// Show detailed rounds for technical discussion
	fmt.Println("Detailed Discussion:")
	for _, round := range techResult.Rounds {
		fmt.Printf("\n--- Round %d ---\n", round.Number)
		for _, contrib := range round.Contributions {
			fmt.Printf("\n%s:\n%s\n", contrib.Agent, contrib.Content)
		}
		if round.Synthesis != "" {
			fmt.Printf("\nSynthesis:\n%s\n", round.Synthesis)
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
