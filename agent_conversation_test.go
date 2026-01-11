package agentkit


import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	errConversationStoreNotConfigured = "agentkit: conversation store not configured"
	convID                            = "conv-123"
)

func assertConversationStoreError(t *testing.T, err error) {
	t.Helper()
	if err == nil || err.Error() != errConversationStoreNotConfigured {
		t.Errorf("expected '%s' error, got %v", errConversationStoreNotConfigured, err)
	}
}

func TestAgent_ConversationMethods_NotConfigured(t *testing.T) {
	// Agent without conversation store
	agent, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "GetConversation",
			fn: func() error {
				_, err := agent.GetConversation(ctx, convID)
				return err
			},
		},
		{
			name: "SaveConversation",
			fn: func() error {
				conv := Conversation{ID: convID}
				return agent.SaveConversation(ctx, conv)
			},
		},
		{
			name: "AppendToConversation",
			fn: func() error {
				turn := ConversationTurn{Role: "user", Content: "Hello", Timestamp: time.Now()}
				return agent.AppendToConversation(ctx, convID, turn)
			},
		},
		{
			name: "DeleteConversation",
			fn: func() error {
				return agent.DeleteConversation(ctx, convID)
			},
		},
		{
			name: "AddContext",
			fn: func() error {
				return agent.AddContext(ctx, convID, "context")
			},
		},
		{
			name: "ClearConversation",
			fn: func() error {
				return agent.ClearConversation(ctx, convID)
			},
		},
		{
			name: "ForkConversation",
			fn: func() error {
				return agent.ForkConversation(ctx, convID, "conv-456", "new message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			assertConversationStoreError(t, err)
		})
	}
}

func TestAgent_GetConversation(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	// Create conversation
	conv := Conversation{
		ID: convID,
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}
	err = store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Get conversation
	loaded, err := agent.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if loaded.ID != convID {
		t.Errorf("expected ID=%s, got %s", convID, loaded.ID)
	}
	if len(loaded.Turns) != 1 {
		t.Errorf("expected 1 turn, got %d", len(loaded.Turns))
	}
}

func TestAgent_SaveConversation(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	conv := Conversation{
		ID: convID,
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}

	// Save conversation
	err = agent.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Verify it was saved
	loaded, err := store.Load(ctx, convID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != convID {
		t.Errorf("expected ID=%s, got %s", convID, loaded.ID)
	}
}

func TestAgent_AppendToConversation(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	// Create initial conversation
	conv := Conversation{
		ID: convID,
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}
	err = store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Append turn
	turn := ConversationTurn{
		Role:      "assistant",
		Content:   "Hi there!",
		Timestamp: time.Now(),
	}
	err = agent.AppendToConversation(ctx, convID, turn)
	if err != nil {
		t.Fatalf("AppendToConversation failed: %v", err)
	}

	// Verify it was appended
	loaded, err := store.Load(ctx, convID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(loaded.Turns))
	}
	if loaded.Turns[1].Content != "Hi there!" {
		t.Errorf("expected content='Hi there!', got %s", loaded.Turns[1].Content)
	}
}

func TestAgent_DeleteConversation(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	// Create conversation
	conv := Conversation{
		ID: convID,
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}
	err = store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Delete conversation
	err = agent.DeleteConversation(ctx, convID)
	if err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}

	// Verify deletion
	_, err = store.Load(ctx, convID)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestAgent_AddContext(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	// Create initial conversation
	conv := Conversation{
		ID: convID,
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}
	err = store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Add context
	err = agent.AddContext(ctx, convID, "Additional context: Project deadline is Friday")
	if err != nil {
		t.Fatalf("AddContext failed: %v", err)
	}

	// Verify context was added
	loaded, err := store.Load(ctx, convID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(loaded.Turns))
	}
	if loaded.Turns[1].Role != "user" {
		t.Errorf("expected role=user, got %s", loaded.Turns[1].Role)
	}
	if loaded.Turns[1].Content != "Additional context: Project deadline is Friday" {
		t.Errorf("unexpected content: %s", loaded.Turns[1].Content)
	}
}

func TestAgent_ClearConversation(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	// Create conversation with metadata
	conv := Conversation{
		ID:      convID,
		AgentID: "agent-1",
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "Hi!", Timestamp: time.Now()},
		},
		Metadata: map[string]any{
			"user_id": "user-456",
		},
	}
	err = store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Clear conversation
	err = agent.ClearConversation(ctx, convID)
	if err != nil {
		t.Fatalf("ClearConversation failed: %v", err)
	}

	// Verify it was cleared
	loaded, err := store.Load(ctx, convID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Turns) != 0 {
		t.Errorf("expected 0 turns, got %d", len(loaded.Turns))
	}
	// Metadata should be preserved
	if loaded.Metadata["user_id"] != "user-456" {
		t.Errorf("metadata not preserved")
	}
}

func TestAgent_ForkConversation(t *testing.T) {
	store := NewMemoryConversationStore()
	agent, err := New(Config{
		APIKey:            "test-key",
		Model:             "gpt-4o",
		ConversationStore: store,
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx := context.Background()

	// Create original conversation
	conv := Conversation{
		ID:      convID,
		AgentID: "agent-1",
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "Hi!", Timestamp: time.Now()},
		},
		Metadata: map[string]any{
			"user_id": "user-456",
		},
	}
	err = store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Fork conversation
	err = agent.ForkConversation(ctx, convID, "conv-456", "What if we took a different approach?")
	if err != nil {
		t.Fatalf("ForkConversation failed: %v", err)
	}

	// Verify fork
	forked, err := store.Load(ctx, "conv-456")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should have 3 turns (2 original + 1 new)
	if len(forked.Turns) != 3 {
		t.Errorf("expected 3 turns, got %d", len(forked.Turns))
	}

	// Last turn should be the new message
	if forked.Turns[2].Content != "What if we took a different approach?" {
		t.Errorf("unexpected content: %s", forked.Turns[2].Content)
	}

	// Metadata should be copied
	if forked.Metadata["user_id"] != "user-456" {
		t.Errorf("metadata not copied")
	}

	// AgentID should be copied
	if forked.AgentID != "agent-1" {
		t.Errorf("expected AgentID=agent-1, got %s", forked.AgentID)
	}

	// Original should be unchanged
	original, err := store.Load(ctx, convID)
	if err != nil {
		t.Fatalf("Load original failed: %v", err)
	}
	if len(original.Turns) != 2 {
		t.Errorf("original conversation should have 2 turns, got %d", len(original.Turns))
	}
}
