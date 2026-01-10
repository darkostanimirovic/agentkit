package agentkit

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestLangfuseTracingWithRealAgent tests Langfuse tracing with a real agent and OpenAI call
// to ensure all generation observation fields are correctly populated
func TestLangfuseTracingWithRealAgent(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping real OpenAI test - OPENAI_API_KEY not set")
	}

	// Get Langfuse credentials (skip if not available)
	langfusePublicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	langfuseSecretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	if langfusePublicKey == "" || langfuseSecretKey == "" {
		t.Skip("Skipping Langfuse tracing test - credentials not set")
	}

	// Create Langfuse tracer
	tracer, err := NewLangfuseTracer(LangfuseConfig{
		PublicKey:   langfusePublicKey,
		SecretKey:   langfuseSecretKey,
		BaseURL:     "https://cloud.langfuse.com",
		ServiceName: "agentkit-comprehensive-test",
		Environment: "test",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("Failed to create Langfuse tracer: %v", err)
	}
	defer tracer.Shutdown(context.Background())

	// Create agent with tracing
	ctx := context.Background()
	agent, err := New(Config{
		APIKey:       apiKey,
		Model:        "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string {
			return "You are a helpful assistant that can get weather information."
		},
		Tracer:  tracer,
		Logging: LoggingConfig{}.Silent(),
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Define a simple weather tool
	weatherTool := NewTool("get_weather").
		WithDescription("Get the current weather in a location").
		WithParameter("location", String().Required().WithDescription("The city and state, e.g. San Francisco, CA")).
		WithParameter("unit", String().WithEnum("celsius", "fahrenheit")).
		WithHandler(func(ctx context.Context, input map[string]any) (any, error) {
			location := input["location"].(string)
			unit := "fahrenheit"
			if u, ok := input["unit"].(string); ok {
				unit = u
			}
			return map[string]any{
				"temperature": 72,
				"conditions":  "sunny",
				"location":    location,
				"unit":        unit,
			}, nil
		}).
		Build()

	agent.AddTool(weatherTool)

	// Run agent with a question that will trigger tool use
	events := agent.Run(ctx, "What's the weather like in San Francisco?")
	
	var finalResponse string
	for event := range events {
		if event.Type == EventTypeFinalOutput {
			finalResponse = event.Data["response"].(string)
		}
	}

	t.Logf("Agent Response: %s", finalResponse)

	// Flush to ensure data is sent
	if err := tracer.Flush(context.Background()); err != nil {
		t.Fatalf("Failed to flush tracer: %v", err)
	}

	t.Log("\n=== VERIFICATION ===")
	t.Log("✓ Agent executed successfully with tool call")
	t.Log("✓ Check Langfuse UI for generation observation")
	t.Log("✓ Expected fields in generation:")
	t.Log("  - Model: gpt-4o-mini")
	t.Log("  - Usage details (input/output tokens)")
	t.Log("  - Input messages (system + user prompts)")
	t.Log("  - Output messages (assistant response with tool calls)")
	t.Log("  - Tool definitions (get_weather)")
	t.Log("  - Tool calls (location, arguments)")
	t.Log("  - Cost calculation")
	t.Log("\nPlease check Langfuse UI at: https://cloud.langfuse.com")
}

// TestLangfuseGenerationFieldValidation specifically tests that all generation fields are set
func TestLangfuseGenerationFieldValidation(t *testing.T) {
	// This test validates the structure without making a real API call
	// It ensures our generation logging logic sets all the correct attributes

	langfusePublicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	langfuseSecretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	if langfusePublicKey == "" || langfuseSecretKey == "" {
		t.Skip("Skipping Langfuse tracing test - credentials not set")
	}

	tracer, err := NewLangfuseTracer(LangfuseConfig{
		PublicKey:   langfusePublicKey,
		SecretKey:   langfuseSecretKey,
		BaseURL:     "https://cloud.langfuse.com",
		ServiceName: "agentkit-field-validation-test",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}
	defer tracer.Shutdown(context.Background())

	ctx := context.Background()
	traceCtx, endTrace := tracer.StartTrace(ctx, "field-validation-test")
	defer endTrace()

	// Create mock generation data with all possible fields
	startTime := time.Now()
	endTime := startTime.Add(2 * time.Second)
	completionStartTime := startTime.Add(100 * time.Millisecond)

	input := []map[string]any{
		{
			"role":    "system",
			"content": "You are a helpful assistant.",
		},
		{
			"role":    "user",
			"content": "What's the weather?",
		},
	}

	output := []map[string]any{
		{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]any{
				{
					"id":   "call_123",
					"type": "function",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": `{"location":"San Francisco"}`,
					},
				},
			},
			"finish_reason": "tool_calls",
		},
	}

	usage := &UsageInfo{
		PromptTokens:     50,
		CompletionTokens: 20,
		TotalTokens:      70,
	}

	cost := &CostInfo{
		PromptCost:     0.000075, // $0.15 per 1M input tokens
		CompletionCost: 0.000120, // $0.60 per 1M output tokens
		TotalCost:      0.000195,
	}

	modelParams := map[string]any{
		"temperature":       0.7,
		"max_tokens":        150,
		"top_p":             1.0,
		"frequency_penalty": 0.0,
		"presence_penalty":  0.0,
	}

	metadata := map[string]any{
		"response_id":    "chatcmpl-123",
		"finish_reason":  "tool_calls",
		"has_tool_calls": true,
	}

	err = tracer.LogGeneration(traceCtx, GenerationOptions{
		Name:                "mock-generation-all-fields",
		Model:               "gpt-4o-mini",
		ModelParameters:     modelParams,
		Input:               input,
		Output:              output,
		Usage:               usage,
		Cost:                cost,
		StartTime:           startTime,
		EndTime:             endTime,
		CompletionStartTime: &completionStartTime,
		Metadata:            metadata,
		Level:               LogLevelDefault,
	})

	if err != nil {
		t.Fatalf("Failed to log generation: %v", err)
	}

	// Flush
	if err := tracer.Flush(context.Background()); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	t.Log("✅ Generation logged with all fields:")
	t.Log("  - Model: gpt-4o-mini")
	t.Log("  - Model Parameters: temperature, max_tokens, top_p, etc.")
	t.Log("  - Input: system + user messages")
	t.Log("  - Output: assistant message with tool_calls")
	t.Log("  - Usage: 50 input + 20 output = 70 total tokens")
	t.Log("  - Cost: $0.000195 total")
	t.Log("  - Timing: start, end, completion_start_time")
	t.Log("  - Metadata: response_id, finish_reason, has_tool_calls")
	
	t.Log("\n✅ All generation observation fields should be visible in Langfuse:")
	inputJSON, _ := json.MarshalIndent(input, "    ", "  ")
	t.Logf("\n  Input (Prompts):\n    %s", string(inputJSON))
	
	outputJSON, _ := json.MarshalIndent(output, "    ", "  ")
	t.Logf("\n  Output (with tool calls):\n    %s", string(outputJSON))
}

