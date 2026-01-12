package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderFunctionCallOutputItemDone(t *testing.T) {
	sse := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"id\":\"call_1\",\"name\":\"delegate_to_color_specialist\",\"arguments\":\"{\\\"prompt\\\":\\\"hi\\\"}\"}}\n\n" +
		"data: {\"type\":\"response.done\"}\n\n"
	reader := newStreamReader(io.NopCloser(strings.NewReader(sse)), nil)
	defer reader.Close()

	chunk, err := reader.Next()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if chunk.ToolCallID != "call_1" {
		t.Fatalf("expected tool call id 'call_1', got '%s'", chunk.ToolCallID)
	}
	if chunk.ToolName != "delegate_to_color_specialist" {
		t.Fatalf("expected tool name, got '%s'", chunk.ToolName)
	}
	if chunk.ToolArgs != "{\"prompt\":\"hi\"}" {
		t.Fatalf("expected tool args, got '%s'", chunk.ToolArgs)
	}

	chunk, err = reader.Next()
	if err != nil {
		t.Fatalf("expected completion chunk, got error: %v", err)
	}
	if !chunk.IsComplete {
		t.Fatal("expected completion chunk")
	}
}
