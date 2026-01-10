package agentkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrSubAgentNotConfigured is returned when a sub-agent is nil.
var ErrSubAgentNotConfigured = errors.New("agentkit: sub-agent is required")

// SubAgentConfig configures a sub-agent tool.
type SubAgentConfig struct {
	Name        string
	Description string
	// IncludeTrace controls whether sub-agent reasoning/tool trace is captured and returned in the parent's context.
	// When true, the parent agent will receive detailed execution steps from the sub-agent.
	// This consumes additional context window space, so enable only when debugging or when the parent needs to learn from the sub-agent's approach.
	IncludeTrace bool
}

// NewSubAgentTool creates a tool that delegates to a sub-agent.
func NewSubAgentTool(cfg SubAgentConfig, sub *Agent) (Tool, error) {
	if sub == nil {
		return Tool{}, ErrSubAgentNotConfigured
	}

	normalized, err := normalizeSubAgentConfig(cfg)
	if err != nil {
		return Tool{}, err
	}

	handler := subAgentHandler(sub, normalized)
	tool := NewTool(normalized.Name).
		WithDescription(normalized.Description).
		WithParameter("input", String().Required().WithDescription("Task or question for the sub-agent")).
		WithHandler(handler).
		WithResultFormatter(func(_ string, result any) string {
return formatSubAgentResult(normalized, result)
}).
		Build()

	return tool, nil
}

func normalizeSubAgentConfig(cfg SubAgentConfig) (SubAgentConfig, error) {
	if cfg.Name == "" {
		return SubAgentConfig{}, errors.New("agentkit: sub-agent tool name is required")
	}
	return cfg, nil
}

type SubAgentTraceItem struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func subAgentHandler(sub *Agent, cfg SubAgentConfig) ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		message, err := extractSubAgentMessage(args)
		if err != nil {
			return nil, err
		}

		// Create a span for the sub-agent delegation if tracer is available
		var spanCtx context.Context
		var endSpan func()
		if sub.tracer != nil {
			spanCtx, endSpan = sub.tracer.StartSpan(ctx, fmt.Sprintf("sub_agent.%s", cfg.Name))
			defer endSpan()
			
			// Add metadata about the delegation
			sub.tracer.SetSpanAttributes(spanCtx, map[string]any{
"sub_agent_name": cfg.Name,
"input_length":   len(message),
"include_trace":  cfg.IncludeTrace,
})
		} else {
			spanCtx = ctx
		}

		finalResponse, finalSummary, trace, err := runSubAgent(spanCtx, sub, message, cfg)
		if err != nil {
			// Record error in span if tracer exists
			if sub.tracer != nil && spanCtx != nil {
				sub.tracer.SetSpanAttributes(spanCtx, map[string]any{
"error": err.Error(),
				})
			}
			return nil, err
		}

		// Record success metrics in span
		if sub.tracer != nil && spanCtx != nil {
			sub.tracer.SetSpanAttributes(spanCtx, map[string]any{
"response_length": len(finalResponse),
"trace_items":     len(trace),
"has_summary":     finalSummary != "",
})
		}

		// Return the sub-agent's response directly
		// If tracing is enabled, include trace in result formatter for parent visibility
		if cfg.IncludeTrace {
			return map[string]any{
				"response": finalResponse,
				"summary":  finalSummary,
				"trace":    trace,
			}, nil
		}
		return finalResponse, nil
	}
}

func extractSubAgentMessage(args map[string]any) (string, error) {
	raw, ok := args["input"]
	if !ok {
		return "", fmt.Errorf("missing required field: input")
	}
	message, ok := raw.(string)
	if !ok || message == "" {
		return "", errors.New("invalid input: expected non-empty string")
	}
	return message, nil
}

func appendTrace(items []SubAgentTraceItem, cfg SubAgentConfig, item SubAgentTraceItem) []SubAgentTraceItem {
	if !cfg.IncludeTrace {
		return items
	}
	return append(items, item)
}

func formatSubAgentResult(cfg SubAgentConfig, result any) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("✓ %s completed", formatToolName(cfg.Name)))
	
	// If trace is enabled and result is a map, extract and display trace
	if cfg.IncludeTrace {
		if resultMap, ok := result.(map[string]any); ok {
			if traceRaw, ok := resultMap["trace"]; ok {
				if traceItems, ok := traceRaw.([]SubAgentTraceItem); ok && len(traceItems) > 0 {
					b.WriteString("\n\nSub-agent trace:")
					for _, item := range traceItems {
						b.WriteString(fmt.Sprintf("\n- %s: %s", strings.ToUpper(item.Type), item.Content))
					}
				}
			}
		}
	}

	return b.String()
}

func runSubAgent(ctx context.Context, sub *Agent, message string, cfg SubAgentConfig) (string, string, []SubAgentTraceItem, error) {
	events := sub.Run(ctx, message)
	var finalResponse string
	var finalSummary string
	trace := make([]SubAgentTraceItem, 0, 8)
	var narrative strings.Builder
	flushNarrative := func() {
		text := strings.TrimSpace(narrative.String())
		if text != "" {
			trace = appendTrace(trace, cfg, SubAgentTraceItem{
Type:    "reasoning",
Content: text,
})
			narrative.Reset()
		}
	}
	for event := range events {
		switch event.Type {
		case EventTypeFinalOutput:
			if response, ok := event.Data["response"].(string); ok {
				finalResponse = response
			}
			if summary, ok := event.Data["summary"].(string); ok {
				finalSummary = summary
			}
		case EventTypeError:
			if errMsg, ok := event.Data["error"].(string); ok && errMsg != "" {
				return "", "", trace, fmt.Errorf("sub-agent error: %s", errMsg)
			}
			return "", "", trace, errors.New("sub-agent error")
		case EventTypeThinkingChunk:
			if chunk, ok := event.Data["chunk"].(string); ok {
				narrative.WriteString(chunk)
			}
		case EventTypeActionDetected:
			flushNarrative()
			if desc, ok := event.Data["description"].(string); ok && desc != "" {
				trace = appendTrace(trace, cfg, SubAgentTraceItem{
Type:    "tool_call",
Content: desc,
})
			}
		case EventTypeActionResult:
			flushNarrative()
			if desc, ok := event.Data["description"].(string); ok && desc != "" {
				trace = appendTrace(trace, cfg, SubAgentTraceItem{
Type:    "tool_result",
Content: desc,
})
			}
		case EventTypeProgress:
			flushNarrative()
			if desc, ok := event.Data["description"].(string); ok && desc != "" {
				trace = appendTrace(trace, cfg, SubAgentTraceItem{
Type:    "progress",
Content: desc,
})
			}
		case EventTypeDecision:
			flushNarrative()
			action, _ := event.Data["action"].(string)
			reasoning, _ := event.Data["reasoning"].(string)
			content := strings.TrimSpace(strings.Join([]string{action, reasoning}, " — "))
			if content != "" {
				trace = appendTrace(trace, cfg, SubAgentTraceItem{
Type:    "decision",
Content: content,
})
			}
		}
	}

	if finalResponse == "" {
		return "", "", trace, errors.New("sub-agent returned no response")
	}

	flushNarrative()
	return finalResponse, finalSummary, trace, nil
}

// AddSubAgent registers a sub-agent as a tool on the agent.
func (a *Agent) AddSubAgent(name string, sub *Agent) error {
	tool, err := NewSubAgentTool(SubAgentConfig{
Name:        name,
Description: fmt.Sprintf("Delegate to %s agent", name),
}, sub)
	if err != nil {
		return err
	}

	a.AddTool(tool)
	return nil
}
