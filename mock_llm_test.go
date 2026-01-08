package agentkit

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestMockLLM_CreateResponse(t *testing.T) {
	mock := NewMockLLM().WithFinalResponse("hello")

	resp, err := mock.CreateResponse(context.Background(), ResponseRequest{})
	if err != nil {
		t.Fatalf("expected response, got error: %v", err)
	}

	if resp.Status != "completed" {
		t.Fatalf("expected status completed, got %s", resp.Status)
	}

	if len(resp.Output) == 0 {
		t.Fatal("expected output items")
	}

	content := resp.Output[0].Content
	if len(content) == 0 || content[0].Text != "hello" {
		t.Fatalf("expected content 'hello', got %#v", content)
	}

	_, err = mock.CreateResponse(context.Background(), ResponseRequest{})
	if !errors.Is(err, ErrMockResponseNotFound) {
		t.Fatalf("expected ErrMockResponseNotFound, got %v", err)
	}
}

func TestMockLLM_CreateResponseStream(t *testing.T) {
	mock := NewMockLLM().WithStream(ResponseStreamChunk{Type: "response.output_text.delta", Delta: "hi"})

	stream, err := mock.CreateResponseStream(context.Background(), ResponseRequest{})
	if err != nil {
		t.Fatalf("expected stream, got error: %v", err)
	}

	chunk, err := stream.Recv()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if chunk.Delta != "hi" {
		t.Fatalf("expected delta hi, got %q", chunk.Delta)
	}

	_, err = stream.Recv()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}
