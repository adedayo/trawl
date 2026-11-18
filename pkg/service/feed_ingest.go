package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/store"
)

// DefaultFeedStalenessThreshold is the age beyond which a snapshot is marked
// stale at every point of use unless deployment configuration overrides it.
const DefaultFeedStalenessThreshold = 48 * time.Hour

// FeedIngestService applies complete, already-validated catalogue snapshots.
// Fetching and applying are separate so an air-gapped installation can use the
// same path with a file while a scheduled worker uses an HTTP client.
type FeedIngestService struct {
	store store.Store
}

func NewFeedIngestService(s store.Store) *FeedIngestService {
	return &FeedIngestService{store: s}
}

// FetchAndApply retrieves one catalogue from the configured client and applies
// it. For NVD, only CVEs already referenced by stored findings are queried.
//
// When a fetch fails, no snapshot is stored, the failure is recorded against
// the feed, and any finding that had never been checked against this feed is
// marked check_failed while findings already covered by a previous snapshot
// keep their prior snapshot provenance intact.
func (s *FeedIngestService) FetchAndApply(ctx context.Context, client *feed.Client, name feed.Name, epssThreshold float64) ([]store.Regression, error) {
	if client == nil {
		return nil, fmt.Errorf("feed client is nil")
	}

	var result *feed.FetchResult
	var fetchErr error
	if name == feed.NVD {
		cves, err := s.referencedCVEs(ctx)
		if err != nil {
			return nil, err
		}
		result, fetchErr = client.FetchNVDReferenced(ctx, cves)
	} else {
		result, fetchErr = client.Fetch(ctx, name)
	}
	if fetchErr != nil {
		_ = s.recordFetchFailure(ctx, name, time.Now().UTC(), fetchErr)
		return nil, fetchErr
	}
	if result.NotModified {
		_ = s.store.ClearFeedFailure(ctx, string(name))
		return nil, nil
	}
	return s.ApplySnapshot(ctx, result.Snapshot, epssThreshold)
}

func (s *FeedIngestService) referencedCVEs(ctx context.Context) ([]string, error) {
	findings, err := s.store.GetFindings(ctx, "")
	if err != nil {
		return nil, err
	}
	var cves []string
	for _, f := range findings {
		if f.CVE != "" {
			cves = append(cves, f.CVE)
		}
	}
	return cves, nil
}

func (s *FeedIngestService) recordFetchFailure(ctx context.Context, name feed.Name, failedAt time.Time, fetchErr error) error {
	if err := s.store.RecordFeedFailure(ctx, string(name), failedAt, fetchErr.Error()); err != nil {
		return err
	}
	findings, err := s.store.GetFindings(ctx, "")
	if err != nil {
		return err
	}
	for _, finding := range findings {
		if finding.CVE == "" {
			continue
		}
		prev, err := s.previous(ctx, finding.ID, name)
		if err != nil {
			return err
		}
		// Preserve any previous snapshot in force: only findings with no
		// snapshot yet move from not_checked to check_failed.
		if prev != nil && prev.SnapshotID != "" {
			continue
		}
		if err := s.store.SaveFindingEnrichment(ctx, &store.FindingEnrichment{
			FindingID: finding.ID,
			Feed:      string(name),
			CVE:       finding.CVE,
			State:     store.EnrichmentCheckFailed,
			CheckedAt: &failedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

// FeedStatuses returns the operational state of each feed with staleness
// computed against stalenessThreshold (defaulting to 48 hours if <= 0).
func (s *FeedIngestService) FeedStatuses(ctx context.Context, now time.Time, stalenessThreshold time.Duration) ([]store.FeedStatus, error) {
	if stalenessThreshold <= 0 {
		stalenessThreshold = DefaultFeedStalenessThreshold
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	statuses, err := s.store.GetFeedStatuses(ctx)
	if err != nil {
		return nil, err
	}
	for i := range statuses {
		if statuses[i].Snapshot != nil && now.Sub(statuses[i].Snapshot.RetrievedAt) > stalenessThreshold {
			statuses[i].Stale = true
		}
	}
	return statuses, nil
}

// ApplySnapshot persists a snapshot and re-evaluates existing CVE findings.
// Priority is recomputed from the updated catalogue inputs by
// store.ComputeFindingPriority rather than written directly by any caller.
func (s *FeedIngestService) ApplySnapshot(ctx context.Context, snapshot *feed.Snapshot, epssThreshold float64) ([]store.Regression, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("feed snapshot is nil")
	}
	if existing, err := s.store.GetFeedSnapshot(ctx, snapshot.ID); err != nil {
		return nil, err
	} else if existing != nil {
		_ = s.store.ClearFeedFailure(ctx, string(snapshot.Feed))
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
	if err := s.store.ClearFeedFailure(ctx, string(snapshot.Feed)); err != nil {
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
		effectiveEPSS := finding.EPSS
		if updateEPSS {
			effectiveEPSS = epss
		}
		effectiveKEV := finding.KEVListed
		if updateKEV {
			effectiveKEV = kev
		}
		priority := store.ComputeFindingPriority(finding.Severity, effectiveEPSS, effectiveKEV)
		if err := s.store.UpdateFindingCatalogue(ctx, finding.ID, epss, kev, priority, updateEPSS, updateKEV); err != nil {
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
