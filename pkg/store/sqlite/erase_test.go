package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// seedEstate writes one of everything the engine discovers, plus the
// configuration the erase must not touch.
func seedEstate(t *testing.T, s *sqlite.SQLiteStore, ctx context.Context) {
	t.Helper()

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
	if err := s.SaveFinding(ctx, &store.Finding{
		ID:        "finding-1",
		AssetID:   "asset-1",
		Title:     "open port",
		Severity:  "high",
		Priority:  "high",
		Category:  "network",
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}); err != nil {
		t.Fatalf("Failed to seed finding: %v", err)
	}
	if err := s.SaveEmailPosture(ctx, &store.EmailPosture{
		Domain:      "example.com",
		DMARCPolicy: "reject",
		Priority:    "low",
		LastChecked: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to seed email posture: %v", err)
	}
	if err := s.SaveSignalObservation(ctx, &store.SignalObservation{
		ID:         "obs-1",
		AssetID:    "asset-1",
		SignalID:   "SURF-SPF-001",
		CheckID:    "spf",
		State:      "ok",
		Severity:   "medium",
		ObservedAt: time.Now(),
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}); err != nil {
		t.Fatalf("Failed to seed observation: %v", err)
	}
	if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		AssetID:        "asset-1",
		CheckID:        "spf",
		State:          store.CoverageOK,
		LibraryVersion: "v1.1.0",
	}); err != nil {
		t.Fatalf("Failed to seed coverage: %v", err)
	}
	if err := s.RecordAssessmentRun(ctx, &store.AssessmentRun{
		AssetID:    "asset-1",
		Outcome:    "completed",
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to seed run: %v", err)
	}
	if _, err := s.EnqueueJob(ctx, "discovery", []string{"example.com"}); err != nil {
		t.Fatalf("Failed to seed job: %v", err)
	}

	// Configuration, which must survive.
	if err := s.SaveSetting(ctx, "scope_settings", `{"isAuthorized":true}`); err != nil {
		t.Fatalf("Failed to seed setting: %v", err)
	}
	if err := s.ReplaceSignalRegistry(ctx, []store.SignalRegistryEntry{{
		SignalID:        "SURF-SPF-001",
		Condition:       "no SPF record",
		WeaknessClass:   "email-authentication",
		Scenario:        "spoofing",
		Stage:           "delivery",
		DedupGroup:      "spf:email-authentication",
		Control:         "spf",
		Direction:       "aggravating",
		RegistryVersion: "v1",
	}}); err != nil {
		t.Fatalf("Failed to seed registry: %v", err)
	}
}

func newEraseStore(t *testing.T) (*sqlite.SQLiteStore, context.Context) {
	t.Helper()
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "erase.db"))
	if err != nil {
		t.Fatalf("Failed to initialise store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, context.Background()
}

// The button promises two things at once, and both must hold: everything
// discovered is gone, and everything configured remains. A wipe that took the
// scope with it would silently revoke the operator's authorisation.
func TestEraseDiscoveredDataClearsTheEstate(t *testing.T) {
	s, ctx := newEraseStore(t)
	seedEstate(t, s, ctx)

	if err := s.EraseDiscoveredData(ctx); err != nil {
		t.Fatalf("EraseDiscoveredData failed: %v", err)
	}

	assets, err := s.GetAssets(ctx, "")
	if err != nil {
		t.Fatalf("Failed to read assets: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("Expected no assets, got %d", len(assets))
	}

	findings, err := s.GetFindings(ctx, "asset-1")
	if err != nil {
		t.Fatalf("Failed to read findings: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Expected no findings, got %d", len(findings))
	}

	postures, err := s.GetEmailPostures(ctx)
	if err != nil {
		t.Fatalf("Failed to read email postures: %v", err)
	}
	if len(postures) != 0 {
		t.Errorf("Expected no email postures, got %d", len(postures))
	}

	observations, err := s.GetSignalObservations(ctx, "")
	if err != nil {
		t.Fatalf("Failed to read observations: %v", err)
	}
	if len(observations) != 0 {
		t.Errorf("Expected no observations, got %d", len(observations))
	}

	coverage, err := s.GetAssessmentCoverage(ctx, "")
	if err != nil {
		t.Fatalf("Failed to read coverage: %v", err)
	}
	if len(coverage) != 0 {
		t.Errorf("Expected no coverage, got %d", len(coverage))
	}

	runs, err := s.GetAssessmentRuns(ctx, "")
	if err != nil {
		t.Fatalf("Failed to read runs: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("Expected no assessment runs, got %d", len(runs))
	}

	jobs, err := s.GetJobs(ctx, "")
	if err != nil {
		t.Fatalf("Failed to read jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("Expected no jobs, got %d", len(jobs))
	}
}

// The operator's authorisation is not this button's to revoke. Erasing it
// would take away the consent record that makes any future scan lawful.
func TestEraseDiscoveredDataPreservesConfiguration(t *testing.T) {
	s, ctx := newEraseStore(t)
	seedEstate(t, s, ctx)

	if err := s.EraseDiscoveredData(ctx); err != nil {
		t.Fatalf("EraseDiscoveredData failed: %v", err)
	}

	scope, err := s.GetSetting(ctx, "scope_settings")
	if err != nil {
		t.Fatalf("Failed to read the scope setting: %v", err)
	}
	if scope != `{"isAuthorized":true}` {
		t.Errorf("Expected the authorised scope to survive, got %q", scope)
	}

	registry, err := s.GetSignalRegistry(ctx)
	if err != nil {
		t.Fatalf("Failed to read the registry: %v", err)
	}
	if len(registry) != 1 {
		t.Errorf("Expected the signal registry to survive, got %d entries", len(registry))
	}
}

// The estate must be usable again immediately, not merely empty. A wipe that
// left the schema damaged would only surface on the next scan.
func TestEraseDiscoveredDataLeavesTheStoreWritable(t *testing.T) {
	s, ctx := newEraseStore(t)
	seedEstate(t, s, ctx)

	if err := s.EraseDiscoveredData(ctx); err != nil {
		t.Fatalf("EraseDiscoveredData failed: %v", err)
	}

	seedEstate(t, s, ctx)

	assets, err := s.GetAssets(ctx, "")
	if err != nil {
		t.Fatalf("Failed to read assets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("Expected the estate to be rebuildable, got %d assets", len(assets))
	}
}

// Erasing an already-empty estate is what a second click does. It must be a
// no-op rather than an error, or the UI would report a failure for a state
// that is exactly what the operator asked for.
func TestEraseDiscoveredDataIsIdempotent(t *testing.T) {
	s, ctx := newEraseStore(t)

	for i := 0; i < 2; i++ {
		if err := s.EraseDiscoveredData(ctx); err != nil {
			t.Fatalf("EraseDiscoveredData on an empty store failed: %v", err)
		}
	}
}
