# LLM Tracing with Langfuse

AgentKit supports LLM observability through an extensible tracing interface. Currently, Langfuse is supported via OpenTelemetry.

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"
	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Create Langfuse tracer
	tracer, err := agentkit.NewLangfuseTracer(agentkit.LangfuseConfig{
		PublicKey:   os.Getenv("LANGFUSE_PUBLIC_KEY"),   // pk-lf-...
		SecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),   // sk-lf-...
		BaseURL:     "https://cloud.langfuse.com",       // or EU: https://cloud.langfuse.com
		ServiceName: "my-agent",
		Environment: "production",
		Enabled:     true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer tracer.Shutdown(context.Background())

	// Create agent with tracing
	agent, err := agentkit.New(agentkit.Config{
		APIKey:   os.Getenv("OPENAI_API_KEY"),
		Model:    "gpt-4o-mini",
		Tracer:   tracer,  // Enable tracing
	})
	if err != nil {
		log.Fatal(err)
	}

	// Run agent - traces are automatically captured
	events := agent.Run(context.Background(), "Your prompt here")
	for event := range events {
		// Process events...
	}

	// Flush traces before exiting
	tracer.Flush(context.Background())
}
```

## Configuration

### Langfuse Setup

1. Sign up for [Langfuse Cloud](https://cloud.langfuse.com) or self-host
2. Create a project and get your API keys (Settings → API Keys)
3. Set environment variables:
   ```bash
   export LANGFUSE_PUBLIC_KEY="pk-lf-..."
   export LANGFUSE_SECRET_KEY="sk-lf-..."
   ```

### Langfuse Regions

- **US/Cloud**: `https://cloud.langfuse.com` (default)
- **EU**: `https://cloud.langfuse.com` 
- **Self-hosted**: Your instance URL

### Configuration Options

```go
agentkit.LangfuseConfig{
	PublicKey:      "pk-lf-...",              // Required
	SecretKey:      "sk-lf-...",              // Required
	BaseURL:        "https://cloud.langfuse.com", // Optional, defaults to US cloud
	ServiceName:    "my-service",             // Optional, defaults to "agentkit"
	ServiceVersion: "1.0.0",                  // Optional
	Environment:    "production",             // Optional (production, staging, development)
	Enabled:        true,                     // Optional, defaults to true
}
```

## What Gets Traced

AgentKit automatically traces:

- **Agent Runs**: Each agent execution as a trace
- **LLM Generations**: Model calls with input/output, tokens, cost
- **Tool Executions**: Tool calls with parameters and results
- **Errors**: Failed operations with error details

## Architecture

### Extensible Design

The tracing system is designed to support multiple providers:

```go
type Tracer interface {
	StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func())
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func())
	LogGeneration(ctx context.Context, opts GenerationOptions) error
	LogEvent(ctx context.Context, name string, attributes map[string]any) error
	SetTraceAttributes(ctx context.Context, attributes map[string]any) error
	SetSpanOutput(ctx context.Context, output any) error
	SetSpanAttributes(ctx context.Context, attributes map[string]any) error
	Flush(ctx context.Context) error
}
```

**Method Descriptions:**
- `StartTrace`: Creates a root trace context for an agent run
- `StartSpan`: Creates a child span within a trace (e.g., for tool execution)
- `LogGeneration`: Records an LLM generation with input, output, and token usage
- `LogEvent`: Records a simple point-in-time event
- `SetTraceAttributes`: Sets trace-level metadata (applies to entire trace)
- `SetSpanOutput`: Sets the output on the current span/observation
- `SetSpanAttributes`: Sets observation-level metadata on the current span
- `Flush`: Ensures all pending traces are sent to the backend

### OpenTelemetry Integration

Langfuse integration uses OpenTelemetry Go SDK:

- **Protocol**: OTLP over HTTP with protobuf
- **Endpoint**: `{baseURL}/api/public/otel/v1/traces`
- **Authentication**: Basic Auth with base64(publicKey:secretKey)
- **Batching**: Spans are batched and sent asynchronously

### Langfuse Attribute Mapping

Agentkit uses specific OpenTelemetry attributes that map to Langfuse's data model. See the [Langfuse OpenTelemetry documentation](https://langfuse.com/docs/integrations/opentelemetry#property-mapping) for full details.

#### Trace-Level Attributes

| OpenTelemetry Attribute | Langfuse Field | Purpose |
|------------------------|----------------|---------|
| `langfuse.user.id`, `user.id` | User ID | End-user identifier |
| `langfuse.session.id`, `session.id` | Session ID | Conversation/session grouping |
| `langfuse.trace.name` | Trace Name | Name of the trace |
| `langfuse.trace.input` | Trace Input | Initial input for the trace |
| `langfuse.trace.output` | Trace Output | Final output of the trace |
| `langfuse.trace.metadata.*` | Metadata | Custom trace-level metadata |
| `langfuse.version` | Version | Application version |
| `langfuse.release` | Release | Release identifier |
| `langfuse.environment` | Environment | Deployment environment |

#### Observation-Level Attributes

| OpenTelemetry Attribute | Langfuse Field | Purpose |
|------------------------|----------------|---------|
| `langfuse.observation.type` | Type | span, generation, event, tool, retrieval |
| `langfuse.observation.input` | Input | Input data for this operation |
| `langfuse.observation.output` | Output | Output data from this operation |
| `langfuse.observation.metadata.*` | Metadata | Custom observation-level metadata |
| `langfuse.observation.model.name` | Model | LLM model name (generations only) |
| `langfuse.observation.model.parameters` | Model Params | Model settings (generations only) |
| `langfuse.observation.usage_details` | Token Usage | Token counts (generations only) |
| `langfuse.observation.cost_details` | Cost | Cost breakdown (generations only) |
| `langfuse.observation.level` | Level | DEBUG, DEFAULT, WARNING, ERROR |

#### Standard GenAI Attributes (also supported)

| OpenTelemetry Attribute | Langfuse Field | Purpose |
|------------------------|----------------|---------|
| `gen_ai.request.model` | Model Name | Model identifier (e.g., "gpt-4") |
| `gen_ai.prompt` | Input | Prompt text (fallback) |
| `gen_ai.completion` | Output | Completion text (fallback) |
| `gen_ai.usage.input_tokens` | Tokens In | Input token count |
| `gen_ai.usage.output_tokens` | Tokens Out | Output token count |
| `gen_ai.usage.cost` | Cost | Total cost in USD |

## Known Issues

### Go 1.24+ Compatibility

**Issue**: Some dependencies (golang.org/x/net, google.golang.org/grpc) have not yet been fully updated for Go 1.24+ breaking changes.

**Symptom**: Build errors like `undefined: context.Canceled` when using Go 1.25+

**Workaround**: The library requires Go 1.24. Users with Go 1.25 runtime can still use the library - the issue is in transitive dependencies which will be resolved when they release updates.

**Status**: Tracking upstream updates. This does not affect runtime compatibility.

## Advanced Usage

### Custom Tracer Implementation

To add support for another tracing provider:

```go
type MyTracer struct {
	// Your implementation
}

func (m *MyTracer) StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func()) {
	// Implement trace creation
}

// Implement other Tracer interface methods...
```

### Disabling Tracing

Tracing is disabled by default. If you don't provide a tracer in the agent config, a `NoOpTracer` is used which has zero overhead.

```go
agent, err := agentkit.New(agentkit.Config{
	APIKey: os.Getenv("OPENAI_API_KEY"),
	// No Tracer field = tracing disabled
})
```

## Best Practices

1. **Always call Flush()**: For short-lived applications, call `tracer.Flush(ctx)` before exiting to ensure all traces are sent
2. **Use defer for cleanup**: `defer tracer.Shutdown(ctx)` ensures proper cleanup
3. **Environment-based enabling**: Use `Enabled: os.Getenv("TRACING_ENABLED") == "true"` for conditional tracing
4. **Cost tracking**: Langfuse automatically calculates costs if you log token usage

## Troubleshooting

### Traces not appearing in Langfuse

1. Check API keys are correct
2. Verify BaseURL matches your region
3. Call `tracer.Flush(ctx)` before program exits
4. Check Langfuse dashboard for errors

### Performance impact

- Tracing adds minimal overhead (~1-2ms per span)
- Spans are batched and sent asynchronously
- Use `NoOpTracer` in performance-critical paths if needed

## Examples

See [examples/tracing-langfuse](../examples/tracing-langfuse) for a complete working example.

## References

- [Langfuse Documentation](https://langfuse.com/docs)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [Langfuse OpenTelemetry Integration](https://langfuse.com/docs/integrations/opentelemetry)
