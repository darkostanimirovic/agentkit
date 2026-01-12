package openai

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderSkipsToolCallWithoutCallID(t *testing.T) {
	sse := "data: {\"type\":\"response.done\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"created_at\":0,\"status\":\"completed\",\"model\":\"gpt-5-mini\",\"output\":[{\"type\":\"function_call\",\"id\":\"fc_123\",\"name\":\"delegate_to_color_specialist\",\"arguments\":\"{\\\"task\\\":\\\"colors\\\"}\"}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}\n\n"
	reader := newStreamReader(io.NopCloser(strings.NewReader(sse)), nil)
	defer reader.Close()

	chunk, err := reader.Next()
	if err != nil {
		t.Fatalf("expected chunk, got error: %v", err)
	}
	if !chunk.IsComplete {
		t.Fatal("expected completion chunk")
	}
	if chunk.ToolCallID != "" || chunk.ToolName != "" || chunk.ToolArgs != "" {
		t.Fatalf("expected no tool call emitted, got id=%q name=%q args=%q", chunk.ToolCallID, chunk.ToolName, chunk.ToolArgs)
	}
}
