package main

import (
	"context"
	"fmt"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	fmt.Println("Demonstrating automatic additionalProperties: false")
	fmt.Println("===================================================\n")

	// Example 1: Using WithParameter
	_ = agentkit.NewTool("check_context_size").
		WithDescription("Check the context size").
		WithParameter("input", agentkit.String().Required().WithDescription("Input text")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"size": len(args["input"].(string))}, nil
		}).
		Build()

	fmt.Println("✅ Tool 1 (WithParameter): additionalProperties added automatically")

	// Example 2: Using WithRawParameters
	_ = agentkit.NewTool("process_data").
		WithDescription("Process data").
		WithRawParameters(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"data": map[string]any{
					"type":        "string",
					"description": "Data to process",
				},
			},
			"required": []string{"data"},
		}).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"processed": true}, nil
		}).
		Build()

	fmt.Println("✅ Tool 2 (WithRawParameters): additionalProperties added at Build()")

	// Example 3: Using WithJSONSchema
	_ = agentkit.NewTool("analyze").
		WithDescription("Analyze input").
		WithJSONSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
			},
			"required": []string{"text"},
		}).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"analyzed": true}, nil
		}).
		Build()

	fmt.Println("✅ Tool 3 (WithJSONSchema): additionalProperties added at Build()")

	fmt.Println("\n🎉 All tools created successfully!")
	fmt.Println("\n📝 This fix prevents the OpenAI API error:")
	fmt.Println("   'additionalProperties' is required to be supplied and to be false")
	fmt.Println("\n💡 The Build() method automatically adds additionalProperties: false")
	fmt.Println("   when strict mode is enabled (default) for tools using:")
	fmt.Println("   - WithParameter()")
	fmt.Println("   - WithRawParameters()")
	fmt.Println("   - WithJSONSchema()")
}
