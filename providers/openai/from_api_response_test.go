package openai

import "testing"

func TestFromAPIResponseSkipsFunctionCallWithoutCallID(t *testing.T) {
	p := New("test", nil)
	resp := &responseObject{
		ID:     "resp_1",
		Model:  "gpt-5-mini",
		Status: "completed",
		Output: []outputItem{
			{
				Type:      "function_call",
				ID:        "fc_123",
				Name:      "delegate_to_color_specialist",
				Arguments: "{\"task\":\"colors\"}",
			},
		},
	}
	domain := p.fromAPIResponse(resp)
	if len(domain.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls without call_id, got %d", len(domain.ToolCalls))
	}
}
