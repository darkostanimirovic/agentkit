// Package mock implements a mock Provider for testing.
package mock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/darkostanimirovic/agentkit/providers"
)

var (
	ErrNoResponse = errors.New("mock: no response configured")
	ErrNoStream   = errors.New("mock: no stream configured")
)

// Provider implements providers.Provider for testing.
type Provider struct {
	mu        sync.Mutex
	responses []*providers.CompletionResponse
	streams   [][]providers.StreamChunk
	callCount int
}

// New creates a new mock provider.
func New() *Provider {
	return &Provider{}
}

// WithResponse appends a mock completion response.
func (m *Provider) WithResponse(content string, toolCalls []providers.ToolCall) *Provider {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp := &providers.CompletionResponse{
		ID:           fmt.Sprintf("mock-resp-%d", len(m.responses)+1),
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: providers.FinishReasonStop,
		Model:        "mock-model",
		Created:      time.Now(),
		Usage: providers.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	if len(toolCalls) > 0 {
		resp.FinishReason = providers.FinishReasonToolCalls
	}

	m.responses = append(m.responses, resp)
	return m
}

// WithStream appends a mock stream of chunks.
func (m *Provider) WithStream(chunks []providers.StreamChunk) *Provider {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream := make([]providers.StreamChunk, len(chunks))
	copy(stream, chunks)
	m.streams = append(m.streams, stream)
	return m
}

// Name returns the provider name.
func (m *Provider) Name() string {
	return "mock"
}

// Complete returns the next configured mock response.
func (m *Provider) Complete(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.responses) == 0 {
		return nil, ErrNoResponse
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]
	m.callCount++
	return resp, nil
}

// Stream returns the next configured mock stream.
func (m *Provider) Stream(ctx context.Context, req providers.CompletionRequest) (providers.StreamReader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.streams) == 0 {
		return nil, ErrNoStream
	}

	stream := &streamReader{chunks: m.streams[0]}
	m.streams = m.streams[1:]
	m.callCount++
	return stream, nil
}

// CallCount returns the number of times Complete or Stream was called.
func (m *Provider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

type streamReader struct {
	mu     sync.Mutex
	chunks []providers.StreamChunk
	idx    int
	closed bool
}

func (s *streamReader) Next() (*providers.StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrNoStream
	}

	if s.idx >= len(s.chunks) {
		return nil, io.EOF
	}

	chunk := s.chunks[s.idx]
	s.idx++
	return &chunk, nil
}

func (s *streamReader) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
