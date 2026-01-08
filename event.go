package agentkit

import "time"

// EventType represents the type of streaming event
type EventType string

const (
	EventTypeThinkingChunk    EventType = "thinking_chunk"
	EventTypeActionDetected   EventType = "action_detected"
	EventTypeActionResult     EventType = "action_result"
	EventTypeProgress         EventType = "progress"
	EventTypeDecision         EventType = "decision"
	EventTypeFinalOutput      EventType = "final_output"
	EventTypeError            EventType = "error"
	EventTypeApprovalRequired EventType = "approval_required"
	EventTypeApprovalGranted  EventType = "approval_granted"
	EventTypeApprovalDenied   EventType = "approval_denied"
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

// ActionDetected creates an action detected event
func ActionDetected(description, toolID string) Event {
	return NewEvent(EventTypeActionDetected, map[string]any{
		"description": description,
		"tool_id":     toolID,
	})
}

// ActionResult creates an action result event
func ActionResult(description string, result interface{}) Event {
	return NewEvent(EventTypeActionResult, map[string]any{
		"description": description,
		"result":      result,
	})
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
