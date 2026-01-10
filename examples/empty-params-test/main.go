package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	fmt.Println("Testing tools with various parameter configurations")
	fmt.Println("====================================================\n")

	// Test 1: Tool with no parameters at all
	tool1 := agentkit.NewTool("no_params").
		WithDescription("A tool with no parameters").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
return "ok", nil
}).
		Build()

	printSchema("Tool 1 (No parameters)", tool1.ToOpenAI().Function.Parameters)

	// Test 2: Tool with empty raw parameters
	tool2 := agentkit.NewTool("empty_raw").
		WithDescription("Tool with empty raw params").
		WithRawParameters(map[string]any{}).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
return "ok", nil
}).
		Build()

	printSchema("Tool 2 (Empty raw parameters)", tool2.ToOpenAI().Function.Parameters)

	// Test 3: Tool with minimal schema (only required field)
	tool3 := agentkit.NewTool("minimal_schema").
		WithDescription("Tool with minimal schema").
		WithRawParameters(map[string]any{
"required": []string{},
}).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
return "ok", nil
}).
		Build()

	printSchema("Tool 3 (Minimal schema)", tool3.ToOpenAI().Function.Parameters)

	// Test 4: Tool with one parameter
	tool4 := agentkit.NewTool("with_param").
		WithDescription("Tool with one parameter").
		WithParameter("input", agentkit.String().Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
return "ok", nil
}).
		Build()

	printSchema("Tool 4 (With parameter)", tool4.ToOpenAI().Function.Parameters)

	fmt.Println("\n✅ All tools built successfully!")
}

func printSchema(label string, params any) {
	jsonBytes, _ := json.MarshalIndent(params, "  ", "  ")
	fmt.Printf("%s:\n  %s\n\n", label, string(jsonBytes))
}
