package timeout

import (
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
