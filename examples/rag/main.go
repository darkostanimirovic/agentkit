package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/darkostanimirovic/agentkit"
)

// Simulated vector database
var knowledgeBase = map[string]string{
	"go_concurrency":    "Go uses goroutines and channels for concurrency. Goroutines are lightweight threads managed by the Go runtime.",
	"go_interfaces":     "Interfaces in Go provide a way to specify the behavior of an object. They are implicit and satisfied automatically.",
	"go_error_handling": "Go uses explicit error handling with the error type. Functions return errors as values to be checked by the caller.",
}

func main() {
	agent, err := agentkit.New(agentkit.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string {
			return "You are a Go programming expert with access to a knowledge base. Use the retrieve_context tool to get relevant information before answering."
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add RAG retrieval tool
	agent.AddTool(
		agentkit.NewTool("retrieve_context").
			WithDescription("Search the knowledge base for relevant Go programming information").
			WithParameter("query", agentkit.String().Required().WithDescription("Search query")).
			WithHandler(retrieveHandler).
			Build(),
	)

	ctx := context.Background()
	events := agent.Run(ctx, "How does error handling work in Go?")

	for event := range events {
		switch event.Type {
		case agentkit.EventTypeActionDetected:
			fmt.Printf("🔍 Searching knowledge base...\n")
		case agentkit.EventTypeActionResult:
			fmt.Printf("📚 Retrieved context\n")
		case agentkit.EventTypeFinalOutput:
			fmt.Printf("\n✨ Answer: %s\n", event.Data["response"])
		case agentkit.EventTypeError:
			fmt.Printf("❌ Error: %v\n", event.Data["error"])
		}
	}
}

func retrieveHandler(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query := strings.ToLower(args["query"].(string))

	// Simple keyword matching for demo purposes
	var results []string
	for key, content := range knowledgeBase {
		if strings.Contains(key, query) || strings.Contains(strings.ToLower(content), query) {
			results = append(results, content)
		}
	}

	if len(results) == 0 {
		return map[string]interface{}{
			"found":   false,
			"message": "No relevant information found",
		}, nil
	}

	return map[string]interface{}{
		"found":  true,
		"chunks": results,
		"count":  len(results),
	}, nil
}
