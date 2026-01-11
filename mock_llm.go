package agentkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// MockToolCall is a simplified tool call used for mock responses.
// DEPRECATED: Use ToolCall from provider.go instead.
type MockToolCall struct {
	Name string
	Args map[string]any
}

// ErrMockResponseNotFound is returned when no mock responses are configured.
var ErrMockResponseNotFound = errors.New("agentkit: mock response not found")

// ErrMockStreamNotFound is returned when no mock streams are configured.
var ErrMockStreamNotFound = errors.New("agentkit: mock stream not found")

// MockLLM provides deterministic responses for testing without hitting real APIs.
type MockLLM struct {
	mu        sync.Mutex
	responses []*ResponseObject
	streams   [][]ResponseStreamChunk
	callCount int
}

// NewMockLLM creates a new mock LLM provider.
func NewMockLLM() *MockLLM {
	return &MockLLM{}
}

// WithResponse appends a mock response with optional tool calls.
func (m *MockLLM) WithResponse(text string, toolCalls []MockToolCall) *MockLLM {
	m.mu.Lock()
	defer m.mu.Unlock()

	responseID := fmt.Sprintf("mock-response-%d", len(m.responses)+1)
	output := ResponseOutputItem{
		Type:    "message",
		Role:    "assistant",
		Content: []ResponseContentItem{{Type: "text", Text: text}},
	}

	if len(toolCalls) > 0 {
		calls := make([]ResponseToolCall, 0, len(toolCalls))
		for i, call := range toolCalls {
			argsJSON, err := json.Marshal(call.Args)
			if err != nil {
				argsJSON = []byte("{}")
			}
			callID := fmt.Sprintf("mock-call-%d-%d", len(m.responses)+1, i+1)
			calls = append(calls, ResponseToolCall{
				ID:        callID,
				CallID:    callID,
				Type:      "function_call",
				Name:      call.Name,
				Arguments: string(argsJSON),
			})
		}
		output.ToolCalls = calls
	}

	m.responses = append(m.responses, &ResponseObject{
		ID:        responseID,
		Status:    "completed",
		CreatedAt: time.Now().Unix(),
		Output:    []ResponseOutputItem{output},
	})

	return m
}

// WithFinalResponse appends a mock final response.
func (m *MockLLM) WithFinalResponse(text string) *MockLLM {
	return m.WithResponse(text, nil)
}

// WithStream appends a mock stream of response chunks.
func (m *MockLLM) WithStream(chunks ...ResponseStreamChunk) *MockLLM {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream := make([]ResponseStreamChunk, len(chunks))
	copy(stream, chunks)
	m.streams = append(m.streams, stream)
	return m
}

// CreateResponse returns the next configured mock response.
func (m *MockLLM) CreateResponse(_ context.Context, _ ResponseRequest) (*ResponseObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.responses) == 0 {
		return nil, ErrMockResponseNotFound
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]
	m.callCount++
	return resp, nil
}

// CreateResponseStream returns the next configured mock stream.
func (m *MockLLM) CreateResponseStream(_ context.Context, _ ResponseRequest) (ResponseStreamClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.streams) == 0 {
		return nil, ErrMockStreamNotFound
	}

	stream := &mockStream{chunks: m.streams[0]}
	m.streams = m.streams[1:]
	m.callCount++
	return stream, nil
}

type mockStream struct {
	mu     sync.Mutex
	chunks []ResponseStreamChunk
	idx    int
	closed bool
}

func (m *mockStream) ReadChunk() (*ResponseStreamChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, ErrMockStreamNotFound
	}

	if m.idx >= len(m.chunks) {
		return nil, io.EOF
	}

	chunk := m.chunks[m.idx]
	m.idx++
	return &chunk, nil
}

func (m *mockStream) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
