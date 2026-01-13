package agentkit


import (
	"errors"
	"testing"
	"time"
)

func TestThinkingChunk(t *testing.T) {
	chunk := "Analyzing the issue..."
	event := ThinkingChunk(chunk)

	if event.Type != EventTypeThinkingChunk {
		t.Errorf("expected type %s, got %s", EventTypeThinkingChunk, event.Type)
	}

	if event.Data["chunk"] != chunk {
		t.Errorf("expected chunk %s, got %v", chunk, event.Data["chunk"])
	}

	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	if time.Since(event.Timestamp) > time.Second {
		t.Error("timestamp is too old")
	}
}

func TestActionDetected(t *testing.T) {
	description := "Assigning to platform team..."
	toolID := "call_123"
	event := ActionDetected(description, toolID)

	if event.Type != EventTypeActionDetected {
		t.Errorf("expected type %s, got %s", EventTypeActionDetected, event.Type)
	}

	if event.Data["description"] != description {
		t.Errorf("expected description %s, got %v", description, event.Data["description"])
	}

	if event.Data["tool_id"] != toolID {
		t.Errorf("expected tool_id %s, got %v", toolID, event.Data["tool_id"])
	}
}

func TestReasoningChunk(t *testing.T) {
	chunk := "Reasoning summary..."
	event := ReasoningChunk(chunk)

	if event.Type != EventTypeReasoningChunk {
		t.Errorf("expected type %s, got %s", EventTypeReasoningChunk, event.Type)
	}

	if event.Data["chunk"] != chunk {
		t.Errorf("expected chunk %s, got %v", chunk, event.Data["chunk"])
	}
}

func TestResponseChunk(t *testing.T) {
	chunk := "Response delta..."
	event := ResponseChunk(chunk)

	if event.Type != EventTypeResponseChunk {
		t.Errorf("expected type %s, got %s", EventTypeResponseChunk, event.Type)
	}

	if event.Data["chunk"] != chunk {
		t.Errorf("expected chunk %s, got %v", chunk, event.Data["chunk"])
	}
}

func TestActionResult(t *testing.T) {
	description := "✓ Assigned to platform team"
	result := map[string]any{
		"success": true,
		"team_id": "123",
	}
	event := ActionResult(description, result)

	if event.Type != EventTypeActionResult {
		t.Errorf("expected type %s, got %s", EventTypeActionResult, event.Type)
	}

	if event.Data["description"] != description {
		t.Errorf("expected description %s, got %v", description, event.Data["description"])
	}

	resultData, ok := event.Data["result"].(map[string]any)
	if !ok {
		t.Fatal("expected result to be map[string]any")
	}

	if resultData["success"] != true {
		t.Error("expected success to be true")
	}

	if resultData["team_id"] != "123" {
		t.Errorf("expected team_id 123, got %v", resultData["team_id"])
	}
}

func TestFinalOutput(t *testing.T) {
	summary := "Analysis complete"
	response := "I've analyzed the issue and assigned it to the infrastructure team."
	event := FinalOutput(summary, response)

	if event.Type != EventTypeFinalOutput {
		t.Errorf("expected type %s, got %s", EventTypeFinalOutput, event.Type)
	}

	if event.Data["summary"] != summary {
		t.Errorf("expected summary %s, got %v", summary, event.Data["summary"])
	}

	if event.Data["response"] != response {
		t.Errorf("expected response %s, got %v", response, event.Data["response"])
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "simple error",
			err:      errors.New("test error"),
			expected: "test error",
		},
		{
			name:     "formatted error",
			err:      errors.New("API call failed: connection timeout"),
			expected: "API call failed: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Error(tt.err)

			if event.Type != EventTypeError {
				t.Errorf("expected type %s, got %s", EventTypeError, event.Type)
			}

			if event.Data["error"] != tt.expected {
				t.Errorf("expected error %s, got %v", tt.expected, event.Data["error"])
			}
		})
	}
}

func TestEventTimestamps(t *testing.T) {
	event1 := ThinkingChunk("first")
	time.Sleep(10 * time.Millisecond)
	event2 := ThinkingChunk("second")

	if !event2.Timestamp.After(event1.Timestamp) {
		t.Error("expected event2 timestamp to be after event1")
	}
}

func TestEventTypes(t *testing.T) {
	types := []EventType{
		EventTypeThinkingChunk,
		EventTypeReasoningChunk,
		EventTypeResponseChunk,
		EventTypeActionDetected,
		EventTypeActionResult,
		EventTypeFinalOutput,
		EventTypeError,
	}

	seen := make(map[EventType]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate event type: %s", typ)
		}
		seen[typ] = true
	}

	expectedTypes := map[EventType]string{
		EventTypeThinkingChunk:  "thinking_chunk",
		EventTypeReasoningChunk: "reasoning_chunk",
		EventTypeResponseChunk:  "response_chunk",
		EventTypeActionDetected: "action_detected",
		EventTypeActionResult:   "action_result",
		EventTypeFinalOutput:    "final_output",
		EventTypeError:          "error",
	}

	for typ, expected := range expectedTypes {
		if string(typ) != expected {
			t.Errorf("expected %s, got %s", expected, string(typ))
		}
	}
}

func TestFilterEvents(t *testing.T) {
	input := make(chan Event, 4)
	input <- ThinkingChunk("one")
	input <- ActionDetected("do", "call-1")
	input <- FinalOutput("done", "ok")
	close(input)

	filtered := FilterEvents(input, EventTypeActionDetected, EventTypeFinalOutput)

	got := make([]EventType, 0, 2)
	for event := range filtered {
		got = append(got, event.Type)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0] != EventTypeActionDetected || got[1] != EventTypeFinalOutput {
		t.Fatalf("unexpected event types: %v", got)
	}
}

func TestEventRecorder(t *testing.T) {
	input := make(chan Event, 2)
	input <- ThinkingChunk("hi")
	input <- FinalOutput("done", "ok")
	close(input)

	recorder := NewEventRecorder()
	out := recorder.Record(input)

	count := 0
	for range out {
		count++
	}

	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 recorded events, got %d", len(events))
	}
}
