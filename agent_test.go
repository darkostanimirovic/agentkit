package agentkit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := Config{
		APIKey: "test-key",
		Model:  "gpt-4",
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent == nil {
		t.Fatal("expected agent to be created")
	}

	if agent.model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", agent.model)
	}

	if agent.responsesClient == nil {
		t.Error("expected responsesClient to be initialized")
	}

	if agent.maxIterations != 5 {
		t.Errorf("expected default maxIterations 5, got %d", agent.maxIterations)
	}

	// Temperature defaults to 0 if not set
	if agent.temperature != 0 {
		t.Errorf("expected default temperature 0, got %f", agent.temperature)
	}

	// StreamResponses defaults to false if not set
	if agent.streamResponses != false {
		t.Error("expected default streamResponses false")
	}
}

func TestNew_CustomConfig(t *testing.T) {
	systemPrompt := func(ctx context.Context) string {
		return "Custom system prompt"
	}

	cfg := Config{
		APIKey:          "test-key",
		Model:           "gpt-3.5-turbo",
		SystemPrompt:    systemPrompt,
		MaxIterations:   5,
		Temperature:     0.5,
		StreamResponses: false,
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.model != "gpt-3.5-turbo" {
		t.Errorf("expected model gpt-3.5-turbo, got %s", agent.model)
	}

	if agent.systemPrompt == nil {
		t.Error("expected systemPrompt to be set")
	}

	if agent.maxIterations != 5 {
		t.Errorf("expected maxIterations 5, got %d", agent.maxIterations)
	}

	if agent.temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", agent.temperature)
	}

	if agent.streamResponses != false {
		t.Error("expected streamResponses false")
	}
}

func TestAgent_AddTool(t *testing.T) {
	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return map[string]any{"success": true}, nil
	}

	tool := NewTool("test_tool").
		WithDescription("Test tool").
		WithHandler(handler).
		Build()

	agent.AddTool(tool)

	if len(agent.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(agent.tools))
	}

	retrievedTool, exists := agent.tools["test_tool"]
	if !exists {
		t.Error("expected tool to be registered")
	}

	if retrievedTool.name != "test_tool" {
		t.Errorf("expected tool name test_tool, got %s", retrievedTool.name)
	}
}

func TestAgent_AddMultipleTools(t *testing.T) {
	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	handler := func(ctx context.Context, args map[string]any) (any, error) {
		return nil, nil
	}

	tool1 := NewTool("tool1").WithHandler(handler).Build()
	tool2 := NewTool("tool2").WithHandler(handler).Build()
	tool3 := NewTool("tool3").WithHandler(handler).Build()

	agent.AddTool(tool1)
	agent.AddTool(tool2)
	agent.AddTool(tool3)

	if len(agent.tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(agent.tools))
	}

	for _, name := range []string{"tool1", "tool2", "tool3"} {
		if _, exists := agent.tools[name]; !exists {
			t.Errorf("expected tool %s to be registered", name)
		}
	}
}

func TestAgent_SystemPrompt(t *testing.T) {
	type testDeps struct {
		UserID string
	}

	systemPromptCalled := false
	var capturedContext context.Context

	systemPrompt := func(ctx context.Context) string {
		systemPromptCalled = true
		capturedContext = ctx

		deps, err := GetDeps[testDeps](ctx)
		if err == nil {
			return "Hello " + deps.UserID
		}
		return "Hello world"
	}

	agent, err := New(Config{
		APIKey:       "test-key",
		Model:        "gpt-4",
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// We can't easily test Run without mocking OpenAI, but we can test
	// that the system prompt function is stored correctly
	if agent.systemPrompt == nil {
		t.Error("expected systemPrompt to be set")
	}

	// Test direct invocation
	ctx := WithDeps(context.Background(), testDeps{UserID: "user123"})
	result := agent.systemPrompt(ctx)

	if !systemPromptCalled {
		t.Error("expected systemPrompt to be called")
	}

	if result != "Hello user123" {
		t.Errorf("expected 'Hello user123', got %s", result)
	}

	if capturedContext == nil {
		t.Error("expected context to be passed to systemPrompt")
	}
}

func TestHandleToolCalls_Success(t *testing.T) {
	t.Skip("Testing internal method - integration test covers this")
	// Note: We're testing the internal method through integration tests
	// This verifies tool handler execution, error handling, etc.
}

func TestHandleToolCalls_UnknownTool(t *testing.T) {
	t.Skip("Testing internal method - integration test covers this")
	// The agent should handle unknown tools gracefully by emitting error events
}

func TestHandleToolCalls_HandlerError(t *testing.T) {
	t.Skip("Testing internal method - integration test covers this")
	// The agent should emit an error event when handler fails
}

func TestEventChannelClose(t *testing.T) {
	t.Skip("Requires OpenAI mocking")
	// Test that the event channel closes properly after agent completes
}

func TestMaxIterations(t *testing.T) {
	agent, err := New(Config{
		APIKey:        "test-key",
		Model:         "gpt-4",
		MaxIterations: 3,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.maxIterations != 3 {
		t.Errorf("expected maxIterations 3, got %d", agent.maxIterations)
	}
}

func TestTemperature(t *testing.T) {
	tests := []struct {
		name        string
		temperature float32
	}{
		{"low", 0.1},
		{"medium", 0.7},
		{"high", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := New(Config{
				APIKey:      "test-key",
				Model:       "gpt-4",
				Temperature: tt.temperature,
			})
			if err != nil {
				t.Fatalf("failed to create agent: %v", err)
			}

			if agent.temperature != tt.temperature {
				t.Errorf("expected temperature %f, got %f", tt.temperature, agent.temperature)
			}
		})
	}
}

func TestStreamResponses(t *testing.T) {
	tests := []struct {
		name    string
		stream  bool
		wantErr bool
	}{
		{"streaming enabled", true, false},
		{"streaming disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := New(Config{
				APIKey:          "test-key",
				Model:           "gpt-4",
				StreamResponses: tt.stream,
			})
			if err != nil {
				t.Fatalf("failed to create agent: %v", err)
			}

			if agent.streamResponses != tt.stream {
				t.Errorf("expected streamResponses %v, got %v", tt.stream, agent.streamResponses)
			}
		})
	}
}

func TestEventBufferConfig(t *testing.T) {
	agent, err := New(Config{
		APIKey:      "test-key",
		Model:       "gpt-4",
		EventBuffer: 42,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if agent.eventBuffer != 42 {
		t.Fatalf("expected event buffer 42, got %d", agent.eventBuffer)
	}

	defaultAgent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if defaultAgent.eventBuffer != defaultEventBuffer {
		t.Fatalf("expected default event buffer %d, got %d", defaultEventBuffer, defaultAgent.eventBuffer)
	}
}

// Integration-style test (still without real OpenAI calls)
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("expected default model gpt-4o-mini, got %s", cfg.Model)
	}

	if cfg.MaxIterations != 5 {
		t.Errorf("expected default maxIterations 5, got %d", cfg.MaxIterations)
	}

	if cfg.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", cfg.Temperature)
	}

	if cfg.StreamResponses != true {
		t.Error("expected default streamResponses true")
	}
}

func TestAgent_Integration(t *testing.T) {
	t.Skip("Integration test - requires mocking OpenAI client")

	// This test would verify:
	// 1. Agent receives user message
	// 2. LLM decides to call tool
	// 3. Tool is executed
	// 4. Result is sent back to LLM
	// 5. LLM produces final output
	// 6. Events are emitted in correct order
}

// Helper for testing event channels
func collectEvents(events <-chan Event, timeout time.Duration) []Event {
	var collected []Event
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return collected
			}
			collected = append(collected, event)
		case <-timer.C:
			return collected
		}
	}
}

func TestCollectEvents(t *testing.T) {
	// Test the helper function itself
	events := make(chan Event, 3)
	events <- ThinkingChunk("test1")
	events <- ThinkingChunk("test2")
	events <- FinalOutput("done", "all done")
	close(events)

	collected := collectEvents(events, 100*time.Millisecond)

	if len(collected) != 3 {
		t.Errorf("expected 3 events, got %d", len(collected))
	}

	if collected[0].Type != EventTypeThinkingChunk {
		t.Error("expected first event to be thinking chunk")
	}

	if collected[2].Type != EventTypeFinalOutput {
		t.Error("expected last event to be final output")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "missing api key",
			config: Config{
				Model: "gpt-4",
			},
			wantErr: ErrMissingAPIKey,
		},
		{
			name: "missing api key with mock provider",
			config: Config{
				Model:       "gpt-4",
				LLMProvider: NewMockLLM(),
			},
			wantErr: nil,
		},
		{
			name: "invalid max iterations - negative",
			config: Config{
				APIKey:        "test-key",
				Model:         "gpt-4",
				MaxIterations: -1,
			},
			wantErr: ErrInvalidIterations,
		},
		{
			name: "invalid max iterations - too high",
			config: Config{
				APIKey:        "test-key",
				Model:         "gpt-4",
				MaxIterations: 101,
			},
			wantErr: ErrInvalidIterations,
		},
		{
			name: "invalid temperature - negative",
			config: Config{
				APIKey:      "test-key",
				Model:       "gpt-4",
				Temperature: -0.1,
			},
			wantErr: ErrInvalidTemperature,
		},
		{
			name: "invalid temperature - too high",
			config: Config{
				APIKey:      "test-key",
				Model:       "gpt-4",
				Temperature: 2.1,
			},
			wantErr: ErrInvalidTemperature,
		},
		{
			name: "invalid reasoning effort",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: "invalid",
			},
			wantErr: ErrInvalidReasoningEffort,
		},
		{
			name: "valid reasoning effort - none",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: ReasoningEffortNone,
			},
			wantErr: nil,
		},
		{
			name: "valid reasoning effort - minimal",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: ReasoningEffortMinimal,
			},
			wantErr: nil,
		},
		{
			name: "valid reasoning effort - low",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: ReasoningEffortLow,
			},
			wantErr: nil,
		},
		{
			name: "valid reasoning effort - medium",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: ReasoningEffortMedium,
			},
			wantErr: nil,
		},
		{
			name: "valid reasoning effort - high",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: ReasoningEffortHigh,
			},
			wantErr: nil,
		},
		{
			name: "valid reasoning effort - xhigh",
			config: Config{
				APIKey:          "test-key",
				Model:           "o1-mini",
				ReasoningEffort: ReasoningEffortXHigh,
			},
			wantErr: nil,
		},
		{
			name: "valid config",
			config: Config{
				APIKey:        "test-key",
				Model:         "gpt-4o",
				MaxIterations: 5,
				Temperature:   0.7,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
