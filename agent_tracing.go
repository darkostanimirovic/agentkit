package agentkit

// Helper methods for tracing integration

import (
	"context"
	"time"
)

// llmCallTiming holds timing information for an LLM call
type llmCallTiming struct {
	startTime           time.Time
	endTime             time.Time
	completionStartTime *time.Time
}

// llmCallTimingContextKey is a custom type for context keys to avoid collisions
type llmCallTimingContextKey string

const llmCallTimingKey llmCallTimingContextKey = "agentkit.llmCallTiming"

// startLLMCallTiming records the start time of an LLM call in the context
func startLLMCallTiming(ctx context.Context) context.Context {
	timing := &llmCallTiming{
		startTime: time.Now(),
	}
	return context.WithValue(ctx, llmCallTimingKey, timing)
}

// getLLMCallTiming retrieves timing information from context
func getLLMCallTiming(ctx context.Context) *llmCallTiming {
	if timing, ok := ctx.Value(llmCallTimingKey).(*llmCallTiming); ok {
		return timing
	}
	return nil
}

// extractLLMCallTiming extracts timing information from context, returning a non-nil value
func extractLLMCallTiming(ctx context.Context) llmCallTiming {
	if timing := getLLMCallTiming(ctx); timing != nil {
		return *timing
	}
	// Return empty timing if not found
	now := time.Now()
	return llmCallTiming{
		startTime: now,
		endTime:   now,
	}
}
