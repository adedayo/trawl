package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// kevSnapshot builds a CISA KEV snapshot as the client would, with an explicit
// retrieval time so the tests control the clock.
func kevSnapshot(at time.Time, cves ...string) *feed.Snapshot {
	records := make(map[string]feed.Record, len(cves))
	for _, cve := range cves {
		listed := true
		records[cve] = feed.Record{CVE: cve, KEVListed: &listed}
	}
	return &feed.Snapshot{
		ID: "cisa-kev-fixed", Feed: feed.KEV, SourceURL: "https://example.test/kev.json",
		RetrievedAt: at, ContentDigest: "sha256:fixed", RecordCount: len(records), Records: records,
	}
}

func feedFixture(t *testing.T) (*sqlite.SQLiteStore, *FeedIngestService, time.Time) {
	t.Helper()
	db, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "feed.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if err := db.SaveAsset(context.Background(), &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "test", FirstSeen: at, LastSeen: at,
	}); err != nil {
		t.Fatal(err)
	}
	return db, NewFeedIngestService(db), at
}

func addCVEFinding(t *testing.T, db *sqlite.SQLiteStore, id, cve string, at time.Time) {
	t.Helper()
	if err := db.SaveFinding(context.Background(), &store.Finding{
		ID: id, AssetID: "asset-1", Title: id, Severity: store.SeverityMedium,
		CVE: cve, Category: "vulnerability", FirstSeen: at,
	}); err != nil {
		t.Fatal(err)
	}
}

func kevStatusOf(t *testing.T, svc *FeedIngestService, now time.Time) store.FeedStatus {
	t.Helper()
	statuses, err := svc.FeedStatuses(context.Background(), now, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if s.Feed == string(feed.KEV) {
			return s
		}
	}
	t.Fatal("no KEV status")
	return store.FeedStatus{}
}

func TestAnIngestedKEVCatalogueIsStoredWithItsEntries(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()

	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at, "CVE-2026-0001", "CVE-2026-0002"), 0); err != nil {
		t.Fatal(err)
	}

	records, err := db.GetFeedRecords(ctx, "cisa-kev-fixed")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].CVE != "CVE-2026-0001" || records[0].KEVListed == nil || !*records[0].KEVListed {
		t.Fatalf("entries were not persisted with the snapshot: %+v", records)
	}
	status := kevStatusOf(t, svc, at.Add(time.Hour))
	if !status.RecordsStored || status.LastCheckedAt == nil || !status.LastCheckedAt.Equal(at) {
		t.Fatalf("status should report stored entries and the ingest time: %+v", status)
	}
}

func TestOnlyTheLatestKEVSnapshotKeepsItsEntries(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()

	first := kevSnapshot(at, "CVE-2026-0001")
	if _, err := svc.ApplySnapshot(ctx, first, 0); err != nil {
		t.Fatal(err)
	}
	second := kevSnapshot(at.Add(24*time.Hour), "CVE-2026-0001", "CVE-2026-0002")
	second.ID, second.ContentDigest = "cisa-kev-second", "sha256:second"
	if _, err := svc.ApplySnapshot(ctx, second, 0); err != nil {
		t.Fatal(err)
	}

	if old, _ := db.GetFeedRecords(ctx, first.ID); len(old) != 0 {
		t.Fatalf("a superseded snapshot should not keep its entries: %+v", old)
	}
	if kept, _ := db.GetFeedRecords(ctx, second.ID); len(kept) != 2 {
		t.Fatalf("the snapshot in force must keep its entries: %+v", kept)
	}
	if meta, _ := db.GetFeedSnapshot(ctx, first.ID); meta == nil {
		t.Fatal("the metadata of every snapshot is kept for provenance")
	}
}

func TestRefreshingAnUnchangedCatalogueMovesTheLastCheckedTime(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()

	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at, "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}
	later := at.Add(5 * 24 * time.Hour)
	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(later, "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}

	status := kevStatusOf(t, svc, later.Add(time.Hour))
	if status.Snapshot == nil || !status.Snapshot.RetrievedAt.Equal(at) {
		t.Fatalf("unchanged content keeps its original retrieval time: %+v", status.Snapshot)
	}
	if status.LastCheckedAt == nil || !status.LastCheckedAt.Equal(later) {
		t.Fatalf("the refresh must be visible as the last check: %+v", status.LastCheckedAt)
	}
	if status.Stale {
		t.Fatal("a catalogue reconfirmed an hour ago must not be reported stale because its content is old")
	}
	snapshots := 0
	for _, id := range []string{"cisa-kev-fixed"} {
		if s, _ := db.GetFeedSnapshot(ctx, id); s != nil {
			snapshots++
		}
	}
	if snapshots != 1 {
		t.Fatalf("identical content must not create a second snapshot")
	}
}

func TestStalenessFallsBackToTheSnapshotWhenNoCheckIsRecorded(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()
	// A database from before checks were recorded: a snapshot and nothing else.
	if err := db.SaveFeedSnapshot(ctx, &store.FeedSnapshot{
		ID: "legacy", Feed: string(feed.KEV), SourceURL: "x", RetrievedAt: at, ContentDigest: "d", RecordCount: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if !kevStatusOf(t, svc, at.Add(72*time.Hour)).Stale {
		t.Fatal("with no recorded check the snapshot's own age governs")
	}
	if kevStatusOf(t, svc, at.Add(time.Hour)).Stale {
		t.Fatal("a fresh snapshot is not stale")
	}
}

func TestAFailedRefreshDoesNotMoveTheLastCheckedTime(t *testing.T) {
	_, svc, at := feedFixture(t)
	ctx := context.Background()
	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at, "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}
	failedAt := at.Add(30 * time.Hour)
	if err := svc.recordFetchFailure(ctx, feed.KEV, failedAt, errors.New("endpoint returned 502")); err != nil {
		t.Fatal(err)
	}

	status := kevStatusOf(t, svc, failedAt)
	if status.LastCheckedAt == nil || !status.LastCheckedAt.Equal(at) {
		t.Fatalf("a failure is not a check: %+v", status.LastCheckedAt)
	}
	if status.LastFailureAt == nil || status.LastError == "" {
		t.Fatalf("the failure must be reported alongside the last good check: %+v", status)
	}
}

func TestRefreshReachesFindingsRecordedSinceTheCatalogueWasApplied(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()

	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at, "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}
	// A scan after the ingest finds a CVE the catalogue already lists.
	addCVEFinding(t, db, "f-new", "CVE-2026-0001", at.Add(time.Hour))
	runsBefore, _ := db.GetScanRuns(ctx, 0)

	regs, err := svc.ApplySnapshot(ctx, kevSnapshot(at.Add(2*time.Hour), "CVE-2026-0001"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(regs) != 1 {
		t.Fatalf("the newly exploited finding should raise a regression: %+v", regs)
	}
	findings, _ := db.GetFindings(ctx, "asset-1")
	if len(findings) != 1 || !findings[0].KEVListed || findings[0].Priority != string(store.SeverityCritical) {
		t.Fatalf("an unchanged catalogue must still be applied to a finding it has not met: %+v", findings)
	}
	runsAfter, _ := db.GetScanRuns(ctx, 0)
	if len(runsAfter) != len(runsBefore)+1 {
		t.Fatalf("evaluating a finding is a run in the ledger: %d -> %d", len(runsBefore), len(runsAfter))
	}

	// With nothing further to evaluate, reconfirming is not an event.
	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at.Add(3*time.Hour), "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}
	runsAgain, _ := db.GetScanRuns(ctx, 0)
	if len(runsAgain) != len(runsAfter) {
		t.Fatalf("reconfirming an unchanged catalogue added a run: %d -> %d", len(runsAfter), len(runsAgain))
	}
}

func TestANotModifiedSourceIsConfirmedAndEvaluatedFromStoredEntries(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()

	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at, "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}
	addCVEFinding(t, db, "f-new", "CVE-2026-0001", at.Add(time.Hour))

	regs, err := svc.confirmUnchanged(ctx, feed.KEV, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(regs) != 1 {
		t.Fatalf("stored entries should evaluate the new finding offline: %+v", regs)
	}
	status := kevStatusOf(t, svc, time.Now().UTC())
	if status.LastCheckedAt == nil || !status.LastCheckedAt.After(at) {
		t.Fatalf("a not-modified answer is still a check: %+v", status.LastCheckedAt)
	}
}

func TestASnapshotStoredBeforeEntriesWereKeptIsBackfilled(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()
	snap := kevSnapshot(at, "CVE-2026-0001")
	// As an earlier build stored it: metadata only.
	if err := db.SaveFeedSnapshot(ctx, &store.FeedSnapshot{
		ID: snap.ID, Feed: string(snap.Feed), SourceURL: snap.SourceURL, RetrievedAt: at,
		ContentDigest: snap.ContentDigest, RecordCount: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if kept, _ := db.GetFeedRecords(ctx, snap.ID); len(kept) != 0 {
		t.Fatal("precondition: no entries stored")
	}

	if _, err := svc.ApplySnapshot(ctx, kevSnapshot(at.Add(time.Hour), "CVE-2026-0001"), 0); err != nil {
		t.Fatal(err)
	}
	if kept, _ := db.GetFeedRecords(ctx, snap.ID); len(kept) != 1 {
		t.Fatalf("re-ingesting identical content should backfill its entries: %+v", kept)
	}
}

func epssEnrichment(t *testing.T, db *sqlite.SQLiteStore, findingID string) store.FindingEnrichment {
	t.Helper()
	items, err := db.GetFindingEnrichments(context.Background(), findingID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range items {
		if e.Feed == string(feed.EPSS) {
			return e
		}
	}
	t.Fatalf("no EPSS enrichment for %s", findingID)
	return store.FindingEnrichment{}
}

// EPSS is looked up for the CVEs on stored findings and no others, and a CVE
// recorded after the question was asked is left unchecked rather than recorded
// as having no score.
func TestEPSSIsFetchedOnlyForReferencedCVEsAndLaterFindingsAreNotMarkedUnscored(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()
	addCVEFinding(t, db, "f-a", "CVE-2026-0001", at)

	var requests []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked := r.URL.Query().Get("cve")
		requests = append(requests, asked)
		var rows []string
		for _, cve := range strings.Split(asked, ",") {
			rows = append(rows, `{"cve":"`+cve+`","epss":"0.62","date":"2026-11-19"}`)
		}
		_, _ = w.Write([]byte(`{"status":"OK","data":[` + strings.Join(rows, ",") + `]}`))
	}))
	defer api.Close()
	client, err := feed.NewClient(api.Client(), map[feed.Name]string{feed.EPSSAPI: api.URL})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.FetchAndApply(ctx, client, feed.EPSS, 0.3); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0] != "CVE-2026-0001" {
		t.Fatalf("only the referenced CVE should be asked about: %v", requests)
	}
	a := epssEnrichment(t, db, "f-a")
	if a.State != store.EnrichmentOK || a.EPSS == nil || *a.EPSS != 0.62 || a.Snapshot == nil {
		t.Fatalf("the referenced finding should carry its score and provenance: %+v", a)
	}
	status := kevStatusOfFeed(t, svc, string(feed.EPSS), time.Now().UTC())
	if status.Snapshot == nil || status.Snapshot.RecordCount != 1 || status.LastCheckedAt == nil {
		t.Fatalf("status should describe what was fetched: %+v", status)
	}

	// A finding appears after that snapshot was built. Evaluating the old
	// snapshot against it must not conclude it has no EPSS score.
	addCVEFinding(t, db, "f-b", "CVE-2026-0002", at.Add(time.Hour))
	stale := &feed.Snapshot{
		ID: "epss-old", Feed: feed.EPSS, SourceURL: "x", RetrievedAt: at, ContentDigest: "d",
		Records: map[string]feed.Record{}, Queried: map[string]bool{"CVE-2026-0001": true},
	}
	if _, err := svc.ApplySnapshot(ctx, stale, 0); err != nil {
		t.Fatal(err)
	}
	if b := epssEnrichment(t, db, "f-b"); b.State != store.EnrichmentNotChecked {
		t.Fatalf("a CVE nobody asked about is not_checked, not an answer: %+v", b)
	}

	// The next refresh asks about both and covers it.
	if _, err := svc.FetchAndApply(ctx, client, feed.EPSS, 0.3); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[1] != "CVE-2026-0001,CVE-2026-0002" {
		t.Fatalf("the second refresh should ask about both CVEs: %v", requests)
	}
	if b := epssEnrichment(t, db, "f-b"); b.State != store.EnrichmentOK || b.EPSS == nil {
		t.Fatalf("the later finding should now be scored: %+v", b)
	}
}

func kevStatusOfFeed(t *testing.T, svc *FeedIngestService, name string, now time.Time) store.FeedStatus {
	t.Helper()
	statuses, err := svc.FeedStatuses(context.Background(), now, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if s.Feed == name {
			return s
		}
	}
	t.Fatalf("no status for %s", name)
	return store.FeedStatus{}
}

// An operator who configured only a mirror of the daily file still gets the
// whole file, so existing deployments are unchanged.
func TestEPSSFallsBackToTheDailyFileWhenOnlyItIsConfigured(t *testing.T) {
	db, svc, at := feedFixture(t)
	ctx := context.Background()
	addCVEFinding(t, db, "f-a", "CVE-2026-0001", at)

	file := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("cve,epss,percentile\nCVE-2026-0001,0.5,0.9\nCVE-2026-5555,0.1,0.2\n"))
	}))
	defer file.Close()
	client, _ := feed.NewClient(file.Client(), map[feed.Name]string{feed.EPSS: file.URL})

	if _, err := svc.FetchAndApply(ctx, client, feed.EPSS, 0); err != nil {
		t.Fatal(err)
	}
	status := kevStatusOfFeed(t, svc, string(feed.EPSS), time.Now().UTC())
	if status.Snapshot == nil || status.Snapshot.RecordCount != 2 {
		t.Fatalf("the whole file should have been ingested: %+v", status.Snapshot)
	}
	if e := epssEnrichment(t, db, "f-a"); e.State != store.EnrichmentOK {
		t.Fatalf("%+v", e)
	}
}
