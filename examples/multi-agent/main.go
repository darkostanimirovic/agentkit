package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Create specialized agents
	researchAgent := createResearchAgent()
	summaryAgent := createSummaryAgent()

	// Create main orchestrator agent
	mainAgent, err := agentkit.New(agentkit.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string {
			return "You are an orchestrator. Use the research agent to gather information, then use the summary agent to create a concise summary."
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add sub-agents as tools
	mainAgent.AddTool(researchAgent.AsTool("research", "Research a topic in depth"))
	mainAgent.AddTool(summaryAgent.AsTool("summarize", "Create a concise summary"))

	// Run the orchestrator
	ctx := context.Background()
	events := mainAgent.Run(ctx, "Research Go concurrency patterns and summarize the findings")

	for event := range events {
		switch event.Type {
		case agentkit.EventTypeFinalOutput:
			fmt.Printf("✨ Result: %s\n", event.Data["response"])
		case agentkit.EventTypeError:
			fmt.Printf("❌ Error: %v\n", event.Data["error"])
		}
	}
}

func createResearchAgent() *agentkit.Agent {
	agent, _ := agentkit.New(agentkit.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string {
			return "You are a research specialist. Provide detailed, accurate information."
		},
	})
	return agent
}

func createSummaryAgent() *agentkit.Agent {
	agent, _ := agentkit.New(agentkit.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string {
			return "You are a summary specialist. Create concise, clear summaries."
		},
	})
	return agent
}
