package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Configure Langfuse tracer
	tracer, err := agentkit.NewLangfuseTracer(agentkit.LangfuseConfig{
		PublicKey:      os.Getenv("LANGFUSE_PUBLIC_KEY"),   // pk-lf-...
		SecretKey:      os.Getenv("LANGFUSE_SECRET_KEY"),   // sk-lf-...
		BaseURL:        "https://cloud.langfuse.com",       // EU region (default)
		ServiceName:    "agentkit-example",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Enabled:        true,
	})
	if err != nil {
		log.Fatalf("Failed to create tracer: %v", err)
	}
	defer tracer.Shutdown(context.Background())

	// Create agent with tracing enabled
	agent, err := agentkit.New(agentkit.Config{
		APIKey:        os.Getenv("OPENAI_API_KEY"),
		Model:         "gpt-4o-mini",
		SystemPrompt:  buildSystemPrompt,
		MaxIterations: 5,
		Tracer:        tracer, // Enable tracing
	})
	if err != nil {
		log.Fatal(err)
	}

	// Register a tool
	agent.AddTool(
		agentkit.NewTool("get_weather").
			WithDescription("Get current weather for a location").
			WithParameter("location", agentkit.String().
				Required().
				WithDescription("City name")).
			WithHandler(getWeather).
			Build(),
	)

	// Run agent with tracing enabled
	// The trace will automatically include all LLM calls and tool executions
	ctx := context.Background()
	events := agent.Run(ctx, "What's the weather like in San Francisco?")

	// Process events
	for event := range events {
		switch event.Type {
		case agentkit.EventTypeThinkingChunk:
			fmt.Print(event.Data["chunk"])
		case agentkit.EventTypeActionDetected:
			fmt.Printf("\nTool: %s\n", event.Data["description"])
		case agentkit.EventTypeFinalOutput:
			fmt.Printf("\n\nDone: %s\n", event.Data["response"])
		}
	}

	// Flush traces before exiting
	if err := tracer.Flush(context.Background()); err != nil {
		log.Printf("Failed to flush traces: %v", err)
	}

	fmt.Println("\nTrace sent to Langfuse! Check your dashboard at https://cloud.langfuse.com")
}

func buildSystemPrompt(ctx context.Context) string {
	return "You are a helpful weather assistant."
}

func getWeather(ctx context.Context, args map[string]any) (any, error) {
	location := args["location"].(string)
	// Simulate API call
	return map[string]any{
		"location":    location,
		"temperature": "72°F",
		"condition":   "Sunny",
		"humidity":    "45%",
	}, nil
}