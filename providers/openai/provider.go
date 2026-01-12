// Package openai implements the Provider interface for OpenAI's Responses API.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/darkostanimirovic/agentkit/providers"
)

const responsesEndpoint = "https://api.openai.com/v1/responses"

// Provider implements providers.Provider for OpenAI.
type Provider struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a new OpenAI provider.
func New(apiKey string, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		logger:     logger,
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "openai"
}

// Complete generates a non-streaming completion.
func (p *Provider) Complete(ctx context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
	apiReq := p.toAPIRequest(req)

	jsonData, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", responsesEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var apiResp responseObject
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return p.fromAPIResponse(&apiResp), nil
}

// Stream generates a streaming completion.
func (p *Provider) Stream(ctx context.Context, req providers.CompletionRequest) (providers.StreamReader, error) {
	apiReq := p.toAPIRequest(req)
	apiReq.Stream = true

	jsonData, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", responsesEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, parseAPIError(resp.StatusCode, body)
	}

	return newStreamReader(resp.Body, p.logger), nil
}

// toAPIRequest converts provider-agnostic request to OpenAI API format.
func (p *Provider) toAPIRequest(req providers.CompletionRequest) apiRequest {
	apiReq := apiRequest{
		Model:             req.Model,
		Instructions:      req.SystemPrompt,
		Temperature:       req.Temperature,
		MaxOutputTokens:   req.MaxTokens,
		TopP:              req.TopP,
		Stream:            req.Stream,
		ParallelToolCalls: req.ParallelToolCalls,
		Metadata:          req.Metadata,
		ToolChoice:        req.ToolChoice,
	}

	// Convert messages to input
	if len(req.Messages) > 0 {
		apiReq.Input = p.toAPIInput(req.Messages)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		apiReq.Tools = p.toAPITools(req.Tools)
	}

	// Convert reasoning effort
	if req.ReasoningEffort != "" {
		apiReq.Reasoning = &reasoning{
			Effort: string(req.ReasoningEffort),
		}
	}

	return apiReq
}

// toAPIInput converts messages to OpenAI input format.
func (p *Provider) toAPIInput(messages []providers.Message) []any {
	inputs := make([]any, 0, len(messages))

	for _, msg := range messages {
		if msg.ToolCallID != "" {
			inputs = append(inputs, functionCallOutput{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: msg.Content,
			})
			continue
		}

		role := string(msg.Role)
		in := input{
			Role: role,
		}

		// Build content items
		contentItems := []contentItem{}

		if msg.Content != "" {
			contentType := "input_text"
			if msg.Role == providers.RoleAssistant {
				contentType = "output_text"
			}
			contentItems = append(contentItems, contentItem{
				Type: contentType,
				Text: msg.Content,
			})
		}

		in.Content = contentItems
		inputs = append(inputs, in)
	}

	return inputs
}

// toAPITools converts tools to OpenAI tool format.
func (p *Provider) toAPITools(tools []providers.ToolDefinition) []tool {
	apiTools := make([]tool, len(tools))
	for i, t := range tools {
		apiTools[i] = tool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
			Strict:      true, // Enable structured outputs
		}
	}
	return apiTools
}

// fromAPIResponse converts OpenAI API response to provider-agnostic response.
func (p *Provider) fromAPIResponse(resp *responseObject) *providers.CompletionResponse {
	domainResp := &providers.CompletionResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Created: time.Unix(resp.CreatedAt, 0),
		Usage: providers.TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			ReasoningTokens:  resp.Usage.ReasoningTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Extract content and tool calls from output
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			// Extract text content
			for _, content := range item.Content {
				if content.Type == "output_text" && content.Text != "" {
					domainResp.Content += content.Text
				}
			}
		case "function_call":
			// Parse tool call
			var args map[string]any
			if item.Arguments != "" {
				json.Unmarshal([]byte(item.Arguments), &args)
			}
			if item.CallID != "" {
				domainResp.ToolCalls = append(domainResp.ToolCalls, providers.ToolCall{
					ID:        item.CallID,
					Name:      item.Name,
					Arguments: args,
				})
			}
		}
	}

	// Determine finish reason
	if resp.Status == "completed" {
		if len(domainResp.ToolCalls) > 0 {
			domainResp.FinishReason = providers.FinishReasonToolCalls
		} else {
			domainResp.FinishReason = providers.FinishReasonStop
		}
	} else if resp.Error != nil {
		domainResp.FinishReason = providers.FinishReasonError
	}

	return domainResp
}

// Stream reader implementation

type streamReader struct {
	reader     io.ReadCloser
	buffer     string
	logger     *slog.Logger
	toolCalls  map[string]*toolCall
	toolByItem map[string]*toolCall
	textBuffer string
	responseID string
	pending    []*providers.StreamChunk
}

type toolCall struct {
	ID        string
	CallID    string
	Name      string
	Arguments string
}

func newStreamReader(reader io.ReadCloser, logger *slog.Logger) *streamReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &streamReader{
		reader:     reader,
		logger:     logger,
		toolCalls:  make(map[string]*toolCall),
		toolByItem: make(map[string]*toolCall),
	}
}

func (s *streamReader) Next() (*providers.StreamChunk, error) {
	for {
		// Try to parse from buffer first
		if chunk := s.parseNextChunk(); chunk != nil {
			return chunk, nil
		}

		// Read more data
		buf := make([]byte, 4096)
		n, err := s.reader.Read(buf)
		if n > 0 {
			s.buffer += string(buf[:n])
		}
		if err != nil {
			if err == io.EOF && s.buffer == "" {
				return nil, io.EOF
			}
			if err != io.EOF {
				return nil, err
			}
		}
	}
}

func (s *streamReader) Close() error {
	return s.reader.Close()
}

func (s *streamReader) parseNextChunk() *providers.StreamChunk {
	if len(s.pending) > 0 {
		chunk := s.pending[0]
		s.pending = s.pending[1:]
		return chunk
	}

	// Find next SSE event
	idx := strings.Index(s.buffer, "\n\n")
	if idx == -1 {
		return nil
	}

	event := s.buffer[:idx]
	s.buffer = s.buffer[idx+2:]

	// Extract data from SSE event
	data := extractSSEData(event)
	if data == "" || data == "[DONE]" {
		return nil
	}

	// Parse JSON chunk
	var apiChunk streamChunk
	if err := json.Unmarshal([]byte(data), &apiChunk); err != nil {
		s.logger.Error("failed to parse stream chunk", "error", err)
		return nil
	}

	if apiChunk.ResponseID != "" {
		s.responseID = apiChunk.ResponseID
	}

	// Handle different event types
	switch apiChunk.Type {
	case "response.output_text.delta":
		return s.emitTextDelta(apiChunk.Delta)
	case "response.output_text.done":
		return s.emitTextFinal(apiChunk.Text)
	case "response.output_item.done":
		if apiChunk.Item != nil {
			if apiChunk.Item.Type == "function_call" {
				if chunk := s.storeToolCallFromItem(*apiChunk.Item); chunk != nil {
					return chunk
				}
				return nil
			}
			return s.emitTextFinal(extractOutputTextFromItem(*apiChunk.Item))
		}
	case "response.output_item.added":
		if apiChunk.Item != nil && apiChunk.Item.Type == "function_call" {
			if chunk := s.storeToolCallFromItem(*apiChunk.Item); chunk != nil {
				return chunk
			}
			return nil
		}
	case "response.content_part.done":
		if apiChunk.Part != nil && apiChunk.Part.Type == "output_text" {
			return s.emitTextFinal(apiChunk.Part.Text)
		}

	case "response.function_call_arguments.delta":
		tc := s.ensureToolCall(apiChunk.CallID, apiChunk.ItemID, apiChunk.OutputIndex)
		tc.Arguments += apiChunk.Delta
		if apiChunk.CallID == "" {
			return nil
		}
		return &providers.StreamChunk{
			ToolCallID: apiChunk.CallID,
		}

	case "response.function_call_arguments.done":
		tc := s.ensureToolCall(apiChunk.CallID, apiChunk.ItemID, apiChunk.OutputIndex)
		if apiChunk.Name != "" {
			tc.Name = apiChunk.Name
		}
		if apiChunk.Arguments != "" {
			tc.Arguments = apiChunk.Arguments
		}
		if tc != nil && tc.CallID != "" {
			if chunk := buildToolChunkIfReady(tc); chunk != nil {
				return chunk
			}
		}

	case "response.done", "response.completed":
		chunk := &providers.StreamChunk{
			IsComplete:   true,
			FinishReason: providers.FinishReasonStop,
		}
		if apiChunk.Usage != nil {
			chunk.Usage = &providers.TokenUsage{
				PromptTokens:     apiChunk.Usage.InputTokens,
				CompletionTokens: apiChunk.Usage.OutputTokens,
				ReasoningTokens:  apiChunk.Usage.ReasoningTokens,
				TotalTokens:      apiChunk.Usage.TotalTokens,
			}
		} else if apiChunk.Response != nil {
			chunk.Usage = &providers.TokenUsage{
				PromptTokens:     apiChunk.Response.Usage.InputTokens,
				CompletionTokens: apiChunk.Response.Usage.OutputTokens,
				ReasoningTokens:  apiChunk.Response.Usage.ReasoningTokens,
				TotalTokens:      apiChunk.Response.Usage.TotalTokens,
			}
		}
		if len(s.toolCalls) > 0 {
			chunk.FinishReason = providers.FinishReasonToolCalls
		}
		if apiChunk.Response != nil {
			if toolChunks := streamChunksFromResponseTools(apiChunk.Response); len(toolChunks) > 0 {
				s.pending = append(s.pending, toolChunks...)
				chunk.FinishReason = providers.FinishReasonToolCalls
			}
			if delta := s.emitTextFinal(extractOutputTextFromResponse(apiChunk.Response)); delta != nil {
				chunk.Content = delta.Content
			}
		}
		s.pending = append(s.pending, chunk)
		next := s.pending[0]
		s.pending = s.pending[1:]
		return next
	}

	return nil
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

func toolCallID(callID, itemID string) string {
	if callID != "" {
		return callID
	}
	return itemID
}

func toolCallKey(callID, itemID string, outputIndex int) string {
	if callID != "" {
		return callID
	}
	if itemID != "" {
		return itemID
	}
	return fmt.Sprintf("index-%d", outputIndex)
}

func (s *streamReader) ensureToolCall(callID, itemID string, outputIndex int) *toolCall {
	if itemID != "" {
		if tc := s.toolByItem[itemID]; tc != nil {
			if callID != "" {
				tc.CallID = callID
				s.toolCalls[callID] = tc
			}
			return tc
		}
	}
	if callID != "" {
		if tc := s.toolCalls[callID]; tc != nil {
			if itemID != "" && tc.ID == "" {
				tc.ID = itemID
				s.toolByItem[itemID] = tc
			}
			return tc
		}
	}
	key := toolCallKey(callID, itemID, outputIndex)
	if tc := s.toolCalls[key]; tc != nil {
		return tc
	}
	tc := &toolCall{
		ID:     itemID,
		CallID: callID,
	}
	s.toolCalls[key] = tc
	if itemID != "" {
		s.toolByItem[itemID] = tc
	}
	if callID != "" {
		s.toolCalls[callID] = tc
	}
	return tc
}

func buildToolChunkIfReady(tc *toolCall) *providers.StreamChunk {
	if tc == nil || tc.CallID == "" || tc.Name == "" || tc.Arguments == "" {
		return nil
	}
	return &providers.StreamChunk{
		ToolCallID: tc.CallID,
		ToolName:   tc.Name,
		ToolArgs:   tc.Arguments,
	}
}

func (s *streamReader) storeToolCallFromItem(item outputItem) *providers.StreamChunk {
	if item.Type != "function_call" {
		return nil
	}
	itemID := item.ID
	if itemID == "" {
		itemID = item.CallID
	}
	if itemID == "" {
		return nil
	}
	tc := s.toolByItem[itemID]
	if tc == nil {
		tc = &toolCall{ID: item.ID, CallID: item.CallID}
		s.toolByItem[itemID] = tc
		if item.CallID != "" {
			s.toolCalls[item.CallID] = tc
		}
		if item.ID != "" {
			s.toolCalls[item.ID] = tc
		}
	}
	if item.Name != "" {
		tc.Name = item.Name
	}
	if item.Arguments != "" {
		tc.Arguments = item.Arguments
	}
	if tc.CallID == "" && item.CallID != "" {
		tc.CallID = item.CallID
		s.toolCalls[item.CallID] = tc
	}
	return buildToolChunkIfReady(tc)
}

func (s *streamReader) emitTextDelta(delta string) *providers.StreamChunk {
	if delta == "" {
		return nil
	}
	s.textBuffer += delta
	return &providers.StreamChunk{
		Content: delta,
	}
}

func (s *streamReader) emitTextFinal(text string) *providers.StreamChunk {
	if text == "" {
		return nil
	}
	delta := text
	if s.textBuffer != "" && strings.HasPrefix(text, s.textBuffer) {
		delta = text[len(s.textBuffer):]
	}
	if delta == "" {
		s.textBuffer = text
		return nil
	}
	s.textBuffer = text
	return &providers.StreamChunk{
		Content: delta,
	}
}

func extractOutputText(items []contentItem) string {
	var builder strings.Builder
	for _, item := range items {
		if item.Type == "output_text" && item.Text != "" {
			builder.WriteString(item.Text)
		}
	}
	return builder.String()
}

func extractOutputTextFromItem(item outputItem) string {
	if item.Type != "message" {
		return ""
	}
	return extractOutputText(item.Content)
}

func extractOutputTextFromResponse(resp *responseObject) string {
	if resp == nil {
		return ""
	}
	var builder strings.Builder
	for _, item := range resp.Output {
		builder.WriteString(extractOutputTextFromItem(item))
	}
	return builder.String()
}

func streamChunkFromFunctionCall(item outputItem) *providers.StreamChunk {
	if item.CallID == "" {
		return nil
	}
	return &providers.StreamChunk{
		ToolCallID: item.CallID,
		ToolName:   item.Name,
		ToolArgs:   item.Arguments,
	}
}

func streamChunksFromResponseTools(resp *responseObject) []*providers.StreamChunk {
	if resp == nil {
		return nil
	}
	var chunks []*providers.StreamChunk
	for _, item := range resp.Output {
		if item.Type == "function_call" {
			if chunk := streamChunkFromFunctionCall(item); chunk != nil {
				chunks = append(chunks, chunk)
			}
		}
	}
	return chunks
}

// OpenAI API types (internal to this package)

type apiRequest struct {
	Model             string            `json:"model"`
	Instructions      string            `json:"instructions,omitempty"`
	Input             any               `json:"input,omitempty"`
	Tools             []tool            `json:"tools,omitempty"`
	ToolChoice        string            `json:"tool_choice,omitempty"`
	Temperature       float32           `json:"temperature,omitempty"`
	MaxOutputTokens   int               `json:"max_output_tokens,omitempty"`
	TopP              float32           `json:"top_p,omitempty"`
	Stream            bool              `json:"stream,omitempty"`
	ParallelToolCalls bool              `json:"parallel_tool_calls,omitempty"`
	Reasoning         *reasoning        `json:"reasoning,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type input struct {
	Role    string        `json:"role"`
	Content []contentItem `json:"content"`
}

type functionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type contentItem struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	CallID  string `json:"call_id,omitempty"`
	Content string `json:"content,omitempty"`
}

type tool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
}

type reasoning struct {
	Effort string `json:"effort"`
}

type responseObject struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"`
	CreatedAt int64        `json:"created_at"`
	Status    string       `json:"status"`
	Model     string       `json:"model"`
	Output    []outputItem `json:"output"`
	Usage     usage        `json:"usage"`
	Error     *apiError    `json:"error,omitempty"`
}

type outputItem struct {
	Type      string        `json:"type"`
	ID        string        `json:"id,omitempty"`
	Name      string        `json:"name,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Status    string        `json:"status,omitempty"`
	Role      string        `json:"role,omitempty"`
	Content   []contentItem `json:"content,omitempty"`
}

type usage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	TotalTokens     int `json:"total_tokens"`
}

type streamChunk struct {
	Type        string          `json:"type"`
	ResponseID  string          `json:"response_id,omitempty"`
	ItemID      string          `json:"item_id,omitempty"`
	CallID      string          `json:"call_id,omitempty"`
	OutputIndex int             `json:"output_index,omitempty"`
	Delta       string          `json:"delta,omitempty"`
	Text        string          `json:"text,omitempty"`
	Name        string          `json:"name,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
	Usage       *usage          `json:"usage,omitempty"`
	Response    *responseObject `json:"response,omitempty"`
	Item        *outputItem     `json:"item,omitempty"`
	Part        *contentItem    `json:"part,omitempty"`
}

type apiError struct {
	Code    interface{} `json:"code"`
	Message string      `json:"message"`
	Type    string      `json:"type"`
}

func parseAPIError(statusCode int, body []byte) error {
	var errResp struct {
		Error apiError `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error (status %d): %s", statusCode, string(body))
	}

	msg := fmt.Sprintf("API error (status %d): %s", statusCode, errResp.Error.Message)
	if errResp.Error.Code != nil {
		msg += fmt.Sprintf(" (code: %v)", errResp.Error.Code)
	}
	return fmt.Errorf("%s", msg)
}
