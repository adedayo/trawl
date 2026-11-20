package core

import (
	"context"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/service"
)

// The methods below are the application layer for Change 025's read model.
// Both transports forward to them, so the desktop and container deployments
// cannot differ in what they can show.

// FindingIndex lists findings across every source.
func (c *Core) FindingIndex(ctx context.Context, filter service.FindingFilter) ([]service.FindingSummary, error) {
	return c.findings.Index(ctx, filter)
}

// FindingDetail returns the drill-down for one finding, or
// service.ErrFindingNotFound.
func (c *Core) FindingDetail(ctx context.Context, key string) (service.FindingDetail, error) {
	return c.findings.Detail(ctx, key)
}

// FindingChanges reports what moved between two runs. Either id may be empty:
// an empty `to` is the latest run and an empty `from` the run before `to`.
func (c *Core) FindingChanges(ctx context.Context, from, to string) (service.ChangeSet, error) {
	return c.findings.Changes(ctx, from, to)
}

// ScanRuns lists the run ledger, newest first.
func (c *Core) ScanRuns(ctx context.Context) ([]service.ScanRunView, error) {
	return c.findings.Scans(ctx)
}

// HistorySummary returns one point per recorded run from `since` (RFC 3339; empty
// for all of history), oldest first.
func (c *Core) HistorySummary(ctx context.Context, since string) ([]service.HistoryPoint, error) {
	var from time.Time
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return nil, fmt.Errorf("core: since %q is not an RFC 3339 time: %w", since, err)
		}
		from = t
	}
	return c.findings.History(ctx, from)
}
