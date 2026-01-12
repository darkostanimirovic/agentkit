package openai

import (
	"testing"

	"github.com/darkostanimirovic/agentkit/providers"
)

func TestToAPIInput_AssistantUsesOutputText(t *testing.T) {
	p := New("test", nil)
	inputs := p.toAPIInput([]providers.Message{
		{
			Role:    providers.RoleAssistant,
			Content: "assistant reply",
		},
	})
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	in, ok := inputs[0].(input)
	if !ok {
		t.Fatalf("expected input type, got %T", inputs[0])
	}
	if in.Role != "assistant" {
		t.Fatalf("expected role 'assistant', got '%s'", in.Role)
	}
	if len(in.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(in.Content))
	}
	item := in.Content[0]
	if item.Type != "output_text" {
		t.Fatalf("expected content type 'output_text', got '%s'", item.Type)
	}
	if item.Text != "assistant reply" {
		t.Fatalf("expected text 'assistant reply', got '%s'", item.Text)
	}
}
