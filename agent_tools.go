package agentkit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/darkostanimirovic/agentkit/providers"
)

// executeToolCalls executes all tool calls and returns messages for the conversation history.
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []providers.ToolCall, events chan<- Event) []providers.Message {
	if len(toolCalls) == 0 {
		return nil
	}

	messages := make([]providers.Message, 0, len(toolCalls))
	
	if a.parallelConfig.Enabled {
		messages = a.executeToolCallsParallel(ctx, toolCalls, events)
	} else {
		messages = a.executeToolCallsSequential(ctx, toolCalls, events)
	}
	
	return messages
}

func (a *Agent) executeToolCallsSequential(ctx context.Context, toolCalls []providers.ToolCall, events chan<- Event) []providers.Message {
	messages := make([]providers.Message, 0, len(toolCalls))
	
	for _, call := range toolCalls {
		msg := a.executeToolCall(ctx, call, events)
		messages = append(messages, msg)
	}
	
	return messages
}

func (a *Agent) executeToolCallsParallel(ctx context.Context, toolCalls []providers.ToolCall, events chan<- Event) []providers.Message {
	type result struct {
		index int
		msg   providers.Message
	}
	
	resultChan := make(chan result, len(toolCalls))
	sem := make(chan struct{}, a.parallelConfig.MaxConcurrent)
	
	for i, call := range toolCalls {
		sem <- struct{}{}
		go func(idx int, tc providers.ToolCall) {
			defer func() { <-sem }()
			msg := a.executeToolCall(ctx, tc, events)
			resultChan <- result{index: idx, msg: msg}
		}(i, call)
	}
	
	// Collect results
	results := make([]result, 0, len(toolCalls))
	for i := 0; i < len(toolCalls); i++ {
		results = append(results, <-resultChan)
	}
	
	// Sort by original order
	messages := make([]providers.Message, len(toolCalls))
	for _, r := range results {
		messages[r.index] = r.msg
	}
	
	return messages
}

func (a *Agent) executeToolCall(ctx context.Context, toolCall providers.ToolCall, events chan<- Event) providers.Message {
	tool, exists := a.tools[toolCall.Name]
	
	// Check if tool exists
	if !exists {
		a.logger.Warn("tool not found", "tool", toolCall.Name)
		a.emit(ctx, events, ToolError(toolCall.Name, fmt.Errorf("tool not found")))
		return providers.Message{
			Role:       providers.RoleTool,
			Content:    fmt.Sprintf("Error: Tool '%s' not found", toolCall.Name),
			ToolCallID: toolCall.ID,
		}
	}

	// Check approval if required
	if a.approvalConfig.requiresApproval(toolCall.Name) {
		approved, rejectMsg := a.requestToolApproval(ctx, toolCall, tool, events)
		if !approved {
			return *rejectMsg
		}
	}

	// Start tool execution
	toolCtx := a.applyToolStart(ctx, toolCall.Name, toolCall.Arguments)
	toolCtx, cancel := a.withToolTimeout(toolCtx)
	if cancel != nil {
		defer cancel()
	}

	// Execute tool with retry
	var result any
	var err error
	
	// Marshal arguments to JSON string for tool.Execute
	argsJSON, err := json.Marshal(toolCall.Arguments)
	if err != nil {
		a.logger.Error("failed to marshal tool arguments", "tool", toolCall.Name, "error", err)
		a.emit(ctx, events, ToolError(toolCall.Name, err))
		return providers.Message{
			Role:       providers.RoleTool,
			Content:    fmt.Sprintf("Error marshaling arguments: %v", err),
			ToolCallID: toolCall.ID,
		}
	}
	
	result, err = WithRetry(toolCtx, a.retryConfig, func() (any, error) {
		return tool.Execute(toolCtx, string(argsJSON))
	})

	// Complete tool execution
	a.applyToolComplete(toolCtx, toolCall.Name, result, err)

	// Format result
	var content string
	if err != nil {
		content = fmt.Sprintf("Error executing tool: %v", err)
		a.logger.Error("tool execution failed", "tool", toolCall.Name, "error", err)
		a.emit(ctx, events, ToolError(toolCall.Name, err))
	} else {
		content = formatToolResult(result)
		a.logger.Info("tool executed successfully", "tool", toolCall.Name)
		a.emit(ctx, events, ToolResult(toolCall.Name, result))
	}

	return providers.Message{
		Role:       providers.RoleTool,
		Content:    content,
		ToolCallID: toolCall.ID,
		Name:       toolCall.Name,
	}
}

func (a *Agent) requestToolApproval(ctx context.Context, toolCall providers.ToolCall, tool Tool, events chan<- Event) (bool, *providers.Message) {
	approvalReq := ApprovalRequest{
		ToolName:    toolCall.Name,
		Arguments:   toolCall.Arguments,
		Description: tool.description,
		CallID:      toolCall.ID,
	}

	// Emit approval request
	a.emit(ctx, events, ApprovalNeeded(approvalReq))

	// Wait for approval
	approved, err := a.evaluateApproval(ctx, toolCall, approvalReq)
	if err != nil {
		msg := providers.Message{
			Role:       providers.RoleTool,
			Content:    fmt.Sprintf("Approval timeout or error: %v", err),
			ToolCallID: toolCall.ID,
		}
		return false, &msg
	}

	if !approved {
		msg := providers.Message{
			Role:       providers.RoleTool,
			Content:    "Tool execution rejected by user",
			ToolCallID: toolCall.ID,
		}
		a.emit(ctx, events, ApprovalRejected(approvalReq))
		return false, &msg
	}

	a.emit(ctx, events, ApprovalGranted(toolCall.Name, toolCall.ID))
	return true, nil
}

func (a *Agent) evaluateApproval(ctx context.Context, toolCall providers.ToolCall, req ApprovalRequest) (bool, error) {
	if a.approvalConfig.Handler != nil {
		return a.approvalConfig.Handler(ctx, req)
	}
	return false, fmt.Errorf("no approval handler configured")
}

func formatToolResult(result any) string {
	if result == nil {
		return "null"
	}
	
	switch v := result.(type) {
	case string:
		return v
	case error:
		return fmt.Sprintf("Error: %v", v)
	default:
		// Try JSON encoding
		if data, err := json.Marshal(result); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", result)
	}
}
