package agentkit

import (
	"context"
	"fmt"

	"github.com/darkostanimirovic/agentkit/providers"
)

// buildCompletionRequest creates a provider-agnostic completion request from current conversation state.
func (a *Agent) buildCompletionRequest(conversationHistory []providers.Message) providers.CompletionRequest {
	// Build tool definitions
	tools := make([]providers.ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool.ToToolDefinition())
	}

	req := providers.CompletionRequest{
		Model:             a.model,
		SystemPrompt:      a.buildSystemPrompt(context.Background()),
		Messages:          conversationHistory,
		Tools:             tools,
		Temperature:       a.temperature,
		MaxTokens:         0, // Let provider use default
		TopP:              0,  // Let provider use default
		ToolChoice:        "auto",
		ParallelToolCalls: true,
		ReasoningEffort:   a.reasoningEffort,
	}

	return req
}

// runNonStreamingIteration executes a single non-streaming iteration.
func (a *Agent) runNonStreamingIteration(ctx context.Context, req providers.CompletionRequest, events chan<- Event) (*providers.CompletionResponse, error) {
	callCtx := a.applyLLMCall(ctx, req)
	callCtx, cancel := a.withLLMTimeout(callCtx)
	if cancel != nil {
		defer cancel()
	}

	// Start timing for tracing
	callCtx = startLLMCallTiming(callCtx)

	resp, err := a.provider.Complete(callCtx, req)
	if err != nil {
		iterationErr := fmt.Errorf("provider completion error: %w", err)
		a.applyLLMResponse(callCtx, nil, iterationErr)
		return nil, a.handleIterationError(callCtx, events, iterationErr, "completion failed", "model", a.model)
	}
	
	a.applyLLMResponse(callCtx, resp, nil)

	// TODO: Re-enable tracing after refactoring to use Tracer.LogGeneration
	// a.logLLMGeneration(callCtx, req, resp)

	if a.loggingConfig.LogResponses {
		a.logger.Info("completion received", 
			"content_length", len(resp.Content),
			"tool_calls", len(resp.ToolCalls),
			"finish_reason", resp.FinishReason)
	}

	return resp, nil
}

// runStreamingIteration executes a single streaming iteration.
func (a *Agent) runStreamingIteration(ctx context.Context, req providers.CompletionRequest, events chan<- Event) (*providers.CompletionResponse, error) {
	callCtx := a.applyLLMCall(ctx, req)
	callCtx, cancel := a.withLLMTimeout(callCtx)
	if cancel != nil {
		defer cancel()
	}

	// Start timing for tracing
	callCtx = startLLMCallTiming(callCtx)

	stream, err := a.provider.Stream(callCtx, req)
	if err != nil {
		iterationErr := fmt.Errorf("provider stream error: %w", err)
		a.applyLLMResponse(callCtx, nil, iterationErr)
		return nil, a.handleIterationError(callCtx, events, iterationErr, "streaming failed", "model", a.model)
	}
	defer stream.Close()

	// Accumulate streaming response
	var content string
	var toolCalls []providers.ToolCall
	var usage *providers.TokenUsage
	var finishReason providers.FinishReason

	// Track tool calls being built
	activeToolCalls := make(map[string]*providers.ToolCall)

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "io: EOF" {
				break
			}
			return nil, fmt.Errorf("stream read error: %w", err)
		}

		// Emit thinking chunks
		if chunk.Content != "" {
			content += chunk.Content
			a.emit(ctx, events, Thinking(chunk.Content))
		}

		// Handle tool call chunks
		if chunk.ToolCallID != "" {
			if activeToolCalls[chunk.ToolCallID] == nil {
				activeToolCalls[chunk.ToolCallID] = &providers.ToolCall{
					ID:        chunk.ToolCallID,
					Arguments: make(map[string]any),
				}
			}
			tc := activeToolCalls[chunk.ToolCallID]
			if chunk.ToolName != "" {
				tc.Name = chunk.ToolName
			}
			// Note: ToolArgs come as delta, would need to accumulate and parse at end
			// For now, we'll handle this when chunk.IsComplete
		}

		// Handle completion
		if chunk.IsComplete {
			finishReason = chunk.FinishReason
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
			
			// Collect completed tool calls
			for _, tc := range activeToolCalls {
				toolCalls = append(toolCalls, *tc)
			}
			break
		}
	}

	resp := &providers.CompletionResponse{
		ID:           fmt.Sprintf("stream-%d", len(content)), // Generate ID
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Model:        a.model,
	}
	if usage != nil {
		resp.Usage = *usage
	}

	a.applyLLMResponse(callCtx, resp, nil)
	
	// TODO: Re-enable tracing after refactoring to use Tracer.LogGeneration
	// a.logLLMGeneration(callCtx, req, resp)

	return resp, nil
}
