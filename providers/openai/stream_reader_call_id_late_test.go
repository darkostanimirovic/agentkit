package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderCallIDArrivesAfterArguments(t *testing.T) {
	sse := "data: {\"type\":\"response.function_call_arguments.done\",\"item_id\":\"fc_abc\",\"arguments\":\"{\\\"task\\\":\\\"colors\\\"}\"}\n\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_abc\",\"call_id\":\"call_xyz\",\"name\":\"delegate_to_color_specialist\"}}\n\n"
	reader := newStreamReader(io.NopCloser(strings.NewReader(sse)), nil)
	defer reader.Close()

	chunk, err := reader.Next()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if chunk.ToolCallID != "call_xyz" {
		t.Fatalf("expected tool call id 'call_xyz', got '%s'", chunk.ToolCallID)
	}
	if chunk.ToolName != "delegate_to_color_specialist" {
		t.Fatalf("expected tool name, got '%s'", chunk.ToolName)
	}
	if chunk.ToolArgs != "{\"task\":\"colors\"}" {
		t.Fatalf("expected tool args, got '%s'", chunk.ToolArgs)
	}
}
