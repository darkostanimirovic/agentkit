package agentkit

import (
	"context"
)

// ApprovalHandler is called when a tool requires approval before execution
// Returns true to approve, false to deny
type ApprovalHandler func(ctx context.Context, request ApprovalRequest) (bool, error)

// ApprovalRequest contains information about a tool call that requires approval
type ApprovalRequest struct {
	ToolName       string         `json:"tool_name"`
	Arguments      map[string]any `json:"arguments"`
	Description    string         `json:"description"`     // Human-friendly description
	ConversationID string         `json:"conversation_id"` // If available
	CallID         string         `json:"call_id"`         // Unique call identifier
}

// ApprovalConfig configures which tools require approval
type ApprovalConfig struct {
	// Tools is a list of tool names that require approval
	// If empty, no tools require approval
	Tools []string

	// Handler is called for approval requests
	// If nil, all tools in Tools list will be automatically denied
	Handler ApprovalHandler

	// AllTools, if true, requires approval for ALL tool calls
	AllTools bool
}

// requiresApproval checks if a tool name requires approval
func (c ApprovalConfig) requiresApproval(toolName string) bool {
	if c.AllTools {
		return true
	}

	for _, t := range c.Tools {
		if t == toolName {
			return true
		}
	}

	return false
}
