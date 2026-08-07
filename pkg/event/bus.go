package event

import (
	"context"
	"sync"
	"time"
)

// EventType defines the type of event being dispatched.
type EventType string

const (
	EventAssetUpdated  EventType = "asset:updated"
	EventFindingNew    EventType = "finding:new"
	EventScanProgress  EventType = "scan:progress"
	EventRegressionNew EventType = "regression:new"

	// Job lifecycle, emitted by the headless server as workers claim and
	// complete units of work.
	EventJobEnqueued  EventType = "job:enqueued"
	EventJobStarted   EventType = "job:started"
	EventJobCompleted EventType = "job:completed"

	// EventIngestReceived is emitted once a worker payload has been durably
	// recorded, not when it is received — subscribers may assume persistence.
	EventIngestReceived EventType = "ingest:received"

	// EventDataErased is emitted once the discovered estate has been cleared.
	//
	// It is emitted after the erasure commits, so a subscriber that reloads on
	// receipt cannot read the state it was about to be told had gone. Every
	// view must refresh from the store rather than clear itself: a view that
	// emptied its own state would be asserting the erasure happened, and the
	// two differ precisely when it did not.
	EventDataErased EventType = "data:erased"
)

// Event represents a mutation or state change event.
type Event struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Handler is a function that processes an event.
type Handler func(ctx context.Context, e Event)

// Bus represents an event dispatcher.
type Bus interface {
	Subscribe(eventType EventType, handler Handler)
	Publish(ctx context.Context, e Event)
}

// MemoryBus is a thread-safe, in-memory event bus.
type MemoryBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

// NewMemoryBus creates a new MemoryBus.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		handlers: make(map[EventType][]Handler),
	}
}

// Subscribe registers a handler for a specific event type.
func (b *MemoryBus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish dispatches an event to all registered handlers for its type.
// Handlers are executed synchronously. For async execution, the handler itself should spin up a goroutine.
func (b *MemoryBus) Publish(ctx context.Context, e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	b.mu.RLock()
	handlers, ok := b.handlers[e.Type]
	b.mu.RUnlock()

	if !ok {
		return
	}

	for _, h := range handlers {
		h(ctx, e)
	}
}
