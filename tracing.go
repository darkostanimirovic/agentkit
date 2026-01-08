// Package agentkit provides tracing capabilities for LLM applications
package agentkit

import (
	"context"
	"time"
)

// Tracer defines the interface for tracing LLM operations
// This interface allows for multiple tracing backend implementations
type Tracer interface {
	// StartTrace creates a new trace context for the agent run
	// Returns a context with the trace attached and a function to end the trace
	StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func())

	// StartSpan creates a new span within the current trace
	// Spans represent individual operations like tool calls or LLM generations
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func())

	// LogGeneration records an LLM generation event
	LogGeneration(ctx context.Context, opts GenerationOptions) error

	// LogEvent records a simple event within the trace
	LogEvent(ctx context.Context, name string, attributes map[string]any) error

	// SetTraceAttributes sets attributes on the current trace
	SetTraceAttributes(ctx context.Context, attributes map[string]any) error

	// SetSpanOutput sets the output on the current span (observation)
	SetSpanOutput(ctx context.Context, output any) error

	// SetSpanAttributes sets attributes on the current span as observation metadata
	SetSpanAttributes(ctx context.Context, attributes map[string]any) error

	// Flush ensures all pending traces are sent (important for short-lived applications)
	Flush(ctx context.Context) error
}

// TraceOption configures trace creation
type TraceOption func(*TraceConfig)

// SpanOption configures span creation
type SpanOption func(*SpanConfig)

// TraceConfig holds configuration for a trace
type TraceConfig struct {
	// UserID identifies the end-user
	UserID string
	// SessionID groups related traces (e.g., conversation thread)
	SessionID string
	// Tags categorize the trace
	Tags []string
	// Metadata stores arbitrary key-value data
	Metadata map[string]any
	// Input is the initial input for the trace
	Input any
	// Version tracks the application version
	Version string
	// Environment specifies the deployment environment (production, staging, etc.)
	Environment string
	// Release identifies the release version
	Release string
}

// SpanConfig holds configuration for a span
type SpanConfig struct {
	// Type specifies the span type (span, generation, event, tool, retrieval)
	Type SpanType
	// Input is the input data for this operation
	Input any
	// Metadata stores arbitrary key-value data
	Metadata map[string]any
	// Level specifies the log level (DEBUG, DEFAULT, WARNING, ERROR)
	Level LogLevel
}

// SpanType represents the type of observation
type SpanType string

const (
	// SpanTypeSpan is a generic span for non-LLM operations
	SpanTypeSpan SpanType = "span"
	// SpanTypeGeneration tracks LLM calls
	SpanTypeGeneration SpanType = "generation"
	// SpanTypeEvent tracks point-in-time events
	SpanTypeEvent SpanType = "event"
	// SpanTypeTool tracks tool/function calls
	SpanTypeTool SpanType = "tool"
	// SpanTypeRetrieval tracks RAG retrieval steps
	SpanTypeRetrieval SpanType = "retrieval"
)

// LogLevel represents the severity level
type LogLevel string

const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelDefault LogLevel = "DEFAULT"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
)

// GenerationOptions holds data for an LLM generation
type GenerationOptions struct {
	// Name of the generation
	Name string
	// Model name (e.g., "gpt-4o")
	Model string
	// ModelParameters like temperature, max_tokens, etc.
	ModelParameters map[string]any
	// Input prompt/messages
	Input any
	// Output completion/response
	Output any
	// Usage token counts
	Usage *UsageInfo
	// Cost in USD (optional, can be calculated from usage)
	Cost *CostInfo
	// Metadata stores arbitrary key-value data
	Metadata map[string]any
	// StartTime when generation started
	StartTime time.Time
	// EndTime when generation completed
	EndTime time.Time
	// CompletionStartTime when the model began generating (for streaming)
	CompletionStartTime *time.Time
	// PromptName links to a managed prompt in Langfuse
	PromptName string
	// PromptVersion links to a specific prompt version
	PromptVersion int
	// Level specifies the log level
	Level LogLevel
	// StatusMessage describes errors or warnings
	StatusMessage string
}

// UsageInfo tracks token consumption
// This data comes directly from OpenAI's API response
type UsageInfo struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// CostInfo tracks cost breakdown
// NOTE: OpenAI's API does NOT provide cost information.
// Cost is estimated based on published pricing and may be inaccurate.
// Set DisableCostCalculation = true to disable cost estimation entirely.
type CostInfo struct {
	PromptCost     float64 // Estimated cost for prompt tokens in USD
	CompletionCost float64 // Estimated cost for completion tokens in USD
	TotalCost      float64 // Estimated total cost in USD
}

// Option functions for trace configuration
func WithUserID(userID string) TraceOption {
	return func(c *TraceConfig) {
		c.UserID = userID
	}
}

func WithSessionID(sessionID string) TraceOption {
	return func(c *TraceConfig) {
		c.SessionID = sessionID
	}
}

func WithTags(tags ...string) TraceOption {
	return func(c *TraceConfig) {
		c.Tags = append(c.Tags, tags...)
	}
}

func WithMetadata(metadata map[string]any) TraceOption {
	return func(c *TraceConfig) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]any)
		}
		for k, v := range metadata {
			c.Metadata[k] = v
		}
	}
}

func WithTraceInput(input any) TraceOption {
	return func(c *TraceConfig) {
		c.Input = input
	}
}

func WithVersion(version string) TraceOption {
	return func(c *TraceConfig) {
		c.Version = version
	}
}

func WithEnvironment(env string) TraceOption {
	return func(c *TraceConfig) {
		c.Environment = env
	}
}

func WithRelease(release string) TraceOption {
	return func(c *TraceConfig) {
		c.Release = release
	}
}

// Option functions for span configuration
func WithSpanType(spanType SpanType) SpanOption {
	return func(c *SpanConfig) {
		c.Type = spanType
	}
}

func WithSpanInput(input any) SpanOption {
	return func(c *SpanConfig) {
		c.Input = input
	}
}

func WithSpanMetadata(metadata map[string]any) SpanOption {
	return func(c *SpanConfig) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]any)
		}
		for k, v := range metadata {
			c.Metadata[k] = v
		}
	}
}

func WithLogLevel(level LogLevel) SpanOption {
	return func(c *SpanConfig) {
		c.Level = level
	}
}

// NoOpTracer is a tracer that does nothing (used when tracing is disabled)
type NoOpTracer struct{}

func (n *NoOpTracer) StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func()) {
	return ctx, func() {}
}

func (n *NoOpTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func()) {
	return ctx, func() {}
}

func (n *NoOpTracer) LogGeneration(ctx context.Context, opts GenerationOptions) error {
	return nil
}

func (n *NoOpTracer) LogEvent(ctx context.Context, name string, attributes map[string]any) error {
	return nil
}

func (n *NoOpTracer) SetTraceAttributes(ctx context.Context, attributes map[string]any) error {
	return nil
}

func (n *NoOpTracer) SetSpanOutput(ctx context.Context, output any) error {
	return nil
}

func (n *NoOpTracer) SetSpanAttributes(ctx context.Context, attributes map[string]any) error {
	return nil
}

func (n *NoOpTracer) Flush(ctx context.Context) error {
	return nil
}
