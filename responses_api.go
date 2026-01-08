// Package agentkit provides types and client for OpenAI's Responses API
package agentkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	responsesEndpoint = "https://api.openai.com/v1/responses"
)

// ResponsesClient wraps OpenAI's Responses API
type ResponsesClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewResponsesClient creates a new Responses API client
func NewResponsesClient(apiKey string, logger *slog.Logger) *ResponsesClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &ResponsesClient{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		logger:     logger,
	}
}

// ResponseInput represents input to the model
type ResponseInput struct {
	Role    string                `json:"role"`
	Content []ResponseContentItem `json:"content"`
}

// ResponseContentItem represents a content item in input/output
type ResponseContentItem struct {
	Type        string               `json:"type"`
	Text        string               `json:"text,omitempty"`
	ImageURL    *ResponseImageURL    `json:"image_url,omitempty"`
	Annotations []ResponseAnnotation `json:"annotations,omitempty"`
	ToolCallID  string               `json:"tool_call_id,omitempty"`
	CallID      string               `json:"call_id,omitempty"`
	Content     string               `json:"content,omitempty"`
	Output      string               `json:"output,omitempty"`
}

// ResponseImageURL represents an image URL in content
type ResponseImageURL struct {
	URL string `json:"url"`
}

// ResponseAnnotation represents an annotation in content
type ResponseAnnotation struct {
	Type string `json:"type"`
}

// ResponseTextFormat represents text format configuration
type ResponseTextFormat struct {
	Type       string      `json:"type"` // "text" or "json_schema"
	JSONSchema any `json:"json_schema,omitempty"`
}

// ResponseTextConfig represents text configuration
type ResponseTextConfig struct {
	Format ResponseTextFormat `json:"format"`
}

// ResponseToolChoice represents tool choice configuration
type ResponseToolChoice struct {
	Type     string                `json:"type"` // "auto", "required", "none", or "function"
	Function *ResponseToolFunction `json:"function,omitempty"`
}

// ResponseToolFunction represents a specific function choice
type ResponseToolFunction struct {
	Name string `json:"name"`
}

// ResponseTool represents a tool definition for Responses API
// Note: In Responses API, name/description/parameters are at top level, not nested
type ResponseTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      bool                   `json:"strict,omitempty"`
}

// ResponseRequest represents a request to create a response
type ResponseRequest struct {
	Model              string              `json:"model"`
	Input              any         `json:"input,omitempty"` // string or []ResponseInput
	Instructions       string              `json:"instructions,omitempty"`
	Temperature        float32             `json:"temperature,omitempty"`
	MaxOutputTokens    int                 `json:"max_output_tokens,omitempty"`
	Tools              []ResponseTool      `json:"tools,omitempty"`
	ToolChoice         any         `json:"tool_choice,omitempty"` // string or ResponseToolChoice
	Stream             bool                `json:"stream,omitempty"`
	Store              bool                `json:"store,omitempty"`
	PreviousResponseID string              `json:"previous_response_id,omitempty"`
	ParallelToolCalls  bool                `json:"parallel_tool_calls,omitempty"`
	TopP               float32             `json:"top_p,omitempty"`
	Text               *ResponseTextConfig `json:"text,omitempty"`
	Metadata           map[string]string   `json:"metadata,omitempty"`
}

// ResponseObject represents a response from the API
type ResponseObject struct {
	ID                 string               `json:"id"`
	Object             string               `json:"object"`
	CreatedAt          int64                `json:"created_at"`
	Status             string               `json:"status"` // "completed", "failed", "in_progress", "cancelled", "queued", "incomplete"
	Model              string               `json:"model"`
	Output             []ResponseOutputItem `json:"output"`
	Usage              ResponseUsage        `json:"usage"`
	Error              *ResponseError       `json:"error"`
	PreviousResponseID string               `json:"previous_response_id"`
	Temperature        float32              `json:"temperature"`
	ParallelToolCalls  bool                 `json:"parallel_tool_calls"`
	ToolChoice         any          `json:"tool_choice"`
	Tools              []ResponseTool       `json:"tools"`
}

// ResponseOutputItem represents an item in the output array
type ResponseOutputItem struct {
	Type      string                `json:"type"` // "message", "reasoning", "function_call", etc.
	ID        string                `json:"id"`
	Status    string                `json:"status"`
	Role      string                `json:"role,omitempty"`
	Name      string                `json:"name,omitempty"`      // For function_call type
	CallID    string                `json:"call_id,omitempty"`   // For function_call type
	Arguments string                `json:"arguments,omitempty"` // For function_call type
	Content   []ResponseContentItem `json:"content,omitempty"`
	ToolCalls []ResponseToolCall    `json:"tool_calls,omitempty"` // Deprecated - function_call items are separate
}

// ResponseToolCall represents a tool call in output
// For Responses API, tool calls are of type "function_call" not "tool_call"
type ResponseToolCall struct {
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Type      string `json:"type"` // "function_call"
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseUsage represents token usage
type ResponseUsage struct {
	InputTokens         int                   `json:"input_tokens"`
	OutputTokens        int                   `json:"output_tokens"`
	TotalTokens         int                   `json:"total_tokens"`
	InputTokensDetails  ResponseTokensDetails `json:"input_tokens_details"`
	OutputTokensDetails ResponseTokensDetails `json:"output_tokens_details"`
}

// ResponseTokensDetails represents detailed token information
type ResponseTokensDetails struct {
	CachedTokens    int `json:"cached_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// ResponseError represents an error in the response
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponseStreamChunk represents a streaming response chunk
// Responses API uses event-based streaming with specific event types:
// - response.output_item.added: New output item started
// - response.output_text.delta: Text content delta
// - response.function_call_arguments.delta: Function arguments delta
// - response.function_call_arguments.done: Function call complete (includes name and arguments)
// - response.done: Response complete
type ResponseStreamChunk struct {
	Type           string               `json:"type"` // Event type
	SequenceNumber int                  `json:"sequence_number,omitempty"`
	ResponseID     string               `json:"response_id,omitempty"`
	Response       *ResponseObject      `json:"response,omitempty"`
	ItemID         string               `json:"item_id,omitempty"`
	OutputIndex    int                  `json:"output_index,omitempty"`
	Delta          string               `json:"delta,omitempty"`     // For delta events
	Item           *ResponseOutputItem  `json:"item,omitempty"`      // For added/done events
	Name           string               `json:"name,omitempty"`      // For function_call_arguments.done
	Arguments      string               `json:"arguments,omitempty"` // For function_call_arguments.done
	Status         string               `json:"status,omitempty"`
	Output         []ResponseOutputItem `json:"output,omitempty"` // For response.done
	Usage          *ResponseUsage       `json:"usage,omitempty"`
	Obfuscation    string               `json:"obfuscation,omitempty"` // Sent by API, purpose unclear
}

// ResponseDelta is deprecated - using event-based streaming instead
type ResponseDelta struct {
	Type      string                `json:"type"`
	Index     int                   `json:"index"`
	Content   []ResponseContentItem `json:"content,omitempty"`
	ToolCalls []ResponseToolCall    `json:"tool_calls,omitempty"`
}

// CreateResponse creates a non-streaming response
func (c *ResponsesClient) CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", responsesEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Explicitly ignore error
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ResponseObject
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ResponseStream wraps a streaming response
type ResponseStream struct {
	reader *io.ReadCloser
	buffer string
	logger *slog.Logger
}

// CreateResponseStream creates a streaming response
func (c *ResponsesClient) CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error) {
	req.Stream = true

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", responsesEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return &ResponseStream{
		reader: &resp.Body,
		logger: c.logger,
	}, nil
}

// Recv receives the next chunk from the stream
func (s *ResponseStream) Recv() (*ResponseStreamChunk, error) {
	if s.reader == nil {
		return nil, io.EOF
	}
	s.ensureLogger()

	// Read from the SSE stream
	reader := *s.reader
	buf := make([]byte, 8192)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			s.appendToBuffer(buf[:n])
		}

		if err != nil && err != io.EOF {
			return nil, err
		}

		if chunk := s.nextChunk(); chunk != nil {
			return chunk, nil
		}

		if err == io.EOF {
			s.warnIncompleteBuffer()
			return nil, io.EOF
		}
	}
}

func (s *ResponseStream) ensureLogger() {
	if s.logger == nil {
		s.logger = slog.Default()
	}
}

func (s *ResponseStream) appendToBuffer(data []byte) {
	s.buffer += string(data)
	s.logger.Info("read bytes from stream", "n", len(data), "buffer_len", len(s.buffer))
}

func (s *ResponseStream) nextChunk() *ResponseStreamChunk {
	for {
		event, ok := s.popNextEvent()
		if !ok {
			return nil
		}

		data := extractSSEData(event)
		if data == "" || data == "[DONE]" {
			continue
		}

		chunk := s.parseChunk(data)
		if chunk == nil {
			continue
		}

		return chunk
	}
}

func (s *ResponseStream) popNextEvent() (string, bool) {
	idx := strings.Index(s.buffer, "\n\n")
	if idx == -1 {
		return "", false
	}

	event := s.buffer[:idx]
	s.buffer = s.buffer[idx+2:]
	return event, true
}

func extractSSEData(event string) string {
	lines := strings.Split(event, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: ")
		}
	}
	return ""
}

func (s *ResponseStream) parseChunk(data string) *ResponseStreamChunk {
	var chunk ResponseStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		s.logger.Error("failed to unmarshal chunk", "error", err, "data", data)
		return nil
	}
	return &chunk
}

func (s *ResponseStream) warnIncompleteBuffer() {
	if s.buffer != "" {
		s.logger.Warn("stream ended with incomplete event in buffer", "remaining", s.buffer)
	}
}

// Close closes the stream
func (s *ResponseStream) Close() error {
	if s.reader != nil {
		return (*s.reader).Close()
	}
	return nil
}

// ConvertOpenAIToolsToResponseTools converts OpenAI tools to Response API tools
func ConvertOpenAIToolsToResponseTools(tools []openai.Tool) []ResponseTool {
	result := make([]ResponseTool, len(tools))
	for i, tool := range tools {
		// Convert parameters with type assertion
		var params map[string]any
		if tool.Function.Parameters != nil {
			if p, ok := tool.Function.Parameters.(map[string]any); ok {
				params = p
			} else {
				// Try to marshal and unmarshal to convert
				if data, err := json.Marshal(tool.Function.Parameters); err == nil {
					_ = json.Unmarshal(data, &params)
				}
			}
		}

		// Responses API format: name/description/parameters at top level
		result[i] = ResponseTool{
			Type:        string(tool.Type),
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  params,
		}
	}
	return result
}
