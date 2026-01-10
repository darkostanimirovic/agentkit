// Package agentkit provides Langfuse tracing implementation via OpenTelemetry
package agentkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/darkostanimirovic/agentkit"
)

// LangfuseTracer implements the Tracer interface using OpenTelemetry to send traces to Langfuse
type LangfuseTracer struct {
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
}

// LangfuseConfig holds configuration for Langfuse tracing
type LangfuseConfig struct {
	// PublicKey is the Langfuse public API key (pk-lf-...)
	PublicKey string
	// SecretKey is the Langfuse secret API key (sk-lf-...)
	SecretKey string
	// BaseURL is the Langfuse API endpoint
	// Defaults to "https://cloud.langfuse.com" (EU region)
	// Use "https://us.cloud.langfuse.com" for US region
	BaseURL string
	// ServiceName identifies your application in traces
	ServiceName string
	// ServiceVersion tracks your application version
	ServiceVersion string
	// Environment specifies the deployment environment (production, staging, etc.)
	Environment string
	// Enabled controls whether tracing is active (defaults to true)
	Enabled bool
}

// NewLangfuseTracer creates a new Langfuse tracer instance
func NewLangfuseTracer(cfg LangfuseConfig) (*LangfuseTracer, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("tracing is disabled")
	}

	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("both PublicKey and SecretKey are required")
	}

	// Default to EU cloud endpoint
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://cloud.langfuse.com"
	}

	// Default service name
	if cfg.ServiceName == "" {
		cfg.ServiceName = "agentkit-app"
	}

	// Create Basic Auth header
	authString := cfg.PublicKey + ":" + cfg.SecretKey
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(authString))

	// Configure OTLP HTTP exporter for Langfuse
	// Extract host from BaseURL (remove scheme)
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}
	// Remove scheme prefix to get just host:port
	endpoint := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")

	// Determine if using HTTP or HTTPS
	useInsecure := strings.HasPrefix(cfg.BaseURL, "http://")

	// Configure OTLP HTTP exporter
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": authHeader,
		}),
		otlptracehttp.WithURLPath("/api/public/otel/v1/traces"),
	}
	if useInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	// Create resource with service information (without Default to avoid schema conflicts)
	res := resource.NewSchemaless(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		attribute.String("deployment.environment", cfg.Environment),
	)

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set propagator for distributed tracing
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return &LangfuseTracer{
		tracer:         tp.Tracer(tracerName),
		tracerProvider: tp,
	}, nil
}

// StartTrace creates a new trace context
func (l *LangfuseTracer) StartTrace(ctx context.Context, name string, opts ...TraceOption) (context.Context, func()) {
	cfg := &TraceConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Use explicit start time if provided, otherwise use current time
	startTime := time.Now()
	if cfg.StartTime != nil {
		startTime = *cfg.StartTime
	}

	// Create root span
	spanCtx, span := l.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(startTime),
	)

	// Set trace-level attributes
	l.setTraceAttributes(span, cfg)

	// Create end function
	endFunc := func() {
		// Set output if provided
		if cfg.Metadata != nil {
			if output, ok := cfg.Metadata["output"]; ok {
				outputJSON, _ := json.Marshal(output)
				span.SetAttributes(attribute.String("langfuse.trace.output", string(outputJSON)))
			}
		}
		span.End()
	}

	return spanCtx, endFunc
}

// StartSpan creates a new span within the current trace
func (l *LangfuseTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func()) {
	cfg := &SpanConfig{
		Type:  SpanTypeSpan,
		Level: LogLevelDefault,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	spanCtx, span := l.tracer.Start(ctx, name,
		trace.WithTimestamp(time.Now()),
	)

	// Set observation type
	span.SetAttributes(attribute.String("langfuse.observation.type", string(cfg.Type)))

	// Set level
	span.SetAttributes(attribute.String("langfuse.observation.level", string(cfg.Level)))

	// Set input
	if cfg.Input != nil {
		inputJSON, _ := json.Marshal(cfg.Input)
		span.SetAttributes(attribute.String("langfuse.observation.input", string(inputJSON)))
	}

	// Set metadata
	if cfg.Metadata != nil {
		for k, v := range cfg.Metadata {
			valueJSON, _ := json.Marshal(v)
			span.SetAttributes(attribute.String(fmt.Sprintf("langfuse.observation.metadata.%s", k), string(valueJSON)))
		}
	}

	endFunc := func() {
		span.End()
	}

	return spanCtx, endFunc
}

// LogGeneration records an LLM generation
func (l *LangfuseTracer) LogGeneration(ctx context.Context, opts GenerationOptions) error {
	// Create a span for the generation
	_, span := l.tracer.Start(ctx, opts.Name,
		trace.WithTimestamp(opts.StartTime),
	)
	defer span.End(trace.WithTimestamp(opts.EndTime))

	// Set observation type as generation
	span.SetAttributes(attribute.String("langfuse.observation.type", string(SpanTypeGeneration)))

	// Set model information using Langfuse-specific attribute
	// Per docs: langfuse.observation.model.name takes precedence
	if opts.Model != "" {
		span.SetAttributes(attribute.String("langfuse.observation.model.name", opts.Model))
		// Also set OpenTelemetry GenAI semantic convention
		span.SetAttributes(attribute.String("gen_ai.request.model", opts.Model))
	}

	// Set model parameters
	// Per docs: langfuse.observation.model.parameters should be a JSON string
	if opts.ModelParameters != nil {
		paramsJSON, _ := json.Marshal(opts.ModelParameters)
		span.SetAttributes(attribute.String("langfuse.observation.model.parameters", string(paramsJSON)))
	}

	// Set input - Langfuse expects a JSON string
	// Per docs: langfuse.observation.input and gen_ai.prompt are both supported
	if opts.Input != nil {
		inputJSON, _ := json.Marshal(opts.Input)
		span.SetAttributes(attribute.String("langfuse.observation.input", string(inputJSON)))
		span.SetAttributes(attribute.String("gen_ai.prompt", string(inputJSON)))
	}

	// Set output - Langfuse expects a JSON string
	// Per docs: langfuse.observation.output and gen_ai.completion are both supported
	if opts.Output != nil {
		outputJSON, _ := json.Marshal(opts.Output)
		span.SetAttributes(attribute.String("langfuse.observation.output", string(outputJSON)))
		span.SetAttributes(attribute.String("gen_ai.completion", string(outputJSON)))
	}

	// Set usage information
	// Per docs: both langfuse.observation.usage_details (JSON string) and gen_ai.usage.* (integers) are supported
	if opts.Usage != nil {
		// Set individual OpenTelemetry GenAI attributes (total_tokens is critical for Langfuse)
		attrs := []attribute.KeyValue{
			attribute.Int("gen_ai.usage.input_tokens", opts.Usage.PromptTokens),
			attribute.Int("gen_ai.usage.output_tokens", opts.Usage.CompletionTokens),
			attribute.Int("gen_ai.usage.total_tokens", opts.Usage.TotalTokens),
		}
		// Add reasoning tokens if present (for reasoning models like o1, o3)
		if opts.Usage.ReasoningTokens > 0 {
			attrs = append(attrs, attribute.Int("gen_ai.usage.reasoning_tokens", opts.Usage.ReasoningTokens))
		}
		span.SetAttributes(attrs...)
		
		// Set Langfuse usage_details as JSON with correct keys: input, output, total
		usageDetails := map[string]int{
			"input":  opts.Usage.PromptTokens,
			"output": opts.Usage.CompletionTokens,
			"total":  opts.Usage.TotalTokens,
		}
		if opts.Usage.ReasoningTokens > 0 {
			usageDetails["reasoning"] = opts.Usage.ReasoningTokens
		}
		usageJSON, _ := json.Marshal(usageDetails)
		span.SetAttributes(attribute.String("langfuse.observation.usage_details", string(usageJSON)))
	}

	// Set cost information
	if opts.Cost != nil {
		costDetails := map[string]float64{
			"input":  opts.Cost.PromptCost,
			"output": opts.Cost.CompletionCost,
			"total":  opts.Cost.TotalCost,
		}
		costJSON, _ := json.Marshal(costDetails)
		span.SetAttributes(attribute.String("langfuse.observation.cost_details", string(costJSON)))
		span.SetAttributes(attribute.Float64("gen_ai.usage.cost", opts.Cost.TotalCost))
	}

	// Set completion start time if provided
	if opts.CompletionStartTime != nil {
		span.SetAttributes(attribute.String("langfuse.observation.completion_start_time", opts.CompletionStartTime.Format(time.RFC3339)))
	}

	// Set prompt link if provided
	if opts.PromptName != "" {
		span.SetAttributes(attribute.String("langfuse.observation.prompt.name", opts.PromptName))
		if opts.PromptVersion > 0 {
			span.SetAttributes(attribute.Int("langfuse.observation.prompt.version", opts.PromptVersion))
		}
	}

	// Set metadata
	if opts.Metadata != nil {
		for k, v := range opts.Metadata {
			valueJSON, _ := json.Marshal(v)
			span.SetAttributes(attribute.String(fmt.Sprintf("langfuse.observation.metadata.%s", k), string(valueJSON)))
		}
	}

	// Set level
	span.SetAttributes(attribute.String("langfuse.observation.level", string(opts.Level)))

	// Set status message if error
	if opts.StatusMessage != "" {
		span.SetAttributes(attribute.String("langfuse.observation.status_message", opts.StatusMessage))
		if opts.Level == LogLevelError {
			span.SetStatus(codes.Error, opts.StatusMessage)
		}
	}

	return nil
}

// LogEvent records a simple event
func (l *LangfuseTracer) LogEvent(ctx context.Context, name string, attributes map[string]any) error {
	_, span := l.tracer.Start(ctx, name,
		trace.WithTimestamp(time.Now()),
	)
	defer span.End(trace.WithTimestamp(time.Now()))

	// Set observation type as event
	span.SetAttributes(attribute.String("langfuse.observation.type", string(SpanTypeEvent)))

	// Set attributes
	if attributes != nil {
		for k, v := range attributes {
			valueJSON, _ := json.Marshal(v)
			span.SetAttributes(attribute.String(fmt.Sprintf("langfuse.observation.metadata.%s", k), string(valueJSON)))
		}
	}

	return nil
}

// SetTraceAttributes sets attributes on the current trace
func (l *LangfuseTracer) SetTraceAttributes(ctx context.Context, attributes map[string]any) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	for k, v := range attributes {
		valueJSON, _ := json.Marshal(v)
		span.SetAttributes(attribute.String(fmt.Sprintf("langfuse.trace.metadata.%s", k), string(valueJSON)))
	}

	return nil
}

// SetSpanOutput sets the output on the current span (observation)
func (l *LangfuseTracer) SetSpanOutput(ctx context.Context, output any) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	if output != nil {
		outputJSON, _ := json.Marshal(output)
		span.SetAttributes(attribute.String("langfuse.observation.output", string(outputJSON)))
	}

	return nil
}

// SetSpanAttributes sets attributes on the current span as observation metadata
func (l *LangfuseTracer) SetSpanAttributes(ctx context.Context, attributes map[string]any) error {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	for k, v := range attributes {
		valueJSON, _ := json.Marshal(v)
		span.SetAttributes(attribute.String(fmt.Sprintf("langfuse.observation.metadata.%s", k), string(valueJSON)))
	}

	return nil
}

// Flush ensures all pending traces are sent
func (l *LangfuseTracer) Flush(ctx context.Context) error {
	return l.tracerProvider.ForceFlush(ctx)
}

// Shutdown gracefully shuts down the tracer
func (l *LangfuseTracer) Shutdown(ctx context.Context) error {
	return l.tracerProvider.Shutdown(ctx)
}

// setTraceAttributes sets trace-level attributes from config
func (l *LangfuseTracer) setTraceAttributes(span trace.Span, cfg *TraceConfig) {
	if cfg.UserID != "" {
		span.SetAttributes(attribute.String("langfuse.user.id", cfg.UserID))
		span.SetAttributes(attribute.String("user.id", cfg.UserID))
	}

	if cfg.SessionID != "" {
		span.SetAttributes(attribute.String("langfuse.session.id", cfg.SessionID))
		span.SetAttributes(attribute.String("session.id", cfg.SessionID))
	}

	if len(cfg.Tags) > 0 {
		tagsJSON, _ := json.Marshal(cfg.Tags)
		span.SetAttributes(attribute.String("langfuse.trace.tags", string(tagsJSON)))
	}

	if cfg.Version != "" {
		span.SetAttributes(attribute.String("langfuse.version", cfg.Version))
	}

	if cfg.Environment != "" {
		span.SetAttributes(attribute.String("langfuse.environment", cfg.Environment))
	}

	if cfg.Release != "" {
		span.SetAttributes(attribute.String("langfuse.release", cfg.Release))
	}

	if cfg.Input != nil {
		inputJSON, _ := json.Marshal(cfg.Input)
		span.SetAttributes(attribute.String("langfuse.trace.input", string(inputJSON)))
	}

	if cfg.Metadata != nil {
		for k, v := range cfg.Metadata {
			// Skip output as it's set at the end
			if k == "output" {
				continue
			}
			valueJSON, _ := json.Marshal(v)
			span.SetAttributes(attribute.String(fmt.Sprintf("langfuse.trace.metadata.%s", k), string(valueJSON)))
		}
	}
}
