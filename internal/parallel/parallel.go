package parallel

// SafetyMode controls how aggressively to run tools in parallel.
type SafetyMode string

const (
	SafetyModeOptimistic  SafetyMode = "optimistic"
	SafetyModePessimistic SafetyMode = "pessimistic"
)

// ParallelConfig controls parallel tool execution.
type ParallelConfig struct {
	Enabled       bool
	MaxConcurrent int
	SafetyMode    SafetyMode
}

// DefaultParallelConfig returns default settings for tool execution.
func DefaultParallelConfig() ParallelConfig {
	return ParallelConfig{
		Enabled:       false,
		MaxConcurrent: 1,
		SafetyMode:    SafetyModeOptimistic,
	}
}
