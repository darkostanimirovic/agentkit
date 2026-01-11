package conversation

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrConversationNotFound is returned when a conversation doesn't exist
var ErrConversationNotFound = errors.New("agentkit: conversation not found")

// MemoryConversationStore provides an in-memory implementation of ConversationStore
// Useful for testing and development. Not suitable for production.
type MemoryConversationStore struct {
	mu            sync.RWMutex
	conversations map[string]Conversation
}

// NewMemoryConversationStore creates a new in-memory conversation store
func NewMemoryConversationStore() *MemoryConversationStore {
	return &MemoryConversationStore{
		conversations: make(map[string]Conversation),
	}
}

// Save persists a complete conversation
func (s *MemoryConversationStore) Save(ctx context.Context, conv Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv.UpdatedAt = time.Now()
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = conv.UpdatedAt
	}

	s.conversations[conv.ID] = conv
	return nil
}

// Load retrieves a conversation by ID
func (s *MemoryConversationStore) Load(ctx context.Context, id string) (Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, exists := s.conversations[id]
	if !exists {
		return Conversation{}, ErrConversationNotFound
	}

	return conv, nil
}

// Append adds a turn to an existing conversation
func (s *MemoryConversationStore) Append(ctx context.Context, id string, turn ConversationTurn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, exists := s.conversations[id]
	if !exists {
		return ErrConversationNotFound
	}

	conv.Turns = append(conv.Turns, turn)
	conv.UpdatedAt = time.Now()
	s.conversations[id] = conv

	return nil
}

// Delete removes a conversation
func (s *MemoryConversationStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.conversations[id]; !exists {
		return ErrConversationNotFound
	}

	delete(s.conversations, id)
	return nil
}

// Count returns the number of conversations (useful for testing)
func (s *MemoryConversationStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.conversations)
}

// Clear removes all conversations (useful for testing)
func (s *MemoryConversationStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversations = make(map[string]Conversation)
}
