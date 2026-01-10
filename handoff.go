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
	includeTrace bool          // Whether to capture the delegated agent's reasoning
	maxTurns     int           // Limit on conversation turns for the handoff
	context      HandoffContext // Additional context to provide
}

// HandoffContext provides additional information for the delegated agent.
type HandoffContext struct {
	Background string         // Context about why this handoff is happening
	Metadata   map[string]any // Additional structured data
}

// HandoffOption configures a handoff.
type HandoffOption func(*handoffOptions)

// WithIncludeTrace enables capturing the delegated agent's reasoning steps.
// This is useful for debugging or when the delegating agent needs to learn
// from the approach taken. It increases context usage.
func WithIncludeTrace(include bool) HandoffOption {
	return func(o *handoffOptions) {
		o.includeTrace = include
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
	Trace    []HandoffTraceItem  // Execution trace (if includeTrace was enabled)
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

// Handoff delegates a task to another agent.
// The receiving agent works independently with an isolated context,
// then returns the result. The delegating agent can optionally see
// the execution trace to understand how the work was done.
//
// Example:
//
//	researchAgent := agentkit.NewAgent(researchConfig)
//	result, err := coordinator.Handoff(ctx, researchAgent, 
//	    "Research the top 3 Go web frameworks in 2026",
//	    WithIncludeTrace(true),
//	)
func (a *Agent) Handoff(ctx context.Context, to *Agent, task string, opts ...HandoffOption) (*HandoffResult, error) {
	if to == nil {
		return nil, ErrHandoffAgentNil
	}
	if task == "" {
		return nil, ErrHandoffTaskEmpty
	}

	options := handoffOptions{
		includeTrace: false,
		maxTurns:     10, // Reasonable default
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
			"include_trace":  options.includeTrace,
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

	// Execute the handoff in isolation
	response, summary, trace, err := executeHandoff(spanCtx, &delegatedAgent, fullTask, options)
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

	if options.includeTrace {
		result.Trace = trace
	}

	return result, nil
}

// executeHandoff runs the delegated agent in isolation and captures results.
func executeHandoff(ctx context.Context, agent *Agent, task string, opts handoffOptions) (string, string, []HandoffTraceItem, error) {
	var trace []HandoffTraceItem
	var response string

	// Run the agent and get the event channel
	events := agent.Run(ctx, task)

	// Capture trace items if requested
	var lastContent string
	var runErr error
	
	for event := range events {
		switch event.Type {
		case EventTypeThinkingChunk:
			if chunk, ok := event.Data["chunk"].(string); ok {
				lastContent += chunk
				if opts.includeTrace {
					trace = append(trace, HandoffTraceItem{
						Type:    "thought",
						Content: chunk,
					})
				}
			}
		case EventTypeActionDetected:
			if opts.includeTrace {
				desc, _ := event.Data["description"].(string)
				toolID, _ := event.Data["tool_id"].(string)
				trace = append(trace, HandoffTraceItem{
					Type:    "tool_call",
					Content: fmt.Sprintf("%s (%s)", desc, toolID),
				})
			}
		case EventTypeActionResult:
			if opts.includeTrace {
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
				if opts.includeTrace {
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
				includeTrace: false,
				maxTurns:     10,
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

			// Execute the handoff
			response, summary, trace, err := executeHandoff(ctx, a, task, handoffOpts)
			if err != nil {
				return nil, err
			}

			// Return result structure
			result := &HandoffResult{
				Response: response,
				Summary:  summary,
				Metadata: make(map[string]any),
			}
			
			if handoffOpts.includeTrace {
				result.Trace = trace
			}

			return result, nil
		}).
		Build()
}
