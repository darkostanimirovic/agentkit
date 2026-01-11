package agentkit

import (
	"context"
	"encoding/json"
)

// ResponseStreamClient provides streaming access to model responses.
type ResponseStreamClient interface {
	Recv() (*ResponseStreamChunk, error)
	Close() error
}

// LLMProvider abstracts the Responses API client for testing and custom providers.
// DEPRECATED: Use Provider interface instead for better decoupling.
type LLMProvider interface {
	CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error)
	CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error)
}

// ProviderAdapter adapts the new Provider interface to the legacy LLMProvider interface.
// This maintains backward compatibility while allowing new code to use the cleaner Provider interface.
type ProviderAdapter struct {
	provider Provider
}

// NewProviderAdapter creates an adapter from a Provider to LLMProvider.
func NewProviderAdapter(provider Provider) LLMProvider {
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

func (a *ProviderAdapter) convertRequest(req ResponseRequest) CompletionRequest {
	domainReq := CompletionRequest{
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
		domainReq.Tools = make([]ToolDefinition, len(req.Tools))
		for i, tool := range req.Tools {
			domainReq.Tools[i] = ToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			}
		}
	}
	
	// Convert reasoning effort
	if req.Reasoning != nil {
		domainReq.ReasoningEffort = req.Reasoning.Effort
	}
	
	return domainReq
}

func (a *ProviderAdapter) convertResponse(resp *CompletionResponse, originalReq ResponseRequest) *ResponseObject {
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
	stream StreamReader
}

func (s *streamAdapter) Recv() (*ResponseStreamChunk, error) {
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

