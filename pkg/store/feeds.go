package store

import (
	"context"
	"time"
)

// FeedStore persists immutable feed snapshots and their per-finding results.
// A backend must not replace a previous snapshot when a new fetch is partial.
type FeedStore interface {
	SaveFeedSnapshot(context.Context, *FeedSnapshot) error
	GetFeedSnapshot(context.Context, string) (*FeedSnapshot, error)
	GetLatestFeedSnapshot(context.Context, string) (*FeedSnapshot, error)
	// GetFeedRecords returns the stored entries of a snapshot, empty when none
	// were kept. Only the latest snapshot of a feed retains its entries.
	GetFeedRecords(ctx context.Context, snapshotID string) ([]FeedRecord, error)
	// SaveFeedRecords stores the entries of a snapshot already held without
	// them, as every snapshot ingested before entries were kept is.
	SaveFeedRecords(ctx context.Context, snapshotID string, records []FeedRecord) error
	// RecordFeedCheck notes a successful consultation of the source, including
	// one that found the content unchanged.
	RecordFeedCheck(ctx context.Context, feed string, checkedAt time.Time, snapshotID string) error
	RecordFeedFailure(ctx context.Context, feed string, failedAt time.Time, reason string) error
	ClearFeedFailure(ctx context.Context, feed string) error
	GetFeedStatuses(ctx context.Context) ([]FeedStatus, error)
	SaveFindingEnrichment(context.Context, *FindingEnrichment) error
	GetFindingEnrichments(context.Context, string) ([]FindingEnrichment, error)
	UpdateFindingCatalogue(ctx context.Context, findingID string, epss float64, kev bool, priority string, updateEPSS, updateKEV bool) error
	RecordFeedRegression(context.Context, string, string, string, string, string) (*Regression, error)
}
