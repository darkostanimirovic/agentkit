package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderFunctionCallArgumentsDoneUsesArguments(t *testing.T) {
	sse := "data: {\"type\":\"response.function_call_arguments.done\",\"item_id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"delegate_to_color_specialist\",\"arguments\":\"{\\\"task\\\":\\\"color palette\\\"}\"}\n\n" +
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
	if chunk.ToolArgs != "{\"task\":\"color palette\"}" {
		t.Fatalf("expected tool args, got '%s'", chunk.ToolArgs)
	}
}
