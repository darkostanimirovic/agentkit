// Package providers defines provider-agnostic interfaces and domain models for LLM interactions.
package providers

import (
	"context"
	"time"
)

// Provider defines the interface for any LLM provider.
// Implementations: OpenAI, Anthropic, local models, mocks, etc.
type Provider interface {
	// Complete generates a non-streaming completion.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	
	// Stream generates a streaming completion.
	Stream(ctx context.Context, req CompletionRequest) (StreamReader, error)
	
	// Name returns the provider name (e.g., "openai", "anthropic").
	Name() string
}

// StreamReader provides access to streaming chunks.
type StreamReader interface {
	// Next returns the next chunk or io.EOF when complete.
	Next() (*StreamChunk, error)
	
	// Close closes the stream.
	Close() error
}

// CompletionRequest represents a provider-agnostic request for completion.
type CompletionRequest struct {
	Model             string
	Messages          []Message
	Tools             []ToolDefinition
	Temperature       float32
	MaxTokens         int
	SystemPrompt      string
	TopP              float32
	Stream            bool
	ToolChoice        string
	ParallelToolCalls bool
	ReasoningEffort   ReasoningEffort
	ReasoningSummary  string
	TextVerbosity     string
	TextFormat        string
	Store             bool
	Metadata          map[string]string
}

// CompletionResponse represents a provider-agnostic completion response.
type CompletionResponse struct {
	ID           string
	Content      string
	ToolCalls    []ToolCall
	ReasoningSummary string
	FinishReason FinishReason
	Usage        TokenUsage
	Model        string
	Created      time.Time
	Metadata     map[string]string
}

// Message represents a single message in a conversation.
type Message struct {
	Role       MessageRole
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string // For tool result messages
	Name       string // Optional name
}

// MessageRole defines the role of a message sender.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// ToolDefinition defines a tool that can be called by the agent.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// FinishReason indicates why the model stopped generating.
type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"
	FinishReasonToolCalls FinishReason = "tool_calls"
	FinishReasonLength    FinishReason = "length"
	FinishReasonError     FinishReason = "error"
)

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	ReasoningTokens  int // For reasoning models
	TotalTokens      int
}

// StreamChunk represents a chunk of streaming response.
type StreamChunk struct {
	Content      string
	ReasoningSummary string
	ToolCallID   string
	ToolName     string
	ToolArgs     string
	IsComplete   bool
	FinishReason FinishReason
	Usage        *TokenUsage
}

// ReasoningEffort controls compute for reasoning models.
type ReasoningEffort string

const (
	ReasoningEffortNone    ReasoningEffort = ""
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)
