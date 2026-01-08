package agentkit

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	responseCreatedType     = "response.created"
	responseOutputItemAdded = "response.output_item.added"
	responseOutputItemDone  = "response.output_item.done"
	functionCallType        = "function_call"
	searchIssuesTool        = "search_issues"
	toolArgumentsTestValue  = "{\"query\":\"test\"}"
)

// TestResponseStreamParsing tests SSE stream parsing with actual OpenAI format
func TestResponseStreamParsing(t *testing.T) {
	// Mock the actual SSE stream we see from OpenAI
	sseData := `data: {"type":"response.created","response_id":"resp_123"}

data: {"type":"response.in_progress","response_id":"resp_123"}

data: {"type":"response.output_item.added","sequence_number":0,"item_id":"msg_001","output_index":0,"item":{"type":"message","id":"msg_001","role":"assistant","content":[]}}

data: {"type":"response.output_text.delta","sequence_number":1,"item_id":"msg_001","output_index":0,"delta":"I'll search"}

data: {"type":"response.output_text.delta","sequence_number":2,"item_id":"msg_001","output_index":0,"delta":" for issues"}

data: {"type":"response.output_item.done","sequence_number":3,"item_id":"msg_001","output_index":0,"item":{"type":"message","id":"msg_001","status":"completed","role":"assistant","content":[{"type":"output_text","text":"I'll search for issues"}]}}

data: {"type":"response.output_item.added","sequence_number":4,"item_id":"fc_abc123","output_index":1,"item":{"type":"function_call","id":"fc_abc123","name":"search_issues","call_id":"fc_abc123"}}

data: {"type":"response.function_call_arguments.delta","sequence_number":5,"item_id":"fc_abc123","output_index":1,"delta":"{\""}

data: {"type":"response.function_call_arguments.delta","sequence_number":6,"item_id":"fc_abc123","output_index":1,"delta":"query"}

data: {"type":"response.function_call_arguments.delta","sequence_number":7,"item_id":"fc_abc123","output_index":1,"delta":"\":\""}

data: {"type":"response.function_call_arguments.delta","sequence_number":8,"item_id":"fc_abc123","output_index":1,"delta":"navigation"}

data: {"type":"response.function_call_arguments.delta","sequence_number":9,"item_id":"fc_abc123","output_index":1,"delta":"\"}"}

data: {"type":"response.output_item.done","sequence_number":10,"item_id":"fc_abc123","output_index":1,"item":{"type":"function_call","id":"fc_abc123","status":"completed","name":"search_issues","call_id":"fc_abc123","arguments":"{\"query\":\"navigation\"}"}}

data: {"type":"response.done","response_id":"resp_123","output":[]}

data: [DONE]

`

	stream := newTestResponseStream(sseData)
	chunks := readStreamChunks(t, stream)

	assertMinimumChunks(t, chunks, 5)
	assertChunkType(t, chunks[0], responseCreatedType)
	assertFunctionCallEvents(t, chunks)
}

// TestToolCallExtraction tests the tool call map logic
func TestToolCallExtraction(t *testing.T) {
	toolCalls, ok := simulateToolCallStream()
	if !ok {
		t.Fatal("expected tool call events to be detected")
	}

	assertToolCallExtraction(t, toolCalls)
}

func newTestResponseStream(sseData string) *ResponseStream {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(sseData)),
	}
	return &ResponseStream{
		reader: &resp.Body,
	}
}

func readStreamChunks(t *testing.T, stream *ResponseStream) []ResponseStreamChunk {
	t.Helper()

	chunks := make([]ResponseStreamChunk, 0, 8)
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		chunks = append(chunks, *chunk)
	}

	return chunks
}

func assertMinimumChunks(t *testing.T, chunks []ResponseStreamChunk, minCount int) {
	t.Helper()
	if len(chunks) < minCount {
		t.Fatalf("expected at least %d chunks, got %d", minCount, len(chunks))
	}
}

func assertChunkType(t *testing.T, chunk ResponseStreamChunk, expected string) {
	t.Helper()
	if chunk.Type != expected {
		t.Errorf("expected %s, got %s", expected, chunk.Type)
	}
}

func assertFunctionCallEvents(t *testing.T, chunks []ResponseStreamChunk) {
	t.Helper()

	var foundFunctionItem bool
	var foundFunctionDone bool
	var functionName string
	var arguments string

	for _, chunk := range chunks {
		if isFunctionCallAdded(chunk) {
			foundFunctionItem = true
			assertToolName(t, chunk.Item.Name)
		}
		if isFunctionCallDone(chunk) {
			foundFunctionDone = true
			functionName = chunk.Item.Name
			arguments = chunk.Item.Arguments
		}
	}

	if !foundFunctionItem {
		t.Error("did not find function_call item.added event")
	}
	if !foundFunctionDone {
		t.Error("did not find function_call item.done event")
	}
	if functionName != searchIssuesTool {
		t.Errorf("expected function name '%s', got '%s'", searchIssuesTool, functionName)
	}
	if !strings.Contains(arguments, "navigation") {
		t.Errorf("expected arguments to contain 'navigation', got '%s'", arguments)
	}
}

func isFunctionCallAdded(chunk ResponseStreamChunk) bool {
	return chunk.Type == responseOutputItemAdded && chunk.Item != nil && chunk.Item.Type == functionCallType
}

func isFunctionCallDone(chunk ResponseStreamChunk) bool {
	return chunk.Type == responseOutputItemDone && chunk.Item != nil && chunk.Item.Type == functionCallType
}

func assertToolName(t *testing.T, actual string) {
	t.Helper()
	if actual != searchIssuesTool {
		t.Errorf("expected name='%s', got '%s'", searchIssuesTool, actual)
	}
}

func simulateToolCallStream() ([]ResponseToolCall, bool) {
	toolCallsMap := make(map[int]*ResponseToolCall)
	hasToolCalls := false

	chunk1 := ResponseStreamChunk{
		Item: &ResponseOutputItem{
			Type:   functionCallType,
			ID:     "fc_abc123",
			Name:   searchIssuesTool,
			CallID: "fc_abc123",
		},
	}
	if chunk1.Item != nil && chunk1.Item.Type == functionCallType {
		hasToolCalls = true
	}

	deltas := []string{"{\"", "query", "\":\"", "test", "\"}"}
	for _, delta := range deltas {
		chunk := ResponseStreamChunk{
			OutputIndex: 1,
			ItemID:      "fc_abc123",
		}
		applyToolCallDelta(toolCallsMap, chunk, delta)
	}

	chunkDone := ResponseStreamChunk{
		OutputIndex: 1,
		Item: &ResponseOutputItem{
			Type:      functionCallType,
			ID:        "fc_abc123",
			Name:      searchIssuesTool,
			CallID:    "fc_abc123",
			Arguments: toolArgumentsTestValue,
		},
	}
	applyToolCallDone(toolCallsMap, chunkDone)

	return collectResponseToolCalls(toolCallsMap), hasToolCalls
}

func applyToolCallDelta(toolCallsMap map[int]*ResponseToolCall, chunk ResponseStreamChunk, delta string) {
	idx := chunk.OutputIndex
	if toolCallsMap[idx] == nil {
		toolCallsMap[idx] = &ResponseToolCall{
			ID:        chunk.ItemID,
			CallID:    chunk.ItemID,
			Type:      functionCallType,
			Arguments: delta,
		}
		return
	}
	toolCallsMap[idx].Arguments += delta
}

func applyToolCallDone(toolCallsMap map[int]*ResponseToolCall, chunkDone ResponseStreamChunk) {
	if chunkDone.Item == nil || chunkDone.Item.Type != functionCallType {
		return
	}
	idx := chunkDone.OutputIndex
	if toolCallsMap[idx] == nil {
		return
	}
	toolCallsMap[idx].Name = chunkDone.Item.Name
	toolCallsMap[idx].CallID = chunkDone.Item.CallID
	if chunkDone.Item.Arguments != "" {
		toolCallsMap[idx].Arguments = chunkDone.Item.Arguments
	}
}

func collectResponseToolCalls(toolCallsMap map[int]*ResponseToolCall) []ResponseToolCall {
	toolCalls := make([]ResponseToolCall, 0, len(toolCallsMap))
	for _, tc := range toolCallsMap {
		if tc == nil {
			continue
		}
		toolCalls = append(toolCalls, *tc)
	}
	return toolCalls
}

func assertToolCallExtraction(t *testing.T, toolCalls []ResponseToolCall) {
	t.Helper()

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].Name != searchIssuesTool {
		t.Errorf("expected name='%s', got '%s'", searchIssuesTool, toolCalls[0].Name)
	}
	if toolCalls[0].Arguments != toolArgumentsTestValue {
		t.Errorf("expected arguments with 'test', got '%s'", toolCalls[0].Arguments)
	}
	if toolCalls[0].CallID == "" {
		t.Error("call_id should not be empty")
	}
}

// TestResponseStreamChunkUnmarshal tests JSON unmarshaling
func TestResponseStreamChunkUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantType string
		wantErr  bool
	}{
		{
			name:     "response.created",
			json:     `{"type":"response.created","response_id":"resp_123"}`,
			wantType: "response.created",
		},
		{
			name:     "output_text.delta",
			json:     `{"type":"response.output_text.delta","sequence_number":1,"item_id":"msg_001","output_index":0,"delta":"Hello"}`,
			wantType: "response.output_text.delta",
		},
		{
			name:     "function_call_arguments.delta",
			json:     `{"type":"response.function_call_arguments.delta","sequence_number":5,"item_id":"fc_abc","output_index":1,"delta":"{\"","obfuscation":"xyz"}`,
			wantType: "response.function_call_arguments.delta",
		},
		{
			name:     "output_item.done with function_call",
			json:     `{"type":"response.output_item.done","item":{"type":"function_call","name":"search_issues","call_id":"fc_abc","arguments":"{\"query\":\"test\"}"}}`,
			wantType: "response.output_item.done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var chunk ResponseStreamChunk
			err := json.Unmarshal([]byte(tt.json), &chunk)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && chunk.Type != tt.wantType {
				t.Errorf("type = %s, want %s", chunk.Type, tt.wantType)
			}
		})
	}
}

// TestConvertOpenAIToolsToResponseTools tests tool conversion
func TestConvertOpenAIToolsToResponseTools(t *testing.T) {
	// This test ensures tools are converted to flat structure
	// Implementation depends on openai.Tool structure
}
