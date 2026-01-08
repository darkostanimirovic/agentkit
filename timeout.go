package agentkit

import (
	"time"
)

// TimeoutConfig configures timeout behavior for different operations
type TimeoutConfig struct {
	AgentExecution time.Duration // Total agent run timeout (0 = no timeout)
	LLMCall        time.Duration // Per LLM API call timeout (0 = no timeout)
	ToolExecution  time.Duration // Per tool execution timeout (0 = no timeout)
	StreamChunk    time.Duration // Timeout for receiving stream chunks (0 = no timeout)
}

// DefaultTimeoutConfig returns sensible timeout defaults
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		AgentExecution: 5 * time.Minute,  // Total agent run
		LLMCall:        30 * time.Second, // Per API call
		ToolExecution:  10 * time.Second, // Per tool
		StreamChunk:    5 * time.Second,  // Stream read
	}
}

// NoTimeouts returns a config with all timeouts disabled
func NoTimeouts() TimeoutConfig {
	return TimeoutConfig{}
}
