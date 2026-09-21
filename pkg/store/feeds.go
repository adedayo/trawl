package store

import "context"

// FeedStore persists immutable feed snapshots and their per-finding results.
// A backend must not replace a previous snapshot when a new fetch is partial.
type FeedStore interface {
	SaveFeedSnapshot(context.Context, *FeedSnapshot) error
	GetFeedSnapshot(context.Context, string) (*FeedSnapshot, error)
	GetLatestFeedSnapshot(context.Context, string) (*FeedSnapshot, error)
	SaveFindingEnrichment(context.Context, *FindingEnrichment) error
	GetFindingEnrichments(context.Context, string) ([]FindingEnrichment, error)
	UpdateFindingCatalogue(context.Context, string, float64, bool, bool, bool) error
	RecordFeedRegression(context.Context, string, string, string, string, string) (*Regression, error)
}
