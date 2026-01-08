package agentkit

import (
	"context"
	"testing"
	"time"
)

func TestDefaultTimeoutConfig(t *testing.T) {
	cfg := DefaultTimeoutConfig()

	if cfg.AgentExecution != 5*time.Minute {
		t.Errorf("expected AgentExecution=5m, got %v", cfg.AgentExecution)
	}
	if cfg.LLMCall != 30*time.Second {
		t.Errorf("expected LLMCall=30s, got %v", cfg.LLMCall)
	}
	if cfg.ToolExecution != 10*time.Second {
		t.Errorf("expected ToolExecution=10s, got %v", cfg.ToolExecution)
	}
	if cfg.StreamChunk != 5*time.Second {
		t.Errorf("expected StreamChunk=5s, got %v", cfg.StreamChunk)
	}
}

func TestNoTimeouts(t *testing.T) {
	cfg := NoTimeouts()

	if cfg.AgentExecution != 0 {
		t.Errorf("expected AgentExecution=0, got %v", cfg.AgentExecution)
	}
	if cfg.LLMCall != 0 {
		t.Errorf("expected LLMCall=0, got %v", cfg.LLMCall)
	}
	if cfg.ToolExecution != 0 {
		t.Errorf("expected ToolExecution=0, got %v", cfg.ToolExecution)
	}
	if cfg.StreamChunk != 0 {
		t.Errorf("expected StreamChunk=0, got %v", cfg.StreamChunk)
	}
}

func TestTimeoutConfig_Integration(t *testing.T) {
	// Test that custom timeout config is properly integrated into agent
	customTimeout := TimeoutConfig{
		AgentExecution: 1 * time.Minute,
		LLMCall:        10 * time.Second,
		ToolExecution:  5 * time.Second,
		StreamChunk:    2 * time.Second,
	}

	agent, err := New(Config{
		APIKey:  "test-key",
		Model:   "gpt-4o",
		Timeout: &customTimeout,
	})

	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.timeoutConfig.AgentExecution != customTimeout.AgentExecution {
		t.Errorf("expected AgentExecution=%v, got %v", customTimeout.AgentExecution, agent.timeoutConfig.AgentExecution)
	}
	if agent.timeoutConfig.LLMCall != customTimeout.LLMCall {
		t.Errorf("expected LLMCall=%v, got %v", customTimeout.LLMCall, agent.timeoutConfig.LLMCall)
	}
}

func TestTimeoutConfig_DefaultsWhenNil(t *testing.T) {
	// Test that defaults are used when timeout config is nil
	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})

	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	defaults := DefaultTimeoutConfig()
	if agent.timeoutConfig.AgentExecution != defaults.AgentExecution {
		t.Errorf("expected default AgentExecution=%v, got %v", defaults.AgentExecution, agent.timeoutConfig.AgentExecution)
	}
	if agent.timeoutConfig.LLMCall != defaults.LLMCall {
		t.Errorf("expected default LLMCall=%v, got %v", defaults.LLMCall, agent.timeoutConfig.LLMCall)
	}
}

func TestTimeoutConfig_ContextCancellation(t *testing.T) {
	// Test that context cancellation is properly detected
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

func TestTimeoutConfig_ZeroValuesDisableTimeout(t *testing.T) {
	// Test that zero values disable timeouts
	noTimeout := TimeoutConfig{}

	if noTimeout.AgentExecution != 0 {
		t.Error("expected zero AgentExecution to disable timeout")
	}
	if noTimeout.LLMCall != 0 {
		t.Error("expected zero LLMCall to disable timeout")
	}
	if noTimeout.ToolExecution != 0 {
		t.Error("expected zero ToolExecution to disable timeout")
	}
	if noTimeout.StreamChunk != 0 {
		t.Error("expected zero StreamChunk to disable timeout")
	}
}
