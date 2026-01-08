package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Initialize Langfuse tracer
	tracer, err := agentkit.NewLangfuseTracer(agentkit.LangfuseConfig{
		PublicKey:   os.Getenv("LANGFUSE_PUBLIC_KEY"),
		SecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),
		BaseURL:     "https://cloud.langfuse.com",
		ServiceName: "agentkit-tracing-test",
		Enabled:     true,
	})
	if err != nil {
		log.Fatalf("Failed to create Langfuse tracer: %v", err)
	}
	defer tracer.Shutdown(context.Background())

	// Create test tool
	generatePalette := agentkit.NewTool("generate_palette").
		WithDescription("Generates a color palette based on a primary color").
		WithParameter("primary_color", agentkit.String().
			Required().
			WithDescription("The primary color in hex format (e.g., #FF0000)")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			primaryColor := args["primary_color"].(string)
			paletteSize := 5
			if size, ok := args["palette_size"].(float64); ok {
				paletteSize = int(size)
			}

			// Simulate palette generation
			palette := map[string]any{
				"primary": primaryColor,
				"colors": []string{
					primaryColor,
					"#FF5733",
					"#33FF57",
					"#3357FF",
					"#FF33F5",
				}[:paletteSize],
			}
			return palette, nil
		}).
		Build()

	// Create agent with tracer
	agent, err := agentkit.New(agentkit.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-mini",
		Tracer: tracer,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Add the tool
	agent.AddTool(generatePalette)

	// Create a trace context
	ctx := context.Background()
	ctx, endTrace := tracer.StartTrace(ctx, "palette-generation-test",
		agentkit.WithSessionID("test-session"),
		agentkit.WithUserID("test-user"),
		agentkit.WithTraceInput(map[string]any{
			"query": "Generate a color palette based on red",
		}),
	)
	defer endTrace()

	// Run the agent
	fmt.Println("Running agent with Langfuse tracing...")
	events := agent.Run(ctx, "Generate a color palette based on the color red (#FF0000)")

	// Process events
	var finalResponse string
	for event := range events {
		switch event.Type {
		case agentkit.EventTypeThinkingChunk:
			fmt.Print(event.Data["chunk"])
		case agentkit.EventTypeActionDetected:
			fmt.Printf("\nTool: %s\n", event.Data["description"])
		case agentkit.EventTypeFinalOutput:
			if response, ok := event.Data["response"].(string); ok {
				finalResponse = response
			}
		}
	}

	fmt.Printf("\nAgent Response:\n%s\n", finalResponse)

	// Flush traces
	fmt.Println("\nFlushing traces to Langfuse...")
	if err := tracer.Flush(context.Background()); err != nil {
		log.Printf("Warning: failed to flush traces: %v", err)
	}

	fmt.Println("Check your Langfuse dashboard to see:")
	fmt.Println("  - LLM generation details (input, output, token usage)")
	fmt.Println("  - Tool outputs (generated palette)")
	fmt.Println("  - Tool inputs (primary_color, palette_size)")
	fmt.Println("\n✅ Trace sent to Langfuse!")
	time.Sleep(2 * time.Second) // Wait a moment for traces to be sent
}