package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestApplySnapshotReevaluatesFindingWithoutTouchingLastSeen(t *testing.T) {
	db, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "feed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	observedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveAsset(ctx, &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "test",
		FirstSeen: observedAt, LastSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFinding(ctx, &store.Finding{
		ID: "finding-1", AssetID: "asset-1", Title: "CVE finding",
		CVE: "CVE-2026-0001", Category: "vulnerability",
		FirstSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := db.GetFindings(ctx, "asset-1")
	if err != nil || len(before) != 1 {
		t.Fatalf("baseline finding: %+v, %v", before, err)
	}

	listed := true
	snapshot := &feed.Snapshot{
		ID: "cisa-kev-test", Feed: feed.KEV, SourceURL: "file:///kev.json",
		RetrievedAt: observedAt.Add(time.Hour), ContentDigest: "sha256:test",
		RecordCount: 1, Records: map[string]feed.Record{
			"CVE-2026-0001": {CVE: "CVE-2026-0001", KEVListed: &listed},
		},
	}
	regressions, err := NewFeedIngestService(db).ApplySnapshot(ctx, snapshot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(regressions) != 1 || regressions[0].AttributeType != "catalogue:cisa-kev:CVE-2026-0001" {
		t.Fatalf("unexpected regressions: %+v", regressions)
	}

	findings, err := db.GetFindings(ctx, "asset-1")
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings: %+v, %v", findings, err)
	}
	if !findings[0].KEVListed || !findings[0].LastSeen.Equal(before[0].LastSeen) {
		t.Fatalf("catalogue update changed finding incorrectly: %+v", findings[0])
	}
	if len(findings[0].Enrichments) != 3 {
		t.Fatalf("expected three enrichment states, got %+v", findings[0].Enrichments)
	}
	for _, enrichment := range findings[0].Enrichments {
		if enrichment.Feed == string(feed.KEV) && (enrichment.State != store.EnrichmentOK || enrichment.Snapshot == nil) {
			t.Fatalf("missing KEV provenance: %+v", enrichment)
		}
	}
	if findings[0].Priority != string(store.SeverityCritical) {
		t.Fatalf("KEV match must recompute priority via pure function to critical, got %q", findings[0].Priority)
	}
}

func TestFailedFetchRecordsFailureAndLeavesPreviousSnapshotInForce(t *testing.T) {
	db, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "feed-fail.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	observedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveAsset(ctx, &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "test",
		FirstSeen: observedAt, LastSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFinding(ctx, &store.Finding{
		ID: "f-covered", AssetID: "asset-1", Title: "Covered CVE",
		Severity: store.SeverityLow, CVE: "CVE-2026-0001", Category: "vulnerability", FirstSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}

	healthy := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"vulnerabilities":[{"cveID":"CVE-2026-0001"}]}`))
	}))
	defer srv.Close()

	client, err := feed.NewClient(srv.Client(), map[feed.Name]string{feed.KEV: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewFeedIngestService(db)
	if _, err := svc.FetchAndApply(ctx, client, feed.KEV, 0); err != nil {
		t.Fatalf("initial FetchAndApply: %v", err)
	}
	priorSnap, err := db.GetLatestFeedSnapshot(ctx, string(feed.KEV))
	if err != nil || priorSnap == nil {
		t.Fatalf("expected prior snapshot: %+v, %v", priorSnap, err)
	}

	// Add a second CVE finding that has never been checked, then fail the next fetch.
	if err := db.SaveFinding(ctx, &store.Finding{
		ID: "f-uncovered", AssetID: "asset-1", Title: "Uncovered CVE",
		Severity: store.SeverityMedium, CVE: "CVE-2026-9999", Category: "vulnerability", FirstSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}
	healthy = false
	if _, err := svc.FetchAndApply(ctx, client, feed.KEV, 0); err == nil {
		t.Fatal("expected failed fetch to return an error")
	}

	activeSnap, err := db.GetLatestFeedSnapshot(ctx, string(feed.KEV))
	if err != nil || activeSnap == nil || activeSnap.ID != priorSnap.ID {
		t.Fatalf("previous snapshot did not remain in force: got %+v, want %s", activeSnap, priorSnap.ID)
	}

	statuses, err := svc.FeedStatuses(ctx, time.Now().UTC().Add(72*time.Hour), 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var kevStatus *store.FeedStatus
	for i := range statuses {
		if statuses[i].Feed == string(feed.KEV) {
			kevStatus = &statuses[i]
		}
	}
	if kevStatus == nil || kevStatus.Snapshot == nil || kevStatus.Snapshot.ID != priorSnap.ID || kevStatus.LastError == "" || !kevStatus.Stale {
		t.Fatalf("expected feed status to retain prior snapshot, record failure, and flag staleness: %+v", kevStatus)
	}

	coveredEnrichments, _ := db.GetFindingEnrichments(ctx, "f-covered")
	for _, e := range coveredEnrichments {
		if e.Feed == string(feed.KEV) && (e.State != store.EnrichmentOK || e.SnapshotID != priorSnap.ID) {
			t.Fatalf("covered finding lost its prior snapshot provenance on failed fetch: %+v", e)
		}
	}
	uncoveredEnrichments, _ := db.GetFindingEnrichments(ctx, "f-uncovered")
	for _, e := range uncoveredEnrichments {
		if e.Feed == string(feed.KEV) && e.State != store.EnrichmentCheckFailed {
			t.Fatalf("uncovered finding must be marked check_failed after failed fetch, got %+v", e)
		}
	}
}

func TestFetchAndApplyNVDQueriesOnlyReferencedCVEs(t *testing.T) {
	db, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "feed-nvd.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_ = db.SaveAsset(ctx, &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "test", FirstSeen: now, LastSeen: now,
	})
	_ = db.SaveFinding(ctx, &store.Finding{
		ID: "f-1", AssetID: "asset-1", Title: "CVE-2026-0042", Severity: store.SeverityMedium,
		CVE: "CVE-2026-0042", Category: "vulnerability", FirstSeen: now,
	})

	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("cveId"))
		_, _ = w.Write([]byte(`{"vulnerabilities":[{"cve":{"id":"CVE-2026-0042"}}]}`))
	}))
	defer srv.Close()

	client, _ := feed.NewClient(srv.Client(), map[feed.Name]string{feed.NVD: srv.URL})
	if _, err := NewFeedIngestService(db).FetchAndApply(ctx, client, feed.NVD, 0); err != nil {
		t.Fatalf("FetchAndApply NVD: %v", err)
	}
	if len(asked) != 1 || asked[0] != "CVE-2026-0042" {
		t.Fatalf("expected single targeted NVD lookup, got %v", asked)
	}
	enrichments, _ := db.GetFindingEnrichments(ctx, "f-1")
	for _, e := range enrichments {
		if e.Feed == string(feed.NVD) && e.State != store.EnrichmentOK {
			t.Fatalf("expected NVD enrichment state ok, got %+v", e)
		}
	}
}
