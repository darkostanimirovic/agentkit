package agentkit

import "sync"

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
