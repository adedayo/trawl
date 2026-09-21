package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/store"
)

// FeedIngestService applies complete, already-validated catalogue snapshots.
// Fetching and applying are separate so an air-gapped installation can use the
// same path with a file while a scheduled worker uses an HTTP client.
type FeedIngestService struct {
	store store.Store
}

func NewFeedIngestService(s store.Store) *FeedIngestService {
	return &FeedIngestService{store: s}
}

// ApplySnapshot persists a snapshot and re-evaluates existing CVE findings.
// Priority is intentionally untouched: it remains a consumer-side pure
// function of the updated catalogue inputs.
func (s *FeedIngestService) ApplySnapshot(ctx context.Context, snapshot *feed.Snapshot, epssThreshold float64) ([]store.Regression, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("feed snapshot is nil")
	}
	if existing, err := s.store.GetFeedSnapshot(ctx, snapshot.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, nil
	}

	stored := &store.FeedSnapshot{
		ID: snapshot.ID, Feed: string(snapshot.Feed), SourceURL: snapshot.SourceURL,
		RetrievedAt: snapshot.RetrievedAt, ContentDigest: snapshot.ContentDigest,
		RecordCount: snapshot.RecordCount,
	}
	if err := s.store.SaveFeedSnapshot(ctx, stored); err != nil {
		return nil, err
	}

	findings, err := s.store.GetFindings(ctx, "")
	if err != nil {
		return nil, err
	}
	var regressions []store.Regression
	for _, finding := range findings {
		if finding.CVE == "" {
			continue
		}
		cve := finding.CVE
		record, found := snapshot.Records[cve]
		state := store.EnrichmentNotFound
		if found {
			state = store.EnrichmentOK
		}
		previous, err := s.previous(ctx, finding.ID, snapshot.Feed)
		if err != nil {
			return nil, err
		}

		var epss float64
		var kev bool
		updateEPSS, updateKEV := false, false
		if snapshot.Feed == feed.EPSS {
			updateEPSS = true
			if record.EPSS != nil {
				epss = *record.EPSS
			}
		}
		if snapshot.Feed == feed.KEV {
			updateKEV = true
			if record.KEVListed != nil {
				kev = *record.KEVListed
			}
		}
		if err := s.store.UpdateFindingCatalogue(ctx, finding.ID, epss, kev, updateEPSS, updateKEV); err != nil {
			return nil, err
		}

		checkedAt := snapshot.RetrievedAt
		enrichment := &store.FindingEnrichment{
			FindingID: finding.ID, Feed: string(snapshot.Feed), CVE: cve,
			State: state, SnapshotID: snapshot.ID, CheckedAt: &checkedAt,
		}
		if updateEPSS && found {
			enrichment.EPSS = record.EPSS
		}
		if updateKEV && found {
			enrichment.KEVListed = record.KEVListed
		}
		if err := s.store.SaveFindingEnrichment(ctx, enrichment); err != nil {
			return nil, err
		}

		if snapshot.Feed == feed.KEV && found && record.KEVListed != nil && *record.KEVListed &&
			(previous == nil || previous.KEVListed == nil || !*previous.KEVListed) {
			regression, err := s.store.RecordFeedRegression(ctx, finding.AssetID, string(snapshot.Feed), cve,
				"not-listed", "listed")
			if err != nil {
				return nil, err
			}
			regressions = append(regressions, *regression)
		}
		if snapshot.Feed == feed.EPSS && found && record.EPSS != nil && epssThreshold > 0 &&
			(previous == nil || previous.EPSS == nil || *previous.EPSS < epssThreshold) && *record.EPSS >= epssThreshold {
			regression, err := s.store.RecordFeedRegression(ctx, finding.AssetID, string(snapshot.Feed), cve,
				previousEPSS(previous), strconv.FormatFloat(*record.EPSS, 'f', -1, 64))
			if err != nil {
				return nil, err
			}
			regressions = append(regressions, *regression)
		}
	}
	return regressions, nil
}

func (s *FeedIngestService) previous(ctx context.Context, findingID string, feedName feed.Name) (*store.FindingEnrichment, error) {
	items, err := s.store.GetFindingEnrichments(ctx, findingID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Feed == string(feedName) {
			return &items[i], nil
		}
	}
	return nil, nil
}

func previousEPSS(previous *store.FindingEnrichment) string {
	if previous == nil || previous.EPSS == nil {
		return "not_checked"
	}
	return strconv.FormatFloat(*previous.EPSS, 'f', -1, 64)
}
