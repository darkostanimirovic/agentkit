package agentkit

import (
	"context"
	"errors"
	"testing"
)

const assignTeamToolName = "assign_team"

func TestApprovalConfig_requiresApproval(t *testing.T) {
	tests := []struct {
		name       string
		config     ApprovalConfig
		toolName   string
		wantResult bool
	}{
		{
			name:       "no approval required - empty config",
			config:     ApprovalConfig{},
			toolName:   "search_issues",
			wantResult: false,
		},
		{
			name: "no approval required - tool not in list",
			config: ApprovalConfig{
				Tools: []string{assignTeamToolName, "update_status"},
			},
			toolName:   "search_issues",
			wantResult: false,
		},
		{
			name: "approval required - tool in list",
			config: ApprovalConfig{
				Tools: []string{assignTeamToolName, "update_status"},
			},
			toolName:   assignTeamToolName,
			wantResult: true,
		},
		{
			name: "approval required - all tools",
			config: ApprovalConfig{
				AllTools: true,
			},
			toolName:   "search_issues",
			wantResult: true,
		},
		{
			name: "approval required - all tools overrides list",
			config: ApprovalConfig{
				AllTools: true,
				Tools:    []string{assignTeamToolName},
			},
			toolName:   "search_issues",
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.requiresApproval(tt.toolName)
			if result != tt.wantResult {
				t.Errorf("requiresApproval(%s) = %v, want %v", tt.toolName, result, tt.wantResult)
			}
		})
	}
}

func TestApprovalEvents(t *testing.T) {
	t.Run("ApprovalRequired event", func(t *testing.T) {
		req := ApprovalRequest{
			ToolName: assignTeamToolName,
			Arguments: map[string]any{
				"team_slug": "backend",
			},
			Description:    "Assigning to backend team",
			ConversationID: "conv-123",
			CallID:         "call-1",
		}

		event := ApprovalRequired(req)

		if event.Type != EventTypeApprovalRequired {
			t.Errorf("expected type=%s, got %s", EventTypeApprovalRequired, event.Type)
		}
		if event.Data["tool_name"] != assignTeamToolName {
			t.Errorf("expected tool_name=%s, got %v", assignTeamToolName, event.Data["tool_name"])
		}
		if event.Data["call_id"] != "call-1" {
			t.Errorf("expected call_id=call-1, got %v", event.Data["call_id"])
		}
	})

	t.Run("ApprovalGranted event", func(t *testing.T) {
		event := ApprovalGranted(assignTeamToolName, "call-1")

		if event.Type != EventTypeApprovalGranted {
			t.Errorf("expected type=%s, got %s", EventTypeApprovalGranted, event.Type)
		}
		if event.Data["tool_name"] != assignTeamToolName {
			t.Errorf("expected tool_name=%s, got %v", assignTeamToolName, event.Data["tool_name"])
		}
	})

	t.Run("ApprovalDenied event", func(t *testing.T) {
		event := ApprovalDenied(assignTeamToolName, "call-1", "user denied")

		if event.Type != EventTypeApprovalDenied {
			t.Errorf("expected type=%s, got %s", EventTypeApprovalDenied, event.Type)
		}
		if event.Data["reason"] != "user denied" {
			t.Errorf("expected reason='user denied', got %v", event.Data["reason"])
		}
	})
}

func TestApprovalHandler_AutoDeny(t *testing.T) {
	// When no handler is configured, tools requiring approval should be denied
	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
		Approval: &ApprovalConfig{
			Tools: []string{"sensitive_operation"},
			// No Handler - should auto-deny
		},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add a test tool
	tool := NewTool("sensitive_operation").
		WithDescription("A sensitive operation").
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"executed": true}, nil
		}).
		Build()

	agent.AddTool(tool)

	// Verify the tool requires approval
	if !agent.approvalConfig.requiresApproval("sensitive_operation") {
		t.Error("expected sensitive_operation to require approval")
	}

	// Verify auto-deny behavior
	if agent.approvalConfig.Handler != nil {
		t.Error("expected no handler configured")
	}
}

func TestApprovalHandler_CustomHandler(t *testing.T) {
	approvalCalls := 0
	approvedTools := map[string]bool{
		"safe_tool":   true,
		"unsafe_tool": false,
	}

	handler := func(ctx context.Context, req ApprovalRequest) (bool, error) {
		approvalCalls++
		approved, exists := approvedTools[req.ToolName]
		if !exists {
			return false, errors.New("unknown tool")
		}
		return approved, nil
	}

	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
		Approval: &ApprovalConfig{
			AllTools: true,
			Handler:  handler,
		},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if !agent.approvalConfig.AllTools {
		t.Error("expected AllTools=true")
	}
	if agent.approvalConfig.Handler == nil {
		t.Error("expected handler to be configured")
	}
}

func TestApprovalRequest_Structure(t *testing.T) {
	req := ApprovalRequest{
		ToolName: assignTeamToolName,
		Arguments: map[string]any{
			"team_slug": "backend",
			"reasoning": "Best fit for this issue",
		},
		Description:    "Assigning to backend team",
		ConversationID: "conv-123",
		CallID:         "call-456",
	}

	if req.ToolName != assignTeamToolName {
		t.Errorf("expected ToolName=%s, got %s", assignTeamToolName, req.ToolName)
	}
	if req.ConversationID != "conv-123" {
		t.Errorf("expected ConversationID=conv-123, got %s", req.ConversationID)
	}
	if req.CallID != "call-456" {
		t.Errorf("expected CallID=call-456, got %s", req.CallID)
	}
	if len(req.Arguments) != 2 {
		t.Errorf("expected 2 arguments, got %d", len(req.Arguments))
	}
}

func TestApprovalHandler_ErrorHandling(t *testing.T) {
	handlerErr := errors.New("approval system unavailable")

	handler := func(ctx context.Context, req ApprovalRequest) (bool, error) {
		return false, handlerErr
	}

	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
		Approval: &ApprovalConfig{
			Tools:   []string{"test_tool"},
			Handler: handler,
		},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// The agent should handle handler errors gracefully (denying the tool)
	if agent.approvalConfig.Handler == nil {
		t.Error("expected handler to be configured")
	}

	ctx := context.Background()
	req := ApprovalRequest{
		ToolName:  "test_tool",
		Arguments: map[string]any{},
	}

	// Call handler directly to verify error handling
	approved, err := agent.approvalConfig.Handler(ctx, req)
	if err == nil {
		t.Error("expected error from handler")
	}
	if approved {
		t.Error("expected approval to be denied on error")
	}
}
