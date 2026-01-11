// Package agentkit provides a flexible framework for building LLM-powered agents with tool calling.
// It offers streaming responses, dependency injection, and a fluent API for tool registration.
package agentkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

const (
	messageType        = "message"
	outputTextType     = "output_text"
	defaultEventBuffer = 10
)

// SystemPromptFunc is a function that builds the system prompt from context
type SystemPromptFunc func(ctx context.Context) string

// Agent orchestrates LLM interactions with tool calling and streaming
type Agent struct {
	responsesClient   LLMProvider
	model             string
	systemPrompt      SystemPromptFunc
	tools             map[string]Tool
	maxIterations     int
	temperature       float32
	reasoningEffort   ReasoningEffort
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

var promptLogMu sync.Mutex

// Config holds agent configuration
type Config struct {
	APIKey                string
	Model                 string
	SystemPrompt          SystemPromptFunc
	MaxIterations         int
	Temperature           float32
	ReasoningEffort       ReasoningEffort   // For reasoning models: ReasoningEffortNone, ReasoningEffortMinimal, ReasoningEffortLow, ReasoningEffortMedium, ReasoningEffortHigh, ReasoningEffortXHigh
	StreamResponses       bool
	Retry                 *RetryConfig      // Optional retry configuration
	Timeout               *TimeoutConfig    // Optional timeout configuration
	ConversationStore     ConversationStore // Optional conversation persistence
	Approval              *ApprovalConfig   // Optional tool approval configuration
	LLMProvider           LLMProvider       // Optional custom LLM provider (for testing or alternative backends)
	Logging               *LoggingConfig    // Optional logging configuration
	EventBuffer           int               // Optional event channel buffer size (0 = default)
	ParallelToolExecution *ParallelConfig   // Optional parallel tool execution configuration
	Tracer                Tracer            // Optional tracer for LLM observability (e.g., Langfuse)
}

// Common errors for config validation
var (
	ErrMissingAPIKey         = errors.New("agentkit: APIKey is required")
	ErrInvalidModel          = errors.New("agentkit: invalid or unsupported model")
	ErrInvalidIterations     = errors.New("agentkit: MaxIterations must be between 1 and 100")
	ErrInvalidTemperature    = errors.New("agentkit: Temperature must be between 0.0 and 2.0")
	ErrInvalidReasoningEffort = errors.New("agentkit: ReasoningEffort must be 'none', 'minimal', 'low', 'medium', 'high', or 'xhigh'")
)

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	if c.APIKey == "" && c.LLMProvider == nil {
		return ErrMissingAPIKey
	}

	// Validate max iterations
	if c.MaxIterations < 0 || c.MaxIterations > 100 {
		return ErrInvalidIterations
	}

	// Validate temperature
	if c.Temperature < 0.0 || c.Temperature > 2.0 {
		return ErrInvalidTemperature
	}

	// Validate reasoning effort if provided
	if c.ReasoningEffort != "" {
		if c.ReasoningEffort != ReasoningEffortNone &&
			c.ReasoningEffort != ReasoningEffortMinimal &&
			c.ReasoningEffort != ReasoningEffortLow &&
			c.ReasoningEffort != ReasoningEffortMedium &&
			c.ReasoningEffort != ReasoningEffortHigh &&
			c.ReasoningEffort != ReasoningEffortXHigh {
			return ErrInvalidReasoningEffort
		}
	}

	return nil
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Model:           "gpt-4o-mini",
		MaxIterations:   5,
		Temperature:     0.7,
		StreamResponses: true,
	}
}

// New creates a new agent with the given configuration.
// Returns an error if the configuration is invalid.
func New(cfg Config) (*Agent, error) {
	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 5
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid agent config: %w", err)
	}

	loggingConfig := DefaultLoggingConfig()
	if cfg.Logging != nil {
		loggingConfig = *cfg.Logging
	}
	logger := resolveLogger(loggingConfig)

	// Default retry config if not provided
	retryConfig := DefaultRetryConfig()
	if cfg.Retry != nil {
		retryConfig = *cfg.Retry
	}

	// Default timeout config if not provided
	timeoutConfig := DefaultTimeoutConfig()
	if cfg.Timeout != nil {
		timeoutConfig = *cfg.Timeout
	}

	// Default approval config (no approvals required)
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

	responsesClient := cfg.LLMProvider
	if responsesClient == nil {
		responsesClient = NewResponsesClient(cfg.APIKey, logger)
	}

	eventBuffer := cfg.EventBuffer
	if eventBuffer <= 0 {
		eventBuffer = defaultEventBuffer
	}

	// Default tracer (NoOp if not provided)
	tracer := cfg.Tracer
	if tracer == nil {
		tracer = &NoOpTracer{}
	}

	return &Agent{
		responsesClient:   responsesClient,
		model:             cfg.Model,
		systemPrompt:      cfg.SystemPrompt,
		tools:             make(map[string]Tool),
		maxIterations:     cfg.MaxIterations,
		temperature:       cfg.Temperature,
		reasoningEffort:   cfg.ReasoningEffort,
		streamResponses:   cfg.StreamResponses,
		retryConfig:       retryConfig,
		timeoutConfig:     timeoutConfig,
		conversationStore: cfg.ConversationStore, // May be nil
		approvalConfig:    approvalConfig,
		loggingConfig:     loggingConfig,
		logger:            logger,
		eventBuffer:       eventBuffer,
		parallelConfig:    parallelConfig,
		tracer:            tracer,
	}, nil
}

// AddTool registers a tool with the agent
func (a *Agent) AddTool(tool Tool) {
	a.tools[tool.Name()] = tool
}

// AsTool converts the agent into a tool that can be used by other agents.
// The tool will automatically bubble up events to the parent agent.
func (a *Agent) AsTool(name, description string) Tool {
	return NewTool(name).
		WithDescription(description).
		WithParameter("input", String().Required().WithDescription("Task input")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			input, ok := args["input"].(string)
			if !ok {
				return nil, fmt.Errorf("input required")
			}

			// Run the agent. The Run method handles event bubbling via context.
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

// GetConversation retrieves a conversation by ID
// Returns ErrConversationNotFound if conversation store is not configured or conversation doesn't exist
func (a *Agent) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	if a.conversationStore == nil {
		return Conversation{}, errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Load(ctx, conversationID)
}

// SaveConversation persists a conversation
// Returns an error if conversation store is not configured
func (a *Agent) SaveConversation(ctx context.Context, conv Conversation) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Save(ctx, conv)
}

// AppendToConversation adds a turn to an existing conversation
// Returns an error if conversation store is not configured or conversation doesn't exist
func (a *Agent) AppendToConversation(ctx context.Context, conversationID string, turn ConversationTurn) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Append(ctx, conversationID, turn)
}

// DeleteConversation removes a conversation
// Returns an error if conversation store is not configured or conversation doesn't exist
func (a *Agent) DeleteConversation(ctx context.Context, conversationID string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}
	return a.conversationStore.Delete(ctx, conversationID)
}

// AddContext manually adds context to a conversation
// This is a convenience method that appends a user turn with the given content
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

// ClearConversation removes all turns from a conversation
// This is done by deleting and recreating the conversation
func (a *Agent) ClearConversation(ctx context.Context, conversationID string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}

	// Load conversation to get metadata
	conv, err := a.conversationStore.Load(ctx, conversationID)
	if err != nil {
		return err
	}

	// Delete old conversation
	if err := a.conversationStore.Delete(ctx, conversationID); err != nil {
		return err
	}

	// Create fresh conversation with same ID and metadata
	conv.Turns = nil
	conv.CreatedAt = time.Now()
	conv.UpdatedAt = time.Now()

	return a.conversationStore.Save(ctx, conv)
}

// ForkConversation creates a new conversation based on an existing one
// The new conversation will have all the turns from the original plus the new message
func (a *Agent) ForkConversation(ctx context.Context, originalID, newID, userMessage string) error {
	if a.conversationStore == nil {
		return errors.New("agentkit: conversation store not configured")
	}

	// Load original conversation
	original, err := a.conversationStore.Load(ctx, originalID)
	if err != nil {
		return err
	}

	// Create new conversation with copied turns
	forked := Conversation{
		ID:       newID,
		AgentID:  original.AgentID,
		Turns:    make([]ConversationTurn, len(original.Turns)),
		Metadata: make(map[string]any),
	}

	// Deep copy turns
	copy(forked.Turns, original.Turns)

	// Copy metadata
	for k, v := range original.Metadata {
		forked.Metadata[k] = v
	}

	// Add new user message
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

	// Capture start time BEFORE launching goroutine to fix timing race condition
	startTime := time.Now()

	go func() {
		// Start trace for this agent run with explicit start time
		traceCtx, endTrace := a.tracer.StartTrace(ctx, "agent.run",
			WithTraceInput(userMessage),
			WithTraceStartTime(startTime),
		)
		defer endTrace()
		ctx = traceCtx

		// Add tracer to context so delegated agents (handoffs/collaboration) can inherit it
		ctx = WithTracer(ctx, a.tracer)

		// Parent publisher handling for event bubbling
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
					parentPub(e)
					events <- e
				}
			}()
		} else {
			runLoopChan = events
		}

		// Setup context for children to bubble up events to this agent.
		// Use runLoopChan so events go to the correct destination (internal or external).
		childPub := func(e Event) {
			runLoopChan <- e
		}
		execCtx := WithEventPublisher(ctx, childPub)

		execCtx, cancel := a.withExecutionTimeout(execCtx)
		if cancel != nil {
			defer cancel()
		}

		execCtx = a.applyAgentStart(execCtx, userMessage)
		
		// Emit agent.start event
		agentName := "agent" // Default agent name
		a.emit(execCtx, runLoopChan, AgentStart(agentName))
		
		finalOutput, runErr := a.runLoop(execCtx, userMessage, runLoopChan)
		a.applyAgentComplete(execCtx, finalOutput, runErr)
		
		// Emit agent.complete event with metrics
		duration := time.Since(startTime).Milliseconds()
		// Note: totalTokens and iterations would ideally come from cost tracking middleware
		// For now, setting to 0 as baseline - can be enhanced via middleware
		a.emit(execCtx, runLoopChan, AgentComplete(agentName, finalOutput, 0, 0, duration))

		if hasParent {
			close(internalChan) // Stop the pump
			wg.Wait()           // Wait for pump to finish
		}
		close(events)
	}()

	return events
}

func (a *Agent) withExecutionTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeoutConfig.AgentExecution <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, a.timeoutConfig.AgentExecution)
}

func (a *Agent) runLoop(ctx context.Context, userMessage string, events chan<- Event) (string, error) {
	systemPrompt := a.buildSystemPrompt(ctx)
	responseTools := a.buildResponseTools()
	initialInput := buildInitialInput(userMessage)

	var previousResponseID string
	var toolInputs []ResponseContentItem
	var finalOutput string

	for iteration := 0; iteration < a.maxIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			runErr := fmt.Errorf("agent execution timeout: %w", err)
			a.emit(ctx, events, Error(runErr))
			return finalOutput, runErr
		}

		a.logger.Debug("agent iteration", "iteration", iteration, "max", a.maxIterations)

		req := a.buildResponseRequest(systemPrompt, responseTools, previousResponseID, initialInput, toolInputs)
		a.logPromptIteration(iteration+1, req)

		responseID, nextInputs, shouldContinue, iterationOutput, iterationErr := a.runIteration(ctx, req, events)
		if iterationErr != nil {
			return finalOutput, iterationErr
		}

		if responseID != "" {
			previousResponseID = responseID
		}

		toolInputs = nextInputs
		if iterationOutput != "" {
			finalOutput = iterationOutput
		}

		if !shouldContinue {
			break
		}
	}

	a.logger.Debug("agent finished", "iterations", a.maxIterations)
	return finalOutput, nil
}

func (a *Agent) buildSystemPrompt(ctx context.Context) string {
	if a.systemPrompt == nil {
		return ""
	}

	systemPrompt := a.systemPrompt(ctx)
	a.logger.Debug("agent system prompt built", "length", len(systemPrompt), "preview", systemPrompt[:min(200, len(systemPrompt))])
	return systemPrompt
}

func (a *Agent) buildResponseTools() []ResponseTool {
	responseTools := make([]ResponseTool, 0, len(a.tools))
	for _, tool := range a.tools {
		openaiTool := tool.ToOpenAI()
		
		// Convert to ResponseTool with strict mode based on tool configuration
		var params map[string]any
		if openaiTool.Function.Parameters != nil {
			if p, ok := openaiTool.Function.Parameters.(map[string]any); ok {
				params = p
			} else {
				// Try to marshal and unmarshal to convert
				if data, err := json.Marshal(openaiTool.Function.Parameters); err == nil {
					_ = json.Unmarshal(data, &params)
				}
			}
		}
		
		responseTools = append(responseTools, ResponseTool{
			Type:        string(openaiTool.Type),
			Name:        openaiTool.Function.Name,
			Description: openaiTool.Function.Description,
			Parameters:  params,
			Strict:      tool.strict,
		})
	}
	return responseTools
}

func buildInitialInput(userMessage string) []ResponseInput {
	return []ResponseInput{
		{
			Role: "user",
			Content: []ResponseContentItem{
				{
					Type: "input_text",
					Text: userMessage,
				},
			},
		},
	}
}

func (a *Agent) buildResponseRequest(systemPrompt string, responseTools []ResponseTool, previousResponseID string, initialInput []ResponseInput, toolInputs []ResponseContentItem) ResponseRequest {
	req := ResponseRequest{
		Model:              a.model,
		Instructions:       systemPrompt,
		Tools:              responseTools,
		ToolChoice:         "auto",
		ParallelToolCalls:  true,
		Store:              true,
		PreviousResponseID: previousResponseID,
	}

	// If reasoning effort is specified, use it; otherwise use temperature
	if a.reasoningEffort != "" {
		req.Reasoning = &ResponseReasoning{
			Effort: a.reasoningEffort,
		}
	} else {
		req.Temperature = a.temperature
	}

	if previousResponseID == "" {
		req.Input = initialInput
	} else if len(toolInputs) > 0 {
		req.Input = toolInputs
	}

	return req
}

func (a *Agent) runIteration(ctx context.Context, req ResponseRequest, events chan<- Event) (string, []ResponseContentItem, bool, string, error) {
	if !a.streamResponses {
		return a.runNonStreamingIteration(ctx, req, events)
	}

	return a.runStreamingIteration(ctx, req, events)
}

type promptLogEntry struct {
	Timestamp          string      `json:"timestamp"`
	Iteration          int         `json:"iteration"`
	Model              string      `json:"model"`
	PreviousResponseID string      `json:"previous_response_id,omitempty"`
	Instructions       string      `json:"instructions,omitempty"`
	Input              any `json:"input,omitempty"`
	ToolChoice         any `json:"tool_choice,omitempty"`
	Tools              []string    `json:"tools,omitempty"`
}

func (a *Agent) logPromptIteration(iteration int, req ResponseRequest) {
	if !a.loggingConfig.LogPrompts {
		return
	}

	toolNames := make([]string, 0, len(req.Tools))
	for _, tool := range req.Tools {
		toolNames = append(toolNames, tool.Name)
	}

	entry := promptLogEntry{
		Timestamp:          time.Now().UTC().Format(time.RFC3339Nano),
		Iteration:          iteration,
		Model:              req.Model,
		PreviousResponseID: req.PreviousResponseID,
		Instructions:       req.Instructions,
		Input:              req.Input,
		ToolChoice:         req.ToolChoice,
		Tools:              toolNames,
	}

	if a.loggingConfig.RedactSensitive {
		entry.Input = redactSensitiveValue(entry.Input)
		entry.ToolChoice = redactSensitiveValue(entry.ToolChoice)
	}

	promptLogMu.Lock()
	defer promptLogMu.Unlock()

	path := resolvePromptLogPath(a.loggingConfig)
	if err := writeJSONLine(path, entry); err != nil {
		a.logger.Error("failed to write prompt log entry", "error", err, "path", path)
	}
}

func (a *Agent) withLLMTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeoutConfig.LLMCall <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, a.timeoutConfig.LLMCall)
}

func (a *Agent) handleIterationError(ctx context.Context, events chan<- Event, err error, logMsg string, fields ...any) error {
	fields = append(fields, "error", err)
	a.logger.Error(logMsg, fields...)
	a.emit(ctx, events, Error(err))
	return err
}

func (a *Agent) validateResponse(ctx context.Context, resp *ResponseObject, events chan<- Event) error {
	if resp.Status == "failed" {
		iterationErr := fmt.Errorf("response failed: %s", resp.Error.Message)
		return a.handleIterationError(ctx, events, iterationErr, "response failed", "model", a.model)
	}

	if len(resp.Output) == 0 {
		iterationErr := fmt.Errorf("no output items returned")
		return a.handleIterationError(ctx, events, iterationErr, "no output items returned from LLM", "model", a.model)
	}

	return nil
}

func (a *Agent) isAssistantMessage(item ResponseOutputItem) bool {
	return item.Type == messageType && item.Role == "assistant"
}

func (a *Agent) emitThinkingChunks(ctx context.Context, content []ResponseContentItem, events chan<- Event) {
	for _, item := range content {
		if item.Type == outputTextType && item.Text != "" {
			a.emit(ctx, events, ThinkingChunk(item.Text))
		}
	}
}

func (a *Agent) extractFinalText(outputs []ResponseOutputItem) string {
	var finalText string
	for _, item := range outputs {
		if !a.isAssistantMessage(item) {
			continue
		}
		for _, content := range item.Content {
			if content.Type == outputTextType {
				finalText += content.Text
			}
		}
	}
	return finalText
}

func (a *Agent) processNonStreamingOutputs(ctx context.Context, callCtx context.Context, resp *ResponseObject, events chan<- Event) ([]ResponseContentItem, bool) {
	var hasToolCalls bool
	toolInputs := make([]ResponseContentItem, 0)

	for _, item := range resp.Output {
		if !a.isAssistantMessage(item) {
			continue
		}

		a.emitThinkingChunks(callCtx, item.Content, events)

		if len(item.ToolCalls) > 0 {
			hasToolCalls = true
			inputs := a.handleResponseToolCalls(ctx, resp.ID, item.ToolCalls, events)
			toolInputs = append(toolInputs, inputs...)
		}
	}

	return toolInputs, hasToolCalls
}

// runNonStreamingIteration handles a single non-streaming iteration
func (a *Agent) runNonStreamingIteration(ctx context.Context, req ResponseRequest, events chan<- Event) (string, []ResponseContentItem, bool, string, error) {
	callCtx := a.applyLLMCall(ctx, req)
	callCtx, cancel := a.withLLMTimeout(callCtx)

// Start timing for LLM call tracing
callCtx = startLLMCallTiming(callCtx)
	if cancel != nil {
		defer cancel()
	}

	resp, err := a.responsesClient.CreateResponse(callCtx, req)
	if err != nil {
		iterationErr := fmt.Errorf("response creation error: %w", err)
		a.applyLLMResponse(callCtx, nil, iterationErr)
		return "", nil, false, "", a.handleIterationError(callCtx, events, iterationErr, "response creation failed", "model", a.model)
	}
	a.applyLLMResponse(callCtx, resp, nil)

	// Log LLM generation to tracer
	a.logLLMGeneration(callCtx, req, resp)

	if err := a.validateResponse(callCtx, resp, events); err != nil {
		return "", nil, false, "", err
	}

	if a.loggingConfig.LogResponses {
		a.logger.Info("response received", "output_items", len(resp.Output), "response_id", resp.ID, "status", resp.Status)
	}

	toolInputs, hasToolCalls := a.processNonStreamingOutputs(ctx, callCtx, resp, events)
	if hasToolCalls {
		return resp.ID, toolInputs, true, "", nil
	}

	finalText := a.extractFinalText(resp.Output)
	a.emit(callCtx, events, FinalOutput("Agent completed", finalText))
	return resp.ID, nil, false, finalText, nil
}

// runStreamingIteration handles a single streaming iteration
func (a *Agent) runStreamingIteration(ctx context.Context, req ResponseRequest, events chan<- Event) (string, []ResponseContentItem, bool, string, error) {
	callCtx := a.applyLLMCall(ctx, req)
	callCtx, cancel := a.withLLMTimeout(callCtx)

	// Start timing for LLM call tracing
	callCtx = startLLMCallTiming(callCtx)
	if cancel != nil {
		defer cancel()
	}

	a.logger.Info("creating response stream", "model", a.model, "has_tools", len(req.Tools) > 0)
	stream, err := a.responsesClient.CreateResponseStream(callCtx, req)
	if err != nil {
		iterationErr := fmt.Errorf("stream creation error: %w", err)
		a.applyLLMResponse(callCtx, nil, iterationErr)
		return "", nil, false, "", a.handleIterationError(callCtx, events, iterationErr, "stream creation failed", "model", a.model)
	}
	defer func() {
		if err := stream.Close(); err != nil {
			a.logger.Error("failed to close stream", "error", err)
		}
	}()

	state := newStreamState()

	for {
		chunk, err := a.receiveStreamChunk(callCtx, stream)
		if errors.Is(err, io.EOF) {
			a.logger.Info("stream ended", "chunks_received", state.chunkCount, "response_id", state.responseID, "has_tool_calls", state.hasToolCalls, "final_text_len", len(state.finalText))
			break
		}
		if err != nil {
			iterationErr := fmt.Errorf("stream error: %w", err)
			a.applyLLMResponse(callCtx, nil, iterationErr)
			return "", nil, false, "", a.handleIterationError(callCtx, events, iterationErr, "stream reading failed", "model", a.model)
		}

		// Check if chunk indicates an error
		if chunk.Type == "error" || chunk.Type == "response.failed" {
			a.processStreamChunk(callCtx, state, chunk, events)
			// Extract error and return
			errorMsg := "Unknown error"
			if chunk.Error != nil {
				errorMsg = chunk.Error.Message
			} else if chunk.Response != nil && chunk.Response.Error != nil {
				errorMsg = chunk.Response.Error.Message
			}
			iterationErr := fmt.Errorf("LLM returned error: %s", errorMsg)
			a.applyLLMResponse(callCtx, nil, iterationErr)
			return "", nil, false, "", iterationErr
		}

		a.processStreamChunk(callCtx, state, chunk, events)
	}

	a.applyLLMResponse(callCtx, map[string]any{"response_id": state.responseID, "final_text_len": len(state.finalText)}, nil)
	if a.loggingConfig.LogResponses {
		a.logger.Info("stream response completed", "response_id", state.responseID, "final_text_len", len(state.finalText))
	}

	toolCalls := collectToolCalls(state.toolCallsMap)
	if state.hasToolCalls && len(toolCalls) > 0 {
		a.logger.Info("executing tool calls", "count", len(toolCalls))
		toolInputs := a.handleResponseToolCalls(ctx, state.responseID, toolCalls, events)
		return state.responseID, toolInputs, true, "", nil
	}

	a.logger.Info("agent iteration complete", "final_text_len", len(state.finalText), "response_id", state.responseID)
	
	// Log LLM generation to tracer (for streaming we log the accumulated response)
	a.logLLMGenerationFromStream(callCtx, req, state)
	
	a.emit(callCtx, events, FinalOutput("Agent completed", state.finalText))
	return state.responseID, nil, false, state.finalText, nil
}

type streamState struct {
	responseID   string
	hasToolCalls bool
	toolCallsMap map[int]*ResponseToolCall
	finalText    string
	chunkCount   int
	usage        *ResponseUsage // Captured from stream final chunk
}

func newStreamState() *streamState {
	return &streamState{
		toolCallsMap: make(map[int]*ResponseToolCall),
	}
}

func (s *streamState) updateResponseID(chunk *ResponseStreamChunk) {
	if chunk.ResponseID != "" {
		s.responseID = chunk.ResponseID
		return
	}
	if chunk.Response != nil && chunk.Response.ID != "" {
		s.responseID = chunk.Response.ID
	}
}

func (a *Agent) receiveStreamChunk(callCtx context.Context, stream ResponseStreamClient) (*ResponseStreamChunk, error) {
	chunkCtx := callCtx
	var cancel context.CancelFunc
	if a.timeoutConfig.StreamChunk > 0 {
		chunkCtx, cancel = context.WithTimeout(callCtx, a.timeoutConfig.StreamChunk)
		defer cancel()
	}
	if err := chunkCtx.Err(); err != nil {
		return nil, fmt.Errorf("stream timeout: %w", err)
	}

	chunk, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	return chunk, nil
}

func (a *Agent) processStreamChunk(ctx context.Context, state *streamState, chunk *ResponseStreamChunk, events chan<- Event) {
	state.chunkCount++
	state.updateResponseID(chunk)

	a.logger.Debug("received chunk", "type", chunk.Type, "response_id", chunk.ResponseID, "item_id", chunk.ItemID, "delta_len", len(chunk.Delta))

	switch chunk.Type {
	case "error", "response.failed":
		a.handleErrorChunk(ctx, chunk, events)
	case "response.output_item.added":
		a.handleOutputItemAdded(state, chunk)
	case "response.output_text.delta":
		a.handleOutputTextDelta(ctx, state, chunk, events)
	case "response.function_call_arguments.delta":
		a.handleFunctionCallDelta(state, chunk)
	case "response.function_call_arguments.done":
		a.handleFunctionCallDone(state, chunk)
	case "response.output_item.done":
		a.handleOutputItemDone(state, chunk)
	case "response.done":
		a.logger.Info("response done", "response_id", chunk.ResponseID)
		// Capture usage data from final chunk
		// Check both chunk.Usage (if present) and chunk.Response.Usage
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			state.usage = chunk.Usage
			a.logger.Info("captured usage from chunk.Usage", "input_tokens", chunk.Usage.InputTokens, "output_tokens", chunk.Usage.OutputTokens, "total_tokens", chunk.Usage.TotalTokens)
		} else if chunk.Response != nil && chunk.Response.Usage.TotalTokens > 0 {
			// Make a copy of the usage struct since it's not a pointer
			usageCopy := chunk.Response.Usage
			state.usage = &usageCopy
			a.logger.Info("captured usage from chunk.Response.Usage", "input_tokens", usageCopy.InputTokens, "output_tokens", usageCopy.OutputTokens, "reasoning_tokens", usageCopy.ReasoningTokens, "total_tokens", usageCopy.TotalTokens)
		} else {
			// Detailed logging to diagnose why usage data is missing
			a.logger.Warn("response.done received but no usage data found",
				"has_chunk_usage", chunk.Usage != nil,
				"has_response", chunk.Response != nil,
				"chunk_usage_total", func() int {
					if chunk.Usage != nil {
						return chunk.Usage.TotalTokens
					}
					return 0
				}(),
				"response_usage_total", func() int {
					if chunk.Response != nil {
						return chunk.Response.Usage.TotalTokens
					}
					return 0
				}(),
			)
		}
	case "response.created", "response.completed":
		a.handleResponseEvent(chunk)
	}
}

func (a *Agent) handleOutputItemAdded(state *streamState, chunk *ResponseStreamChunk) {
	if chunk.Item == nil {
		return
	}

	a.logger.Debug("output item added", "item_type", chunk.Item.Type, "item_id", chunk.Item.ID)
	if chunk.Item.Type != "function_call" {
		return
	}

	state.hasToolCalls = true
	idx := chunk.OutputIndex
	state.toolCallsMap[idx] = &ResponseToolCall{
		ID:        chunk.Item.ID,
		CallID:    chunk.Item.CallID,
		Type:      "function_call",
		Name:      chunk.Item.Name,
		Arguments: "",
	}
	if chunk.Item.Name != "" {
		a.logger.Info("function call item added", "index", idx, "name", chunk.Item.Name, "call_id", chunk.Item.CallID)
	}
}

func (a *Agent) handleOutputTextDelta(ctx context.Context, state *streamState, chunk *ResponseStreamChunk, events chan<- Event) {
	if chunk.Delta == "" {
		return
	}
	a.logger.Info("text delta received", "delta_len", len(chunk.Delta), "delta_preview", chunk.Delta[:min(50, len(chunk.Delta))])
	state.finalText += chunk.Delta
	a.emit(ctx, events, ThinkingChunk(chunk.Delta))
}

func (a *Agent) handleFunctionCallDelta(state *streamState, chunk *ResponseStreamChunk) {
	if chunk.Delta == "" {
		return
	}

	idx := chunk.OutputIndex
	call := ensureToolCall(state, idx, &ResponseToolCall{
		ID:        chunk.ItemID,
		CallID:    chunk.ItemID,
		Type:      "function_call",
		Arguments: "",
	})

	call.Arguments += chunk.Delta
	a.logger.Info("accumulated function arguments", "index", idx, "delta", chunk.Delta, "total_len", len(call.Arguments))
}

func (a *Agent) handleFunctionCallDone(state *streamState, chunk *ResponseStreamChunk) {
	idx := chunk.OutputIndex
	call := state.toolCallsMap[idx]
	if call == nil {
		a.logger.Warn("received done event but no tool call in map", "index", idx, "name", chunk.Name)
		return
	}

	if chunk.Name != "" {
		call.Name = chunk.Name
	}
	if chunk.Arguments != "" {
		call.Arguments = chunk.Arguments
	}
	a.logger.Info("updated tool call from done event", "index", idx, "name", call.Name, "final_args_len", len(call.Arguments))
}

func (a *Agent) handleOutputItemDone(state *streamState, chunk *ResponseStreamChunk) {
	if chunk.Item == nil || chunk.Item.Type != "function_call" {
		return
	}

	idx := chunk.OutputIndex
	call := ensureToolCall(state, idx, &ResponseToolCall{
		ID:        chunk.Item.ID,
		CallID:    chunk.Item.CallID,
		Type:      "function_call",
		Arguments: "",
	})

	call.Name = chunk.Item.Name
	call.CallID = chunk.Item.CallID
	if chunk.Item.Arguments != "" {
		call.Arguments = chunk.Item.Arguments
	}
	a.logger.Info("function call item done", "index", idx, "name", call.Name, "call_id", call.CallID)
}

func (a *Agent) handleErrorChunk(ctx context.Context, chunk *ResponseStreamChunk, events chan<- Event) {
	errorMsg := "Unknown error"
	errorCode := ""

	// Try to extract error from the chunk directly or from Response object
	if chunk.Error != nil {
		errorMsg = chunk.Error.Message
		errorCode = chunk.Error.Code
	} else if chunk.Response != nil && chunk.Response.Error != nil {
		errorMsg = chunk.Response.Error.Message
		errorCode = chunk.Response.Error.Code
	}

	err := fmt.Errorf("LLM error (%s): %s", errorCode, errorMsg)
	a.logger.Error("stream error chunk received", "code", errorCode, "message", errorMsg)
	a.emit(ctx, events, Error(err))
}

func (a *Agent) handleResponseEvent(chunk *ResponseStreamChunk) {
	if chunk.Response != nil {
		a.logger.Info("response event", "event_type", chunk.Type, "response_id", chunk.Response.ID)
	}
}

func ensureToolCall(state *streamState, idx int, fallback *ResponseToolCall) *ResponseToolCall {
	if call, ok := state.toolCallsMap[idx]; ok && call != nil {
		return call
	}
	state.toolCallsMap[idx] = fallback
	return fallback
}

func collectToolCalls(toolCallsMap map[int]*ResponseToolCall) []ResponseToolCall {
	toolCalls := make([]ResponseToolCall, 0, len(toolCallsMap))
	for _, tc := range toolCallsMap {
		if tc == nil {
			continue
		}
		toolCalls = append(toolCalls, *tc)
	}
	return toolCalls
}

// handleResponseToolCalls executes tool calls and returns inputs for the next response
func (a *Agent) handleResponseToolCalls(ctx context.Context, _ string, toolCalls []ResponseToolCall, events chan<- Event) []ResponseContentItem {
	if !a.parallelConfig.Enabled || a.parallelConfig.MaxConcurrent <= 1 || a.parallelConfig.SafetyMode == SafetyModePessimistic {
		return a.executeToolCallsSequential(ctx, toolCalls, events)
	}

	return a.executeToolCallsParallel(ctx, toolCalls, events)
}

func (a *Agent) executeToolCallsSequential(ctx context.Context, toolCalls []ResponseToolCall, events chan<- Event) []ResponseContentItem {
	toolOutputs := make([]ResponseContentItem, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		output := a.executeToolCall(ctx, toolCall, events)
		toolOutputs = append(toolOutputs, output)
	}

	if len(toolOutputs) == 0 {
		return nil
	}

	return toolOutputs
}

func (a *Agent) executeToolCallsParallel(ctx context.Context, toolCalls []ResponseToolCall, events chan<- Event) []ResponseContentItem {
	type outputSlot struct {
		item ResponseContentItem
	}

	slots := make([]outputSlot, len(toolCalls))
	sem := make(chan struct{}, a.parallelConfig.MaxConcurrent)
	var wg sync.WaitGroup

	waitForInflight := func() {
		wg.Wait()
	}

	for i, toolCall := range toolCalls {
		tool, exists := a.tools[toolCall.Name]
		concurrency := ConcurrencyParallel
		if exists {
			concurrency = tool.concurrency
		}

		if concurrency == ConcurrencySerial {
			waitForInflight()
			output := a.executeToolCall(ctx, toolCall, events)
			slots[i] = outputSlot{item: output}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, call ResponseToolCall) {
			defer wg.Done()
			defer func() { <-sem }()
			output := a.executeToolCall(ctx, call, events)
			slots[idx] = outputSlot{item: output}
		}(i, toolCall)
	}

	waitForInflight()

	toolOutputs := make([]ResponseContentItem, 0, len(slots))
	for _, slot := range slots {
		toolOutputs = append(toolOutputs, slot.item)
	}

	if len(toolOutputs) == 0 {
		return nil
	}

	return toolOutputs
}

func (a *Agent) executeToolCall(ctx context.Context, toolCall ResponseToolCall, events chan<- Event) ResponseContentItem {
	tool, exists := a.tools[toolCall.Name]
	args := a.parseToolArgs(toolCall)
	description := a.describeToolCall(toolCall.Name, exists, tool, args)

	approved, denialOutput := a.requestToolApproval(ctx, toolCall, args, description, events)
	if !approved {
		return *denialOutput
	}

	// Start span for tool execution
	spanCtx, endSpan := a.tracer.StartSpan(ctx, toolCall.Name,
		WithSpanType(SpanTypeTool),
		WithSpanInput(args),
	)
	defer endSpan()

	toolCtx := a.startToolCall(spanCtx, toolCall, args)
	a.emit(toolCtx, events, ActionDetected(description, toolCall.CallID))

	result, err := a.runTool(toolCtx, toolCall, tool, exists)
	resultDisplay := a.formatToolResult(toolCall, tool, exists, result, err)
	a.emit(toolCtx, events, ActionResult(resultDisplay, result))

	// Log tool execution to span
	if err != nil {
		a.tracer.SetSpanAttributes(spanCtx, map[string]any{
			"error": err.Error(),
			"tool.name": toolCall.Name,
		})
	} else {
		// Set the tool output on the span
		a.tracer.SetSpanOutput(spanCtx, result)
		// Set additional metadata
		a.tracer.SetSpanAttributes(spanCtx, map[string]any{
			"tool.name": toolCall.Name,
		})
	}

	a.finishToolCall(toolCtx, toolCall, result, err)

	return a.buildToolOutput(toolCall.CallID, result)
}

func (a *Agent) parseToolArgs(toolCall ResponseToolCall) map[string]any {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		a.logger.Error("failed to parse tool arguments", "tool", toolCall.Name, "error", err)
		return map[string]any{}
	}
	if args == nil {
		return map[string]any{}
	}
	return args
}

func (a *Agent) describeToolCall(name string, exists bool, tool Tool, args map[string]any) string {
	if exists {
		return tool.FormatPending(args)
	}
	return fmt.Sprintf("Running %s...", name)
}

func (a *Agent) requestToolApproval(ctx context.Context, toolCall ResponseToolCall, args map[string]any, description string, events chan<- Event) (bool, *ResponseContentItem) {
	if !a.approvalConfig.requiresApproval(toolCall.Name) {
		return true, nil
	}

	conversationID, _ := GetConversationID(ctx)
	approvalReq := ApprovalRequest{
		ToolName:       toolCall.Name,
		Arguments:      args,
		Description:    description,
		ConversationID: conversationID,
		CallID:         toolCall.CallID,
	}

	a.emit(ctx, events, ApprovalRequired(approvalReq))

	approved, err := a.evaluateApproval(ctx, toolCall, approvalReq)
	if approved {
		a.emit(ctx, events, ApprovalGranted(toolCall.Name, toolCall.CallID))
		return true, nil
	}

	result := map[string]any{
		"error":    "tool execution requires approval and was denied",
		"tool":     toolCall.Name,
		"approved": false,
	}
	output := a.buildToolOutput(toolCall.CallID, result)
	if err != nil {
		a.emit(ctx, events, ApprovalDenied(toolCall.Name, toolCall.CallID, fmt.Sprintf("approval error: %v", err)))
	} else {
		a.emit(ctx, events, ApprovalDenied(toolCall.Name, toolCall.CallID, "approval denied by handler"))
	}
	a.emit(ctx, events, ActionResult("❌ Approval denied", result))

	return false, &output
}

func (a *Agent) evaluateApproval(ctx context.Context, toolCall ResponseToolCall, approvalReq ApprovalRequest) (bool, error) {
	if a.approvalConfig.Handler == nil {
		return false, nil
	}

	approved, err := a.approvalConfig.Handler(ctx, approvalReq)
	if err != nil {
		a.logger.Error("approval handler failed", "tool", toolCall.Name, "error", err)
		return false, err
	}

	return approved, nil
}

func (a *Agent) startToolCall(ctx context.Context, toolCall ResponseToolCall, args map[string]any) context.Context {
	toolCtx := a.applyToolStart(ctx, toolCall.Name, args)
	if a.loggingConfig.LogToolCalls {
		logArgs := any(args)
		if a.loggingConfig.RedactSensitive {
			logArgs = redactSensitiveValue(args)
		}
		a.logger.Info("tool call starting", "tool", toolCall.Name, "call_id", toolCall.CallID, "args", logArgs)
	}
	return toolCtx
}

func (a *Agent) runTool(ctx context.Context, toolCall ResponseToolCall, tool Tool, exists bool) (any, error) {
	if !exists {
		err := fmt.Errorf("unknown tool: %s", toolCall.Name)
		a.logger.Error("unknown tool", "tool", toolCall.Name)
		return map[string]any{"error": err.Error()}, err
	}

	execCtx := ctx
	if a.timeoutConfig.ToolExecution > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, a.timeoutConfig.ToolExecution)
		defer cancel()
	}

	result, err := tool.Execute(execCtx, toolCall.Arguments)
	if err != nil {
		a.logger.Error("tool execution failed", "tool", toolCall.Name, "error", err)
		return map[string]any{"error": err.Error()}, err
	}

	return result, nil
}

func (a *Agent) formatToolResult(toolCall ResponseToolCall, tool Tool, exists bool, result any, err error) string {
	switch {
	case err != nil:
		return fmt.Sprintf("Error: %v", err)
	case !exists:
		return fmt.Sprintf("Unknown tool: %s", toolCall.Name)
	default:
		return tool.FormatResult(result)
	}
}

func (a *Agent) finishToolCall(ctx context.Context, toolCall ResponseToolCall, result any, err error) {
	if a.loggingConfig.LogToolCalls {
		logResult := result
		if a.loggingConfig.RedactSensitive {
			logResult = redactSensitiveValue(result)
		}
		a.logger.Info("tool call completed", "tool", toolCall.Name, "call_id", toolCall.CallID, "error", err, "result", logResult)
	}

	a.applyToolComplete(ctx, toolCall.Name, result, err)
}

func (a *Agent) buildToolOutput(callID string, result any) ResponseContentItem {
	resultJSON, _ := marshalResult(result)
	return ResponseContentItem{
		Type:   "function_call_output",
		CallID: callID,
		Output: resultJSON,
	}
}

// marshalResult converts a result to JSON string
func marshalResult(result any) (string, error) {
	switch v := result.(type) {
	case string:
		return v, nil
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		data, err := json.Marshal(result)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
