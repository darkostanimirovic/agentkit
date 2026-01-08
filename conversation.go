package agentkit

import (
	"context"
	"time"
)

// ConversationStore defines the interface for persisting conversations
type ConversationStore interface {
	// Save persists a complete conversation
	Save(ctx context.Context, conv Conversation) error

	// Load retrieves a conversation by ID
	Load(ctx context.Context, id string) (Conversation, error)

	// Append adds a turn to an existing conversation
	Append(ctx context.Context, id string, turn ConversationTurn) error

	// Delete removes a conversation
	Delete(ctx context.Context, id string) error
}

// Conversation represents a multi-turn conversation with an agent
type Conversation struct {
	ID        string             `json:"id"`
	AgentID   string             `json:"agent_id,omitempty"`
	Turns     []ConversationTurn `json:"turns"`
	Metadata  map[string]any     `json:"metadata,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// ConversationTurn represents a single interaction in a conversation
type ConversationTurn struct {
	Role        string                   `json:"role"` // "user", "assistant", "tool"
	Content     string                   `json:"content"`
	ToolCalls   []ConversationToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ConversationToolResult `json:"tool_results,omitempty"`
	ResponseID  string                   `json:"response_id,omitempty"` // OpenAI Response ID
	Timestamp   time.Time                `json:"timestamp"`
}

// ConversationToolCall represents a tool invocation
type ConversationToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ConversationToolResult represents the result of a tool execution
type ConversationToolResult struct {
	CallID string `json:"call_id"`
	Result any    `json:"result"`
	Error  string `json:"error,omitempty"`
}
