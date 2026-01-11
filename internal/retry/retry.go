package retry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// Common retryable errors
var (
	ErrRateLimited = errors.New("agentkit: rate limit exceeded")
	ErrTimeout     = errors.New("agentkit: request timeout")
	ErrServerError = errors.New("agentkit: server error (5xx)")
)

// RetryConfig configures retry behavior for API calls
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts (0 = no retries)
	InitialDelay    time.Duration // Initial delay before first retry
	MaxDelay        time.Duration // Maximum delay between retries
	Multiplier      float64       // Backoff multiplier (e.g., 2.0 for exponential)
	RetryableErrors []error       // Errors that should trigger a retry
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: []error{
			ErrRateLimited,
			ErrTimeout,
			ErrServerError,
		},
	}
}

// IsRetryable checks if an error should trigger a retry
func (rc RetryConfig) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	for _, retryableErr := range rc.RetryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

// CalculateDelay calculates the delay for a given retry attempt using exponential backoff
func (rc RetryConfig) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.InitialDelay
	}

	delay := float64(rc.InitialDelay) * math.Pow(rc.Multiplier, float64(attempt))

	if time.Duration(delay) > rc.MaxDelay {
		return rc.MaxDelay
	}

	return time.Duration(delay)
}

// WithRetry wraps a function with retry logic
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("context cancelled: %w", err)
		}

		// Attempt the operation
		result, lastErr = fn()

		// Success - return immediately
		if lastErr == nil {
			if attempt > 0 {
				slog.Info("operation succeeded after retry", "attempt", attempt)
			}
			return result, nil
		}

		// Check if error is retryable
		if !cfg.IsRetryable(lastErr) {
			return result, fmt.Errorf("non-retryable error: %w", lastErr)
		}

		// Don't wait after the last attempt
		if attempt >= cfg.MaxRetries {
			break
		}

		// Calculate delay and wait
		delay := cfg.CalculateDelay(attempt)
		slog.Warn("operation failed, retrying",
			"attempt", attempt+1,
			"max_retries", cfg.MaxRetries,
			"delay", delay,
			"error", lastErr,
		)

		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return result, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
		}
	}

	return result, fmt.Errorf("max retries (%d) exceeded: %w", cfg.MaxRetries, lastErr)
}
