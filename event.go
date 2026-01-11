package agentkit

import (
	"sync"
	"time"
)

// EventType represents the type of streaming event
type EventType string

const (
	// Content streaming events
	EventTypeThinkingChunk EventType = "thinking_chunk"
	EventTypeFinalOutput   EventType = "final_output"

	// Agent lifecycle events
	EventTypeAgentStart    EventType = "agent.start"
	EventTypeAgentComplete EventType = "agent.complete"

	// Tool execution events
	EventTypeActionDetected EventType = "action_detected"
	EventTypeActionResult   EventType = "action_result"

	// Multi-agent coordination events
	EventTypeHandoffStart                EventType = "handoff.start"
	EventTypeHandoffComplete             EventType = "handoff.complete"
	EventTypeCollaborationAgentMessage   EventType = "collaboration.agent.contribution"

	// Human-in-the-loop events
	EventTypeApprovalRequired EventType = "approval_required"
	EventTypeApprovalGranted  EventType = "approval_granted"
	EventTypeApprovalDenied   EventType = "approval_denied"

	// Progress and decision events
	EventTypeProgress EventType = "progress"
	EventTypeDecision EventType = "decision"

	// Error events
	EventTypeError EventType = "error"
)

// Event represents a streaming event emitted during agent execution
type Event struct {
	Type      EventType      `json:"type"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
	TraceID   string         `json:"trace_id,omitempty"`
	SpanID    string         `json:"span_id,omitempty"`
}

// NewEvent creates a new event with the current timestamp
func NewEvent(eventType EventType, data map[string]any) Event {
	return Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// ThinkingChunk creates a thinking chunk event
func ThinkingChunk(chunk string) Event {
	return NewEvent(EventTypeThinkingChunk, map[string]any{
		"chunk": chunk,
	})
}

// Thinking creates a thinking event (alias for ThinkingChunk)
func Thinking(content string) Event {
	return ThinkingChunk(content)
}

// ActionDetected creates an action detected event
func ActionDetected(description, toolID string) Event {
	return NewEvent(EventTypeActionDetected, map[string]any{
		"description": description,
		"tool_id":     toolID,
	})
}

// ActionResult creates an action result event
func ActionResult(description string, result any) Event {
	return NewEvent(EventTypeActionResult, map[string]any{
		"description": description,
		"result":      result,
	})
}

// ToolResult creates a tool result event (alias for ActionResult)
func ToolResult(toolName string, result any) Event {
	return ActionResult(toolName, result)
}

// FinalOutput creates a final output event
func FinalOutput(summary, response string) Event {
	return NewEvent(EventTypeFinalOutput, map[string]any{
		"summary":  summary,
		"response": response,
	})
}

// Error creates an error event
func Error(err error) Event {
	return NewEvent(EventTypeError, map[string]any{
		"error": err.Error(),
	})
}

// ToolError creates a tool execution error event
func ToolError(toolName string, err error) Event {
	return NewEvent(EventTypeError, map[string]any{
		"tool_name": toolName,
		"error":     err.Error(),
	})
}

// Progress creates a progress event
func Progress(iteration, maxIterations int, description string) Event {
	return NewEvent(EventTypeProgress, map[string]any{
		"iteration":      iteration,
		"max_iterations": maxIterations,
		"description":    description,
	})
}

// Decision creates a decision event
func Decision(action string, confidence float64, reasoning string) Event {
	return NewEvent(EventTypeDecision, map[string]any{
		"action":     action,
		"confidence": confidence,
		"reasoning":  reasoning,
	})
}

// ApprovalRequired creates an approval required event
func ApprovalRequired(request ApprovalRequest) Event {
	return NewEvent(EventTypeApprovalRequired, map[string]any{
		"tool_name":       request.ToolName,
		"arguments":       request.Arguments,
		"description":     request.Description,
		"conversation_id": request.ConversationID,
		"call_id":         request.CallID,
	})
}

// ApprovalNeeded is an alias for ApprovalRequired
func ApprovalNeeded(request ApprovalRequest) Event {
	return ApprovalRequired(request)
}

// ApprovalGranted creates an approval granted event
func ApprovalGranted(toolName, callID string) Event {
	return NewEvent(EventTypeApprovalGranted, map[string]any{
		"tool_name": toolName,
		"call_id":   callID,
	})
}

// ApprovalDenied creates an approval denied event
func ApprovalDenied(toolName, callID, reason string) Event {
	return NewEvent(EventTypeApprovalDenied, map[string]any{
		"tool_name": toolName,
		"call_id":   callID,
		"reason":    reason,
	})
}

// ApprovalRejected creates an approval rejected event
func ApprovalRejected(request ApprovalRequest) Event {
	return ApprovalDenied(request.ToolName, request.CallID, "User rejected")
}

// AgentStart creates an agent start event
func AgentStart(agentName string) Event {
	return NewEvent(EventTypeAgentStart, map[string]any{
		"agent_name": agentName,
	})
}

// AgentComplete creates an agent complete event
func AgentComplete(agentName, output string, totalTokens, iterations int, durationMs int64) Event {
	return NewEvent(EventTypeAgentComplete, map[string]any{
		"agent_name":   agentName,
		"output":       output,
		"total_tokens": totalTokens,
		"iterations":   iterations,
		"duration_ms":  durationMs,
	})
}

// HandoffStart creates a handoff start event
func HandoffStart(fromAgent, toAgent, task string) Event {
	return NewEvent(EventTypeHandoffStart, map[string]any{
		"from_agent": fromAgent,
		"to_agent":   toAgent,
		"task":       task,
	})
}

// HandoffComplete creates a handoff complete event
func HandoffComplete(fromAgent, toAgent, result string) Event {
	return NewEvent(EventTypeHandoffComplete, map[string]any{
		"from_agent": fromAgent,
		"to_agent":   toAgent,
		"result":     result,
	})
}

// CollaborationAgentContribution creates a collaboration agent contribution event
func CollaborationAgentContribution(agentName, contribution string) Event {
	return NewEvent(EventTypeCollaborationAgentMessage, map[string]any{
		"agent_name":   agentName,
		"contribution": contribution,
	})
}

// FilterEvents forwards only events with matching types.
func FilterEvents(input <-chan Event, types ...EventType) <-chan Event {
	out := make(chan Event)
	if len(types) == 0 {
		go func() {
			defer close(out)
			for event := range input {
				out <- event
			}
		}()
		return out
	}

	allowed := make(map[EventType]struct{}, len(types))
	for _, typ := range types {
		allowed[typ] = struct{}{}
	}

	go func() {
		defer close(out)
		for event := range input {
			if _, ok := allowed[event.Type]; ok {
				out <- event
			}
		}
	}()

	return out
}

// EventRecorder captures events for replay or inspection.
type EventRecorder struct {
	mu     sync.Mutex
	events []Event
}

// NewEventRecorder creates a new recorder.
func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

// Record captures events while forwarding them.
func (r *EventRecorder) Record(input <-chan Event) <-chan Event {
	out := make(chan Event)

	go func() {
		defer close(out)
		for event := range input {
			r.mu.Lock()
			r.events = append(r.events, event)
			r.mu.Unlock()
			out <- event
		}
	}()

	return out
}

// Events returns a copy of recorded events.
func (r *EventRecorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	copied := make([]Event, len(r.events))
	copy(copied, r.events)
	return copied
}
