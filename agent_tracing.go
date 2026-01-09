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

// chatMessage represents a standard OpenAI chat message format
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildModelParameters creates model parameters map for tracing based on request
func buildModelParameters(req ResponseRequest) map[string]any {
	params := make(map[string]any)
	
	// For reasoning models, include reasoning.effort
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		params["reasoning_effort"] = req.Reasoning.Effort
	} else {
		// For non-reasoning models, include temperature
		params["temperature"] = req.Temperature
	}
	
	if req.TopP > 0 {
		params["top_p"] = req.TopP
	}
	
	return params
}

// buildModelParametersStream creates model parameters map for streaming tracing
func buildModelParametersStream(req ResponseRequest) map[string]any {
	params := buildModelParameters(req)
	params["stream"] = true
	return params
}

// convertInputToChatFormat converts ResponseRequest input to standard chat format
func convertInputToChatFormat(input any) []chatMessage {
	var messages []chatMessage

	switch v := input.(type) {
	case []ResponseInput:
		for _, item := range v {
			msg := chatMessage{
				Role: item.Role,
			}
			// Extract text from content blocks
			for _, content := range item.Content {
				if content.Type == "input_text" && content.Text != "" {
					msg.Content += content.Text
				}
			}
			if msg.Content != "" {
				messages = append(messages, msg)
			}
		}
	case []ResponseContentItem:
		// For tool responses (continuation calls)
		for _, item := range v {
			if item.Type == "function_call_output" {
				messages = append(messages, chatMessage{
					Role:    "tool",
					Content: item.Output,
				})
			}
		}
	case string:
		messages = append(messages, chatMessage{
			Role:    "user",
			Content: v,
		})
	}

	return messages
}

// convertOutputToChatFormat converts ResponseOutputItem to standard chat format
func convertOutputToChatFormat(output []ResponseOutputItem) []chatMessage {
	var messages []chatMessage

	for _, item := range output {
		if item.Type == messageType && item.Role == "assistant" {
			msg := chatMessage{
				Role: item.Role,
			}
			// Extract text from content blocks
			for _, content := range item.Content {
				if content.Type == outputTextType && content.Text != "" {
					msg.Content += content.Text
				}
			}
			if msg.Content != "" {
				messages = append(messages, msg)
			}
		}
	}

	return messages
}

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

	// Convert to standard chat format for Langfuse
	// Langfuse expects OpenAI chat messages format: [{role, content}]
	input := convertInputToChatFormat(req.Input)
	output := convertOutputToChatFormat(resp.Output)

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
		Input:  input,
		Output: output,
		Usage:  usage,
		Cost:   cost,
		ModelParameters: buildModelParameters(req),
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

	// Convert to standard chat format for Langfuse
	input := convertInputToChatFormat(req.Input)

	// For streaming, construct output in chat format
	var output []chatMessage
	if state.finalText != "" {
		output = []chatMessage{
			{
				Role:    "assistant",
				Content: state.finalText,
			},
		}
	}

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
		Input:  input,
		Output: output,
		Usage:  usage,
		Cost:   cost,
		ModelParameters: buildModelParametersStream(req),
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
