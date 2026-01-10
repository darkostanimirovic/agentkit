package agentkit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestLangfuseUsageAttributes(t *testing.T) {
	// Create a test span recorder
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	// Create a Langfuse tracer that uses the test provider
	langfuseTracer := &LangfuseTracer{
		tracer: tracerProvider.Tracer("test"),
	}

	// Test data with realistic usage numbers
	usage := &UsageInfo{
		PromptTokens:     150,
		CompletionTokens: 75,
		TotalTokens:      225,
		ReasoningTokens:  0,
	}

	// Log a generation with usage
	langfuseTracer.LogGeneration(context.Background(), GenerationOptions{
		Name:   "test-generation",
		Model:  "gpt-4o-mini",
		Input:  []interface{}{"test input"},
		Output: []interface{}{"test output"},
		Usage:  usage,
		StartTime: time.Now().Add(-1 * time.Second),
		EndTime:   time.Now(),
	})

	// Get the recorded spans
	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := span.Attributes()

	// Verify all required OTEL GenAI attributes are present
	requiredAttrs := map[string]int{
		"gen_ai.usage.input_tokens":  150,
		"gen_ai.usage.output_tokens": 75,
		"gen_ai.usage.total_tokens":  225,
	}

	for attrName, expectedValue := range requiredAttrs {
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == attrName {
				found = true
				if attr.Value.AsInt64() != int64(expectedValue) {
					t.Errorf("Attribute %s = %d, want %d", attrName, attr.Value.AsInt64(), expectedValue)
				}
				break
			}
		}
		if !found {
			t.Errorf("Missing required attribute: %s", attrName)
		}
	}

	// Verify langfuse.observation.usage_details JSON attribute
	var usageDetailsJSON string
	for _, attr := range attrs {
		if string(attr.Key) == "langfuse.observation.usage_details" {
			usageDetailsJSON = attr.Value.AsString()
			break
		}
	}

	if usageDetailsJSON == "" {
		t.Fatal("Missing langfuse.observation.usage_details attribute")
	}

	// Parse and verify the JSON structure
	var usageDetails map[string]int
	if err := json.Unmarshal([]byte(usageDetailsJSON), &usageDetails); err != nil {
		t.Fatalf("Failed to parse usage_details JSON: %v", err)
	}

	expectedJSON := map[string]int{
		"input":  150,
		"output": 75,
		"total":  225,
	}

	for key, expectedVal := range expectedJSON {
		if val, ok := usageDetails[key]; !ok {
			t.Errorf("Missing key %s in usage_details JSON", key)
		} else if val != expectedVal {
			t.Errorf("usage_details[%s] = %d, want %d", key, val, expectedVal)
		}
	}
}

func TestLangfuseUsageWithReasoningTokens(t *testing.T) {
	// Create a test span recorder
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	langfuseTracer := &LangfuseTracer{
		tracer: tracerProvider.Tracer("test"),
	}

	// Test with reasoning tokens (like o1 model)
	usage := &UsageInfo{
		PromptTokens:     100,
		CompletionTokens: 50,
		ReasoningTokens:  1000, // o1 models use lots of reasoning tokens
		TotalTokens:      1150,
	}

	langfuseTracer.LogGeneration(context.Background(), GenerationOptions{
		Name:   "test-generation-reasoning",
		Model:  "o1-preview",
		Input:  []interface{}{"test input"},
		Output: []interface{}{"test output"},
		Usage:  usage,
		StartTime: time.Now().Add(-1 * time.Second),
		EndTime:   time.Now(),
	})

	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := span.Attributes()

	// Verify reasoning_tokens attribute is present
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "gen_ai.usage.reasoning_tokens" {
			found = true
			if attr.Value.AsInt64() != 1000 {
				t.Errorf("reasoning_tokens = %d, want 1000", attr.Value.AsInt64())
			}
			break
		}
	}
	if !found {
		t.Error("Missing gen_ai.usage.reasoning_tokens attribute for reasoning model")
	}

	// Verify reasoning tokens in JSON
	var usageDetailsJSON string
	for _, attr := range attrs {
		if string(attr.Key) == "langfuse.observation.usage_details" {
			usageDetailsJSON = attr.Value.AsString()
			break
		}
	}

	var usageDetails map[string]int
	if err := json.Unmarshal([]byte(usageDetailsJSON), &usageDetails); err != nil {
		t.Fatalf("Failed to parse usage_details JSON: %v", err)
	}

	if val, ok := usageDetails["reasoning"]; !ok {
		t.Error("Missing 'reasoning' key in usage_details JSON for reasoning model")
	} else if val != 1000 {
		t.Errorf("usageDetails[reasoning] = %d, want 1000", val)
	}
}

func TestLangfuseUsageNil(t *testing.T) {
	// Test that nil usage doesn't cause errors
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	langfuseTracer := &LangfuseTracer{
		tracer: tracerProvider.Tracer("test"),
	}

	// Log generation without usage
	langfuseTracer.LogGeneration(context.Background(), GenerationOptions{
		Name:   "test-generation-no-usage",
		Model:  "gpt-4o-mini",
		Input:  []interface{}{"test input"},
		Output: []interface{}{"test output"},
		Usage:  nil, // No usage
		StartTime: time.Now().Add(-1 * time.Second),
		EndTime:   time.Now(),
	})

	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := span.Attributes()

	// Verify no usage attributes are present
	for _, attr := range attrs {
		key := string(attr.Key)
		if key == "gen_ai.usage.input_tokens" || key == "gen_ai.usage.output_tokens" || key == "gen_ai.usage.total_tokens" || key == "langfuse.observation.usage_details" {
			t.Errorf("Found usage attribute %s when usage was nil", key)
		}
	}
}
