package openai

import (
	"testing"

	"github.com/darkostanimirovic/agentkit/providers"
)

func TestToAPIInput_ToolRoleMappedToUser(t *testing.T) {
	p := New("test", nil)
	inputs := p.toAPIInput([]providers.Message{
		{
			Role:       providers.RoleTool,
			Content:    "tool output",
			ToolCallID: "call_123",
		},
	})
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	item, ok := inputs[0].(functionCallOutput)
	if !ok {
		t.Fatalf("expected functionCallOutput, got %T", inputs[0])
	}
	if item.Type != "function_call_output" {
		t.Fatalf("expected type 'function_call_output', got '%s'", item.Type)
	}
	if item.CallID != "call_123" {
		t.Fatalf("expected call_id 'call_123', got '%s'", item.CallID)
	}
	if item.Output != "tool output" {
		t.Fatalf("expected tool output 'tool output', got '%s'", item.Output)
	}
}
