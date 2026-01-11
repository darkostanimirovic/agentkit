package agentkit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/darkostanimirovic/agentkit/providers"
)

// LLMProvider abstracts the Responses API client for testing and custom providers.
// DEPRECATED: Use Provider interface instead for better decoupling.
type LLMProvider interface {
	CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error)
	CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error)
}

// ProviderAdapter adapts the new Provider interface to the legacy LLMProvider interface.
// This maintains backward compatibility while allowing new code to use the cleaner Provider interface.
type ProviderAdapter struct {
	provider providers.Provider
}

// NewProviderAdapter creates an adapter from a Provider to LLMProvider.
func NewProviderAdapter(provider providers.Provider) LLMProvider {
	return &ProviderAdapter{provider: provider}
}

// CreateResponse implements LLMProvider using the new Provider interface.
func (a *ProviderAdapter) CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error) {
	// Convert old request format to new domain model
	domainReq := a.convertRequest(req)
	
	// Call provider
	resp, err := a.provider.Complete(ctx, domainReq)
	if err != nil {
		return nil, err
	}
	
	// Convert back to old response format
	return a.convertResponse(resp, req), nil
}

// CreateResponseStream implements LLMProvider streaming using the new Provider interface.
func (a *ProviderAdapter) CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error) {
	// Convert old request format to new domain model
	domainReq := a.convertRequest(req)
	
	// Call provider
	stream, err := a.provider.Stream(ctx, domainReq)
	if err != nil {
		return nil, err
	}
	
	// Wrap stream to adapt interface
	return &streamAdapter{stream: stream}, nil
}

func (a *ProviderAdapter) convertRequest(req ResponseRequest) providers.CompletionRequest {
	domainReq := providers.CompletionRequest{
		Model:             req.Model,
		SystemPrompt:      req.Instructions,
		Temperature:       req.Temperature,
		MaxTokens:         req.MaxOutputTokens,
		TopP:              req.TopP,
		Stream:            req.Stream,
		ParallelToolCalls: req.ParallelToolCalls,
		Metadata:          req.Metadata,
	}
	
	// Convert tools
	if len(req.Tools) > 0 {
		domainReq.Tools = make([]providers.ToolDefinition, len(req.Tools))
		for i, tool := range req.Tools {
			domainReq.Tools[i] = providers.ToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			}
		}
	}
	
	// Convert reasoning effort
	if req.Reasoning != nil {
		domainReq.ReasoningEffort = providers.ReasoningEffort(req.Reasoning.Effort)
	}
	
	return domainReq
}

func (a *ProviderAdapter) convertResponse(resp *providers.CompletionResponse, originalReq ResponseRequest) *ResponseObject {
	// Build output items
	output := []ResponseOutputItem{}
	
	// Add message output if there's content
	if resp.Content != "" {
		output = append(output, ResponseOutputItem{
			Type:   "message",
			ID:     "msg_" + resp.ID,
			Status: "completed",
			Role:   "assistant",
			Content: []ResponseContentItem{
				{
					Type: "output_text",
					Text: resp.Content,
				},
			},
		})
	}
	
	// Add tool calls
	for _, tc := range resp.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Arguments)
		output = append(output, ResponseOutputItem{
			Type:      "function_call",
			ID:        tc.ID,
			Name:      tc.Name,
			CallID:    tc.ID,
			Arguments: string(argsJSON),
			Status:    "completed",
		})
	}
	
	return &ResponseObject{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: resp.Created.Unix(),
		Status:    "completed",
		Model:     resp.Model,
		Output:    output,
		Usage: ResponseUsage{
			InputTokens:     resp.Usage.PromptTokens,
			OutputTokens:    resp.Usage.CompletionTokens,
			ReasoningTokens: resp.Usage.ReasoningTokens,
			TotalTokens:     resp.Usage.TotalTokens,
		},
	}
}

// streamAdapter adapts StreamReader to ResponseStreamClient.
type streamAdapter struct {
	stream providers.StreamReader
}

func (s *streamAdapter) ReadChunk() (*ResponseStreamChunk, error) {
	chunk, err := s.stream.Next()
	if err != nil {
		return nil, err
	}
	
	// Convert domain chunk to API chunk
	apiChunk := &ResponseStreamChunk{}
	
	if chunk.Content != "" {
		apiChunk.Type = "response.output_text.delta"
		apiChunk.Delta = chunk.Content
	} else if chunk.ToolName != "" {
		apiChunk.Type = "response.function_call_arguments.done"
		apiChunk.Name = chunk.ToolName
		apiChunk.Arguments = chunk.ToolArgs
	} else if chunk.ToolArgs != "" {
		apiChunk.Type = "response.function_call_arguments.delta"
		apiChunk.Delta = chunk.ToolArgs
	} else if chunk.IsComplete {
		apiChunk.Type = "response.done"
		if chunk.Usage != nil {
			apiChunk.Usage = &ResponseUsage{
				InputTokens:     chunk.Usage.PromptTokens,
				OutputTokens:    chunk.Usage.CompletionTokens,
				ReasoningTokens: chunk.Usage.ReasoningTokens,
				TotalTokens:     chunk.Usage.TotalTokens,
			}
		}
	}
	
	return apiChunk, nil
}

func (s *streamAdapter) Close() error {
	return s.stream.Close()
}

// llmProviderWrapper wraps LLMProvider to implement providers.Provider interface.
type llmProviderWrapper struct {
	llm LLMProvider
}

func (w *llmProviderWrapper) Complete(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	// Convert providers.CompletionRequest to ResponseRequest
	apiReq := w.toResponseRequest(req)
	
	resp, err := w.llm.CreateResponse(ctx, apiReq)
	if err != nil {
		return nil, err
	}
	
	// Convert ResponseObject to providers.CompletionResponse
	return w.fromResponseObject(resp), nil
}

func (w *llmProviderWrapper) Stream(ctx context.Context, req providers.CompletionRequest) (providers.StreamReader, error) {
	// Convert providers.CompletionRequest to ResponseRequest
	apiReq := w.toResponseRequest(req)
	
	stream, err := w.llm.CreateResponseStream(ctx, apiReq)
	if err != nil {
		return nil, err
	}
	
	// Wrap ResponseStreamClient as StreamReader
	return &responseStreamWrapper{stream: stream}, nil
}

func (w *llmProviderWrapper) Name() string {
	return "llm-provider-wrapper"
}

func (w *llmProviderWrapper) toResponseRequest(req providers.CompletionRequest) ResponseRequest {
	apiReq := ResponseRequest{
		Model:             req.Model,
		Instructions:      req.SystemPrompt,
		Temperature:       req.Temperature,
		MaxOutputTokens:   req.MaxTokens,
		TopP:              req.TopP,
		Stream:            req.Stream,
		ParallelToolCalls: req.ParallelToolCalls,
		Metadata:          req.Metadata,
	}
	
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]ResponseTool, len(req.Tools))
		for i, tool := range req.Tools {
			apiReq.Tools[i] = ResponseTool{
				Type:        "function",
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			}
		}
	}
	
	if req.ReasoningEffort != "" {
		apiReq.Reasoning = &ResponseReasoning{
			Effort: ReasoningEffort(req.ReasoningEffort),
		}
	}
	
	return apiReq
}

func (w *llmProviderWrapper) fromResponseObject(resp *ResponseObject) *providers.CompletionResponse {
	domainResp := &providers.CompletionResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Created: time.Unix(resp.CreatedAt, 0),
		Usage: providers.TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			ReasoningTokens:  resp.Usage.ReasoningTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
	
	// Extract content and tool calls from output
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, content := range item.Content {
				if content.Type == "text" || content.Type == "output_text" {
					domainResp.Content += content.Text
				}
			}
		case "function_call":
			var args map[string]any
			if item.Arguments != "" {
				json.Unmarshal([]byte(item.Arguments), &args)
			}
			domainResp.ToolCalls = append(domainResp.ToolCalls, providers.ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: args,
			})
		}
	}
	
	if len(domainResp.ToolCalls) > 0 {
		domainResp.FinishReason = providers.FinishReasonToolCalls
	} else {
		domainResp.FinishReason = providers.FinishReasonStop
	}
	
	return domainResp
}

// responseStreamWrapper wraps ResponseStreamClient to implement providers.StreamReader.
type responseStreamWrapper struct {
	stream ResponseStreamClient
}

func (r *responseStreamWrapper) Next() (*providers.StreamChunk, error) {
	apiChunk, err := r.stream.ReadChunk()
	if err != nil {
		return nil, err
	}
	
	chunk := &providers.StreamChunk{}
	
	switch apiChunk.Type {
	case "response.output_text.delta":
		chunk.Content = apiChunk.Delta
	case "response.function_call_arguments.delta":
		chunk.ToolArgs = apiChunk.Delta
	case "response.function_call_arguments.done":
		chunk.ToolName = apiChunk.Name
		chunk.ToolArgs = apiChunk.Arguments
	case "response.done":
		chunk.IsComplete = true
		if apiChunk.Usage != nil {
			chunk.Usage = &providers.TokenUsage{
				PromptTokens:     apiChunk.Usage.InputTokens,
				CompletionTokens: apiChunk.Usage.OutputTokens,
				ReasoningTokens:  apiChunk.Usage.ReasoningTokens,
				TotalTokens:      apiChunk.Usage.TotalTokens,
			}
		}
	}
	
	return chunk, nil
}

func (r *responseStreamWrapper) Close() error {
	return r.stream.Close()
}
