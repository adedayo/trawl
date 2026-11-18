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
	RecordFeedFailure(ctx context.Context, feed string, failedAt time.Time, reason string) error
	ClearFeedFailure(ctx context.Context, feed string) error
	GetFeedStatuses(ctx context.Context) ([]FeedStatus, error)
	SaveFindingEnrichment(context.Context, *FindingEnrichment) error
	GetFindingEnrichments(context.Context, string) ([]FindingEnrichment, error)
	UpdateFindingCatalogue(ctx context.Context, findingID string, epss float64, kev bool, priority string, updateEPSS, updateKEV bool) error
	RecordFeedRegression(context.Context, string, string, string, string, string) (*Regression, error)
}
