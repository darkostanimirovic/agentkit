package agentkit

// Helper methods for tracing integration

import (
	"context"
	"encoding/json"
	"time"
)

// logLLMGeneration logs an LLM generation to the tracer
func (a *Agent) logLLMGeneration(ctx context.Context, req ResponseRequest, resp *ResponseObject) {
	if resp == nil {
		return
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

	// Extract usage info
	var usage *UsageInfo
	if resp.Usage.TotalTokens > 0 {
		usage = &UsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Log the generation
	a.tracer.LogGeneration(ctx, GenerationOptions{
		Name:   "llm.generate",
		Model:  a.model,
		Input:  inputText,
		Output: outputText,
		Usage:  usage,
		ModelParameters: map[string]any{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
		},
		StartTime: time.Now().Add(-time.Second), // Approximate
		EndTime:   time.Now(),
		Metadata: map[string]any{
			"response_id": resp.ID,
			"status":      resp.Status,
		},
	})
}

// logLLMGenerationFromStream logs an LLM generation from a streaming response
func (a *Agent) logLLMGenerationFromStream(ctx context.Context, req ResponseRequest, state *streamState) {
	if state == nil {
		return
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

	// For streaming we don't have usage info readily available
	// This could be enhanced if the stream provides usage data

	// Log the generation
	a.tracer.LogGeneration(ctx, GenerationOptions{
		Name:   "llm.generate.stream",
		Model:  a.model,
		Input:  inputText,
		Output: outputText,
		ModelParameters: map[string]any{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"stream":      true,
		},
		StartTime: time.Now().Add(-time.Second), // Approximate
		EndTime:   time.Now(),
		Metadata: map[string]any{
			"response_id": state.responseID,
			"chunk_count": state.chunkCount,
		},
	})
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
