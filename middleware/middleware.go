package middleware

import "context"

// Middleware provides hooks into agent execution for observability and instrumentation.
type Middleware interface {
	OnAgentStart(ctx context.Context, input string) context.Context
	OnAgentComplete(ctx context.Context, output string, err error)
	OnToolStart(ctx context.Context, tool string, args any) context.Context
	OnToolComplete(ctx context.Context, tool string, result any, err error)
	OnLLMCall(ctx context.Context, req any) context.Context
	OnLLMResponse(ctx context.Context, resp any, err error)
}

// BaseMiddleware provides no-op implementations for Middleware.
// Embed this in custom middleware to implement only the hooks you need.
type BaseMiddleware struct{}

func (BaseMiddleware) OnAgentStart(ctx context.Context, _ string) context.Context { return ctx }
func (BaseMiddleware) OnAgentComplete(context.Context, string, error)             {}
func (BaseMiddleware) OnToolStart(ctx context.Context, _ string, _ any) context.Context {
	return ctx
}
func (BaseMiddleware) OnToolComplete(context.Context, string, any, error)   {}
func (BaseMiddleware) OnLLMCall(ctx context.Context, _ any) context.Context { return ctx }
func (BaseMiddleware) OnLLMResponse(context.Context, any, error)            {}
