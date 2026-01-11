package agentkit

import (
	"context"
	"errors"
	"fmt"
)

// Handoff represents a delegation of work from one agent to another.
// The receiving agent works in an isolated context and returns results.
// This mimics how real people delegate: "Go figure this out and report back."
type Handoff struct {
	from    *Agent
	to      *Agent
	options handoffOptions
}

type handoffOptions struct {
	fullContext bool          // Whether to return full conversation context OR just final result (real-time streaming always happens)
	maxTurns    int           // Limit on conversation turns for the handoff
	context     HandoffContext // Additional context to provide
}

// HandoffContext provides additional information for the delegated agent.
type HandoffContext struct {
	Background string         // Context about why this handoff is happening
	Metadata   map[string]any // Additional structured data
}

// HandoffOption configures a handoff.
type HandoffOption func(*handoffOptions)

// WithFullContext controls whether to return the full conversation context (thinking, tool calls, etc.)
// OR just the final result in the HandoffResult. When false, only the final response is returned.
// When true, the complete execution trace is included, which is useful for debugging or learning
// from the approach taken but increases context usage.
// NOTE: Real-time event streaming to parent agents ALWAYS happens regardless of this setting.
func WithFullContext(include bool) HandoffOption {
	return func(o *handoffOptions) {
		o.fullContext = include
	}
}

// WithMaxTurns limits the number of conversation turns the delegated agent can take.
func WithMaxTurns(max int) HandoffOption {
	return func(o *handoffOptions) {
		o.maxTurns = max
	}
}

// WithContext provides additional background information to the delegated agent.
func WithContext(ctx HandoffContext) HandoffOption {
	return func(o *handoffOptions) {
		o.context = ctx
	}
}

// HandoffResult contains the outcome of a delegation.
type HandoffResult struct {
	Response string              // The final response from the delegated agent
	Summary  string              // Optional summary of the work done
	Trace    []HandoffTraceItem  // Execution trace (if fullContext was enabled)
	Metadata map[string]any      // Additional result metadata
}

// HandoffTraceItem represents a single step in the delegated agent's execution.
type HandoffTraceItem struct {
	Type    string `json:"type"`    // "thought", "tool_call", "tool_result", "response"
	Content string `json:"content"` // The actual content of this step
}

var (
	ErrHandoffAgentNil      = errors.New("agentkit: handoff target agent cannot be nil")
	ErrHandoffTaskEmpty     = errors.New("agentkit: handoff task cannot be empty")
	ErrHandoffExecutionFail = errors.New("agentkit: handoff execution failed")
)

// HandoffConfiguration represents a reusable handoff setup.
// This allows creating a handoff configuration once and converting it to a tool.
type HandoffConfiguration struct {
	from    *Agent
	to      *Agent
	options handoffOptions
}

// NewHandoffConfiguration creates a reusable handoff configuration.
// This is useful when you want to create the configuration once and convert it to a tool.
//
// Example:
//
//	handoffConfig := agentkit.NewHandoffConfiguration(coordinator, researchAgent, WithFullContext(true))
//	tool := handoffConfig.AsTool("research", "Delegate research tasks")
//	coordinator.RegisterTool(tool)
func NewHandoffConfiguration(from, to *Agent, opts ...HandoffOption) *HandoffConfiguration {
	options := handoffOptions{
		fullContext: false,
		maxTurns:    10,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &HandoffConfiguration{
		from:    from,
		to:      to,
		options: options,
	}
}

// Configure applies additional options to the handoff configuration.
func (h *HandoffConfiguration) Configure(opts ...HandoffOption) *HandoffConfiguration {
	for _, opt := range opts {
		opt(&h.options)
	}
	return h
}

// Execute performs the handoff with a specific task.
func (h *HandoffConfiguration) Execute(ctx context.Context, task string) (*HandoffResult, error) {
	if h.to == nil {
		return nil, ErrHandoffAgentNil
	}
	if task == "" {
		return nil, ErrHandoffTaskEmpty
	}

	return h.from.Handoff(ctx, h.to, task, func(o *handoffOptions) { *o = h.options })
}

// AsTool converts the handoff configuration into a Tool that can be registered with an agent.
// The LLM will decide when to use this tool and what task to provide.
//
// Example:
//
//	researchHandoff := agentkit.NewHandoffConfiguration(coordinator, researchAgent)
//	tool := researchHandoff.AsTool(
//	    "delegate_research",
//	    "Delegate research tasks to a specialized research agent",
//	)
//	coordinator.RegisterTool(tool)
func (h *HandoffConfiguration) AsTool(name, description string) Tool {
	return NewTool(name).
		WithDescription(description).
		WithParameter("task", String().Required().WithDescription("The task to delegate to the agent")).
		WithParameter("background", String().WithDescription("Optional background context for the handoff")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			task, ok := args["task"].(string)
			if !ok || task == "" {
				return nil, ErrHandoffTaskEmpty
			}

			// Create a copy of options
			opts := h.options

			// Add background context if provided
			if bg, ok := args["background"].(string); ok && bg != "" {
				opts.context = HandoffContext{
					Background: bg,
				}
			}

			// Get parent's tracer from context for proper trace propagation
			parentTracer := GetTracer(ctx)
			if parentTracer == nil && h.from != nil {
				parentTracer = h.from.tracer
			}

			// Create a span for the handoff using parent's tracer
			// This ensures delegated agent traces are properly nested
			var spanCtx context.Context
			var endSpan func()
			if parentTracer != nil && !isNoOpTracer(parentTracer) {
				spanCtx, endSpan = parentTracer.StartSpan(ctx, fmt.Sprintf("handoff.%s", name))
				defer endSpan()

				fromName := ""
				if h.from != nil {
					fromName = h.from.getAgentName()
				}
				toName := ""
				if h.to != nil {
					toName = h.to.getAgentName()
				}

				parentTracer.SetSpanAttributes(spanCtx, map[string]any{
					"handoff_tool":   name,
					"handoff_from":   fromName,
					"handoff_to":     toName,
					"task_length":    len(task),
					"full_context":   opts.fullContext,
					"max_turns":      opts.maxTurns,
					"has_background": opts.context.Background != "",
				})
			} else {
				spanCtx = ctx
			}

			// Prepare the full task with context if provided
			fullTask := task
			if opts.context.Background != "" {
				fullTask = fmt.Sprintf("Background: %s\n\nTask: %s", opts.context.Background, task)
			}

			// Create a copy of the receiving agent with the parent's tracer
			// This ensures all LLM calls and operations are properly traced
			delegatedAgent := *h.to
			if parentTracer != nil && !isNoOpTracer(parentTracer) {
				delegatedAgent.tracer = parentTracer
			}

			// Override max iterations if specified
			if opts.maxTurns > 0 && opts.maxTurns < delegatedAgent.maxIterations {
				delegatedAgent.maxIterations = opts.maxTurns
			}

			// Emit handoff.start event
			if parentPub, hasParent := GetEventPublisher(spanCtx); hasParent {
				parentPub(HandoffStart(h.from.getAgentName(), h.to.getAgentName(), task))
			}

			// Execute the handoff with proper trace context
			response, summary, trace, err := executeHandoff(spanCtx, &delegatedAgent, fullTask, opts)
			
			// Emit handoff.complete event
			if parentPub, hasParent := GetEventPublisher(spanCtx); hasParent {
				if err != nil {
					parentPub(HandoffComplete(h.from.getAgentName(), h.to.getAgentName(), fmt.Sprintf("error: %v", err)))
				} else {
					parentPub(HandoffComplete(h.from.getAgentName(), h.to.getAgentName(), response))
				}
			}
			
			if err != nil {
				if parentTracer != nil && spanCtx != nil {
					parentTracer.SetSpanAttributes(spanCtx, map[string]any{
						"error": err.Error(),
					})
				}
				return nil, err
			}

			// Record success metrics
			if parentTracer != nil && spanCtx != nil {
				parentTracer.SetSpanAttributes(spanCtx, map[string]any{
					"response_length": len(response),
					"trace_items":     len(trace),
					"has_summary":     summary != "",
				})
			}

			// Return result structure
			result := &HandoffResult{
				Response: response,
				Summary:  summary,
				Metadata: make(map[string]any),
			}

			if opts.fullContext {
				result.Trace = trace
			}

			return result, nil
		}).
		Build()
}

// Handoff delegates a task to another agent.
// The receiving agent works independently with an isolated context,
// then returns the result. The delegating agent can optionally see
// the full execution trace (thinking, tool calls, etc.) to understand 
// how the work was done. Real-time event streaming to parent ALWAYS happens.
//
// Example:
//
//	researchAgent := agentkit.NewAgent(researchConfig)
//	result, err := coordinator.Handoff(ctx, researchAgent, 
//	    "Research the top 3 Go web frameworks in 2026",
//	    WithFullContext(true),
//	)
func (a *Agent) Handoff(ctx context.Context, to *Agent, task string, opts ...HandoffOption) (*HandoffResult, error) {
	if to == nil {
		return nil, ErrHandoffAgentNil
	}
	if task == "" {
		return nil, ErrHandoffTaskEmpty
	}

	options := handoffOptions{
		fullContext: false,
		maxTurns:    10, // Reasonable default
	}
	for _, opt := range opts {
		opt(&options)
	}

	// Get parent's tracer and create a span for this handoff
	parentTracer := GetTracer(ctx)
	if parentTracer == nil {
		parentTracer = a.tracer
	}

	var spanCtx context.Context
	var endSpan func()
	if parentTracer != nil && !isNoOpTracer(parentTracer) {
		spanCtx, endSpan = parentTracer.StartSpan(ctx, "handoff")
		defer endSpan()

		parentTracer.SetSpanAttributes(spanCtx, map[string]any{
			"handoff_from":   a.getAgentName(),
			"handoff_to":     to.getAgentName(),
			"task_length":    len(task),
			"full_context":   options.fullContext,
			"max_turns":      options.maxTurns,
			"has_background": options.context.Background != "",
		})
	} else {
		spanCtx = ctx
	}

	// Prepare the full task with context if provided
	fullTask := task
	if options.context.Background != "" {
		fullTask = fmt.Sprintf("Background: %s\n\nTask: %s", options.context.Background, task)
	}

	// Create a copy of the receiving agent with the parent's tracer
	// This ensures all work is traced under the handoff span
	delegatedAgent := *to
	if parentTracer != nil && !isNoOpTracer(parentTracer) {
		delegatedAgent.tracer = parentTracer
	}

	// Override max iterations if specified
	if options.maxTurns > 0 && options.maxTurns < delegatedAgent.maxIterations {
		delegatedAgent.maxIterations = options.maxTurns
	}

	// Emit handoff.start event
	if parentPub, hasParent := GetEventPublisher(spanCtx); hasParent {
		parentPub(HandoffStart(a.getAgentName(), to.getAgentName(), task))
	}

	// Execute the handoff in isolation
	response, summary, trace, err := executeHandoff(spanCtx, &delegatedAgent, fullTask, options)
	
	// Emit handoff.complete event
	if parentPub, hasParent := GetEventPublisher(spanCtx); hasParent {
		if err != nil {
			parentPub(HandoffComplete(a.getAgentName(), to.getAgentName(), fmt.Sprintf("error: %v", err)))
		} else {
			parentPub(HandoffComplete(a.getAgentName(), to.getAgentName(), response))
		}
	}
	
	if err != nil {
		if parentTracer != nil && spanCtx != nil {
			parentTracer.SetSpanAttributes(spanCtx, map[string]any{
				"error": err.Error(),
			})
		}
		return nil, fmt.Errorf("%w: %v", ErrHandoffExecutionFail, err)
	}

	// Record success metrics
	if parentTracer != nil && spanCtx != nil {
		parentTracer.SetSpanAttributes(spanCtx, map[string]any{
			"response_length": len(response),
			"trace_items":     len(trace),
			"has_summary":     summary != "",
		})
	}

	result := &HandoffResult{
		Response: response,
		Summary:  summary,
		Metadata: make(map[string]any),
	}

	if options.fullContext {
		result.Trace = trace
	}

	return result, nil
}

// executeHandoff runs the delegated agent in isolation and captures results.
// Events are ALWAYS forwarded to the parent event publisher in real-time.
// The fullContext flag only controls whether trace items are returned in the result.
func executeHandoff(ctx context.Context, agent *Agent, task string, opts handoffOptions) (string, string, []HandoffTraceItem, error) {
	var trace []HandoffTraceItem
	var response string

	// Run the agent and get the event channel
	events := agent.Run(ctx, task)

	// Get parent event publisher to forward events in real-time
	parentPub, hasParent := GetEventPublisher(ctx)

	// Capture trace items if requested
	var lastContent string
	var runErr error
	
	for event := range events {
		// ALWAYS forward events to parent if available (real-time streaming)
		if hasParent {
			parentPub(event)
		}

		// Optionally capture trace items based on fullContext flag
		switch event.Type {
		case EventTypeThinkingChunk:
			if chunk, ok := event.Data["chunk"].(string); ok {
				lastContent += chunk
				if opts.fullContext {
					trace = append(trace, HandoffTraceItem{
						Type:    "thought",
						Content: chunk,
					})
				}
			}
		case EventTypeActionDetected:
			if opts.fullContext {
				desc, _ := event.Data["description"].(string)
				toolID, _ := event.Data["tool_id"].(string)
				trace = append(trace, HandoffTraceItem{
					Type:    "tool_call",
					Content: fmt.Sprintf("%s (%s)", desc, toolID),
				})
			}
		case EventTypeActionResult:
			if opts.fullContext {
				desc, _ := event.Data["description"].(string)
				result := event.Data["result"]
				trace = append(trace, HandoffTraceItem{
					Type:    "tool_result",
					Content: fmt.Sprintf("%s: %v", desc, result),
				})
			}
		case EventTypeFinalOutput:
			if content, ok := event.Data["response"].(string); ok {
				response = content
				if opts.fullContext {
					trace = append(trace, HandoffTraceItem{
						Type:    "response",
						Content: content,
					})
				}
			}
		case EventTypeError:
			if errMsg, ok := event.Data["error"].(string); ok {
				runErr = fmt.Errorf("%s", errMsg)
			}
		}
	}

	if runErr != nil {
		return "", "", nil, runErr
	}

	// Use the final response or last content
	if response == "" {
		response = lastContent
	}

	// Generate a summary of the work done
	summary := generateHandoffSummary(trace)

	return response, summary, trace, nil
}

// generateHandoffSummary creates a brief summary of what happened during the handoff.
func generateHandoffSummary(trace []HandoffTraceItem) string {
	if len(trace) == 0 {
		return ""
	}

	toolCallCount := 0
	for _, item := range trace {
		if item.Type == "tool_call" {
			toolCallCount++
		}
	}

	if toolCallCount > 0 {
		return fmt.Sprintf("Completed with %d tool call(s) across %d step(s)", toolCallCount, len(trace))
	}
	return fmt.Sprintf("Completed in %d step(s)", len(trace))
}

// getAgentName returns a name for the agent for tracing purposes.
func (a *Agent) getAgentName() string {
	// Try to extract from system prompt or model
	if a.systemPrompt != nil {
		// This is a simple heuristic - could be enhanced
		return a.model
	}
	return a.model
}

// AsHandoffTool converts an agent into a Tool that can be registered with another agent.
// This enables handoffs to be triggered by the LLM through tool calling.
//
// Example:
//
//	researchAgent := agentkit.NewAgent(researchConfig)
//	coordinatorAgent := agentkit.NewAgent(coordinatorConfig)
//	
//	// Register as a tool
//	coordinatorAgent.RegisterTool(researchAgent.AsHandoffTool(
//	    "research_agent",
//	    "Delegate research tasks to a specialized research agent",
//	))
func (a *Agent) AsHandoffTool(name, description string, opts ...HandoffOption) Tool {
	return NewTool(name).
		WithDescription(description).
		WithParameter("task", String().Required().WithDescription("The task to delegate to this agent")).
		WithParameter("background", String().WithDescription("Optional background context about why this handoff is happening")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			task, ok := args["task"].(string)
			if !ok || task == "" {
				return nil, ErrHandoffTaskEmpty
			}

			// Extract background if provided
			handoffOpts := handoffOptions{
				fullContext: false,
				maxTurns:    10,
			}
			
			// Add background context if provided
			if bg, ok := args["background"].(string); ok && bg != "" {
				handoffOpts.context = HandoffContext{
					Background: bg,
				}
			}
			
			// Apply any provided options
			for _, opt := range opts {
				opt(&handoffOpts)
			}

			// Get parent's tracer from context for proper trace propagation
			parentTracer := GetTracer(ctx)
			if parentTracer == nil {
				parentTracer = a.tracer
			}

			// Create a span for the handoff using parent's tracer
			// This ensures delegated agent traces are properly nested
			var spanCtx context.Context
			var endSpan func()
			if parentTracer != nil && !isNoOpTracer(parentTracer) {
				spanCtx, endSpan = parentTracer.StartSpan(ctx, fmt.Sprintf("handoff.%s", name))
				defer endSpan()

				parentTracer.SetSpanAttributes(spanCtx, map[string]any{
					"handoff_tool":   name,
					"handoff_to":     a.getAgentName(),
					"task_length":    len(task),
					"full_context":   handoffOpts.fullContext,
					"max_turns":      handoffOpts.maxTurns,
					"has_background": handoffOpts.context.Background != "",
				})
			} else {
				spanCtx = ctx
			}

			// Prepare the full task with context if provided
			fullTask := task
			if handoffOpts.context.Background != "" {
				fullTask = fmt.Sprintf("Background: %s\n\nTask: %s", handoffOpts.context.Background, task)
			}

			// Create a copy of the agent with the parent's tracer
			// This ensures all LLM calls and operations are properly traced
			delegatedAgent := *a
			if parentTracer != nil && !isNoOpTracer(parentTracer) {
				delegatedAgent.tracer = parentTracer
			}

			// Override max iterations if specified
			if handoffOpts.maxTurns > 0 && handoffOpts.maxTurns < delegatedAgent.maxIterations {
				delegatedAgent.maxIterations = handoffOpts.maxTurns
			}

			// Emit handoff.start event
			fromAgentName := "caller" // The agent that called this as a tool
			toAgentName := a.getAgentName()
			if parentPub, hasParent := GetEventPublisher(spanCtx); hasParent {
				parentPub(HandoffStart(fromAgentName, toAgentName, task))
			}

			// Execute the handoff with proper trace context
			response, summary, trace, err := executeHandoff(spanCtx, &delegatedAgent, fullTask, handoffOpts)
			
			// Emit handoff.complete event
			if parentPub, hasParent := GetEventPublisher(spanCtx); hasParent {
				if err != nil {
					parentPub(HandoffComplete(fromAgentName, toAgentName, fmt.Sprintf("error: %v", err)))
				} else {
					parentPub(HandoffComplete(fromAgentName, toAgentName, response))
				}
			}
			
			if err != nil {
				if parentTracer != nil && spanCtx != nil {
					parentTracer.SetSpanAttributes(spanCtx, map[string]any{
						"error": err.Error(),
					})
				}
				return nil, err
			}

			// Record success metrics
			if parentTracer != nil && spanCtx != nil {
				parentTracer.SetSpanAttributes(spanCtx, map[string]any{
					"response_length": len(response),
					"trace_items":     len(trace),
					"has_summary":     summary != "",
				})
			}

			// Return result structure
			result := &HandoffResult{
				Response: response,
				Summary:  summary,
				Metadata: make(map[string]any),
			}
			
			if handoffOpts.fullContext {
				result.Trace = trace
			}

			return result, nil
		}).
		Build()
}
