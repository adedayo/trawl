package scanner

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/google/uuid"
	"github.com/projectdiscovery/subfinder/v2/pkg/runner"
)

type NetworkScanner struct {
	store    store.Store
	eventBus event.Bus
}

func NewNetworkScanner(s store.Store, eb event.Bus) *NetworkScanner {
	return &NetworkScanner{
		store:    s,
		eventBus: eb,
	}
}

// writerAdapter intercepts subfinder output and saves to store
type writerAdapter struct {
	store    store.Store
	eventBus event.Bus
	ctx      context.Context
	buffer   bytes.Buffer
}

func (w *writerAdapter) Write(p []byte) (n int, err error) {
	// Subfinder outputs one subdomain per line
	str := string(p)
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		subdomain := strings.TrimSpace(line)
		if subdomain == "" {
			continue
		}

		// Save to SQLite
		asset := store.Asset{
			ID:              uuid.New().String(),
			Type:            "domain",
			Value:           subdomain,
			Status:          "active",
			DiscoverySource: "subfinder",
			FirstSeen:       time.Now(),
			LastSeen:        time.Now(),
		}
		_ = w.store.SaveAsset(w.ctx, &asset)

		if w.eventBus != nil {
			w.eventBus.Publish(w.ctx, event.Event{
				Type:    event.EventAssetUpdated,
				Payload: asset,
			})
		}
	}
	return len(p), nil
}

// DiscoverSubdomains runs subfinder natively and stores results.
func (n *NetworkScanner) DiscoverSubdomains(ctx context.Context, domain string) error {
	options := &runner.Options{
		Threads:            10,
		Timeout:            30,
		MaxEnumerationTime: 10,
	}

	// Disable standard output formatting in options to just get the raw subdomains
	// options.NoColor = true
	// options.Silent = true

	subfinderRunner, err := runner.NewRunner(options)
	if err != nil {
		return err
	}

	writer := &writerAdapter{
		store:    n.store,
		eventBus: n.eventBus,
		ctx:      ctx,
	}

	_, err = subfinderRunner.EnumerateSingleDomainWithCtx(ctx, domain, []io.Writer{writer})
	return err
}
