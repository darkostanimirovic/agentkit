package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Example 1: Default behavior - usage from API + dynamic costs
	fmt.Println("Example 1: With dynamic cost calculation (default)")
	runAgent()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: Disable cost calculation
	fmt.Println("Example 2: Without cost estimation")
	agentkit.DisableCostCalculation = true
	runAgent()
	agentkit.DisableCostCalculation = false // Reset

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 3: Custom pricing
	fmt.Println("Example 3: With custom pricing")
	agentkit.RegisterModelCost("gpt-4o-mini", agentkit.ModelCostConfig{
		InputCostPer1MTokens:  0.100, // Custom reduced rate
		OutputCostPer1MTokens: 0.400,
	})
	runAgent()
}

func runAgent() {
	// Create a simple tracer that logs to console
	tracer := &ConsoleTracer{}

	agent, err := agentkit.New(agentkit.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-mini",
		Tracer: tracer,
		SystemPrompt: func(ctx context.Context) string {
			return "You are a helpful assistant. Keep responses brief."
		},
	})
	if err != nil {
		panic(err)
	}

	// Run a simple query
	events := agent.Run(context.Background(), "What is 2+2?")
	for event := range events {
		if event.Type == agentkit.EventTypeFinalOutput {
			if response, ok := event.Data["response"].(string); ok {
				fmt.Printf("Response: %s\n", response)
			}
		}
	}
}

// ConsoleTracer is a simple tracer that logs to console
type ConsoleTracer struct{}

func (t *ConsoleTracer) StartTrace(ctx context.Context, name string, opts ...agentkit.TraceOption) (context.Context, func()) {
	return ctx, func() {}
}

func (t *ConsoleTracer) StartSpan(ctx context.Context, name string, opts ...agentkit.SpanOption) (context.Context, func()) {
	return ctx, func() {}
}

func (t *ConsoleTracer) LogGeneration(ctx context.Context, opts agentkit.GenerationOptions) error {
	fmt.Println("\n📊 LLM Generation:")
	fmt.Printf("  Model: %s\n", opts.Model)

	if opts.Usage != nil {
		fmt.Printf("  Usage (from OpenAI API):\n")
		fmt.Printf("    - Prompt tokens: %d\n", opts.Usage.PromptTokens)
		fmt.Printf("    - Completion tokens: %d\n", opts.Usage.CompletionTokens)
		fmt.Printf("    - Total tokens: %d\n", opts.Usage.TotalTokens)
	}

	if opts.Cost != nil {
		fmt.Printf("  Cost (calculated from models.dev API):\n")
		fmt.Printf("    - Prompt cost: $%.6f\n", opts.Cost.PromptCost)
		fmt.Printf("    - Completion cost: $%.6f\n", opts.Cost.CompletionCost)
		fmt.Printf("    - Total cost: $%.6f\n", opts.Cost.TotalCost)
		fmt.Printf("    📊 Cost data sourced from models.dev API\n")
	} else {
		fmt.Printf("  Cost: Not calculated (disabled or model unknown)\n")
	}

	if latency, ok := opts.Metadata["latency_ms"].(int64); ok {
		fmt.Printf("  Latency: %dms\n", latency)
	}

	return nil
}

func (t *ConsoleTracer) LogEvent(ctx context.Context, name string, attributes map[string]any) error {
	return nil
}

func (t *ConsoleTracer) SetTraceAttributes(ctx context.Context, attributes map[string]any) error {
	return nil
}

func (t *ConsoleTracer) SetSpanOutput(ctx context.Context, output any) error {
	return nil
}

func (t *ConsoleTracer) SetSpanAttributes(ctx context.Context, attributes map[string]any) error {
	return nil
}

func (t *ConsoleTracer) Flush(ctx context.Context) error {
	return nil
}
