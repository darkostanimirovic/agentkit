package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderResponseCompletedEmitsText(t *testing.T) {
	sse := "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"created_at\":0,\"status\":\"completed\",\"model\":\"gpt-5-mini\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}\n\n"
	reader := newStreamReader(io.NopCloser(strings.NewReader(sse)), nil)
	defer reader.Close()

	chunk, err := reader.Next()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if chunk == nil {
		t.Fatal("expected chunk, got nil")
	}
	if chunk.Content != "Hello" {
		t.Fatalf("expected content 'Hello', got '%s'", chunk.Content)
	}
	if !chunk.IsComplete {
		t.Fatal("expected completion chunk")
	}
	if chunk.Usage == nil || chunk.Usage.TotalTokens != 3 {
		t.Fatalf("expected usage totals to be populated, got %+v", chunk.Usage)
	}

	_, err = reader.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF after completion, got: %v", err)
	}
}

func TestStreamReaderOutputTextDoneDoesNotDuplicateOnCompleted(t *testing.T) {
	completed := "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_2\",\"object\":\"response\",\"created_at\":0,\"status\":\"completed\",\"model\":\"gpt-5-mini\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}\n\n"
	sse := "data: {\"type\":\"response.output_text.done\",\"text\":\"Hello\"}\n\n" + completed
	reader := newStreamReader(io.NopCloser(strings.NewReader(sse)), nil)
	defer reader.Close()

	chunk, err := reader.Next()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if chunk.Content != "Hello" {
		t.Fatalf("expected content 'Hello', got '%s'", chunk.Content)
	}

	chunk, err = reader.Next()
	if err != nil {
		t.Fatalf("expected completion chunk, got error: %v", err)
	}
	if chunk.Content != "" {
		t.Fatalf("expected no duplicate content, got '%s'", chunk.Content)
	}
	if !chunk.IsComplete {
		t.Fatal("expected completion chunk")
	}
}
