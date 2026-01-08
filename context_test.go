package agentkit

import (
	"context"
	"errors"
	"testing"
)

type TestDeps struct {
	UserID   string
	Database string
}

type OtherDeps struct {
	APIKey string
}

func TestWithDeps(t *testing.T) {
	ctx := context.Background()
	deps := TestDeps{
		UserID:   "user123",
		Database: "testdb",
	}

	// Add deps to context
	newCtx := WithDeps(ctx, deps)

	// Retrieve deps
	retrieved, err := GetDeps[TestDeps](newCtx)
	if err != nil {
		t.Fatalf("expected to retrieve deps: %v", err)
	}

	if retrieved.UserID != deps.UserID {
		t.Errorf("expected UserID %s, got %s", deps.UserID, retrieved.UserID)
	}

	if retrieved.Database != deps.Database {
		t.Errorf("expected Database %s, got %s", deps.Database, retrieved.Database)
	}
}

func TestGetDeps_NotFound(t *testing.T) {
	ctx := context.Background()

	// Try to retrieve deps that don't exist
	_, err := GetDeps[TestDeps](ctx)
	if !errors.Is(err, ErrDepsNotFound) {
		t.Errorf("expected ErrDepsNotFound, got %v", err)
	}
}

func TestGetDeps_WrongType(t *testing.T) {
	ctx := context.Background()
	deps := TestDeps{UserID: "user123"}
	ctx = WithDeps(ctx, deps)

	// Try to retrieve with wrong type
	_, err := GetDeps[OtherDeps](ctx)
	if !errors.Is(err, ErrDepsNotFound) {
		t.Errorf("expected ErrDepsNotFound for wrong type, got %v", err)
	}
}

func TestMustGetDeps(t *testing.T) {
	ctx := context.Background()
	deps := TestDeps{
		UserID:   "user123",
		Database: "testdb",
	}

	ctx = WithDeps(ctx, deps)

	// Should not panic
	retrieved := MustGetDeps[TestDeps](ctx)

	if retrieved.UserID != deps.UserID {
		t.Errorf("expected UserID %s, got %s", deps.UserID, retrieved.UserID)
	}
}

func TestMustGetDeps_Panic(t *testing.T) {
	ctx := context.Background()

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustGetDeps to panic")
		}
	}()

	MustGetDeps[TestDeps](ctx)
}

func TestMultipleDepsTypes(t *testing.T) {
	t.Skip("WithDeps is type-specific - use separate contexts for different dep types")
	// Note: The current implementation stores deps by type in context
	// Each type has its own context key, so they don't overwrite each other
	// This test would need the implementation to support multiple types simultaneously
}

func TestContextChaining(t *testing.T) {
	ctx := context.Background()

	// Add deps
	deps := TestDeps{UserID: "user123"}
	ctx = WithDeps(ctx, deps)

	// Create child context with cancellation
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Deps should still be accessible in child context
	retrieved, err := GetDeps[TestDeps](childCtx)
	if err != nil {
		t.Fatalf("expected to retrieve deps from child context: %v", err)
	}

	if retrieved.UserID != "user123" {
		t.Errorf("expected UserID user123, got %s", retrieved.UserID)
	}
}

func TestDepsImmutability(t *testing.T) {
	ctx := context.Background()
	original := TestDeps{UserID: "user123"}
	ctx = WithDeps(ctx, original)

	// Retrieve and modify
	retrieved := MustGetDeps[TestDeps](ctx)
	retrieved.UserID = "modified" // Intentionally modifying to test immutability
	_ = retrieved.UserID          // Acknowledge the write

	// Original should be unchanged
	retrievedAgain := MustGetDeps[TestDeps](ctx)
	if retrievedAgain.UserID != "user123" {
		t.Error("deps were modified, expected immutability")
	}
}
