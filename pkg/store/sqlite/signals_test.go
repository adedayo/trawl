package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func newSignalStore(t *testing.T) (*sqlite.SQLiteStore, context.Context) {
	t.Helper()
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "signals.db"))
	if err != nil {
		t.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	asset := &store.Asset{
		ID:              "asset-1",
		Type:            store.AssetTypeDomain,
		Value:           "example.com",
		Status:          store.AssetStatusActive,
		DiscoverySource: "manual",
		Confidence:      1,
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}
	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatalf("Failed to seed asset: %v", err)
	}
	return s, ctx
}

func TestCoverageState_NoBinaryCollapse(t *testing.T) {
	cases := []struct {
		state    store.CoverageState
		assessed bool
		passing  bool
	}{
		{store.CoverageOK, true, true},
		{store.CoverageNotFound, true, false},
		{store.CoverageNotChecked, false, false},
		{store.CoverageCheckFailed, false, false},
	}
	for _, c := range cases {
		if !c.state.Valid() {
			t.Errorf("%s should be a valid state", c.state)
		}
		if got := c.state.Assessed(); got != c.assessed {
			t.Errorf("%s: Assessed() = %v, want %v", c.state, got, c.assessed)
		}
		if got := c.state.Passing(); got != c.passing {
			t.Errorf("%s: Passing() = %v, want %v", c.state, got, c.passing)
		}
	}

	if store.CoverageState("unknown").Valid() {
		t.Error("an unrecognised state must not be valid")
	}
}

func TestSignalObservation_UpsertPreservesFirstSeen(t *testing.T) {
	s, ctx := newSignalStore(t)

	first := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	obs := &store.SignalObservation{
		AssetID:         "asset-1",
		SignalID:        "SURF-SPF-005",
		CheckID:         "spf",
		State:           store.CoverageNotFound,
		Severity:        store.SeverityLow,
		Evidence:        "example.com TXT = v=spf1 ~all",
		Mapped:          true,
		RegistryVersion: "vantage-1",
		LibraryVersion:  "v1.1.0",
		ObservedAt:      first,
		FirstSeen:       first,
		LastSeen:        first,
	}
	if err := s.SaveSignalObservation(ctx, obs); err != nil {
		t.Fatalf("Failed to save observation: %v", err)
	}

	obs.State = store.CoverageOK
	obs.LastSeen = time.Now().Truncate(time.Second)
	obs.FirstSeen = obs.LastSeen // an upsert must not be able to rewrite history
	if err := s.SaveSignalObservation(ctx, obs); err != nil {
		t.Fatalf("Failed to upsert observation: %v", err)
	}

	got, err := s.GetSignalObservations(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to read observations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Expected 1 observation after upsert, got %d", len(got))
	}
	if got[0].State != store.CoverageOK {
		t.Errorf("Expected state to update to ok, got %s", got[0].State)
	}
	if !got[0].FirstSeen.Equal(first) {
		t.Errorf("Expected first_seen preserved as %v, got %v", first, got[0].FirstSeen)
	}
}

func TestSignalObservation_RejectsUnknownState(t *testing.T) {
	s, ctx := newSignalStore(t)

	err := s.SaveSignalObservation(ctx, &store.SignalObservation{
		AssetID:  "asset-1",
		SignalID: "SURF-DNSSEC-001",
		CheckID:  "dnssec",
		State:    store.CoverageState("probably_fine"),
	})
	if !errors.Is(err, sqlite.ErrInvalidCoverageState) {
		t.Fatalf("Expected ErrInvalidCoverageState, got %v", err)
	}
}

func TestSignalRegistry_ReplaceAndLookup(t *testing.T) {
	s, ctx := newSignalStore(t)

	entries := []store.SignalRegistryEntry{{
		SignalID:        "SURF-SPF-005",
		Condition:       "SPF terminates in ~all rather than -all",
		WeaknessClass:   "email-spoofing",
		Scenario:        "business-email-compromise",
		Stage:           "delivery",
		DedupGroup:      "spf-terminal-mechanism",
		Control:         "spf",
		Direction:       "aggravating",
		RegistryVersion: "vantage-1",
	}}
	if err := s.ReplaceSignalRegistry(ctx, entries); err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	entry, err := s.GetSignalRegistryEntry(ctx, "SURF-SPF-005")
	if err != nil || entry == nil {
		t.Fatalf("Expected registry entry, got %v (err %v)", entry, err)
	}
	if entry.Scenario != "business-email-compromise" {
		t.Errorf("Unexpected scenario %q", entry.Scenario)
	}

	// An unmapped identifier is an expected upgrade outcome, not an error.
	unmapped, err := s.GetSignalRegistryEntry(ctx, "SURF-NEW-001")
	if err != nil {
		t.Fatalf("Unmapped lookup should not error: %v", err)
	}
	if unmapped != nil {
		t.Error("Expected nil entry for an unmapped identifier")
	}

	// Replacement is a swap, not an accumulation.
	if err := s.ReplaceSignalRegistry(ctx, nil); err != nil {
		t.Fatalf("Failed to clear registry: %v", err)
	}
	all, err := s.GetSignalRegistry(ctx)
	if err != nil {
		t.Fatalf("Failed to read registry: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("Expected registry replaced, got %d entries", len(all))
	}
}

func TestAssessmentCoverage_FourStatesCountedSeparately(t *testing.T) {
	s, ctx := newSignalStore(t)

	states := map[string]store.CoverageState{
		"spf":    store.CoverageOK,
		"dmarc":  store.CoverageNotFound,
		"axfr":   store.CoverageNotChecked,
		"mtasts": store.CoverageCheckFailed,
	}
	for check, state := range states {
		cov := &store.AssessmentCoverage{
			AssetID:        "asset-1",
			CheckID:        check,
			State:          state,
			LibraryVersion: "v1.1.0",
		}
		if state == store.CoverageNotChecked {
			cov.Reason = "excluded by egress policy: intrusive"
		}
		if err := s.RecordAssessmentCoverage(ctx, cov); err != nil {
			t.Fatalf("Failed to record coverage for %s: %v", check, err)
		}
	}

	summary, err := s.ComputeCoverage(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to compute coverage: %v", err)
	}
	if summary.Total != 4 || summary.OK != 1 || summary.NotFound != 1 ||
		summary.NotChecked != 1 || summary.CheckFailed != 1 {
		t.Fatalf("Unexpected coverage summary: %+v", summary)
	}
	if got := summary.Fraction(); got != 0.5 {
		t.Errorf("Expected coverage fraction 0.5, got %v", got)
	}

	// The excluding reason must survive persistence, or a fail-closed
	// exclusion is indistinguishable from an unexplained gap.
	rows, err := s.GetAssessmentCoverage(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to read coverage: %v", err)
	}
	var found bool
	for _, r := range rows {
		if r.CheckID == "axfr" {
			found = r.Reason == "excluded by egress policy: intrusive"
		}
		if r.State == store.CoverageCheckFailed && r.State.Passing() {
			t.Error("check_failed must never read as passing")
		}
	}
	if !found {
		t.Error("Expected the excluding reason to be retained for axfr")
	}
}

func TestCoverageSummary_EmptyClaimsNoCoverage(t *testing.T) {
	s, ctx := newSignalStore(t)

	summary, err := s.ComputeCoverage(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to compute coverage: %v", err)
	}
	if summary.Total != 0 || summary.Fraction() != 0 {
		t.Errorf("Expected zero coverage for an unassessed asset, got %+v", summary)
	}
}

// A refused run writes no coverage and no observations. If the refusal itself
// were not recorded, the asset would be indistinguishable from one that was
// assessed and found clean — the exact confusion the run record exists to
// prevent.
func TestAssessmentRun_RefusalSurvivesWithoutCoverage(t *testing.T) {
	s, ctx := newSignalStore(t)

	started := time.Now().Add(-time.Second)
	run := &store.AssessmentRun{
		AssetID:    "asset-1",
		Outcome:    "refused",
		Error:      `"example.com" is outside the authorised scope`,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}
	if err := s.RecordAssessmentRun(ctx, run); err != nil {
		t.Fatalf("Failed to record assessment run: %v", err)
	}

	runs, err := s.GetAssessmentRuns(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to read assessment runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(runs))
	}
	if runs[0].Outcome != "refused" {
		t.Errorf("Expected the refusal to be retained, got %q", runs[0].Outcome)
	}
	if runs[0].Error == "" {
		t.Error("Expected the refusal to name its reason")
	}
	if runs[0].FinishedAt.IsZero() {
		t.Error("Expected the run to carry when it finished")
	}

	summary, err := s.ComputeCoverage(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to compute coverage: %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("A refusal must claim no coverage, got %+v", summary)
	}
}

// Only the latest run is kept: a second assessment must supersede the first
// rather than leaving a stale verdict for a reader to pick up.
func TestAssessmentRun_LatestSupersedes(t *testing.T) {
	s, ctx := newSignalStore(t)

	for _, outcome := range []string{"failed", "completed"} {
		if err := s.RecordAssessmentRun(ctx, &store.AssessmentRun{
			AssetID:        "asset-1",
			Outcome:        outcome,
			Profile:        "standard",
			LibraryVersion: "v1.1.0",
			StartedAt:      time.Now(),
			FinishedAt:     time.Now(),
		}); err != nil {
			t.Fatalf("Failed to record %s run: %v", outcome, err)
		}
	}

	runs, err := s.GetAssessmentRuns(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to read assessment runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Expected the run to be superseded, got %d rows", len(runs))
	}
	if runs[0].Outcome != "completed" {
		t.Errorf("Expected the latest outcome, got %q", runs[0].Outcome)
	}
	if runs[0].Profile != "standard" || runs[0].LibraryVersion != "v1.1.0" {
		t.Errorf("Expected provenance to survive persistence, got %+v", runs[0])
	}
}

// An outcome is the entire point of the row. A blank one would occupy the slot
// where a verdict belongs while saying nothing.
func TestAssessmentRun_RejectsBlankOutcome(t *testing.T) {
	s, ctx := newSignalStore(t)

	err := s.RecordAssessmentRun(ctx, &store.AssessmentRun{AssetID: "asset-1"})
	if err == nil {
		t.Fatal("Expected a run with no outcome to be rejected")
	}
}

// Never having been assessed is an ordinary state, not a fault.
func TestAssessmentRun_UnassessedYieldsNoRow(t *testing.T) {
	s, ctx := newSignalStore(t)

	runs, err := s.GetAssessmentRuns(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Expected no error for an unassessed asset: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("Expected no runs, got %d", len(runs))
	}
}
