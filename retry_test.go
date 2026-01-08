package agentkit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}

	if cfg.InitialDelay != time.Second {
		t.Errorf("expected InitialDelay 1s, got %v", cfg.InitialDelay)
	}

	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay 30s, got %v", cfg.MaxDelay)
	}

	if cfg.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %f", cfg.Multiplier)
	}
}

func TestRetryConfig_IsRetryable(t *testing.T) {
	cfg := DefaultRetryConfig()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "rate limit error",
			err:      ErrRateLimited,
			expected: true,
		},
		{
			name:     "timeout error",
			err:      ErrTimeout,
			expected: true,
		},
		{
			name:     "server error",
			err:      ErrServerError,
			expected: true,
		},
		{
			name:     "wrapped rate limit error",
			err:      errors.Join(errors.New("context"), ErrRateLimited),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestRetryConfig_CalculateDelay(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second}, // Capped at MaxDelay
		{5, 1 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := cfg.CalculateDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("CalculateDelay(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestWithRetry_Success(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "success", nil
	}

	ctx := context.Background()
	result, err := WithRetry(ctx, cfg, fn)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "success" {
		t.Errorf("expected result 'success', got %s", result)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []error{ErrRateLimited},
	}

	callCount := 0
	fn := func() (int, error) {
		callCount++
		if callCount < 3 {
			return 0, ErrRateLimited
		}
		return 42, nil
	}

	ctx := context.Background()
	result, err := WithRetry(ctx, cfg, fn)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:      2,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []error{ErrTimeout},
	}

	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "", ErrTimeout
	}

	ctx := context.Background()
	_, err := WithRetry(ctx, cfg, fn)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected error to wrap ErrTimeout, got %v", err)
	}

	// Should be called: initial + 2 retries = 3 times
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", callCount)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []error{ErrRateLimited},
	}

	callCount := 0
	customErr := errors.New("custom error")
	fn := func() (string, error) {
		callCount++
		return "", customErr
	}

	ctx := context.Background()
	_, err := WithRetry(ctx, cfg, fn)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, customErr) {
		t.Errorf("expected error to wrap customErr, got %v", err)
	}

	// Should be called only once (no retries for non-retryable errors)
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        1 * time.Second,
		Multiplier:      2.0,
		RetryableErrors: []error{ErrTimeout},
	}

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	fn := func() (string, error) {
		callCount++
		if callCount == 2 {
			// Cancel context after second call
			cancel()
		}
		return "", ErrTimeout
	}

	_, err := WithRetry(ctx, cfg, fn)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should contain context cancellation error
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}
