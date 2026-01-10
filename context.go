package agentkit

import (
	"context"
	"errors"
)

// ErrDepsNotFound is returned when dependencies are not found in context
var ErrDepsNotFound = errors.New("agentkit: dependencies not found in context")

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const (
	depsKey           contextKey = "agentkit_deps"
	conversationIDKey contextKey = "agentkit_conversation_id"
	traceIDKey        contextKey = "agentkit_trace_id"
	spanIDKey         contextKey = "agentkit_span_id"
	eventPublisherKey contextKey = "agentkit_event_publisher"
	tracerKey         contextKey = "agentkit_tracer"
)

// EventPublisher is a function that publishes events
type EventPublisher func(Event)

// WithEventPublisher adds an event publisher to the context
func WithEventPublisher(ctx context.Context, publisher EventPublisher) context.Context {
	return context.WithValue(ctx, eventPublisherKey, publisher)
}

// GetEventPublisher retrieves the event publisher from the context
func GetEventPublisher(ctx context.Context) (EventPublisher, bool) {
	publisher, ok := ctx.Value(eventPublisherKey).(EventPublisher)
	return publisher, ok
}

// WithDeps adds dependencies to the context
func WithDeps(ctx context.Context, deps any) context.Context {
	return context.WithValue(ctx, depsKey, deps)
}

// GetDeps retrieves dependencies from the context, returning an error if not found.
// This is the preferred method for accessing dependencies as it allows for proper error handling.
func GetDeps[T any](ctx context.Context) (T, error) {
	deps, ok := ctx.Value(depsKey).(T)
	if !ok {
		var zero T
		return zero, ErrDepsNotFound
	}
	return deps, nil
}

// MustGetDeps retrieves dependencies from the context or panics.
//
// Deprecated: Use GetDeps instead for better error handling.
// This method is kept for backward compatibility but should only be used
// in controlled environments where dependencies are guaranteed to exist.
func MustGetDeps[T any](ctx context.Context) T {
	deps, err := GetDeps[T](ctx)
	if err != nil {
		panic(err)
	}
	return deps
}

// WithConversation adds a conversation ID to the context
func WithConversation(ctx context.Context, conversationID string) context.Context {
	return context.WithValue(ctx, conversationIDKey, conversationID)
}

// GetConversationID retrieves the conversation ID from the context
func GetConversationID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(conversationIDKey).(string)
	return id, ok
}

// WithTraceID adds a trace ID to the context for request correlation.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID retrieves the trace ID from the context.
func GetTraceID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(traceIDKey).(string)
	return id, ok
}

// WithSpanID adds a span ID to the context for request correlation.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey, spanID)
}

// GetSpanID retrieves the span ID from the context.
func GetSpanID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(spanIDKey).(string)
	return id, ok
}

// WithTracer adds a tracer to the context for delegated agent inheritance (handoffs/collaboration)
func WithTracer(ctx context.Context, tracer Tracer) context.Context {
	return context.WithValue(ctx, tracerKey, tracer)
}

// GetTracer retrieves the tracer from the context
// Returns nil if no tracer is in the context
func GetTracer(ctx context.Context) Tracer {
	tracer, _ := ctx.Value(tracerKey).(Tracer)
	return tracer
}
