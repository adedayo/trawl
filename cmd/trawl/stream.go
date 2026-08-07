package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/adedayo/trawl/pkg/event"
)

// broadcaster fans the in-process event bus out to connected HTTP clients.
//
// The desktop build pushes bus events straight into the webview over Wails
// IPC. Without an equivalent here the container dashboard would be a polling
// client watching a system that already knows the answer, and "live" would
// mean one thing in one deployment and another in the other.
type broadcaster struct {
	mu      sync.RWMutex
	clients map[chan event.Event]struct{}
}

// streamedEvents are the event types a dashboard is entitled to observe.
//
// It is an allowlist rather than a subscription to everything, because the bus
// also carries ingest payload keys and job internals. A stream that forwarded
// whatever happened to be published would disclose more with every new event
// type somebody added, without anyone deciding that it should.
var streamedEvents = []event.EventType{
	event.EventAssetUpdated,
	event.EventFindingNew,
	event.EventScanProgress,
	event.EventRegressionNew,
	event.EventJobEnqueued,
	event.EventJobStarted,
	event.EventJobCompleted,
	event.EventIngestReceived,
	event.EventDataErased,
}

func newBroadcaster(bus event.Bus) *broadcaster {
	b := &broadcaster{clients: map[chan event.Event]struct{}{}}
	for _, t := range streamedEvents {
		bus.Subscribe(t, b.handle)
	}
	return b
}

// handle forwards one event to every connected client.
//
// A client whose buffer is full is skipped rather than waited for. Bus
// handlers run synchronously on the publishing goroutine, so blocking here
// would let one stalled browser tab halt an assessment in progress — a
// dashboard must never be able to slow down the thing it is watching.
func (b *broadcaster) handle(ctx context.Context, e event.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *broadcaster) add() chan event.Event {
	ch := make(chan event.Event, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broadcaster) remove(ch chan event.Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// handleEventStream serves the bus as Server-Sent Events.
//
// SSE rather than WebSocket: the traffic is one-way, it survives proxies that
// mangle upgrade headers, and browsers reconnect on their own. A bidirectional
// protocol would be additional surface bought for a capability nothing needs.
func (s *server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Proxies that buffer would defeat the point of a stream.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.events.add()
	defer s.events.remove(ch)

	// A periodic comment keeps intermediaries from reaping an idle connection.
	// A quiet estate is the normal state, and a stream that dies during quiet
	// is a stream that is absent exactly when the first event arrives.
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case e := <-ch:
			payload, err := json.Marshal(e)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, payload)
			flusher.Flush()

		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
