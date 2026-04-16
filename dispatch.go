package dispatch

// EventFunc is the handler type for event subscriptions.
// ctx is whatever the emitter passed; subscribers are responsible for
// asserting the concrete type they expect.
type EventFunc func(ctx any, payload any) error

// NewEventBus creates a new, empty EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		declared:    make(map[string]bool),
		subscribers: make(map[string][]EventFunc),
	}
}
