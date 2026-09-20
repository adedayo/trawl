package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// Detail is the part of a finding's explanation that names what was found on
// this domain. It is stored rather than derived because the vantage catalogue
// describes an identifier and has never seen the domain.

func TestSignalObservation_DetailSurvivesRoundTrip(t *testing.T) {
	s, ctx := newSignalStore(t)

	want := "Specifically, the term `include:dead.example` names `dead.example`, " +
		"which does not resolve or publishes no SPF record."
	obs := &store.SignalObservation{
		AssetID:  "asset-1",
		SignalID: "SURF-SPF-009",
		CheckID:  "spf",
		State:    store.CoverageOK,
		Severity: store.SeverityMedium,
		Evidence: "spf.term=include:dead.example",
		Detail:   want,
	}
	if err := s.SaveSignalObservation(ctx, obs); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.GetSignalObservations(ctx, "asset-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(got))
	}
	if got[0].Detail != want {
		t.Errorf("Detail = %q, want %q", got[0].Detail, want)
	}
}

// A re-scan that now pinpoints the fault must overwrite the earlier silence,
// or the upgrade would appear to have done nothing to findings already stored.
func TestSignalObservation_DetailIsUpdatedOnUpsert(t *testing.T) {
	s, ctx := newSignalStore(t)

	base := store.SignalObservation{
		AssetID:  "asset-1",
		SignalID: "SURF-SPF-009",
		CheckID:  "spf",
		State:    store.CoverageOK,
		Severity: store.SeverityMedium,
		Evidence: "spf.term=include:dead.example",
	}
	if err := s.SaveSignalObservation(ctx, &base); err != nil {
		t.Fatalf("first save: %v", err)
	}

	updated := base
	updated.ID = ""
	updated.Detail = "Specifically, the term `include:dead.example` is at fault."
	if err := s.SaveSignalObservation(ctx, &updated); err != nil {
		t.Fatalf("second save: %v", err)
	}

	got, err := s.GetSignalObservations(ctx, "asset-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the upsert to keep one row, got %d", len(got))
	}
	if got[0].Detail != updated.Detail {
		t.Errorf("Detail = %q, want %q", got[0].Detail, updated.Detail)
	}
}

// The column was added after the table shipped. An existing database must be
// migrated in place and keep its rows: CREATE TABLE IF NOT EXISTS is a no-op
// against a database that already has the table, so without the additive
// migration every installed copy would fail on the first write.
func TestSignalObservation_DetailColumnIsAddedToAnExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Build the pre-detail shape by hand, and put a row in it.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	legacy := `
	CREATE TABLE assets (id TEXT PRIMARY KEY);
	CREATE TABLE signal_observations (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		signal_id TEXT NOT NULL,
		check_id TEXT NOT NULL,
		state TEXT NOT NULL,
		severity TEXT NOT NULL,
		evidence TEXT,
		mapped INTEGER NOT NULL DEFAULT 0,
		registry_version TEXT NOT NULL,
		library_version TEXT NOT NULL,
		observed_at DATETIME NOT NULL,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		UNIQUE(asset_id, signal_id)
	);
	INSERT INTO assets (id) VALUES ('asset-1');
	INSERT INTO signal_observations VALUES (
		'sig-1', 'asset-1', 'SURF-SPF-009', 'spf', 'ok', 'medium',
		'spf.term=include:dead.example', 1, 'r1', 'v1.5.0',
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);`
	if _, err := raw.Exec(legacy); err != nil {
		t.Fatalf("seeding the legacy schema: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s, err := sqlite.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("opening a pre-detail database must migrate it, got: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	got, err := s.GetSignalObservations(ctx, "asset-1")
	if err != nil {
		t.Fatalf("reading migrated rows: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("migration lost rows: expected 1, got %d", len(got))
	}
	// The old row has no detail, which is the same absence as a signal that
	// names no particular item. It must read as empty, not as an error.
	if got[0].Detail != "" {
		t.Errorf("expected no detail on a legacy row, got %q", got[0].Detail)
	}
	if got[0].Evidence != "spf.term=include:dead.example" {
		t.Errorf("migration altered an existing row: %q", got[0].Evidence)
	}

	// And the migrated database must accept a write that carries one.
	next := store.SignalObservation{
		AssetID:  "asset-1",
		SignalID: "SURF-SPF-009",
		CheckID:  "spf",
		State:    store.CoverageOK,
		Severity: store.SeverityMedium,
		Detail:   "Specifically, the term `include:dead.example` is at fault.",
		LastSeen: time.Now(),
	}
	if err := s.SaveSignalObservation(ctx, &next); err != nil {
		t.Fatalf("writing to a migrated database: %v", err)
	}
}
