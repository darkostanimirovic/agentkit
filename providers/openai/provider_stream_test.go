package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderOutputItemDoneProvidesContent(t *testing.T) {
	sseData := `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_001","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Hello from done."}]}}

data: {"type":"response.done","response_id":"resp_123","output":[]}

data: [DONE]

`

	reader := newStreamReader(io.NopCloser(strings.NewReader(sseData)), nil)

	var content strings.Builder
	var sawComplete bool

	for {
		chunk, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream read error: %v", err)
		}
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
		}
		if chunk.IsComplete {
			sawComplete = true
		}
	}

	if got := content.String(); got != "Hello from done." {
		t.Fatalf("expected content from output_item.done, got %q", got)
	}
	if !sawComplete {
		t.Fatal("expected response.done to mark stream complete")
	}
}

func TestStreamReaderContentPartDeltaProvidesContent(t *testing.T) {
	sseData := `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_001","status":"in_progress","role":"assistant","content":[]}}

data: {"type":"response.content_part.delta","part":{"type":"output_text","text":"Hello "}}

data: {"type":"response.content_part.delta","part":{"type":"output_text","text":"world"}}

data: {"type":"response.content_part.done","part":{"type":"output_text","text":"Hello world"}}

data: {"type":"response.done","response_id":"resp_123","output":[]}

data: [DONE]

`

	reader := newStreamReader(io.NopCloser(strings.NewReader(sseData)), nil)

	var content strings.Builder
	var sawComplete bool

	for {
		chunk, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream read error: %v", err)
		}
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
		}
		if chunk.IsComplete {
			sawComplete = true
		}
	}

	if got := content.String(); got != "Hello world" {
		t.Fatalf("expected content from content_part.delta, got %q", got)
	}
	if !sawComplete {
		t.Fatal("expected response.done to mark stream complete")
	}
}
