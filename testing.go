package agentkit

import (
	"context"

	"github.com/darkostanimirovic/agentkit/providers"
	mockprovider "github.com/darkostanimirovic/agentkit/providers/mock"
)

// ToolCall represents a tool call for testing purposes.
// This is an alias for providers.ToolCall to maintain backward compatibility.
type ToolCall = providers.ToolCall

// MockLLM is a convenience wrapper around providers/mock.Provider for easier testing.
// It provides a builder pattern for configuring mock responses.
type MockLLM struct {
	provider *mockprovider.Provider
}

// NewMockLLM creates a new mock LLM provider for testing.
// This is a convenience wrapper around providers/mock that maintains
// backward compatibility with the old testing API.
//
// Usage:
//
//	mock := agentkit.NewMockLLM().
//	    WithResponse("thinking...", []agentkit.ToolCall{
//	        {Name: "search", Args: map[string]any{"query": "test"}},
//	    }).
//	    WithFinalResponse("done")
//
//	agent, _ := agentkit.New(agentkit.Config{
//	    LLMProvider: mock,
//	    Model:       "test-model",
//	})
func NewMockLLM() *MockLLM {
	return &MockLLM{
		provider: mockprovider.New(),
	}
}

// WithResponse appends a mock response with optional tool calls.
func (m *MockLLM) WithResponse(text string, toolCalls []ToolCall) *MockLLM {
	m.provider.WithResponse(text, toolCalls)
	return m
}

// WithFinalResponse appends a mock final response without tool calls.
func (m *MockLLM) WithFinalResponse(text string) *MockLLM {
	m.provider.WithResponse(text, nil)
	return m
}

// WithStream appends a mock stream of response chunks.
func (m *MockLLM) WithStream(chunks []providers.StreamChunk) *MockLLM {
	m.provider.WithStream(chunks)
	return m
}

// CreateResponse implements the LLMProvider interface for backward compatibility.
func (m *MockLLM) CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error) {
	adapter := NewProviderAdapter(m.provider)
	return adapter.CreateResponse(ctx, req)
}

// CreateResponseStream implements the LLMProvider interface for backward compatibility.
func (m *MockLLM) CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error) {
	adapter := NewProviderAdapter(m.provider)
	return adapter.CreateResponseStream(ctx, req)
}
