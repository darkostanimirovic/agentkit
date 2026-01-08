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
	InputField  string
	OutputField string
	// IncludeTrace controls whether sub-agent reasoning/tool trace is captured and returned.
	IncludeTrace bool
	// MaxTraceItems limits the number of trace items returned (0 = default).
	MaxTraceItems int
	// MaxTraceChars limits the total characters across trace items (0 = default).
	MaxTraceChars int
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
		WithParameter(normalized.InputField, String().Required().WithDescription("Input for sub-agent")).
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

	if cfg.InputField == "" {
		cfg.InputField = "input"
	}
	if cfg.OutputField == "" {
		cfg.OutputField = "response"
	}
	if cfg.MaxTraceItems == 0 {
		cfg.MaxTraceItems = 40
	}
	if cfg.MaxTraceChars == 0 {
		cfg.MaxTraceChars = 8000
	}

	return cfg, nil
}

type SubAgentTraceItem struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func subAgentHandler(sub *Agent, cfg SubAgentConfig) ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		message, err := extractSubAgentMessage(args, cfg.InputField)
		if err != nil {
			return nil, err
		}

		finalResponse, finalSummary, trace, err := runSubAgent(ctx, sub, message, cfg)
		if err != nil {
			return nil, err
		}

		result := map[string]any{
			cfg.OutputField: finalResponse,
			"summary":       finalSummary,
		}
		if cfg.IncludeTrace {
			result["trace"] = trace
		}

		return result, nil
	}
}

func extractSubAgentMessage(args map[string]any, inputField string) (string, error) {
	raw, ok := args[inputField]
	if !ok {
		return "", fmt.Errorf("missing required field: %s", inputField)
	}
	message, ok := raw.(string)
	if !ok || message == "" {
		return "", fmt.Errorf("invalid %s: expected non-empty string", inputField)
	}
	return message, nil
}

func appendTrace(items []SubAgentTraceItem, cfg SubAgentConfig, item SubAgentTraceItem) []SubAgentTraceItem {
	if !cfg.IncludeTrace {
		return items
	}
	if cfg.MaxTraceItems > 0 && len(items) >= cfg.MaxTraceItems {
		return items
	}
	if cfg.MaxTraceChars > 0 {
		current := 0
		for _, it := range items {
			current += len(it.Content)
		}
		remaining := cfg.MaxTraceChars - current
		if remaining <= 0 {
			return items
		}
		if len(item.Content) > remaining {
			if remaining > 3 {
				item.Content = item.Content[:remaining-3] + "..."
			} else {
				item.Content = item.Content[:remaining]
			}
		}
	}
	items = append(items, item)
	return items
}

func formatSubAgentResult(cfg SubAgentConfig, result any) string {
	resultMap, ok := result.(map[string]any)
	if !ok {
		return fmt.Sprintf("✓ %s completed", formatToolName(cfg.Name))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("✓ %s completed", formatToolName(cfg.Name)))

	if cfg.IncludeTrace {
		if traceRaw, ok := resultMap["trace"]; ok {
			if traceItems, ok := traceRaw.([]SubAgentTraceItem); ok && len(traceItems) > 0 {
				b.WriteString("\n\nSub-agent trace:")
				for _, item := range traceItems {
					b.WriteString(fmt.Sprintf("\n- %s: %s", strings.ToUpper(item.Type), item.Content))
				}
			} else if traceAny, ok := traceRaw.([]any); ok && len(traceAny) > 0 {
				b.WriteString("\n\nSub-agent trace:")
				for _, raw := range traceAny {
					if m, ok := raw.(map[string]any); ok {
						typ, _ := m["type"].(string)
						content, _ := m["content"].(string)
						if typ == "" && content == "" {
							continue
						}
						b.WriteString(fmt.Sprintf("\n- %s: %s", strings.ToUpper(typ), content))
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
