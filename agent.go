// Package agentkit provides a flexible framework for building LLM-powered agents with tool calling.
package agentkit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/darkostanimirovic/agentkit/providers"
	"github.com/darkostanimirovic/agentkit/providers/openai"
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
	streamResponses   bool
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
}

// Config holds agent configuration.
type Config struct {
	APIKey                string
	Model                 string
	SystemPrompt          SystemPromptFunc
	MaxIterations         int
	Temperature           float32
	ReasoningEffort       providers.ReasoningEffort
	StreamResponses       bool
	Retry                 *RetryConfig
	Timeout               *TimeoutConfig
	ConversationStore     ConversationStore
	Approval              *ApprovalConfig
	Provider              providers.Provider
	Logging               *LoggingConfig
	EventBuffer           int
	ParallelToolExecution *ParallelConfig
	Tracer                Tracer
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
	if c.APIKey == "" && c.Provider == nil {
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
	logger := resolveLogger(loggingConfig)

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
		provider = openai.New(cfg.APIKey, logger)
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
		streamResponses:   cfg.StreamResponses,
		retryConfig:       retryConfig,
		timeoutConfig:     timeoutConfig,
		conversationStore: cfg.ConversationStore,
		approvalConfig:    approvalConfig,
		loggingConfig:     loggingConfig,
		logger:            logger,
		eventBuffer:       eventBuffer,
		parallelConfig:    parallelConfig,
		tracer:            tracer,
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
		
		agentName := "agent"
		a.emit(execCtx, runLoopChan, AgentStart(agentName))
		
		finalOutput, runErr := a.runLoop(execCtx, userMessage, runLoopChan)
		a.applyAgentComplete(execCtx, finalOutput, runErr)
		
		duration := time.Since(startTime).Milliseconds()
		a.emit(execCtx, runLoopChan, AgentComplete(agentName, finalOutput, 0, 0, duration))

		if hasParent {
			close(internalChan)
			wg.Wait()
		}
		close(events)
	}()

	return events
}

// runLoop orchestrates the multi-turn conversation.
func (a *Agent) runLoop(ctx context.Context, userMessage string, events chan<- Event) (string, error) {
	conversationHistory := []providers.Message{
		{
			Role:    providers.RoleUser,
			Content: userMessage,
		},
	}

	var finalOutput string

	for iteration := 0; iteration < a.maxIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			runErr := fmt.Errorf("agent execution timeout: %w", err)
			a.emit(ctx, events, Error(runErr))
			return finalOutput, runErr
		}

		a.logger.Debug("agent iteration", "iteration", iteration, "max", a.maxIterations)

		req := a.buildCompletionRequest(conversationHistory)
		
		var resp *providers.CompletionResponse
		var err error
		
		if a.streamResponses {
			resp, err = a.runStreamingIteration(ctx, req, events)
		} else {
			resp, err = a.runNonStreamingIteration(ctx, req, events)
		}
		
		if err != nil {
			return finalOutput, err
		}

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

		toolMessages := a.executeToolCalls(ctx, resp.ToolCalls, events)
		conversationHistory = append(conversationHistory, toolMessages...)

		a.logger.Debug("continuing iteration", "tool_calls_executed", len(toolMessages))
	}

	if finalOutput == "" {
		return "", fmt.Errorf("max iterations reached without completion")
	}

	return finalOutput, nil
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
