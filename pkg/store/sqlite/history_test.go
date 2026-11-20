package sqlite_test

import (
	"context"
	"database/sql"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// steppedStore is a signal store whose clock moves one minute per call, so
// that events are ordered by time as well as by sequence.
func steppedStore(t *testing.T) (*sqlite.SQLiteStore, context.Context) {
	t.Helper()
	s, ctx := newSignalStore(t)
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time {
		at = at.Add(time.Minute)
		return at
	})
	return s, ctx
}

func eventTypes(t *testing.T, s *sqlite.SQLiteStore, ctx context.Context, key string) []store.EventType {
	t.Helper()
	events, err := s.GetFindingEvents(ctx, store.EventQuery{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]store.EventType, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

func obs(signal, check string, sev store.FindingSeverity, lib string) *store.SignalObservation {
	return &store.SignalObservation{
		AssetID: "asset-1", SignalID: signal, CheckID: check,
		State: store.CoverageNotFound, Severity: sev, Mapped: true,
		RegistryVersion: "r1", LibraryVersion: lib,
	}
}

func TestFindingKeyRoundTrips(t *testing.T) {
	for _, id := range []string{"sig-asset-1-SURF-SPF-001", "service@host.example:443/tls", "scan:abc", "a/b c"} {
		key := store.FindingKey(store.SourceFinding, id)
		if strings.ContainsAny(key, "/+= ") {
			t.Errorf("key %q is not URL-safe", key)
		}
		src, row, err := store.ParseFindingKey(key)
		if err != nil || src != store.SourceFinding || row != id {
			t.Errorf("round trip of %q gave %q %q %v", id, src, row, err)
		}
	}
	for _, bad := range []string{"", "nodot", "bogus.abc", "signal.", "signal.!!!"} {
		if _, _, err := store.ParseFindingKey(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestSignalFirstSeenThenQuietThenChangedByTheEstate(t *testing.T) {
	s, ctx := steppedStore(t)
	o := obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1")
	if err := s.SaveSignalObservation(ctx, o); err != nil {
		t.Fatal(err)
	}
	key := store.FindingKey(store.SourceSignal, o.ID)

	// The same observation again records nothing: a rescan of an unchanged
	// estate leaves the timeline alone.
	if err := s.SaveSignalObservation(ctx, obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1")); err != nil {
		t.Fatal(err)
	}
	if got := eventTypes(t, s, ctx, key); !reflect.DeepEqual(got, []store.EventType{store.EventFirstSeen}) {
		t.Fatalf("events = %v, want only first_seen", got)
	}

	if err := s.SaveSignalObservation(ctx, obs("SURF-SPF-003", "spf", store.SeverityHigh, "v1")); err != nil {
		t.Fatal(err)
	}
	events, _ := s.GetFindingEvents(ctx, store.EventQuery{Key: key})
	last := events[len(events)-1]
	if last.Type != store.EventChanged || last.Cause != store.CauseEstate ||
		last.PrevSeverity != store.SeverityMedium || last.Severity != store.SeverityHigh {
		t.Fatalf("change event = %+v", last)
	}
}

func TestALibraryDrivenChangeIsNotAttributedToTheEstate(t *testing.T) {
	s, ctx := steppedStore(t)
	if err := s.SaveSignalObservation(ctx, obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1.0.0")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSignalObservation(ctx, obs("SURF-SPF-003", "spf", store.SeverityHigh, "v1.1.0")); err != nil {
		t.Fatal(err)
	}
	events, _ := s.GetFindingEvents(ctx, store.EventQuery{})
	last := events[len(events)-1]
	if last.Type != store.EventChanged || last.Cause != store.CauseLibrary {
		t.Fatalf("event = %+v, want a change caused by the library", last)
	}
	if last.LibraryVersion != "v1.1.0" || !strings.Contains(last.Note, "v1.0.0") {
		t.Errorf("the event should name both library versions, got %+v", last)
	}
}

func TestAFixedWeaknessResolvesOnlyWhenTheCheckConcluded(t *testing.T) {
	s, ctx := steppedStore(t)
	spf := obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1")
	dmarc := obs("SURF-DMARC-001", "dmarc", store.SeverityHigh, "v1")
	for _, o := range []*store.SignalObservation{spf, dmarc} {
		if err := s.SaveSignalObservation(ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	// A run in which only the SPF check concluded and raised nothing. DMARC did
	// not conclude, so its observation must stay exactly as it was.
	n, err := s.ResolveSignalObservations(ctx, "asset-1", []string{"spf"}, map[string]bool{})
	if err != nil || n != 1 {
		t.Fatalf("resolved %d, err %v; want 1", n, err)
	}

	live, _ := s.GetSignalObservations(ctx, "asset-1")
	if len(live) != 1 || live[0].SignalID != "SURF-DMARC-001" {
		t.Fatalf("live observations = %+v, want only the DMARC one", live)
	}
	all, _ := s.GetAllSignalObservations(ctx, "asset-1")
	if len(all) != 2 {
		t.Fatalf("all observations = %d, want 2", len(all))
	}
	for _, o := range all {
		if o.SignalID == "SURF-SPF-003" && (o.Status != store.FindingResolved || o.ResolvedAt == nil) {
			t.Errorf("SPF observation = %+v, want resolved with a time", o)
		}
		if o.SignalID == "SURF-DMARC-001" && o.Status == store.FindingResolved {
			t.Error("a check that did not conclude must not resolve its observation")
		}
	}
	if got := eventTypes(t, s, ctx, store.FindingKey(store.SourceSignal, spf.ID)); !reflect.DeepEqual(got,
		[]store.EventType{store.EventFirstSeen, store.EventResolved}) {
		t.Errorf("SPF events = %v", got)
	}
	if got := eventTypes(t, s, ctx, store.FindingKey(store.SourceSignal, dmarc.ID)); len(got) != 1 {
		t.Errorf("DMARC events = %v, want only first_seen", got)
	}
}

func TestResolvingToleratesAnEmptyConcludedSet(t *testing.T) {
	s, ctx := steppedStore(t)
	_ = s.SaveSignalObservation(ctx, obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1"))
	n, err := s.ResolveSignalObservations(ctx, "asset-1", nil, nil)
	if err != nil || n != 0 {
		t.Fatalf("resolved %d, err %v; nothing concluded means nothing resolves", n, err)
	}
}

func TestAResolvedObservationReturnsAsReopenedWithItsFirstSeenIntact(t *testing.T) {
	s, ctx := steppedStore(t)
	o := obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1")
	_ = s.SaveSignalObservation(ctx, o)
	first := o.FirstSeen
	_, _ = s.ResolveSignalObservations(ctx, "asset-1", []string{"spf"}, map[string]bool{})

	again := obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1")
	again.FirstSeen = time.Time{}
	if err := s.SaveSignalObservation(ctx, again); err != nil {
		t.Fatal(err)
	}

	all, _ := s.GetAllSignalObservations(ctx, "asset-1")
	if len(all) != 1 || all[0].Status != store.FindingReopened || all[0].ResolvedAt != nil {
		t.Fatalf("observation = %+v, want reopened and not resolved", all)
	}
	if !all[0].FirstSeen.Equal(first) {
		t.Errorf("first seen moved from %v to %v", first, all[0].FirstSeen)
	}
	key := store.FindingKey(store.SourceSignal, o.ID)
	if got := eventTypes(t, s, ctx, key); !reflect.DeepEqual(got,
		[]store.EventType{store.EventFirstSeen, store.EventResolved, store.EventReopened}) {
		t.Errorf("events = %v", got)
	}

	// Raised again on a later run, it stays reopened and records nothing new.
	_ = s.SaveSignalObservation(ctx, obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1"))
	if got := eventTypes(t, s, ctx, key); len(got) != 3 {
		t.Errorf("events = %v, want no event for an unchanged reopened finding", got)
	}
}

func TestEventsCarryTheRunTheyWereObservedIn(t *testing.T) {
	s, ctx := steppedStore(t)
	runCtx := store.WithRun(ctx, store.RunRef{ID: "run-7", Kind: store.RunAssessment})
	_ = s.SaveSignalObservation(runCtx, obs("SURF-SPF-003", "spf", store.SeverityMedium, "v1"))
	events, _ := s.GetFindingEvents(ctx, store.EventQuery{RunID: "run-7"})
	if len(events) != 1 || events[0].RunID != "run-7" {
		t.Fatalf("events for run-7 = %+v", events)
	}
}

func finding(id string, sev store.FindingSeverity) *store.Finding {
	return &store.Finding{
		ID: id, AssetID: "asset-1", Title: "t", Severity: sev, Priority: string(sev), Category: "network",
	}
}

func TestFindingLifecycleRecordsEachTransitionOnce(t *testing.T) {
	s, ctx := steppedStore(t)
	f := finding("service:x", store.SeverityMedium)
	if err := s.SaveFinding(ctx, f); err != nil {
		t.Fatal(err)
	}
	_ = s.SaveFinding(ctx, finding("service:x", store.SeverityMedium)) // unchanged: nothing
	_ = s.SaveFinding(ctx, finding("service:x", store.SeverityHigh))   // changed

	res := finding("service:x", store.SeverityHigh)
	res.Status = store.FindingResolved
	_ = s.SaveFinding(ctx, res)
	_ = s.SaveFinding(ctx, finding("service:x", store.SeverityHigh)) // reopened

	key := store.FindingKey(store.SourceFinding, "service:x")
	want := []store.EventType{store.EventFirstSeen, store.EventChanged, store.EventResolved, store.EventReopened}
	if got := eventTypes(t, s, ctx, key); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	got, _ := s.GetFindings(ctx, "asset-1")
	if len(got) != 1 || got[0].Status != store.FindingReopened || got[0].ResolvedAt != nil {
		t.Fatalf("finding = %+v, want reopened", got)
	}
}

func TestAFeedDrivenPriorityChangeNamesTheFeedAndSnapshot(t *testing.T) {
	s, ctx := steppedStore(t)
	f := finding("scan:cve", store.SeverityMedium)
	f.CVE = "CVE-2024-0001"
	if err := s.SaveFinding(ctx, f); err != nil {
		t.Fatal(err)
	}
	feedCtx := store.WithRun(ctx, store.RunRef{ID: "run-feed", Kind: store.RunFeedRefresh, SnapshotID: "snap-9"})
	if err := s.UpdateFindingCatalogue(feedCtx, "scan:cve", 0, true, "critical", false, true); err != nil {
		t.Fatal(err)
	}
	events, _ := s.GetFindingEvents(ctx, store.EventQuery{Key: store.FindingKey(store.SourceFinding, "scan:cve")})
	last := events[len(events)-1]
	if last.Type != store.EventPriorityChanged || last.Cause != store.CauseFeed || last.SnapshotID != "snap-9" ||
		last.PrevPriority != "medium" || last.Priority != "critical" {
		t.Fatalf("event = %+v", last)
	}

	// Re-applying the same priority is not a change.
	if err := s.UpdateFindingCatalogue(feedCtx, "scan:cve", 0, true, "critical", false, true); err != nil {
		t.Fatal(err)
	}
	if got := eventTypes(t, s, ctx, store.FindingKey(store.SourceFinding, "scan:cve")); len(got) != 2 {
		t.Errorf("events = %v, want no event for an unchanged priority", got)
	}
}

func secret(id string, verified bool) *store.SecretFinding {
	return &store.SecretFinding{
		ID: id, AssetID: "asset-1", RepoURL: "https://example.test/r.git", RuleID: "r", SecretType: "aws",
		RedactedRef: "abc", FilePath: "a.env", StartLine: 3, Verified: verified,
	}
}

func TestSecretsAreIdempotentResolveWhenAbsentAndReopen(t *testing.T) {
	s, ctx := steppedStore(t)
	id := store.SecretFindingID("https://example.test/r.git", "r", "a.env", "abc")
	_ = s.SaveSecretFinding(ctx, secret(id, false))
	_ = s.SaveSecretFinding(ctx, secret(id, false)) // the same scan twice

	got, _ := s.GetSecretFindings(ctx, "")
	if len(got) != 1 {
		t.Fatalf("secrets = %d, want 1", len(got))
	}
	key := store.FindingKey(store.SourceSecret, id)
	if ev := eventTypes(t, s, ctx, key); len(ev) != 1 {
		t.Fatalf("events = %v, want one", ev)
	}

	// Verifying it as live changes it.
	_ = s.SaveSecretFinding(ctx, secret(id, true))
	// A completed scan that no longer finds it resolves it.
	n, err := s.ResolveSecretFindings(ctx, "https://example.test/r.git", map[string]bool{})
	if err != nil || n != 1 {
		t.Fatalf("resolved %d, %v", n, err)
	}
	// Seen again: reopened.
	_ = s.SaveSecretFinding(ctx, secret(id, true))

	want := []store.EventType{store.EventFirstSeen, store.EventChanged, store.EventResolved, store.EventReopened}
	if ev := eventTypes(t, s, ctx, key); !reflect.DeepEqual(ev, want) {
		t.Fatalf("events = %v, want %v", ev, want)
	}
}

func TestResolveSecretsLeavesWhatTheScanSaw(t *testing.T) {
	s, ctx := steppedStore(t)
	_ = s.SaveSecretFinding(ctx, secret("secret:keep", false))
	_ = s.SaveSecretFinding(ctx, secret("secret:gone", false))
	n, _ := s.ResolveSecretFindings(ctx, "https://example.test/r.git", map[string]bool{"secret:keep": true})
	if n != 1 {
		t.Fatalf("resolved %d, want 1", n)
	}
}

func TestRunSummaryCountsOpenFindingsAndTheRunsOwnTransitions(t *testing.T) {
	s, ctx := steppedStore(t)
	runCtx := store.WithRun(ctx, store.RunRef{ID: "run-1", Kind: store.RunAssessment})
	_ = s.SaveSignalObservation(runCtx, obs("SURF-SPF-004", "spf", store.SeverityCritical, "v1"))
	_ = s.SaveSignalObservation(runCtx, obs("SURF-SPF-005", "spf", store.SeverityLow, "v1"))
	_ = s.SaveFinding(runCtx, finding("service:a", store.SeverityHigh))

	cov := store.CoverageSummary{Total: 4, OK: 1, NotFound: 1, NotChecked: 1, CheckFailed: 1}
	if err := s.RecordScanRun(ctx, &store.ScanRun{ID: "run-1", Kind: store.RunAssessment, Outcome: "completed", Coverage: cov}); err != nil {
		t.Fatal(err)
	}

	sums, err := s.GetScanSummaries(ctx, time.Time{})
	if err != nil || len(sums) != 1 {
		t.Fatalf("summaries = %v, %v", sums, err)
	}
	m := sums[0]
	if m.Critical != 1 || m.High != 1 || m.Low != 1 || m.Open != 3 || m.New != 3 {
		t.Errorf("summary = %+v", m)
	}

	runs, _ := s.GetScanRuns(ctx, 0)
	if len(runs) != 1 || runs[0].Coverage.CheckFailed != 1 || runs[0].Coverage.AssessedOnly != 2 {
		t.Errorf("runs = %+v", runs)
	}
}

func TestSummaryRetentionPrunesSummariesNeverEvents(t *testing.T) {
	s, ctx := steppedStore(t)
	_ = s.SaveSignalObservation(ctx, obs("SURF-SPF-004", "spf", store.SeverityCritical, "v1"))
	_ = s.SaveSetting(ctx, store.HistorySummaryRetentionKey, "1")

	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = s.RecordScanRun(ctx, &store.ScanRun{ID: "run-old", Kind: store.RunAssessment, FinishedAt: old})
	if err := s.RecordScanRun(ctx, &store.ScanRun{ID: "run-new", Kind: store.RunAssessment, FinishedAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}

	sums, _ := s.GetScanSummaries(ctx, time.Time{})
	if len(sums) != 1 || sums[0].RunID != "run-new" {
		t.Fatalf("summaries = %+v, want only the recent one", sums)
	}
	runs, _ := s.GetScanRuns(ctx, 0)
	if len(runs) != 2 {
		t.Errorf("runs = %d; pruning summaries must not remove the run ledger", len(runs))
	}
	events, _ := s.GetFindingEvents(ctx, store.EventQuery{})
	if len(events) == 0 {
		t.Error("pruning summaries must never remove events")
	}
}

func TestRemovingAnAssetRemovesItsEventsAndErasingClearsTheLedger(t *testing.T) {
	s, ctx := steppedStore(t)
	_ = s.SaveSignalObservation(ctx, obs("SURF-SPF-004", "spf", store.SeverityCritical, "v1"))
	_ = s.RecordScanRun(ctx, &store.ScanRun{ID: "r", Kind: store.RunAssessment})
	if err := s.DeleteAsset(ctx, "asset-1"); err != nil {
		t.Fatal(err)
	}
	if ev, _ := s.GetFindingEvents(ctx, store.EventQuery{}); len(ev) != 0 {
		t.Errorf("events survive the removal of their asset: %v", ev)
	}
	if err := s.EraseDiscoveredData(ctx); err != nil {
		t.Fatal(err)
	}
	if runs, _ := s.GetScanRuns(ctx, 0); len(runs) != 0 {
		t.Errorf("runs survive an erase: %v", runs)
	}
}

// A database written before history existed receives one baseline event per
// finding at its first-seen time, and nothing earlier is implied.
func TestExistingFindingsGetOneLabelledBaselineEvent(t *testing.T) {
	path := seedOldDatabase(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
	INSERT INTO findings (id, asset_id, title, severity, priority, category, first_seen, last_seen)
	VALUES ('f1','a1','t','high','high','network','2026-01-05T10:00:00Z','2026-01-06T10:00:00Z');
	INSERT INTO secret_findings (id, asset_id, repo_url, rule_id, secret_type, redacted_ref, file_path, start_line, verified, first_seen)
	VALUES ('s1','a1','https://example.test/r.git','r','aws','x','a.env',1,1,'2026-01-07T10:00:00Z');
	INSERT INTO signal_observations (id, asset_id, signal_id, check_id, state, severity, mapped, registry_version, library_version, observed_at, first_seen, last_seen)
	VALUES ('o1','a1','SURF-SPF-003','spf','not_found','medium',1,'r1','v1','2026-01-08T10:00:00Z','2026-01-08T10:00:00Z','2026-01-08T10:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	for pass := 1; pass <= 2; pass++ { // the second open proves it is idempotent
		s := openAt(t, path)
		events, err := s.GetFindingEvents(context.Background(), store.EventQuery{})
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 3 {
			t.Fatalf("pass %d: events = %d, want 3", pass, len(events))
		}
		for _, e := range events {
			if e.Type != store.EventBaseline || e.Cause != store.CauseBaseline || e.Note == "" {
				t.Errorf("pass %d: event = %+v, want a labelled baseline", pass, e)
			}
		}
		if events[0].At.Year() != 2026 || events[0].At.Day() != 5 && events[0].At.Day() != 7 && events[0].At.Day() != 8 {
			t.Errorf("baseline time = %v, want the recorded first-seen", events[0].At)
		}
	}

	s := openAt(t, path)
	defer s.Close()
	secrets, _ := s.GetSecretFindings(context.Background(), "")
	if len(secrets) != 1 || secrets[0].Status != store.FindingOpen || !secrets[0].LastSeen.Equal(secrets[0].FirstSeen) {
		t.Errorf("legacy secret = %+v, want open with last seen falling back to first seen", secrets)
	}
}

// History is append-only. The interface offers no way to change or remove an
// individual event, and no engine source file writes one.
func TestHistoryStoreOffersNoMutation(t *testing.T) {
	typ := reflect.TypeOf((*store.HistoryStore)(nil)).Elem()
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		lower := strings.ToLower(name)
		if strings.Contains(lower, "event") && (strings.HasPrefix(lower, "update") || strings.HasPrefix(lower, "delete") ||
			strings.HasPrefix(lower, "remove") || strings.HasPrefix(lower, "set")) {
			t.Errorf("HistoryStore.%s mutates events; history is append-only", name)
		}
	}

	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no source files found: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if _, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.PackageClauseOnly); err != nil {
			t.Fatal(err)
		}
		src, _ := os.ReadFile(f)
		low := strings.ToLower(string(src))
		for _, forbidden := range []string{"update finding_events", "delete from finding_events"} {
			if strings.Contains(low, forbidden) {
				t.Errorf("%s contains %q; events leave the store only by cascade or erase", f, forbidden)
			}
		}
	}
}
