package conversation

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	roleAssistant     = "assistant"
	searchIssuesTool  = "search_issues"
)

func TestMemoryConversationStore_SaveAndLoad(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	conv := Conversation{
		ID:      "conv-123",
		AgentID: "agent-1",
		Turns: []ConversationTurn{
			{
				Role:      "user",
				Content:   "Hello",
				Timestamp: time.Now(),
			},
		},
		Metadata: map[string]any{
			"user_id": "user-456",
		},
	}

	// Save conversation
	err := store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load conversation
	loaded, err := store.Load(ctx, "conv-123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != conv.ID {
		t.Errorf("expected ID=%s, got %s", conv.ID, loaded.ID)
	}
	if loaded.AgentID != conv.AgentID {
		t.Errorf("expected AgentID=%s, got %s", conv.AgentID, loaded.AgentID)
	}
	if len(loaded.Turns) != 1 {
		t.Errorf("expected 1 turn, got %d", len(loaded.Turns))
	}
	if loaded.Turns[0].Role != "user" {
		t.Errorf("expected role=user, got %s", loaded.Turns[0].Role)
	}
}

func TestMemoryConversationStore_LoadNotFound(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestMemoryConversationStore_Append(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	// Create initial conversation
	conv := Conversation{
		ID: "conv-123",
		Turns: []ConversationTurn{
			{
				Role:      "user",
				Content:   "Hello",
				Timestamp: time.Now(),
			},
		},
	}

	err := store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Append a new turn
	newTurn := ConversationTurn{
		Role:      roleAssistant,
		Content:   "Hi there!",
		Timestamp: time.Now(),
	}

	err = store.Append(ctx, "conv-123", newTurn)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load and verify
	loaded, err := store.Load(ctx, "conv-123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(loaded.Turns))
	}
	if loaded.Turns[1].Role != roleAssistant {
		t.Errorf("expected role=%s, got %s", roleAssistant, loaded.Turns[1].Role)
	}
	if loaded.Turns[1].Content != "Hi there!" {
		t.Errorf("expected content='Hi there!', got %s", loaded.Turns[1].Content)
	}
}

func TestMemoryConversationStore_AppendNotFound(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	turn := ConversationTurn{
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	err := store.Append(ctx, "nonexistent", turn)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestMemoryConversationStore_Delete(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	// Create conversation
	conv := Conversation{
		ID: "conv-123",
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}

	err := store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete conversation
	err = store.Delete(ctx, "conv-123")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = store.Load(ctx, "conv-123")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("expected ErrConversationNotFound after delete, got %v", err)
	}
}

func TestMemoryConversationStore_DeleteNotFound(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestMemoryConversationStore_Timestamps(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	before := time.Now()

	conv := Conversation{
		ID: "conv-123",
		Turns: []ConversationTurn{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
	}

	err := store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	after := time.Now()

	loaded, err := store.Load(ctx, "conv-123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// CreatedAt should be set
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if loaded.CreatedAt.Before(before) || loaded.CreatedAt.After(after) {
		t.Errorf("CreatedAt timestamp out of range: %v (expected between %v and %v)", loaded.CreatedAt, before, after)
	}

	// UpdatedAt should be set
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestMemoryConversationStore_Count(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	if store.Count() != 0 {
		t.Errorf("expected count=0, got %d", store.Count())
	}

	// Add conversations
	for i := 1; i <= 3; i++ {
		conv := Conversation{
			ID: string(rune('a' + i)),
			Turns: []ConversationTurn{
				{Role: "user", Content: "Hello", Timestamp: time.Now()},
			},
		}
		err := store.Save(ctx, conv)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	if store.Count() != 3 {
		t.Errorf("expected count=3, got %d", store.Count())
	}
}

func TestMemoryConversationStore_Clear(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	// Add conversations
	for i := 1; i <= 3; i++ {
		conv := Conversation{
			ID: string(rune('a' + i)),
			Turns: []ConversationTurn{
				{Role: "user", Content: "Hello", Timestamp: time.Now()},
			},
		}
		err := store.Save(ctx, conv)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	store.Clear()

	if store.Count() != 0 {
		t.Errorf("expected count=0 after Clear, got %d", store.Count())
	}
}


func TestConversationWithToolCalls(t *testing.T) {
	store := NewMemoryConversationStore()
	ctx := context.Background()

	conv := Conversation{
		ID: "conv-123",
		Turns: []ConversationTurn{
			{
				Role:      roleAssistant,
				Content:   "I'll search for issues",
				Timestamp: time.Now(),
				ToolCalls: []ConversationToolCall{
					{
						ID:   "call-1",
						Name: searchIssuesTool,
						Arguments: map[string]any{
							"query": "bug",
						},
					},
				},
			},
			{
				Role:      "tool",
				Content:   "",
				Timestamp: time.Now(),
				ToolResults: []ConversationToolResult{
					{
						CallID: "call-1",
						Result: map[string]any{
							"count": 5,
						},
					},
				},
			},
		},
	}

	err := store.Save(ctx, conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(ctx, "conv-123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Turns[0].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(loaded.Turns[0].ToolCalls))
	}
	if loaded.Turns[0].ToolCalls[0].Name != searchIssuesTool {
		t.Errorf("expected tool name=%s, got %s", searchIssuesTool, loaded.Turns[0].ToolCalls[0].Name)
	}

	if len(loaded.Turns[1].ToolResults) != 1 {
		t.Errorf("expected 1 tool result, got %d", len(loaded.Turns[1].ToolResults))
	}
	if loaded.Turns[1].ToolResults[0].CallID != "call-1" {
		t.Errorf("expected call_id=call-1, got %s", loaded.Turns[1].ToolResults[0].CallID)
	}
}
