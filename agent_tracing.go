package agentkit

// Helper methods for tracing integration

import (
	"context"
	"encoding/json"
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

// logLLMGeneration logs an LLM generation to the tracer
func (a *Agent) logLLMGeneration(ctx context.Context, req ResponseRequest, resp *ResponseObject) {
	if resp == nil {
		return
	}

	// Get timing from context
	timing := getLLMCallTiming(ctx)
	var startTime, endTime time.Time
	if timing != nil {
		startTime = timing.startTime
		endTime = time.Now()
		timing.endTime = endTime
	} else {
		// Fallback if timing wasn't captured
		endTime = time.Now()
		startTime = endTime.Add(-time.Second)
	}

	// Extract input prompt from req.Input
	var inputText string
	if inputs, ok := req.Input.([]ResponseInput); ok {
		for _, input := range inputs {
			for _, content := range input.Content {
				if content.Type == "input_text" && content.Text != "" {
					inputText += content.Text + "\n"
				}
			}
		}
	} else if str, ok := req.Input.(string); ok {
		inputText = str
	}

	// Extract output text
	var outputText string
	for _, output := range resp.Output {
		if output.Type == messageType && output.Role == "assistant" {
			for _, content := range output.Content {
				if content.Type == outputTextType && content.Text != "" {
					outputText += content.Text
				}
			}
		}
	}

	// Extract usage info from OpenAI API response
	// OpenAI provides token counts but NOT cost information
	var usage *UsageInfo
	if resp.Usage.TotalTokens > 0 {
		usage = &UsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Calculate estimated cost (optional - returns nil if disabled or model unknown)
	var cost *CostInfo
	if usage != nil {
		cost = CalculateCost(a.model, usage.PromptTokens, usage.CompletionTokens)
	}

	// Log the generation
	genOpts := GenerationOptions{
		Name:   "llm.generate",
		Model:  a.model,
		Input:  inputText,
		Output: outputText,
		Usage:  usage,
		Cost:   cost,
		ModelParameters: map[string]any{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
		},
		StartTime: startTime,
		EndTime:   endTime,
		Metadata: map[string]any{
			"response_id":  resp.ID,
			"status":       resp.Status,
			"latency_ms":   endTime.Sub(startTime).Milliseconds(),
			"model_actual": a.model,
		},
		Level: LogLevelDefault,
	}

	// Add completion start time if available
	if timing != nil && timing.completionStartTime != nil {
		genOpts.CompletionStartTime = timing.completionStartTime
		genOpts.Metadata["time_to_first_token_ms"] = timing.completionStartTime.Sub(startTime).Milliseconds()
	}

	a.tracer.LogGeneration(ctx, genOpts)
}

// logLLMGenerationFromStream logs an LLM generation from a streaming response
func (a *Agent) logLLMGenerationFromStream(ctx context.Context, req ResponseRequest, state *streamState) {
	if state == nil {
		return
	}

	// Get timing from context
	timing := getLLMCallTiming(ctx)
	var startTime, endTime time.Time
	var completionStartTime *time.Time
	if timing != nil {
		startTime = timing.startTime
		endTime = time.Now()
		timing.endTime = endTime
		completionStartTime = timing.completionStartTime
	} else {
		// Fallback if timing wasn't captured
		endTime = time.Now()
		startTime = endTime.Add(-time.Second)
	}

	// Extract input prompt from req.Input
	var inputText string
	if inputs, ok := req.Input.([]ResponseInput); ok {
		for _, input := range inputs {
			for _, content := range input.Content {
				if content.Type == "input_text" && content.Text != "" {
					inputText += content.Text + "\n"
				}
			}
		}
	} else if str, ok := req.Input.(string); ok {
		inputText = str
	}

	// Use accumulated final text as output
	outputText := state.finalText

	// Extract usage from stream state (captured from final response.done chunk)
	// OpenAI provides token counts in the final stream chunk
	var usage *UsageInfo
	if state.usage != nil && state.usage.TotalTokens > 0 {
		usage = &UsageInfo{
			PromptTokens:     state.usage.InputTokens,
			CompletionTokens: state.usage.OutputTokens,
			TotalTokens:      state.usage.TotalTokens,
		}
	}

	// Calculate estimated cost (optional - returns nil if disabled or model unknown)
	var cost *CostInfo
	if usage != nil {
		cost = CalculateCost(a.model, usage.PromptTokens, usage.CompletionTokens)
	}

	// Log the generation
	genOpts := GenerationOptions{
		Name:   "llm.generate.stream",
		Model:  a.model,
		Input:  inputText,
		Output: outputText,
		Usage:  usage,
		Cost:   cost,
		ModelParameters: map[string]any{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"stream":      true,
		},
		StartTime: startTime,
		EndTime:   endTime,
		Metadata: map[string]any{
			"response_id":  state.responseID,
			"chunk_count":  state.chunkCount,
			"latency_ms":   endTime.Sub(startTime).Milliseconds(),
			"model_actual": a.model,
		},
		Level: LogLevelDefault,
	}

	// Add completion start time if available
	if completionStartTime != nil {
		genOpts.CompletionStartTime = completionStartTime
		genOpts.Metadata["time_to_first_token_ms"] = completionStartTime.Sub(startTime).Milliseconds()
	}

	a.tracer.LogGeneration(ctx, genOpts)
}

// formatToolArgsForTracing converts tool arguments to a JSON-safe format
func formatToolArgsForTracing(args map[string]any) string {
	if args == nil {
		return "{}"
	}
	bytes, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}
