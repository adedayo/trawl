package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/event"
	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// The whole path, through persist: a weakness is raised, the domain fixes it,
// the next assessment concludes the check and no longer raises it, and the
// posture stops counting it. Then a later run in which the check could not
// conclude leaves a still-standing weakness exactly as it was.
func TestPersistResolvesAFixedWeaknessButNotOnAnOutage(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	svc, err := NewAssessmentService(s, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.SyncRegistry(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAsset(ctx, &store.Asset{ID: "a1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "t", Confidence: 1,
		FirstSeen: time.Now(), LastSeen: time.Now()}); err != nil {
		t.Fatal(err)
	}

	cov := func(check string, state store.CoverageState) store.AssessmentCoverage {
		return store.AssessmentCoverage{AssetID: "a1", CheckID: check, State: state, LibraryVersion: "v1"}
	}
	obs := func(signal, check string, sev store.FindingSeverity) store.SignalObservation {
		return store.SignalObservation{AssetID: "a1", SignalID: signal, CheckID: check, State: store.CoverageNotFound,
			Severity: sev, Mapped: true, RegistryVersion: "r1", LibraryVersion: "v1"}
	}

	// Run 1: SPF and DMARC both concluded and both raised something.
	run1 := store.WithRun(ctx, store.RunRef{ID: "run-1", Kind: store.RunAssessment})
	if err := svc.persist(run1, vadapter.Result{
		Coverage:     []store.AssessmentCoverage{cov("spf", store.CoverageNotFound), cov("dmarc", store.CoverageNotFound)},
		Observations: []store.SignalObservation{obs("SURF-SPF-003", "spf", store.SeverityMedium), obs("SURF-DMARC-001", "dmarc", store.SeverityHigh)},
	}); err != nil {
		t.Fatal(err)
	}

	// Run 2: both concluded, and only DMARC is still raised. SPF was fixed.
	run2 := store.WithRun(ctx, store.RunRef{ID: "run-2", Kind: store.RunAssessment})
	if err := svc.persist(run2, vadapter.Result{
		Coverage:     []store.AssessmentCoverage{cov("spf", store.CoverageOK), cov("dmarc", store.CoverageNotFound)},
		Observations: []store.SignalObservation{obs("SURF-DMARC-001", "dmarc", store.SeverityHigh)},
	}); err != nil {
		t.Fatal(err)
	}
	live, _ := s.GetSignalObservations(ctx, "a1")
	if len(live) != 1 || live[0].SignalID != "SURF-DMARC-001" {
		t.Fatalf("live after the fix = %+v, want only DMARC", live)
	}

	// Run 3: DMARC could not be checked (a resolver outage) and raised nothing.
	// That is silence from a check that did not run: it must not read as a fix.
	run3 := store.WithRun(ctx, store.RunRef{ID: "run-3", Kind: store.RunAssessment})
	if err := svc.persist(run3, vadapter.Result{
		Coverage: []store.AssessmentCoverage{cov("spf", store.CoverageOK), cov("dmarc", store.CoverageCheckFailed)},
	}); err != nil {
		t.Fatal(err)
	}
	live, _ = s.GetSignalObservations(ctx, "a1")
	if len(live) != 1 || live[0].SignalID != "SURF-DMARC-001" || live[0].Status == store.FindingResolved {
		t.Fatalf("live after an outage = %+v; the DMARC weakness must stand", live)
	}

	events, _ := s.GetFindingEvents(ctx, store.EventQuery{RunID: "run-2"})
	if len(events) != 1 || events[0].Type != store.EventResolved {
		t.Errorf("run 2 events = %+v, want exactly one resolved", events)
	}
	if events, _ = s.GetFindingEvents(ctx, store.EventQuery{RunID: "run-3"}); len(events) != 0 {
		t.Errorf("run 3 events = %+v, want none", events)
	}
}
