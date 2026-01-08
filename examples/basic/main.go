package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Create a basic agent
	agent, err := agentkit.New(agentkit.Config{
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		Model:        "gpt-4o-mini",
		SystemPrompt: buildSystemPrompt,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add a simple tool
	agent.AddTool(
		agentkit.NewTool("get_weather").
			WithDescription("Get weather information for a location").
			WithParameter("location", agentkit.String().Required().WithDescription("City name")).
			WithHandler(weatherHandler).
			Build(),
	)

	// Run the agent
	ctx := context.Background()
	events := agent.Run(ctx, "What's the weather like in San Francisco?")

	for event := range events {
		switch event.Type {
		case agentkit.EventTypeThinkingChunk:
			fmt.Print(event.Data["chunk"])
		case agentkit.EventTypeActionDetected:
			fmt.Printf("\n🔧 Tool: %s\n", event.Data["description"])
		case agentkit.EventTypeActionResult:
			fmt.Printf("✓ Result: %v\n", event.Data["result"])
		case agentkit.EventTypeFinalOutput:
			fmt.Printf("\n✨ Final: %s\n", event.Data["response"])
		case agentkit.EventTypeError:
			fmt.Printf("\n❌ Error: %v\n", event.Data["error"])
		}
	}
}

func buildSystemPrompt(ctx context.Context) string {
	return "You are a helpful weather assistant. Use the get_weather tool to fetch weather information."
}

func weatherHandler(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	location := args["location"].(string)
	// In a real app, you'd call a weather API here
	return map[string]interface{}{
		"location":    location,
		"temperature": "72°F",
		"conditions":  "Sunny",
		"humidity":    "45%",
	}, nil
}
