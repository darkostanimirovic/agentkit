package mock

import (
	"context"
	"io"
	"testing"

	"github.com/darkostanimirovic/agentkit/providers"
)

func TestProvider_Name(t *testing.T) {
	p := New()
	if p.Name() != "mock" {
		t.Errorf("Expected name 'mock', got '%s'", p.Name())
	}
}

func TestProvider_WithResponse(t *testing.T) {
	p := New().
		WithResponse("test content", nil).
		WithResponse("second content", []providers.ToolCall{
			{Name: "tool1", Arguments: map[string]any{"key": "value"}},
		})

	// First call should return first response
	resp1, err := p.Complete(context.Background(), providers.CompletionRequest{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp1.Content != "test content" {
		t.Errorf("Expected 'test content', got '%s'", resp1.Content)
	}
	if len(resp1.ToolCalls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(resp1.ToolCalls))
	}
	if resp1.FinishReason != providers.FinishReasonStop {
		t.Errorf("Expected FinishReasonStop, got %s", resp1.FinishReason)
	}

	// Second call should return second response
	resp2, err := p.Complete(context.Background(), providers.CompletionRequest{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp2.Content != "second content" {
		t.Errorf("Expected 'second content', got '%s'", resp2.Content)
	}
	if len(resp2.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(resp2.ToolCalls))
	}
	if resp2.FinishReason != providers.FinishReasonToolCalls {
		t.Errorf("Expected FinishReasonToolCalls, got %s", resp2.FinishReason)
	}

	// Third call should error
	_, err = p.Complete(context.Background(), providers.CompletionRequest{})
	if err != ErrNoResponse {
		t.Errorf("Expected ErrNoResponse, got %v", err)
	}
}

func TestProvider_WithStream(t *testing.T) {
	chunks := []providers.StreamChunk{
		{Content: "Hello"},
		{Content: " world"},
		{Content: "!", IsComplete: true},
	}

	p := New().WithStream(chunks)

	stream, err := p.Stream(context.Background(), providers.CompletionRequest{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer stream.Close()

	// Read all chunks
	var got []string
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error reading chunk: %v", err)
		}
		got = append(got, chunk.Content)
	}

	if len(got) != 3 {
		t.Fatalf("Expected 3 chunks, got %d", len(got))
	}
	if got[0] != "Hello" || got[1] != " world" || got[2] != "!" {
		t.Errorf("Unexpected chunks: %v", got)
	}
}

func TestProvider_StreamNoData(t *testing.T) {
	p := New()

	_, err := p.Stream(context.Background(), providers.CompletionRequest{})
	if err != ErrNoStream {
		t.Errorf("Expected ErrNoStream, got %v", err)
	}
}

func TestProvider_CallCount(t *testing.T) {
	p := New().
		WithResponse("resp1", nil).
		WithResponse("resp2", nil).
		WithStream([]providers.StreamChunk{{Content: "test"}})

	if p.CallCount() != 0 {
		t.Errorf("Expected call count 0, got %d", p.CallCount())
	}

	_, _ = p.Complete(context.Background(), providers.CompletionRequest{})
	if p.CallCount() != 1 {
		t.Errorf("Expected call count 1, got %d", p.CallCount())
	}

	_, _ = p.Complete(context.Background(), providers.CompletionRequest{})
	if p.CallCount() != 2 {
		t.Errorf("Expected call count 2, got %d", p.CallCount())
	}

	_, _ = p.Stream(context.Background(), providers.CompletionRequest{})
	if p.CallCount() != 3 {
		t.Errorf("Expected call count 3, got %d", p.CallCount())
	}
}

func TestStreamReader_ClosedStream(t *testing.T) {
	chunks := []providers.StreamChunk{
		{Content: "test"},
	}

	p := New().WithStream(chunks)
	stream, err := p.Stream(context.Background(), providers.CompletionRequest{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		t.Fatalf("Expected no error closing stream, got %v", err)
	}

	// Try to read after closing
	_, err = stream.Next()
	if err != ErrNoStream {
		t.Errorf("Expected ErrNoStream after closing, got %v", err)
	}
}

func TestProvider_TokenUsage(t *testing.T) {
	p := New().WithResponse("test", nil)

	resp, err := p.Complete(context.Background(), providers.CompletionRequest{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 20 {
		t.Errorf("Expected 20 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", resp.Usage.TotalTokens)
	}
}
