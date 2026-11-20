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
// it. For NVD, and for EPSS when FIRST's per-CVE API is configured, only CVEs
// already referenced by stored findings are queried.
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
	} else if name == feed.EPSS && client.Has(feed.EPSSAPI) {
		// Ask for the scores of the CVEs on stored findings, not the file of
		// every scored CVE. An operator who configured only a mirror of the
		// daily file gets the whole-file path below.
		cves, err := s.referencedCVEs(ctx)
		if err != nil {
			return nil, err
		}
		result, fetchErr = client.FetchEPSSReferenced(ctx, cves)
	} else {
		result, fetchErr = client.Fetch(ctx, name)
	}
	if fetchErr != nil {
		_ = s.recordFetchFailure(ctx, name, time.Now().UTC(), fetchErr)
		return nil, fetchErr
	}
	if result.NotModified {
		return s.confirmUnchanged(ctx, name, epssThreshold)
	}
	return s.ApplySnapshot(ctx, result.Snapshot, epssThreshold)
}

// confirmUnchanged handles a source that reports its content has not changed.
// The consultation is recorded, so the feed is current as of now, and findings
// recorded since the snapshot was applied are evaluated against the entries
// that were stored with it. A feed whose entries were not kept (a catalogue too
// large to store) has nothing to re-evaluate from, and is only confirmed.
func (s *FeedIngestService) confirmUnchanged(ctx context.Context, name feed.Name, epssThreshold float64) ([]store.Regression, error) {
	latest, err := s.store.GetLatestFeedSnapshot(ctx, string(name))
	if err != nil {
		return nil, err
	}
	snapshotID := ""
	if latest != nil {
		snapshotID = latest.ID
	}
	if err := s.store.RecordFeedCheck(ctx, string(name), time.Now().UTC(), snapshotID); err != nil {
		return nil, err
	}
	if err := s.store.ClearFeedFailure(ctx, string(name)); err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, nil
	}
	stored, err := s.store.GetFeedRecords(ctx, latest.ID)
	if err != nil || len(stored) == 0 {
		return nil, err
	}
	return s.evaluate(ctx, s.snapshotFromStore(latest, stored), epssThreshold, true)
}

func (s *FeedIngestService) snapshotFromStore(latest *store.FeedSnapshot, stored []store.FeedRecord) *feed.Snapshot {
	records := make(map[string]feed.Record, len(stored))
	for _, r := range stored {
		records[r.CVE] = feed.Record{CVE: r.CVE, KEVListed: r.KEVListed, EPSS: r.EPSS}
	}
	return &feed.Snapshot{
		ID: latest.ID, Feed: feed.Name(latest.Feed), SourceURL: latest.SourceURL,
		RetrievedAt: latest.RetrievedAt, ContentDigest: latest.ContentDigest,
		RecordCount: latest.RecordCount, Records: records,
	}
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
	candidates, err := s.store.GetCandidateFindings(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, cf := range candidates {
		if cf.CVE != "" {
			cves = append(cves, cf.CVE)
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
	candidates, err := s.store.GetCandidateFindings(ctx, "")
	if err != nil {
		return err
	}
	for _, cf := range candidates {
		if cf.CVE == "" {
			continue
		}
		prev, err := s.previousCandidate(ctx, cf.ID, name)
		if err != nil {
			return err
		}
		if prev != nil && prev.SnapshotID != "" {
			continue
		}
		if err := s.store.SaveCandidateEnrichment(ctx, &store.FindingEnrichment{
			FindingID: cf.ID,
			Feed:      string(name),
			CVE:       cf.CVE,
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
		if statuses[i].Snapshot == nil {
			continue
		}
		// Judged on the last successful consultation, not on when the
		// content first arrived: an unchanged catalogue that was reconfirmed
		// this morning is current. Databases that predate the check record
		// fall back to the snapshot's own time.
		current := statuses[i].Snapshot.RetrievedAt
		if c := statuses[i].LastCheckedAt; c != nil && c.After(current) {
			current = *c
		}
		if now.Sub(current) > stalenessThreshold {
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
		// The catalogue is exactly what is already stored, so there is no new
		// snapshot to write. The consultation still happened, and findings
		// recorded since the last application have not been evaluated against
		// it; refreshing must reach them rather than report nothing to do.
		if latest, err := s.store.GetLatestFeedSnapshot(ctx, string(snapshot.Feed)); err != nil {
			return nil, err
		} else if snapshot.Feed == feed.KEV && latest != nil && latest.ID == existing.ID {
			if kept, err := s.store.GetFeedRecords(ctx, existing.ID); err != nil {
				return nil, err
			} else if len(kept) == 0 {
				// Ingested before entries were kept: backfill from the
				// identical content in hand.
				if err := s.store.SaveFeedRecords(ctx, existing.ID, storedRecords(snapshot)); err != nil {
					return nil, err
				}
			}
		}
		if err := s.store.RecordFeedCheck(ctx, string(snapshot.Feed), snapshot.RetrievedAt, snapshot.ID); err != nil {
			return nil, err
		}
		if err := s.store.ClearFeedFailure(ctx, string(snapshot.Feed)); err != nil {
			return nil, err
		}
		return s.evaluate(ctx, snapshot, epssThreshold, true)
	}

	stored := &store.FeedSnapshot{
		ID: snapshot.ID, Feed: string(snapshot.Feed), SourceURL: snapshot.SourceURL,
		RetrievedAt: snapshot.RetrievedAt, ContentDigest: snapshot.ContentDigest,
		RecordCount: snapshot.RecordCount,
	}
	if snapshot.Feed == feed.KEV {
		stored.Records = storedRecords(snapshot)
	}
	if err := s.store.SaveFeedSnapshot(ctx, stored); err != nil {
		return nil, err
	}
	if err := s.store.RecordFeedCheck(ctx, string(snapshot.Feed), snapshot.RetrievedAt, snapshot.ID); err != nil {
		return nil, err
	}
	if err := s.store.ClearFeedFailure(ctx, string(snapshot.Feed)); err != nil {
		return nil, err
	}
	return s.evaluate(ctx, snapshot, epssThreshold, false)
}

func storedRecords(snapshot *feed.Snapshot) []store.FeedRecord {
	out := make([]store.FeedRecord, 0, len(snapshot.Records))
	for _, r := range snapshot.Records {
		out = append(out, store.FeedRecord{CVE: r.CVE, KEVListed: r.KEVListed, EPSS: r.EPSS})
	}
	return out
}

// evaluate re-assesses stored CVE findings against a snapshot. With
// onlyUnevaluated set, findings already assessed against this very snapshot are
// left alone, and the run is entered in the ledger only if something was
// assessed: reconfirming an unchanged catalogue is not an event in the estate's
// history.
func (s *FeedIngestService) evaluate(ctx context.Context, snapshot *feed.Snapshot, epssThreshold float64, onlyUnevaluated bool) ([]store.Regression, error) {
	// Priority changes the snapshot causes are stamped as the feed's doing and
	// cite the snapshot, so the timeline can say why a finding moved.
	started := time.Now().UTC()
	runID := newRunID()
	ctx = store.WithRun(ctx, store.RunRef{
		ID: runID, Kind: store.RunFeedRefresh, Cause: store.CauseFeed, SnapshotID: snapshot.ID,
	})

	findings, err := s.store.GetFindings(ctx, "")
	if err != nil {
		return nil, err
	}
	var regressions []store.Regression
	evaluated := 0
	for _, finding := range findings {
		if finding.CVE == "" {
			continue
		}
		cve := finding.CVE
		if snapshot.Queried != nil && !snapshot.Queried[cve] {
			// This snapshot was built by asking about specific CVEs and this
			// was not among them (recorded after the request was made).
			// Nothing is known about it, which is not the same as the feed
			// having no entry, so it is left as it was.
			continue
		}
		record, found := snapshot.Records[cve]
		state := store.EnrichmentNotFound
		if found {
			state = store.EnrichmentOK
		}
		previous, err := s.previous(ctx, finding.ID, snapshot.Feed)
		if err != nil {
			return nil, err
		}
		if onlyUnevaluated && previous != nil && previous.SnapshotID == snapshot.ID {
			continue
		}
		evaluated++

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

	candidates, err := s.store.GetCandidateFindings(ctx, "")
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		cf := candidates[i]
		if cf.CVE == "" {
			continue
		}
		cve := cf.CVE
		if snapshot.Queried != nil && !snapshot.Queried[cve] {
			continue
		}
		record, found := snapshot.Records[cve]
		state := store.EnrichmentNotFound
		if found {
			state = store.EnrichmentOK
		}
		previous, err := s.previousCandidate(ctx, cf.ID, snapshot.Feed)
		if err != nil {
			return nil, err
		}
		if onlyUnevaluated && previous != nil && previous.SnapshotID == snapshot.ID {
			continue
		}
		evaluated++

		updateEPSS, updateKEV := false, false
		if snapshot.Feed == feed.EPSS {
			updateEPSS = true
			if record.EPSS != nil {
				cf.EPSS = *record.EPSS
			} else {
				cf.EPSS = 0
			}
		}
		if snapshot.Feed == feed.KEV {
			updateKEV = true
			if record.KEVListed != nil {
				cf.KEVListed = *record.KEVListed
			} else {
				cf.KEVListed = false
			}
		}
		cf.Priority = store.ComputeFindingPriority(cf.Severity, cf.EPSS, cf.KEVListed)
		if err := s.store.SaveCandidateFinding(ctx, &cf); err != nil {
			return nil, err
		}

		checkedAt := snapshot.RetrievedAt
		enrichment := &store.FindingEnrichment{
			FindingID: cf.ID, Feed: string(snapshot.Feed), CVE: cve,
			State: state, SnapshotID: snapshot.ID, CheckedAt: &checkedAt,
		}
		if updateEPSS && found {
			enrichment.EPSS = record.EPSS
		}
		if updateKEV && found {
			enrichment.KEVListed = record.KEVListed
		}
		if err := s.store.SaveCandidateEnrichment(ctx, enrichment); err != nil {
			return nil, err
		}
	}
	if onlyUnevaluated && evaluated == 0 {
		return regressions, nil
	}
	if err := s.store.RecordScanRun(ctx, &store.ScanRun{
		ID: runID, Kind: store.RunFeedRefresh, Outcome: "completed",
		StartedAt: started, FinishedAt: time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("feed: recording the refresh in the run ledger: %w", err)
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

func (s *FeedIngestService) previousCandidate(ctx context.Context, candidateID string, feedName feed.Name) (*store.FindingEnrichment, error) {
	items, err := s.store.GetCandidateEnrichments(ctx, candidateID)
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
