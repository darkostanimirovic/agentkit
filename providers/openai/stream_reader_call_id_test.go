package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderUsesCallIDForToolOutput(t *testing.T) {
	sse := "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_123\",\"name\":\"delegate_to_color_specialist\"}}\n\n" +
		"data: {\"type\":\"response.function_call_arguments.done\",\"item_id\":\"fc_123\",\"call_id\":\"call_456\",\"arguments\":\"{\\\"task\\\":\\\"colors\\\"}\"}\n\n" +
		"data: {\"type\":\"response.done\"}\n\n"
	reader := newStreamReader(io.NopCloser(strings.NewReader(sse)), nil)
	defer reader.Close()

	chunk, err := reader.Next()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if chunk.ToolCallID != "call_456" {
		t.Fatalf("expected tool call id 'call_456', got '%s'", chunk.ToolCallID)
	}
	if chunk.ToolName != "delegate_to_color_specialist" {
		t.Fatalf("expected tool name, got '%s'", chunk.ToolName)
	}
	if chunk.ToolArgs != "{\"task\":\"colors\"}" {
		t.Fatalf("expected tool args, got '%s'", chunk.ToolArgs)
	}
}
