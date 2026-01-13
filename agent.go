// Package agentkit provides a flexible framework for building LLM-powered agents with tool calling.
package agentkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/darkostanimirovic/agentkit/internal/conversation"
	"github.com/darkostanimirovic/agentkit/internal/logging"
	"github.com/darkostanimirovic/agentkit/internal/parallel"
	"github.com/darkostanimirovic/agentkit/internal/retry"
	"github.com/darkostanimirovic/agentkit/internal/timeout"
	"github.com/darkostanimirovic/agentkit/middleware"
	"github.com/darkostanimirovic/agentkit/providers"
	"github.com/darkostanimirovic/agentkit/providers/openai"
)

// Type aliases for internal package types
type (
	ConversationStore = conversation.ConversationStore
	Conversation      = conversation.Conversation
	ConversationTurn  = conversation.ConversationTurn
	RetryConfig       = retry.RetryConfig
	TimeoutConfig     = timeout.TimeoutConfig
	LoggingConfig     = logging.LoggingConfig
	ParallelConfig    = parallel.ParallelConfig
	Middleware        = middleware.Middleware
)

// Function re-exports for convenience
var (
	NewMemoryConversationStore = conversation.NewMemoryConversationStore
	DefaultRetryConfig         = retry.DefaultRetryConfig
	DefaultTimeoutConfig       = timeout.DefaultTimeoutConfig
	DefaultLoggingConfig       = logging.DefaultLoggingConfig
	DefaultParallelConfig      = parallel.DefaultParallelConfig
	ErrConversationNotFound    = conversation.ErrConversationNotFound
)

const defaultEventBuffer = 10

// SystemPromptFunc builds the system prompt from context.
type SystemPromptFunc func(ctx context.Context) string

// Agent orchestrates LLM interactions with tool calling and streaming.
type Agent struct {
	provider          providers.Provider
	model             string
	systemPrompt      SystemPromptFunc
	tools             map[string]Tool
	maxIterations     int
	temperature       float32
	reasoningEffort   providers.ReasoningEffort
	reasoningSummary  string
	textVerbosity     string
	textFormat        string
	store             bool
	streamResponses   bool
	toolChoice        string
	retryConfig       RetryConfig
	timeoutConfig     TimeoutConfig
	conversationStore ConversationStore
	approvalConfig    ApprovalConfig
	loggingConfig     LoggingConfig
	logger            *slog.Logger
	middlewares       []Middleware
	eventBuffer       int
	parallelConfig    ParallelConfig
	tracer            Tracer
	agentName         string
}

// Config holds agent configuration.
type Config struct {
	APIKey                string
	Model                 string
	SystemPrompt          SystemPromptFunc
	MaxIterations         int
	Temperature           float32
	ReasoningEffort       providers.ReasoningEffort
	ReasoningSummary      string
	TextVerbosity         string
	TextFormat            string
	Store                 bool
	StreamResponses       bool
	ToolChoice           string
	Retry                 *RetryConfig
	Timeout               *TimeoutConfig
	ConversationStore     ConversationStore
	Approval              *ApprovalConfig
	Provider              providers.Provider
	LLMProvider           LLMProvider // DEPRECATED: Use Provider instead
	Logging               *LoggingConfig
	EventBuffer           int
	ParallelToolExecution *ParallelConfig
	Tracer                Tracer
	AgentName             string
}

// Common validation errors.
var (
	ErrMissingAPIKey          = errors.New("agentkit: APIKey is required")
	ErrInvalidIterations      = errors.New("agentkit: MaxIterations must be between 1 and 100")
	ErrInvalidTemperature     = errors.New("agentkit: Temperature must be between 0.0 and 2.0")
	ErrInvalidReasoningEffort = errors.New("agentkit: ReasoningEffort must be valid")
)

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if c.APIKey == "" && c.Provider == nil && c.LLMProvider == nil {
		return ErrMissingAPIKey
	}
	if c.MaxIterations < 0 || c.MaxIterations > 100 {
		return ErrInvalidIterations
	}
	if c.Temperature < 0.0 || c.Temperature > 2.0 {
		return ErrInvalidTemperature
	}
	if c.ReasoningEffort != "" {
		if c.ReasoningEffort != providers.ReasoningEffortNone &&
			c.ReasoningEffort != providers.ReasoningEffortMinimal &&
			c.ReasoningEffort != providers.ReasoningEffortLow &&
			c.ReasoningEffort != providers.ReasoningEffortMedium &&
			c.ReasoningEffort != providers.ReasoningEffortHigh &&
			c.ReasoningEffort != providers.ReasoningEffortXHigh {
			return ErrInvalidReasoningEffort
		}
	}
	return nil
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Model:           "gpt-4o-mini",
		MaxIterations:   5,
		Temperature:     0.7,
		StreamResponses: true,
	}
}

// New creates a new agent with the given configuration.
func New(cfg Config) (*Agent, error) {
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 5
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid agent config: %w", err)
	}

	loggingConfig := DefaultLoggingConfig()
	if cfg.Logging != nil {
		loggingConfig = *cfg.Logging
	}
	logger := logging.ResolveLogger(loggingConfig)

	retryConfig := DefaultRetryConfig()
	if cfg.Retry != nil {
		retryConfig = *cfg.Retry
	}

	timeoutConfig := DefaultTimeoutConfig()
	if cfg.Timeout != nil {
		timeoutConfig = *cfg.Timeout
	}

	approvalConfig := ApprovalConfig{}
	if cfg.Approval != nil {
		approvalConfig = *cfg.Approval
	}

	parallelConfig := DefaultParallelConfig()
	if cfg.ParallelToolExecution != nil {
		parallelConfig = *cfg.ParallelToolExecution
	}
	if parallelConfig.MaxConcurrent <= 0 {
		parallelConfig.MaxConcurrent = 1
	}

	provider := cfg.Provider
	if provider == nil {
		if cfg.LLMProvider != nil {
			// Wrap legacy LLMProvider into Provider interface
			provider = &llmProviderWrapper{llm: cfg.LLMProvider}
		} else {
			provider = openai.New(cfg.APIKey, logger)
		}
	}

	agentName := cfg.AgentName
	if agentName == "" {
		agentName = cfg.Model
	}

	eventBuffer := cfg.EventBuffer
	if eventBuffer <= 0 {
		eventBuffer = defaultEventBuffer
	}

	tracer := cfg.Tracer
	if tracer == nil {
		tracer = &NoOpTracer{}
	}

	return &Agent{
		provider:          provider,
		model:             cfg.Model,
		systemPrompt:      cfg.SystemPrompt,
		tools:             make(map[string]Tool),
		maxIterations:     cfg.MaxIterations,
		temperature:       cfg.Temperature,
		reasoningEffort:   cfg.ReasoningEffort,
		reasoningSummary:  cfg.ReasoningSummary,
		textVerbosity:     cfg.TextVerbosity,
		textFormat:        cfg.TextFormat,
		store:             cfg.Store,
		streamResponses:   cfg.StreamResponses,
		toolChoice:        cfg.ToolChoice,
		retryConfig:       retryConfig,
		timeoutConfig:     timeoutConfig,
		conversationStore: cfg.ConversationStore,
		approvalConfig:    approvalConfig,
		loggingConfig:     loggingConfig,
		logger:            logger,
		eventBuffer:       eventBuffer,
		parallelConfig:    parallelConfig,
		tracer:            tracer,
		agentName:         agentName,
	}, nil
}

// AddTool registers a tool with the agent.
func (a *Agent) AddTool(tool Tool) {
	a.tools[tool.Name()] = tool
}

// AsTool converts the agent into a tool that can be used by other agents.
func (a *Agent) AsTool(name, description string) Tool {
	return NewTool(name).
		WithDescription(description).
		WithParameter("input", String().Required().WithDescription("Task input")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			input, ok := args["input"].(string)
			if !ok {
				return nil, fmt.Errorf("input required")
			}

			events := a.Run(ctx, input)
			var lastResponse string
			for event := range events {
				if event.Type == EventTypeFinalOutput {
					if response, ok := event.Data["response"].(string); ok {
						lastResponse = response
					}
				}
			}

			if lastResponse == "" {
				return "Agent completed without final output", nil
			}
			return lastResponse, nil
		}).
		Build()
}

// Use registers middleware for agent execution hooks.
func (a *Agent) Use(m Middleware) {
	if m == nil {
		return
	}
	a.middlewares = append(a.middlewares, m)
}

// Middleware application methods
func (a *Agent) applyAgentStart(ctx context.Context, input string) context.Context {
	for _, m := range a.middlewares {
		ctx = m.OnAgentStart(ctx, input)
	}
	return ctx
}

func (a *Agent) applyAgentComplete(ctx context.Context, output string, err error) {
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		a.middlewares[i].OnAgentComplete(ctx, output, err)
	}
}

func (a *Agent) applyToolStart(ctx context.Context, tool string, args any) context.Context {
	for _, m := range a.middlewares {
		ctx = m.OnToolStart(ctx, tool, args)
	}
	return ctx
}

func (a *Agent) applyToolComplete(ctx context.Context, tool string, result any, err error) {
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		a.middlewares[i].OnToolComplete(ctx, tool, result, err)
	}
}

func (a *Agent) applyLLMCall(ctx context.Context, req any) context.Context {
	for _, m := range a.middlewares {
		ctx = m.OnLLMCall(ctx, req)
	}
	return ctx
}

func (a *Agent) applyLLMResponse(ctx context.Context, resp any, err error) {
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		a.middlewares[i].OnLLMResponse(ctx, resp, err)
	}
}

func (a *Agent) emit(ctx context.Context, events chan<- Event, event Event) {
	if traceID, ok := GetTraceID(ctx); ok && traceID != "" {
		event.TraceID = traceID
	}
	if spanID, ok := GetSpanID(ctx); ok && spanID != "" {
		event.SpanID = spanID
	}
	if name, ok := GetAgentName(ctx); ok && name != "" {
		if event.Data == nil {
			event.Data = map[string]any{}
		}
		if _, exists := event.Data["agent_name"]; !exists {
			event.Data["agent_name"] = name
		}
	}
	if iteration, ok := GetIteration(ctx); ok {
		if event.Data == nil {
			event.Data = map[string]any{}
		}
		if _, exists := event.Data["iteration"]; !exists {
			event.Data["iteration"] = iteration
		}
	}
	events <- event
}

// Conversation management methods
func (a *Agent) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	if a.conversationStore == nil {
		return Conversation{}, errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Load(ctx, conversationID)
}

func (a *Agent) SaveConversation(ctx context.Context, conv Conversation) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Save(ctx, conv)
}

func (a *Agent) AppendToConversation(ctx context.Context, conversationID string, turn ConversationTurn) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Append(ctx, conversationID, turn)
}

func (a *Agent) DeleteConversation(ctx context.Context, conversationID string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Delete(ctx, conversationID)
}

func (a *Agent) AddContext(ctx context.Context, conversationID string, content string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	turn := ConversationTurn{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	return a.conversationStore.Append(ctx, conversationID, turn)
}

func (a *Agent) ClearConversation(ctx context.Context, conversationID string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}

	conv, err := a.conversationStore.Load(ctx, conversationID)
	if err != nil {
		return err
	}

	if err := a.conversationStore.Delete(ctx, conversationID); err != nil {
		return err
	}

	conv.Turns = nil
	conv.CreatedAt = time.Now()
	conv.UpdatedAt = time.Now()
	return a.conversationStore.Save(ctx, conv)
}

func (a *Agent) ForkConversation(ctx context.Context, originalID, newID, userMessage string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}

	original, err := a.conversationStore.Load(ctx, originalID)
	if err != nil {
		return err
	}

	forked := Conversation{
		ID:       newID,
		AgentID:  original.AgentID,
		Turns:    make([]ConversationTurn, len(original.Turns)),
		Metadata: make(map[string]any),
	}

	copy(forked.Turns, original.Turns)
	for k, v := range original.Metadata {
		forked.Metadata[k] = v
	}

	forked.Turns = append(forked.Turns, ConversationTurn{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})

	return a.conversationStore.Save(ctx, forked)
}

// Run executes the agent with streaming events.
func (a *Agent) Run(ctx context.Context, userMessage string) <-chan Event {
	events := make(chan Event, a.eventBuffer)
	startTime := time.Now()

	go func() {
		traceCtx, endTrace := a.tracer.StartTrace(ctx, "agent.run",
			WithTraceInput(userMessage),
			WithTraceStartTime(startTime),
		)
		defer endTrace()
		ctx = traceCtx

		ctx = WithTracer(ctx, a.tracer)
		ctx = WithAgentName(ctx, a.agentName)

		parentPub, hasParent := GetEventPublisher(ctx)
		var runLoopChan chan<- Event
		var internalChan chan Event
		var wg sync.WaitGroup

		if hasParent {
			internalChan = make(chan Event, a.eventBuffer)
			runLoopChan = internalChan
			wg.Add(1)
			go func() {
				defer wg.Done()
				for e := range internalChan {
					if e.Type != EventTypeError {
						parentPub(e)
					}
					events <- e
				}
			}()
		} else {
			runLoopChan = events
		}

		childPub := func(e Event) {
			runLoopChan <- e
		}
		execCtx := WithEventPublisher(ctx, childPub)

		execCtx, cancel := a.withExecutionTimeout(execCtx)
		if cancel != nil {
			defer cancel()
		}

		execCtx = a.applyAgentStart(execCtx, userMessage)

		agentName := a.agentName
		a.emit(execCtx, runLoopChan, AgentStart(agentName))

		finalOutput, usage, iterations, runErr := a.runLoop(execCtx, userMessage, runLoopChan)
		a.applyAgentComplete(execCtx, finalOutput, runErr)

		// Always emit final output event (even if empty)
		// Empty output is still a valid completion state that clients need to know about
		a.emit(execCtx, runLoopChan, FinalOutput("", finalOutput))

		duration := time.Since(startTime).Milliseconds()
		a.emit(execCtx, runLoopChan, AgentCompleteWithUsage(agentName, finalOutput, usage, iterations, duration))

		if hasParent {
			close(internalChan)
			wg.Wait()
		}
		close(events)
	}()

	return events
}

// runLoop orchestrates the multi-turn conversation.
func (a *Agent) runLoop(ctx context.Context, userMessage string, events chan<- Event) (string, providers.TokenUsage, int, error) {
	conversationHistory := []providers.Message{
		{
			Role:    providers.RoleUser,
			Content: userMessage,
		},
	}

	var finalOutput string
	var totalUsage providers.TokenUsage
	iterationsUsed := 0

	for iteration := 0; iteration < a.maxIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			runErr := fmt.Errorf("agent execution timeout: %w", err)
			a.emit(ctx, events, Error(runErr))
			return finalOutput, totalUsage, iterationsUsed, runErr
		}

		a.logger.Debug("agent iteration", "iteration", iteration, "max", a.maxIterations)

		iterCtx := WithIteration(ctx, iteration+1)
		req := a.buildCompletionRequest(conversationHistory)

		var resp *providers.CompletionResponse
		var err error

		if a.streamResponses {
			resp, err = a.runStreamingIteration(iterCtx, req, events)
		} else {
			resp, err = a.runNonStreamingIteration(iterCtx, req, events)
		}

		if err != nil {
			return finalOutput, totalUsage, iterationsUsed, err
		}

		resp.ToolCalls = ensureToolCallIDs(filterCompleteToolCalls(resp.ToolCalls))
		iterationsUsed = iteration + 1

		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens
		totalUsage.ReasoningTokens += resp.Usage.ReasoningTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		assistantMsg := providers.Message{
			Role:      providers.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		conversationHistory = append(conversationHistory, assistantMsg)

		if len(resp.ToolCalls) == 0 {
			finalOutput = resp.Content
			a.logger.Info("agent completed", "iterations", iteration+1, "output_length", len(finalOutput))
			break
		}

		toolMessages := a.executeToolCalls(iterCtx, resp.ToolCalls, events)
		conversationHistory = append(conversationHistory, toolMessages...)

		a.logger.Debug("continuing iteration", "tool_calls_executed", len(toolMessages))
	}

	if finalOutput == "" {
		return "", totalUsage, iterationsUsed, fmt.Errorf("max iterations reached without completion")
	}

	return finalOutput, totalUsage, iterationsUsed, nil
}

// Helper methods
func (a *Agent) buildSystemPrompt(ctx context.Context) string {
	if a.systemPrompt == nil {
		return ""
	}
	return a.systemPrompt(ctx)
}

func (a *Agent) withExecutionTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeoutConfig.AgentExecution <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, a.timeoutConfig.AgentExecution)
}

func (a *Agent) withLLMTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeoutConfig.LLMCall <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, a.timeoutConfig.LLMCall)
}

func (a *Agent) withToolTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeoutConfig.ToolExecution <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, a.timeoutConfig.ToolExecution)
}

func (a *Agent) handleIterationError(ctx context.Context, events chan<- Event, err error, msg string, keyvals ...any) error {
	a.logger.Error(msg, append(keyvals, "error", err)...)
	a.emit(ctx, events, Error(err))
	return err
}

// Helper methods for tracing integration

// llmCallTiming holds timing information for an LLM call
type llmCallTiming struct {
	startTime           time.Time
	endTime             time.Time
	completionStartTime *time.Time
}

// llmCallTimingContextKey is a custom type for context keys to avoid collisions
type llmCallTimingContextKey string

const llmCallTimingKey llmCallTimingContextKey = "agentkit.llmCallTiming"

// startLLMCallTiming records the start time of an LLM call in the context
func startLLMCallTiming(ctx context.Context) context.Context {
	timing := &llmCallTiming{
		startTime: time.Now(),
	}
	return context.WithValue(ctx, llmCallTimingKey, timing)
}

// getLLMCallTiming retrieves timing information from context
func getLLMCallTiming(ctx context.Context) *llmCallTiming {
	if timing, ok := ctx.Value(llmCallTimingKey).(*llmCallTiming); ok {
		return timing
	}
	return nil
}

// extractLLMCallTiming extracts timing information from context, returning a non-nil value
func extractLLMCallTiming(ctx context.Context) llmCallTiming {
	if timing := getLLMCallTiming(ctx); timing != nil {
		return *timing
	}
	// Return empty timing if not found
	now := time.Now()
	return llmCallTiming{
		startTime: now,
		endTime:   now,
	}
}

func (a *Agent) logLLMGeneration(ctx context.Context, req providers.CompletionRequest, resp *providers.CompletionResponse, err error) {
	tracer := GetTracer(ctx)
	if tracer == nil || isNoOpTracer(tracer) {
		return
	}

	timing := extractLLMCallTiming(ctx)
	if timing.endTime.IsZero() || timing.endTime.Equal(timing.startTime) {
		timing.endTime = time.Now()
	}

	modelParams := map[string]any{
		"temperature":         req.Temperature,
		"top_p":               req.TopP,
		"max_tokens":          req.MaxTokens,
		"tool_choice":         req.ToolChoice,
		"parallel_tool_calls": req.ParallelToolCalls,
		"reasoning_effort":    req.ReasoningEffort,
		"reasoning_summary":   req.ReasoningSummary,
		"text_verbosity":      req.TextVerbosity,
		"text_format":         req.TextFormat,
		"store":               req.Store,
	}

	input := map[string]any{
		"system_prompt": req.SystemPrompt,
		"messages":      req.Messages,
		"tools":         req.Tools,
	}

	var output any
	var usage *UsageInfo
	if resp != nil {
		output = map[string]any{
			"content":           resp.Content,
			"reasoning_summary": resp.ReasoningSummary,
			"tool_calls":        resp.ToolCalls,
			"finish_reason":     resp.FinishReason,
		}
		usage = &UsageInfo{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			ReasoningTokens:  resp.Usage.ReasoningTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	} else if err != nil {
		output = map[string]any{
			"error": err.Error(),
		}
	}

	gen := GenerationOptions{
		Name:                "llm.generate",
		Model:               req.Model,
		ModelParameters:     modelParams,
		Input:               input,
		Output:              output,
		Usage:               usage,
		StartTime:           timing.startTime,
		EndTime:             timing.endTime,
		CompletionStartTime: timing.completionStartTime,
		Metadata: map[string]any{
			"tool_definitions": req.Tools,
			"tool_calls": func() []providers.ToolCall {
				if resp != nil {
					return resp.ToolCalls
				}
				return nil
			}(),
		},
		Level: LogLevelDefault,
	}
	if err != nil {
		gen.Level = LogLevelError
		gen.StatusMessage = err.Error()
	}

	_ = tracer.LogGeneration(ctx, gen)
}

// ApprovalHandler is called when a tool requires approval before execution
// Returns true to approve, false to deny
type ApprovalHandler func(ctx context.Context, request ApprovalRequest) (bool, error)

// ApprovalRequest contains information about a tool call that requires approval
type ApprovalRequest struct {
	ToolName       string         `json:"tool_name"`
	Arguments      map[string]any `json:"arguments"`
	Description    string         `json:"description"`     // Human-friendly description
	ConversationID string         `json:"conversation_id"` // If available
	CallID         string         `json:"call_id"`         // Unique call identifier
}

// ApprovalConfig configures which tools require approval
type ApprovalConfig struct {
	// Tools is a list of tool names that require approval
	// If empty, no tools require approval
	Tools []string

	// Handler is called for approval requests
	// If nil, all tools in Tools list will be automatically denied
	Handler ApprovalHandler

	// AllTools, if true, requires approval for ALL tool calls
	AllTools bool
}

// requiresApproval checks if a tool name requires approval
func (c ApprovalConfig) requiresApproval(toolName string) bool {
	if c.AllTools {
		return true
	}

	for _, t := range c.Tools {
		if t == toolName {
			return true
		}
	}

	return false
}

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
	agentNameKey      contextKey = "agentkit_agent_name"
	iterationKey      contextKey = "agentkit_iteration"
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

// WithAgentName adds the agent name to the context.
func WithAgentName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, agentNameKey, name)
}

// GetAgentName retrieves the agent name from the context.
func GetAgentName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(agentNameKey).(string)
	return name, ok
}

// WithIteration adds the iteration index to the context.
func WithIteration(ctx context.Context, iteration int) context.Context {
	if iteration <= 0 {
		return ctx
	}
	return context.WithValue(ctx, iterationKey, iteration)
}

// GetIteration retrieves the iteration index from the context.
func GetIteration(ctx context.Context) (int, bool) {
	val, ok := ctx.Value(iterationKey).(int)
	return val, ok
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

// buildCompletionRequest creates a provider-agnostic completion request from current conversation state.
func (a *Agent) buildCompletionRequest(conversationHistory []providers.Message) providers.CompletionRequest {
	// Build tool definitions
	tools := make([]providers.ToolDefinition, 0, len(a.tools))
	if len(a.tools) > 0 {
		names := make([]string, 0, len(a.tools))
		for name := range a.tools {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			tool := a.tools[name]
			tools = append(tools, tool.ToToolDefinition())
		}
	}

	toolChoice := a.toolChoice
	if toolChoice == "" {
		toolChoice = "auto"
	}

	req := providers.CompletionRequest{
		Model:             a.model,
		SystemPrompt:      a.buildSystemPrompt(context.Background()),
		Messages:          conversationHistory,
		Tools:             tools,
		Temperature:       a.temperature,
		MaxTokens:         0, // Let provider use default
		TopP:              0, // Let provider use default
		ToolChoice:        toolChoice,
		ParallelToolCalls: true,
		ReasoningEffort:   a.reasoningEffort,
		ReasoningSummary:  a.reasoningSummary,
		TextVerbosity:     a.textVerbosity,
		TextFormat:        a.textFormat,
		Store:             a.store,
	}

	return req
}

// runNonStreamingIteration executes a single non-streaming iteration.
func (a *Agent) runNonStreamingIteration(ctx context.Context, req providers.CompletionRequest, events chan<- Event) (*providers.CompletionResponse, error) {
	callCtx := a.applyLLMCall(ctx, req)
	callCtx, cancel := a.withLLMTimeout(callCtx)
	if cancel != nil {
		defer cancel()
	}

	// Start timing for tracing
	callCtx = startLLMCallTiming(callCtx)

	resp, err := a.provider.Complete(callCtx, req)
	if err != nil {
		iterationErr := fmt.Errorf("provider completion error: %w", err)
		a.applyLLMResponse(callCtx, nil, iterationErr)
		a.logLLMGeneration(callCtx, req, nil, iterationErr)
		return nil, a.handleIterationError(callCtx, events, iterationErr, "completion failed", "model", a.model)
	}

	a.applyLLMResponse(callCtx, resp, nil)
	a.logLLMGeneration(callCtx, req, resp, nil)

	if a.loggingConfig.LogResponses {
		a.logger.Info("completion received",
			"content_length", len(resp.Content),
			"tool_calls", len(resp.ToolCalls),
			"finish_reason", resp.FinishReason)
	}

	return resp, nil
}

// runStreamingIteration executes a single streaming iteration.
func (a *Agent) runStreamingIteration(ctx context.Context, req providers.CompletionRequest, events chan<- Event) (*providers.CompletionResponse, error) {
	callCtx := a.applyLLMCall(ctx, req)
	callCtx, cancel := a.withLLMTimeout(callCtx)
	if cancel != nil {
		defer cancel()
	}

	// Start timing for tracing
	callCtx = startLLMCallTiming(callCtx)

	stream, err := a.provider.Stream(callCtx, req)
	if err != nil {
		iterationErr := fmt.Errorf("provider stream error: %w", err)
		a.applyLLMResponse(callCtx, nil, iterationErr)
		return nil, a.handleIterationError(callCtx, events, iterationErr, "streaming failed", "model", a.model)
	}
	defer stream.Close()

	// Accumulate streaming response
	var content string
	var reasoningSummary string
	var toolCalls []providers.ToolCall
	var usage *providers.TokenUsage
	var finishReason providers.FinishReason

	// Track tool calls being built
	activeToolCalls := make(map[string]*providers.ToolCall)
	toolArgsRaw := make(map[string]string)

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "io: EOF" {
				break
			}
			return nil, fmt.Errorf("stream read error: %w", err)
		}

		if timing := getLLMCallTiming(callCtx); timing != nil && timing.completionStartTime == nil {
			if chunk.Content != "" || chunk.ReasoningSummary != "" || chunk.ToolCallID != "" || chunk.ToolArgs != "" {
				start := time.Now()
				timing.completionStartTime = &start
			}
		}

		// Emit thinking chunks
		if chunk.Content != "" {
			content += chunk.Content
			a.emit(ctx, events, ResponseChunk(chunk.Content))
		}

		if chunk.ReasoningSummary != "" {
			reasoningSummary += chunk.ReasoningSummary
			a.emit(ctx, events, ReasoningChunk(chunk.ReasoningSummary))
		}

		// Handle tool call chunks
		if chunk.ToolCallID != "" {
			if activeToolCalls[chunk.ToolCallID] == nil {
				activeToolCalls[chunk.ToolCallID] = &providers.ToolCall{
					ID:        chunk.ToolCallID,
					Arguments: make(map[string]any),
				}
			}
			tc := activeToolCalls[chunk.ToolCallID]
			if chunk.ToolName != "" {
				tc.Name = chunk.ToolName
			}
			if chunk.ToolArgs != "" {
				toolArgsRaw[chunk.ToolCallID] = chunk.ToolArgs
				var args map[string]any
				if err := json.Unmarshal([]byte(chunk.ToolArgs), &args); err == nil {
					tc.Arguments = args
				}
			}
		}

		// Handle completion
		if chunk.IsComplete {
			finishReason = chunk.FinishReason
			if chunk.Usage != nil {
				usage = chunk.Usage
			}

			// Collect completed tool calls
			for _, tc := range activeToolCalls {
				if len(tc.Arguments) == 0 {
					if raw, ok := toolArgsRaw[tc.ID]; ok {
						var args map[string]any
						if err := json.Unmarshal([]byte(raw), &args); err == nil {
							tc.Arguments = args
						}
					}
				}
				if tc.Name == "" {
					continue
				}
				if tc.Arguments == nil {
					tc.Arguments = map[string]any{}
				}
				toolCalls = append(toolCalls, *tc)
			}
			break
		}
	}

	resp := &providers.CompletionResponse{
		ID:               fmt.Sprintf("stream-%d", len(content)), // Generate ID
		Content:          content,
		ToolCalls:        ensureToolCallIDs(toolCalls),
		FinishReason:     finishReason,
		Model:            a.model,
		ReasoningSummary: reasoningSummary,
	}
	if usage != nil {
		resp.Usage = *usage
	}

	a.applyLLMResponse(callCtx, resp, nil)
	a.logLLMGeneration(callCtx, req, resp, nil)

	return resp, nil
}

// executeToolCalls executes all tool calls and returns messages for the conversation history.
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []providers.ToolCall, events chan<- Event) []providers.Message {
	if len(toolCalls) == 0 {
		return nil
	}

	messages := make([]providers.Message, 0, len(toolCalls))

	if a.parallelConfig.Enabled {
		messages = a.executeToolCallsParallel(ctx, toolCalls, events)
	} else {
		messages = a.executeToolCallsSequential(ctx, toolCalls, events)
	}

	return messages
}

func (a *Agent) executeToolCallsSequential(ctx context.Context, toolCalls []providers.ToolCall, events chan<- Event) []providers.Message {
	messages := make([]providers.Message, 0, len(toolCalls))

	for _, call := range toolCalls {
		msg := a.executeToolCall(ctx, call, events)
		messages = append(messages, msg)
	}

	return messages
}

func (a *Agent) executeToolCallsParallel(ctx context.Context, toolCalls []providers.ToolCall, events chan<- Event) []providers.Message {
	type result struct {
		index int
		msg   providers.Message
	}

	resultChan := make(chan result, len(toolCalls))
	sem := make(chan struct{}, a.parallelConfig.MaxConcurrent)

	for i, call := range toolCalls {
		sem <- struct{}{}
		go func(idx int, tc providers.ToolCall) {
			defer func() { <-sem }()
			msg := a.executeToolCall(ctx, tc, events)
			resultChan <- result{index: idx, msg: msg}
		}(i, call)
	}

	// Collect results
	results := make([]result, 0, len(toolCalls))
	for i := 0; i < len(toolCalls); i++ {
		results = append(results, <-resultChan)
	}

	// Sort by original order
	messages := make([]providers.Message, len(toolCalls))
	for _, r := range results {
		messages[r.index] = r.msg
	}

	return messages
}

func filterCompleteToolCalls(toolCalls []providers.ToolCall) []providers.ToolCall {
	if len(toolCalls) == 0 {
		return toolCalls
	}
	filtered := make([]providers.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if tc.Name == "" {
			continue
		}
		if tc.Arguments == nil {
			tc.Arguments = map[string]any{}
		}
		filtered = append(filtered, tc)
	}
	return filtered
}

func ensureToolCallIDs(toolCalls []providers.ToolCall) []providers.ToolCall {
	if len(toolCalls) == 0 {
		return toolCalls
	}
	used := make(map[string]struct{}, len(toolCalls))
	for _, tc := range toolCalls {
		if tc.ID != "" {
			used[tc.ID] = struct{}{}
		}
	}
	next := 1
	for i := range toolCalls {
		if toolCalls[i].ID != "" {
			continue
		}
		for {
			id := fmt.Sprintf("call_%d", next)
			next++
			if _, exists := used[id]; exists {
				continue
			}
			toolCalls[i].ID = id
			used[id] = struct{}{}
			break
		}
	}
	return toolCalls
}

func (a *Agent) executeToolCall(ctx context.Context, toolCall providers.ToolCall, events chan<- Event) providers.Message {
	tool, exists := a.tools[toolCall.Name]

	// Check if tool exists
	if !exists {
		a.logger.Warn("tool not found", "tool", toolCall.Name)
		a.emit(ctx, events, ToolError(toolCall.Name, fmt.Errorf("tool not found")))
		return providers.Message{
			Role:       providers.RoleTool,
			Content:    fmt.Sprintf("Error: Tool '%s' not found", toolCall.Name),
			ToolCallID: toolCall.ID,
		}
	}

	args := toolCall.Arguments
	if args == nil {
		args = map[string]any{}
	}
	a.emit(ctx, events, ActionDetected(tool.FormatPending(args), toolCall.ID))

	// Check approval if required
	if a.approvalConfig.requiresApproval(toolCall.Name) {
		approved, rejectMsg := a.requestToolApproval(ctx, toolCall, tool, events)
		if !approved {
			return *rejectMsg
		}
	}

	// Start tool execution
	toolCtx := a.applyToolStart(ctx, toolCall.Name, toolCall.Arguments)
	toolCtx, cancel := a.withToolTimeout(toolCtx)
	if cancel != nil {
		defer cancel()
	}

	// Execute tool with retry
	var result any
	var err error

	// Marshal arguments to JSON string for tool.Execute
	argsJSON, err := json.Marshal(toolCall.Arguments)
	if err != nil {
		a.logger.Error("failed to marshal tool arguments", "tool", toolCall.Name, "error", err)
		a.emit(ctx, events, ToolError(toolCall.Name, err))
		return providers.Message{
			Role:       providers.RoleTool,
			Content:    fmt.Sprintf("Error marshaling arguments: %v", err),
			ToolCallID: toolCall.ID,
		}
	}

	result, err = retry.WithRetry(toolCtx, a.retryConfig, func() (any, error) {
		return tool.Execute(toolCtx, string(argsJSON))
	})

	// Complete tool execution
	a.applyToolComplete(toolCtx, toolCall.Name, result, err)

	// Format result
	var content string
	if err != nil {
		content = fmt.Sprintf("Error executing tool: %v", err)
		a.logger.Error("tool execution failed", "tool", toolCall.Name, "error", err)
		a.emit(ctx, events, ToolError(toolCall.Name, err))
	} else {
		content = formatToolResult(result)
		a.logger.Info("tool executed successfully", "tool", toolCall.Name)
		a.emit(ctx, events, ActionResult(tool.FormatResult(result), result))
	}

	return providers.Message{
		Role:       providers.RoleTool,
		Content:    content,
		ToolCallID: toolCall.ID,
		Name:       toolCall.Name,
	}
}

func (a *Agent) requestToolApproval(ctx context.Context, toolCall providers.ToolCall, tool Tool, events chan<- Event) (bool, *providers.Message) {
	approvalReq := ApprovalRequest{
		ToolName:    toolCall.Name,
		Arguments:   toolCall.Arguments,
		Description: tool.description,
		CallID:      toolCall.ID,
	}

	// Emit approval request
	a.emit(ctx, events, ApprovalNeeded(approvalReq))

	// Wait for approval
	approved, err := a.evaluateApproval(ctx, toolCall, approvalReq)
	if err != nil {
		msg := providers.Message{
			Role:       providers.RoleTool,
			Content:    fmt.Sprintf("Approval timeout or error: %v", err),
			ToolCallID: toolCall.ID,
		}
		return false, &msg
	}

	if !approved {
		msg := providers.Message{
			Role:       providers.RoleTool,
			Content:    "Tool execution rejected by user",
			ToolCallID: toolCall.ID,
		}
		a.emit(ctx, events, ApprovalRejected(approvalReq))
		return false, &msg
	}

	a.emit(ctx, events, ApprovalGranted(toolCall.Name, toolCall.ID))
	return true, nil
}

func (a *Agent) evaluateApproval(ctx context.Context, toolCall providers.ToolCall, req ApprovalRequest) (bool, error) {
	if a.approvalConfig.Handler != nil {
		return a.approvalConfig.Handler(ctx, req)
	}
	return false, fmt.Errorf("no approval handler configured")
}

func formatToolResult(result any) string {
	if result == nil {
		return "null"
	}

	switch v := result.(type) {
	case string:
		return v
	case error:
		return fmt.Sprintf("Error: %v", v)
	default:
		// Try JSON encoding
		if data, err := json.Marshal(result); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", result)
	}
}
