package agentkit

import "context"

// ResponseStreamClient provides streaming access to model responses.
type ResponseStreamClient interface {
	Recv() (*ResponseStreamChunk, error)
	Close() error
}

// LLMProvider abstracts the Responses API client for testing and custom providers.
type LLMProvider interface {
	CreateResponse(ctx context.Context, req ResponseRequest) (*ResponseObject, error)
	CreateResponseStream(ctx context.Context, req ResponseRequest) (ResponseStreamClient, error)
}
