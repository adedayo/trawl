package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/store"
)

// CandidateRefreshSummary reports what an explicit candidate lookup refresh
// queried, reused from cache, opened/updated, and resolved.
type CandidateRefreshSummary struct {
	RunID        string `json:"runId"`
	SnapshotID   string `json:"snapshotId,omitempty"`
	Queried      int    `json:"queried"`
	CacheHits    int    `json:"cacheHits"`
	Candidates   int    `json:"candidates"`
	Resolved     int    `json:"resolved"`
	SkippedNoVer int    `json:"skippedVersionUnknown"`
	SkippedUnmap int    `json:"skippedUnmapped"`
}

// CandidateLookupService reconciles observed, mapped, versioned software
// against NVD 2.0 to produce unconfirmed candidate findings.
//
// It never runs at startup or on ingest; lookups happen only when an operator
// (or scheduled job) explicitly invokes RefreshCandidates.
type CandidateLookupService struct {
	store store.Store
}

func NewCandidateLookupService(s store.Store) *CandidateLookupService {
	return &CandidateLookupService{store: s}
}

// RefreshCandidates looks up NVD CVEs for every distinct current mapped
// product+version across active assets, caching by (cpePart, vendor, product,
// version) unless force is true, and reconciles candidate_findings through the
// single write path with Change 025 event history.
func (s *CandidateLookupService) RefreshCandidates(ctx context.Context, client *feed.Client, force bool) (*CandidateRefreshSummary, error) {
	if client == nil {
		return nil, fmt.Errorf("candidate lookup: feed client is nil")
	}

	started := time.Now().UTC()
	runID := newRunID()
	summary := &CandidateRefreshSummary{RunID: runID}

	allSoftware, err := s.store.GetAssetSoftware(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("candidate lookup: reading software inventory: %w", err)
	}

	// Group current software per asset and collect distinct mapped+versioned tuples.
	currentByAsset := map[string][]store.AssetSoftware{}
	distinctQueries := map[string]feed.SoftwareQuery{}
	for _, sw := range allSoftware {
		if sw.Status != store.SoftwareCurrent {
			continue
		}
		currentByAsset[sw.AssetID] = append(currentByAsset[sw.AssetID], sw)
		if sw.MappingState != store.SoftwareMapped {
			summary.SkippedUnmap++
			continue
		}
		if sw.VersionState != store.SoftwareVersionKnown || sw.Version == "" {
			summary.SkippedNoVer++
			continue
		}
		q := feed.SoftwareQuery{
			CPEPart: sw.CPEPart,
			Vendor:  sw.Vendor,
			Product: sw.Product,
			Version: sw.Version,
		}
		distinctQueries[q.Key()] = q
	}

	// Consult cache for each distinct (part, vendor, product, version).
	cached := make(map[string]*store.SoftwareLookupCache, len(distinctQueries))
	var toQuery []feed.SoftwareQuery
	for key, q := range distinctQueries {
		entry, err := s.store.GetSoftwareLookupCache(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("candidate lookup: reading cache for %s: %w", key, err)
		}
		if entry != nil && !force {
			cached[key] = entry
			summary.CacheHits++
		} else {
			toQuery = append(toQuery, q)
		}
	}

	// Query NVD for cache misses (or all tuples when force=true).
	if len(toQuery) > 0 {
		bundle, err := client.FetchNVDBySoftware(ctx, toQuery)
		if err != nil {
			failedAt := time.Now().UTC()
			_ = s.store.RecordFeedFailure(ctx, store.FeedNVD, failedAt, err.Error())
			for assetID := range currentByAsset {
				_ = s.store.MarkAssetCandidatesNotReassessed(ctx, assetID, "NVD candidate lookup failed; previously stored candidates retained without re-checking.")
			}
			return nil, fmt.Errorf("candidate lookup: %w", err)
		}
		summary.Queried = len(toQuery)
		summary.SnapshotID = bundle.Snapshot.ID

		existingSnap, err := s.store.GetFeedSnapshot(ctx, bundle.Snapshot.ID)
		if err != nil {
			return nil, fmt.Errorf("candidate lookup: checking snapshot: %w", err)
		}
		if existingSnap == nil {
			if err := s.store.SaveFeedSnapshot(ctx, &store.FeedSnapshot{
				ID:            bundle.Snapshot.ID,
				Feed:          string(bundle.Snapshot.Feed),
				SourceURL:     bundle.Snapshot.SourceURL,
				RetrievedAt:   bundle.Snapshot.RetrievedAt,
				ContentDigest: bundle.Snapshot.ContentDigest,
				RecordCount:   bundle.Snapshot.RecordCount,
			}); err != nil {
				return nil, fmt.Errorf("candidate lookup: saving snapshot: %w", err)
			}
		}
		_ = s.store.RecordFeedCheck(ctx, store.FeedNVD, bundle.Snapshot.RetrievedAt, bundle.Snapshot.ID)
		_ = s.store.ClearFeedFailure(ctx, store.FeedNVD)

		for _, q := range toQuery {
			k := q.Key()
			cves := bundle.ByKey[k]
			entry := &store.SoftwareLookupCache{
				ID:          k,
				CPEPart:     q.CPEPart,
				Vendor:      q.Vendor,
				Product:     q.Product,
				Version:     q.Version,
				SnapshotID:  bundle.Snapshot.ID,
				Fingerprint: lookupFingerprint(k, cves),
				QueriedAt:   bundle.Snapshot.RetrievedAt,
				CVEs:        cves,
			}
			if err := s.store.SaveSoftwareLookupCache(ctx, entry); err != nil {
				return nil, fmt.Errorf("candidate lookup: caching %s: %w", k, err)
			}
			cached[k] = entry
		}
	}

	// Reconcile candidate findings through the single write path.
	runCtx := store.WithRun(ctx, store.RunRef{
		ID:         runID,
		Kind:       store.RunCandidateRefresh,
		Cause:      store.CauseEstate,
		SnapshotID: summary.SnapshotID,
	})

	existingCandidates, err := s.store.GetCandidateFindings(runCtx, "")
	if err != nil {
		return nil, fmt.Errorf("candidate lookup: reading existing candidates: %w", err)
	}
	existingByID := make(map[string]store.CandidateFinding, len(existingCandidates))
	existingByAsset := make(map[string][]store.CandidateFinding)
	for _, cf := range existingCandidates {
		existingByID[cf.ID] = cf
		existingByAsset[cf.AssetID] = append(existingByAsset[cf.AssetID], cf)
	}

	// Deterministic asset ordering.
	assetIDs := make([]string, 0, len(currentByAsset))
	for assetID := range currentByAsset {
		assetIDs = append(assetIDs, assetID)
	}
	sort.Strings(assetIDs)

	for _, assetID := range assetIDs {
		swList := currentByAsset[assetID]
		desired := map[string]bool{}
		hasMappedVersioned := false

		for _, sw := range swList {
			if sw.MappingState != store.SoftwareMapped || sw.VersionState != store.SoftwareVersionKnown || sw.Version == "" {
				continue
			}
			hasMappedVersioned = true
			k := store.SoftwareLookupKey(sw.CPEPart, sw.Vendor, sw.Product, sw.Version)
			entry := cached[k]
			if entry == nil {
				continue
			}
			for _, c := range entry.CVEs {
				id := store.CandidateFindingID(assetID, sw.Vendor, sw.Product, sw.Version, c.CVE)
				desired[id] = true

				cf := store.CandidateFinding{
					ID:               id,
					AssetID:          assetID,
					SoftwareID:       sw.ID,
					Vendor:           sw.Vendor,
					Product:          sw.Product,
					Version:          sw.Version,
					RawString:        sw.RawString,
					CVE:              c.CVE,
					Title:            fmt.Sprintf("%s (%s %s, unconfirmed)", c.CVE, sw.Product, sw.Version),
					Description:      c.Description,
					Severity:         c.Severity,
					SeveritySource:   "nvd",
					Status:           store.FindingOpen,
					NotReassessed:    false,
					LookupSnapshotID: entry.SnapshotID,
					LookupAt:         entry.QueriedAt,
				}
				if prev, ok := existingByID[id]; ok {
					cf.EPSS = prev.EPSS
					cf.KEVListed = prev.KEVListed
					cf.FirstSeen = prev.FirstSeen
				}
				cf.Priority = store.ComputeFindingPriority(cf.Severity, cf.EPSS, cf.KEVListed)
				if err := s.store.SaveCandidateFinding(runCtx, &cf); err != nil {
					return nil, fmt.Errorf("candidate lookup: saving candidate %s: %w", id, err)
				}
				summary.Candidates++
			}
		}

		// Resolution only after a concluded scan of the asset's software.
		// If the asset has mapped+versioned software that was just evaluated,
		// any open candidate no longer matching (e.g. upgraded past affected
		// range) resolves cleanly.
		if hasMappedVersioned {
			for _, prev := range existingByAsset[assetID] {
				if prev.Status == store.FindingResolved || desired[prev.ID] {
					continue
				}
				prev.Status = store.FindingResolved
				prev.NotReassessed = false
				prev.NotReassessedReason = ""
				if err := s.store.SaveCandidateFinding(runCtx, &prev); err != nil {
					return nil, fmt.Errorf("candidate lookup: resolving candidate %s: %w", prev.ID, err)
				}
				summary.Resolved++
			}
		}
	}

	if err := s.store.RecordScanRun(ctx, &store.ScanRun{
		ID:         runID,
		Kind:       store.RunCandidateRefresh,
		Outcome:    "completed",
		StartedAt:  started,
		FinishedAt: time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("candidate lookup: recording run: %w", err)
	}

	return summary, nil
}

func lookupFingerprint(key string, cves []store.CandidateCVE) string {
	h := sha256.New()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte{0})
	for _, c := range cves {
		_, _ = h.Write([]byte(c.CVE))
		_, _ = h.Write([]byte{':'})
		_, _ = h.Write([]byte(string(c.Severity)))
		_, _ = h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))[:16]
}
