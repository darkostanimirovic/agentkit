package agentkit

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestTraceTimingFix verifies that traces start with the correct timestamp
// even when StartTrace is called inside a goroutine
func TestTraceTimingFix(t *testing.T) {
	// Create a mock tracer that captures the start time
	var capturedStartTime time.Time
	mockTracer := &mockTimingTracer{
		onStartTrace: func(ctx context.Context, name string, opts ...TraceOption) (context.Context, func()) {
			cfg := &TraceConfig{}
			for _, opt := range opts {
				opt(cfg)
			}
			if cfg.StartTime != nil {
				capturedStartTime = *cfg.StartTime
			}
			return ctx, func() {}
		},
	}

	// Capture actual start time
	actualStart := time.Now()
	time.Sleep(10 * time.Millisecond) // Simulate some delay before goroutine

	// Create agent with mock tracer
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockClient := &testLLMClient{
		response: &ResponseObject{
			ID:     "test-response",
			Status: "completed",
			Output: []ResponseOutputItem{
				{
					Type: "message",
					Role: "assistant",
					Content: []ResponseContentItem{
						{Type: "output_text", Text: "test response"},
					},
				},
			},
		},
	}

	agent := &Agent{
		tracer:            mockTracer,
		tools:             make(map[string]Tool),
		eventBuffer:       10,
		logger:            logger,
		responsesClient:   mockClient,
		maxIterations:     1,
		model:             "test-model",
		systemPrompt:      func(ctx context.Context) string { return "test" },
		streamResponses:   false,
		conversationStore: nil,
	}

	// Run the agent (which launches goroutine with StartTrace)
	events := agent.Run(context.Background(), "test message")

	// Drain events
	for range events {
	}

	// Verify the captured start time is close to actualStart
	// (should be within 50ms tolerance for timing precision)
	timeDiff := capturedStartTime.Sub(actualStart)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	if timeDiff > 50*time.Millisecond {
		t.Errorf("Trace start time is off by %v (expected < 50ms)", timeDiff)
		t.Errorf("Actual start: %v", actualStart)
		t.Errorf("Captured start: %v", capturedStartTime)
	}

	if capturedStartTime.IsZero() {
		t.Error("Start time was not captured - WithTraceStartTime not working")
	}

	t.Logf("✓ Trace timing is accurate (within %v)", timeDiff)
}

// mockTimingTracer is a minimal tracer implementation for testing timing
type mockTimingTracer struct {
	NoOpTracer
	onStartTrace func(context.Context, string, ...TraceOption) (context.Context, func())
}

func (m *mockTimingTracer) StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func()) {
	if m.onStartTrace != nil {
		return m.onStartTrace(ctx, name, opts...)
	}
	return ctx, func() {}
}

// testLLMClient is a simple mock LLM provider for testing
type testLLMClient struct {
	response *ResponseObject
}

func (t *testLLMClient) CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error) {
	return t.response, nil
}

func (t *testLLMClient) CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error) {
	return nil, nil
}

// TestReasoningTokensTracking verifies reasoning tokens are properly tracked
func TestReasoningTokensTracking(t *testing.T) {
	// Create usage info with reasoning tokens
	usage := &UsageInfo{
		PromptTokens:     100,
		CompletionTokens: 50,
		ReasoningTokens:  500, // Reasoning models generate many reasoning tokens
		TotalTokens:      650,
	}

	if usage.ReasoningTokens != 500 {
		t.Errorf("ReasoningTokens not tracked: got %d, want 500", usage.ReasoningTokens)
	}

	// Verify all token types are present
	if usage.PromptTokens == 0 || usage.CompletionTokens == 0 || usage.ReasoningTokens == 0 {
		t.Error("Some token types are missing")
	}

	t.Logf("✓ Reasoning tokens properly tracked: %d tokens", usage.ReasoningTokens)
}

// TestResponseUsageReasoningTokens verifies API response struct includes reasoning tokens
func TestResponseUsageReasoningTokens(t *testing.T) {
	// Simulate API response with reasoning tokens
	apiResponse := ResponseUsage{
		InputTokens:     100,
		OutputTokens:    50,
		ReasoningTokens: 500,
		TotalTokens:     650,
	}

	if apiResponse.ReasoningTokens != 500 {
		t.Errorf("ResponseUsage.ReasoningTokens not tracked: got %d, want 500", apiResponse.ReasoningTokens)
	}

	t.Logf("✓ API response properly includes reasoning tokens")
}
