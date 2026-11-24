package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/narrative"
	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// recordingExplainer captures what the read model hands the narrative layer, so
// the tests can assert on the inputs rather than on wording.
type recordingExplainer struct{ got narrative.Input }

func (r *recordingExplainer) Explain(in narrative.Input) narrative.Explanation {
	r.got = in
	return narrative.Explanation{Level: narrative.LevelGeneric, LevelKey: "generic", Version: "test", WhatWeFound: in.Title}
}

func newFindingsFixture(t *testing.T) (*service.FindingsService, *sqlite.SQLiteStore, *recordingExplainer, context.Context) {
	t.Helper()
	_, st, ctx := newAssessmentService(t)
	ss := st.(*sqlite.SQLiteStore)
	at := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	ss.SetClock(func() time.Time { at = at.Add(time.Minute); return at })

	seedAsset(t, st, ctx, "asset-1", "example.com")
	ex := &recordingExplainer{}
	svc := service.NewFindingsService(st, ex)
	svc.SetClock(func() time.Time { return time.Date(2026, 5, 11, 8, 0, 0, 0, time.UTC) })
	return svc, ss, ex, ctx
}

func signalObs(signal, check string, sev store.FindingSeverity, lib string) *store.SignalObservation {
	return &store.SignalObservation{
		AssetID: "asset-1", SignalID: signal, CheckID: check, State: store.CoverageNotFound,
		Severity: sev, Mapped: true, RegistryVersion: "r1", LibraryVersion: lib,
		Detail: "this domain", Evidence: "v=spf1 +all",
	}
}

func TestTheIndexListsEverySourceTogetherMostSeriousFirst(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-SPF-005", "spf", store.SeverityLow, "v1"))
	_ = st.SaveFinding(ctx, &store.Finding{ID: "service:a", AssetID: "asset-1", Title: "Exposed ssh", Severity: store.SeverityMedium, Priority: "medium", Category: "service-exposure"})
	_ = st.SaveSecretFinding(ctx, &store.SecretFinding{ID: "secret:1", AssetID: "asset-1", RepoURL: "https://example.test/r", RuleID: "r", SecretType: "aws", RedactedRef: "x", FilePath: "a.env", StartLine: 1, Verified: true})

	all, err := svc.Index(ctx, service.FindingFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("index = %d, want one row per source", len(all))
	}
	if all[0].Source != service.SourceSecret || all[0].Severity != store.SeverityCritical {
		t.Errorf("first row = %+v, want the verified secret", all[0])
	}
	if all[2].Source != service.SourceSignal {
		t.Errorf("last row = %+v, want the low-severity signal", all[2])
	}
	for _, f := range all {
		if f.AssetValue != "example.com" || f.Key == "" {
			t.Errorf("row %+v lacks its asset value or key", f)
		}
	}

	only, _ := svc.Index(ctx, service.FindingFilter{Source: service.SourceFinding})
	if len(only) != 1 || only[0].Title != "Exposed ssh" {
		t.Errorf("source filter = %+v", only)
	}
	sev, _ := svc.Index(ctx, service.FindingFilter{Severity: "critical"})
	if len(sev) != 1 {
		t.Errorf("severity filter = %+v", sev)
	}
}

func TestTheStatusFilterSeparatesActiveFromResolved(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-SPF-005", "spf", store.SeverityLow, "v1"))
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-DMARC-001", "dmarc", store.SeverityHigh, "v1"))
	_, _ = st.ResolveSignalObservations(ctx, "asset-1", []string{"spf"}, map[string]bool{})

	active, _ := svc.Index(ctx, service.FindingFilter{Status: "active"})
	resolved, _ := svc.Index(ctx, service.FindingFilter{Status: "resolved"})
	if len(active) != 1 || active[0].Category != "dmarc" {
		t.Errorf("active = %+v", active)
	}
	if len(resolved) != 1 || resolved[0].ResolvedAt == "" || resolved[0].Category != "spf" {
		t.Errorf("resolved = %+v", resolved)
	}
}

func TestAnOpenFindingWhoseCheckCouldNotConcludeIsMarkedNotReassessed(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-SPF-005", "spf", store.SeverityLow, "v1"))
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-DMARC-001", "dmarc", store.SeverityHigh, "v1"))
	_ = st.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{AssetID: "asset-1", CheckID: "spf", State: store.CoverageCheckFailed, Reason: "resolver timeout", LibraryVersion: "v1"})
	_ = st.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{AssetID: "asset-1", CheckID: "dmarc", State: store.CoverageOK, LibraryVersion: "v1"})

	got, _ := svc.Index(ctx, service.FindingFilter{})
	byCat := map[string]service.FindingSummary{}
	for _, f := range got {
		byCat[f.Category] = f
	}
	if !byCat["spf"].NotReassessed || !strings.Contains(byCat["spf"].NotReassessedReason, "resolver timeout") {
		t.Errorf("spf = %+v, want not re-assessed, with the reason", byCat["spf"])
	}
	if byCat["dmarc"].NotReassessed {
		t.Errorf("dmarc = %+v; a check that concluded must not be marked not re-assessed", byCat["dmarc"])
	}
}

func TestChangedSinceIgnoresBaselineEventsAndUnchangedFindings(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-SPF-005", "spf", store.SeverityLow, "v1"))

	got, err := svc.Index(ctx, service.FindingFilter{ChangedSince: "2026-05-01T00:00:00Z"})
	if err != nil || len(got) != 1 {
		t.Fatalf("changed since before first seen: %v %v", got, err)
	}
	got, _ = svc.Index(ctx, service.FindingFilter{ChangedSince: "2026-06-01T00:00:00Z"})
	if len(got) != 0 {
		t.Errorf("nothing changed after June, got %+v", got)
	}
	if _, err := svc.Index(ctx, service.FindingFilter{ChangedSince: "yesterday"}); err == nil {
		t.Error("an unparseable time must be refused, not ignored")
	}
}

func TestDetailBuildsTheTimelineAndHandsTheNarrativeLayerWhatItNeeds(t *testing.T) {
	svc, st, ex, ctx := newFindingsFixture(t)
	if err := st.ReplaceSignalRegistry(ctx, []store.SignalRegistryEntry{{
		SignalID: "SURF-SPF-004", WeaknessClass: "email-authentication", Scenario: "email-spoofing",
		DedupGroup: "spf:email-authentication", Control: "spf", Direction: "aggravating", RegistryVersion: "r1",
	}}); err != nil {
		t.Fatal(err)
	}
	run1 := store.WithRun(ctx, store.RunRef{ID: "run-1", Kind: store.RunAssessment})
	_ = st.SaveSignalObservation(run1, signalObs("SURF-SPF-004", "spf", store.SeverityHigh, "v1"))
	run2 := store.WithRun(ctx, store.RunRef{ID: "run-2", Kind: store.RunAssessment})
	_ = st.SaveSignalObservation(run2, signalObs("SURF-SPF-004", "spf", store.SeverityCritical, "v1"))

	list, _ := svc.Index(ctx, service.FindingFilter{})
	d, err := svc.Detail(ctx, list[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Timeline) != 2 || d.Timeline[0].Type != "first_seen" || d.Timeline[1].Type != "changed" ||
		d.Timeline[1].PrevSeverity != "high" || d.Timeline[1].RunID != "run-2" {
		t.Fatalf("timeline = %+v", d.Timeline)
	}
	if d.Provenance.RunID != "run-2" || d.Provenance.LibraryVersion != "v1" {
		t.Errorf("provenance = %+v", d.Provenance)
	}
	if d.NarrativeVersion != "test" || d.Evidence != "v=spf1 +all" || d.Detail != "this domain" {
		t.Errorf("detail = %+v", d)
	}
	in := ex.got
	if in.SignalID != "SURF-SPF-004" || in.WeaknessClass != "email-authentication" || in.Check != "spf" ||
		in.Scenario != "email-spoofing" || in.Catalogue == nil || in.Catalogue.Remediation == "" {
		t.Errorf("narrative input = %+v", in)
	}
}

func TestDetailRefusesAnUnknownOrMalformedKey(t *testing.T) {
	svc, _, _, ctx := newFindingsFixture(t)
	for _, key := range []string{"", "garbage", store.FindingKey(store.SourceSignal, "no-such-row")} {
		if _, err := svc.Detail(ctx, key); err != service.ErrFindingNotFound {
			t.Errorf("Detail(%q) error = %v, want ErrFindingNotFound", key, err)
		}
	}
}

func TestDetailStatesWhatTheFeedsSaidAboutACVE(t *testing.T) {
	svc, st, ex, ctx := newFindingsFixture(t)
	_ = st.SaveFinding(ctx, &store.Finding{ID: "scan:cve", AssetID: "asset-1", Title: "Bad", Severity: store.SeverityMedium, Priority: "medium", Category: "network", CVE: "CVE-2024-1"})
	snap := &store.FeedSnapshot{ID: "snap-1", Feed: store.FeedKEV, SourceURL: "https://example.test", RetrievedAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), ContentDigest: "d"}
	_ = st.SaveFeedSnapshot(ctx, snap)
	listed, checked := true, snap.RetrievedAt
	_ = st.SaveFindingEnrichment(ctx, &store.FindingEnrichment{FindingID: "scan:cve", Feed: store.FeedKEV, CVE: "CVE-2024-1", State: store.EnrichmentOK, SnapshotID: "snap-1", KEVListed: &listed, CheckedAt: &checked})

	_, err := svc.Detail(ctx, store.FindingKey(store.SourceFinding, "scan:cve"))
	if err != nil {
		t.Fatal(err)
	}
	if ex.got.CVE != "CVE-2024-1" || ex.got.KEVState != "ok" || !ex.got.KEVListed || ex.got.KEVCheckedAt == "" {
		t.Errorf("KEV input = %+v", ex.got)
	}
	if ex.got.EPSSState != "not_checked" {
		t.Errorf("EPSS state = %q; an unchecked feed must say so rather than be left blank", ex.got.EPSSState)
	}
}

func TestChangesSeparatesNewResolvedAndWorsenedAndStatesLowerCoverage(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)

	run1 := store.WithRun(ctx, store.RunRef{ID: "run-1", Kind: store.RunAssessment})
	_ = st.SaveSignalObservation(run1, signalObs("SURF-SPF-003", "spf", store.SeverityMedium, "v1"))
	_ = st.SaveSignalObservation(run1, signalObs("SURF-DMARC-001", "dmarc", store.SeverityMedium, "v1"))
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "run-1", Kind: store.RunAssessment, AssetID: "asset-1",
		Coverage: store.CoverageSummary{Total: 4, OK: 4, AssessedOnly: 4}})

	run2 := store.WithRun(ctx, store.RunRef{ID: "run-2", Kind: store.RunAssessment})
	_ = st.SaveSignalObservation(run2, signalObs("SURF-DMARC-001", "dmarc", store.SeverityHigh, "v1")) // worsened
	_ = st.SaveSignalObservation(run2, signalObs("SURF-MX-001", "mx", store.SeverityLow, "v1"))        // new
	_, _ = st.ResolveSignalObservations(run2, "asset-1", []string{"spf"}, map[string]bool{})           // resolved
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "run-2", Kind: store.RunAssessment, AssetID: "asset-1",
		Coverage: store.CoverageSummary{Total: 4, OK: 2, NotChecked: 2, AssessedOnly: 2}})

	cs, err := svc.Changes(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cs.To == nil || cs.To.Run.ID != "run-2" || cs.From == nil || cs.From.Run.ID != "run-1" {
		t.Fatalf("compared runs = %+v / %+v", cs.From, cs.To)
	}
	if len(cs.New) != 1 || cs.New[0].Finding.Category != "mx" {
		t.Errorf("new = %+v", cs.New)
	}
	if len(cs.Resolved) != 1 || cs.Resolved[0].Finding.Category != "spf" {
		t.Errorf("resolved = %+v", cs.Resolved)
	}
	if len(cs.Worsened) != 1 || cs.Worsened[0].Finding.Category != "dmarc" || len(cs.Improved) != 0 {
		t.Errorf("worsened = %+v improved = %+v", cs.Worsened, cs.Improved)
	}
	if !strings.Contains(cs.Note, "Coverage was lower") {
		t.Errorf("note = %q; a drop in coverage must be stated beside the comparison", cs.Note)
	}

	// Run 1 against nothing earlier: everything it recorded is new, and the
	// note says why.
	first, err := svc.Changes(ctx, "", "run-1")
	if err != nil || first.From != nil || len(first.New) != 2 || !strings.Contains(first.Note, "earliest") {
		t.Errorf("first run changes = %+v (%v)", first, err)
	}
	if _, err := svc.Changes(ctx, "", "run-404"); err == nil {
		t.Error("an unknown run must be refused")
	}
}

func TestHistoryDrawsOnlyRecordedRunsAndCarriesTheirCoverage(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	_ = st.SaveSignalObservation(ctx, signalObs("SURF-SPF-004", "spf", store.SeverityCritical, "v1"))
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "run-1", Kind: store.RunAssessment,
		Coverage: store.CoverageSummary{Total: 4, OK: 1, AssessedOnly: 1, NotChecked: 3}})

	points, err := svc.History(ctx, time.Time{})
	if err != nil || len(points) != 1 {
		t.Fatalf("history = %v, %v", points, err)
	}
	p := points[0]
	if p.Critical != 1 || p.Open != 1 || p.Run.CoverageFraction != 0.25 {
		t.Errorf("point = %+v; each point must carry the coverage of its run", p)
	}
}

func TestDetailStaysResponsiveDuringConcurrentScanWrites(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	fid := "service:asset-1:example.com:443:http:exposed"
	if err := st.SaveFinding(ctx, &store.Finding{
		ID: fid, AssetID: "asset-1", Title: "Externally reachable https service",
		Severity: store.SeverityLow, Priority: "low", Category: "service-exposure",
	}); err != nil {
		t.Fatalf("SaveFinding: %v", err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				_ = st.SaveServiceObservation(ctx, &store.ServiceObservation{
					ID: fmt.Sprintf("obs-%d", i), AssetID: "asset-1", Host: "example.com", Port: 443,
					Service: "https", Transport: "tcp", Protocol: "https", Layer: "http",
					State: "responding", Coverage: store.CoverageOK, ObservedAt: time.Now().UTC(),
				})
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	key := store.FindingKey(store.SourceFinding, fid)
	for i := 0; i < 10; i++ {
		detail, err := svc.Detail(ctx, key)
		if err != nil {
			t.Fatalf("Detail under concurrent writes failed: %v", err)
		}
		if detail.Key != key || detail.Title != "Externally reachable https service" {
			t.Fatalf("unexpected detail: %+v", detail)
		}
	}
}

func TestConcludedChecksNeverIncludeAnOutageAndExcludeInconclusiveDKIM(t *testing.T) {
	res := vadapter.Result{
		Coverage: []store.AssessmentCoverage{
			{CheckID: "spf", State: store.CoverageOK},
			{CheckID: "dmarc", State: store.CoverageNotFound},
			{CheckID: "mx", State: store.CoverageCheckFailed},
			{CheckID: "caa", State: store.CoverageNotChecked},
			{CheckID: "dkim", State: store.CoverageNotFound},
		},
		EmailPosture: &store.EmailPosture{DKIM: store.EmailControl{State: store.CoverageNotFound, Conclusive: false}},
		Observations: []store.SignalObservation{{SignalID: "SURF-SPF-003"}},
	}
	got := map[string]bool{}
	for _, c := range service.ConcludedChecks(res) {
		got[c] = true
	}
	if !got["spf"] || !got["dmarc"] {
		t.Errorf("concluded = %v, want spf and dmarc", got)
	}
	for _, bad := range []string{"mx", "caa", "dkim"} {
		if got[bad] {
			t.Errorf("%s must not be treated as concluded", bad)
		}
	}

	res.EmailPosture.DKIM.Conclusive = true
	found := false
	for _, c := range service.ConcludedChecks(res) {
		found = found || c == "dkim"
	}
	if !found {
		t.Error("a conclusive DKIM absence may resolve")
	}
	if !service.RaisedSignals(res)["SURF-SPF-003"] {
		t.Error("raised signals should include what the run raised")
	}
}

func TestTheDefaultComparisonIsAgainstTheLastRunOfTheSameKind(t *testing.T) {
	svc, st, _, ctx := newFindingsFixture(t)
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "assess-1", Kind: store.RunAssessment, AssetID: "asset-1"})
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "feed-1", Kind: store.RunFeedRefresh})
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "assess-2", Kind: store.RunAssessment, AssetID: "asset-1"})
	_ = st.RecordScanRun(ctx, &store.ScanRun{ID: "feed-2", Kind: store.RunFeedRefresh})

	cs, err := svc.Changes(ctx, "", "assess-2")
	if err != nil || cs.From == nil || cs.From.Run.ID != "assess-1" {
		t.Fatalf("from = %+v (%v), want the earlier assessment, not the feed refresh between", cs.From, err)
	}
	cs, _ = svc.Changes(ctx, "", "")
	if cs.To.Run.ID != "feed-2" || cs.From == nil || cs.From.Run.ID != "feed-1" {
		t.Errorf("latest run compared %+v -> %+v", cs.From, cs.To)
	}
}
