package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/event"
)

func TestMemoryBus_PubSub(t *testing.T) {
	bus := event.NewMemoryBus()

	received := make(chan event.Event, 1)

	bus.Subscribe(event.EventAssetUpdated, func(ctx context.Context, e event.Event) {
		received <- e
	})

	e := event.Event{
		Type:    event.EventAssetUpdated,
		Payload: "asset-123",
	}

	bus.Publish(context.Background(), e)

	select {
	case rec := <-received:
		if rec.Type != event.EventAssetUpdated {
			t.Errorf("expected type %s, got %s", event.EventAssetUpdated, rec.Type)
		}
		if rec.Payload != "asset-123" {
			t.Errorf("expected payload 'asset-123', got %v", rec.Payload)
		}
		if rec.Timestamp.IsZero() {
			t.Errorf("expected timestamp to be set")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}
