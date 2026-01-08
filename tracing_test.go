package agentkit

import (
	"context"
	"testing"
)

func TestNoOpTracer(t *testing.T) {
	tracer := &NoOpTracer{}

	// Should not panic or error
	ctx := context.Background()

	traceCtx, endTrace := tracer.StartTrace(ctx, "test-trace",
		WithUserID("user-123"),
		WithSessionID("session-456"),
		WithTraceInput("test input"),
	)
	if traceCtx == nil {
		t.Error("StartTrace should return a context")
	}
	defer endTrace()

	spanCtx, endSpan := tracer.StartSpan(traceCtx, "test-span",
		WithSpanType(SpanTypeGeneration),
		WithSpanInput("test span input"),
	)
	if spanCtx == nil {
		t.Error("StartSpan should return a context")
	}
	defer endSpan()

	err := tracer.LogGeneration(spanCtx, GenerationOptions{
		Name:   "test-generation",
		Model:  "gpt-4",
		Input:  "test input",
		Output: "test output",
		Usage: &UsageInfo{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Cost: &CostInfo{
			PromptCost:     0.001,
			CompletionCost: 0.002,
			TotalCost:      0.003,
		},
	})
	if err != nil {
		t.Errorf("LogGeneration should not error: %v", err)
	}

	err = tracer.LogEvent(spanCtx, "test-event", map[string]any{
		"data": "test data",
	})
	if err != nil {
		t.Errorf("LogEvent should not error: %v", err)
	}

	err = tracer.SetTraceAttributes(traceCtx, map[string]any{
		"key": "value",
	})
	if err != nil {
		t.Errorf("SetTraceAttributes should not error: %v", err)
	}

	if err := tracer.Flush(ctx); err != nil {
		t.Errorf("Flush should not error: %v", err)
	}
}

func TestLangfuseTracer_Creation(t *testing.T) {
	tests := []struct {
		name    string
		config  LangfuseConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: LangfuseConfig{
				PublicKey:   "pk-lf-test",
				SecretKey:   "sk-lf-test",
				BaseURL:     "https://cloud.langfuse.com",
				ServiceName: "test-service",
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "disabled tracer",
			config: LangfuseConfig{
				PublicKey: "pk-lf-test",
				SecretKey: "sk-lf-test",
				Enabled:   false,
			},
			wantErr: true, // Returns error when disabled
		},
		{
			name: "missing public key",
			config: LangfuseConfig{
				SecretKey: "sk-lf-test",
				Enabled:   true,
			},
			wantErr: true,
		},
		{
			name: "missing secret key",
			config: LangfuseConfig{
				PublicKey: "pk-lf-test",
				Enabled:   true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer, err := NewLangfuseTracer(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLangfuseTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tracer == nil {
				t.Error("NewLangfuseTracer() returned nil tracer")
			}
		})
	}
}

func TestTraceOptions(t *testing.T) {
	ctx := context.Background()

	// Apply trace options
	opts := []TraceOption{
		WithUserID("user-123"),
		WithSessionID("session-456"),
		WithTags("tag1", "tag2"),
		WithMetadata(map[string]any{
			"key1": "value1",
			"key2": 42,
		}),
	}

	config := &TraceConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if config.UserID != "user-123" {
		t.Errorf("Expected UserID user-123, got %s", config.UserID)
	}
	if config.SessionID != "session-456" {
		t.Errorf("Expected SessionID session-456, got %s", config.SessionID)
	}
	if len(config.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(config.Tags))
	}
	if config.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata key1=value1, got %v", config.Metadata["key1"])
	}

	// Test that options can be extracted from context
	for _, opt := range opts {
		ctx = context.WithValue(ctx, "trace_option", opt)
	}
}

func TestAgentWithTracing(t *testing.T) {
	// Test that agent accepts tracer in config
	noopTracer := &NoOpTracer{}

	agent, err := New(Config{
		APIKey:       "test-key",
		Model:        "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string { return "test" },
		Tracer:       noopTracer,
	})

	if err != nil {
		t.Fatalf("Failed to create agent with tracer: %v", err)
	}

	if agent.tracer == nil {
		t.Error("Agent tracer should not be nil")
	}

	// Test that agent defaults to NoOpTracer if not provided
	agent2, err := New(Config{
		APIKey:       "test-key",
		Model:        "gpt-4o-mini",
		SystemPrompt: func(ctx context.Context) string { return "test" },
	})

	if err != nil {
		t.Fatalf("Failed to create agent without tracer: %v", err)
	}

	if agent2.tracer == nil {
		t.Error("Agent tracer should default to NoOpTracer")
	}
}
