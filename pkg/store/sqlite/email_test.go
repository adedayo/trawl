package sqlite

import (
	"context"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

// TestTheFourStatesSurviveTheStore. The states are only useful if they are
// still four after a write and a read. A store that narrows them on the way
// through undoes the distinction everywhere downstream, and does it in the
// one place nobody thinks to look.
func TestTheFourStatesSurviveTheStore(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	want := store.EmailPosture{
		Domain: "example.com",
		SPF:    store.EmailControl{State: store.CoverageOK, Detail: "v=spf1 -all", Conclusive: true},
		DKIM: store.EmailControl{
			State:  store.CoverageCheckFailed,
			Reason: "none of the 13 common selectors probed resolved",
		},
		DMARC:  store.EmailControl{State: store.CoverageNotFound, Conclusive: true},
		MTASTS: store.EmailControl{State: store.CoverageNotChecked, Reason: "excluded by egress policy"},
		TLSRPT: store.EmailControl{State: store.CoverageNotFound, Conclusive: true},
		BIMI:   store.EmailControl{State: store.CoverageNotChecked},
		CAA:    store.EmailControl{State: store.CoverageOK, Conclusive: true},

		DMARCPercent:          100,
		SPFAllMechanism:       "-",
		SPFLookups:            4,
		DKIMSelectorsExamined: []string{"default", "google"},
		Priority:              store.SeverityHigh,
	}

	if err := s.SaveEmailPosture(ctx, &want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.GetEmailPostures(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one posture, got %d", len(got))
	}
	p := got[0]

	for name, pair := range map[string][2]store.EmailControl{
		"spf":    {want.SPF, p.SPF},
		"dkim":   {want.DKIM, p.DKIM},
		"dmarc":  {want.DMARC, p.DMARC},
		"mtasts": {want.MTASTS, p.MTASTS},
		"tlsrpt": {want.TLSRPT, p.TLSRPT},
		"bimi":   {want.BIMI, p.BIMI},
		"caa":    {want.CAA, p.CAA},
	} {
		if pair[0] != pair[1] {
			t.Fatalf("%s round-tripped as %+v, want %+v", name, pair[1], pair[0])
		}
	}

	if p.Priority != store.SeverityHigh || p.SPFAllMechanism != "-" || p.SPFLookups != 4 {
		t.Fatalf("the evidence severity was computed from must survive too; got %+v", p)
	}
	if len(p.DKIMSelectorsExamined) != 2 {
		t.Fatalf("selectors examined = %v; the search must stay legible", p.DKIMSelectorsExamined)
	}
}

// TestTheReasonSurvives. "We could not tell" is only actionable when it says
// why; without the reason a reader has a gap they cannot close.
func TestTheReasonSurvives(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	in := store.EmailPosture{
		Domain: "example.com",
		MTASTS: store.EmailControl{
			State:  store.CoverageNotChecked,
			Reason: "third-party endpoint mta-sts.example.com is not on the consented list",
		},
	}
	if err := s.SaveEmailPosture(ctx, &in); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.GetEmailPostures(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got[0].MTASTS.Reason == "" {
		t.Fatal("an excluded check must name what excluded it, or the operator cannot consent to it")
	}
}

// TestALegacyRowReadsAsUnassessed.
//
// A row written before the widening holds booleans that cannot distinguish an
// absent control from a failed lookup. Reading "ok" out of one would preserve
// the collapse inside the migration — the very thing being removed — so the
// honest reading is that nothing about it was established.
func TestALegacyRowReadsAsUnassessed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO email_postures (domain, spf_valid, dkim_found, dmarc_policy, priority, last_checked)
		VALUES ('legacy.example.com', 1, 1, 'reject', 'low', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("seeding a pre-migration row: %v", err)
	}

	got, err := s.GetEmailPostures(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the legacy row, got %d rows", len(got))
	}

	if got[0].SPF.State.Assessed() {
		t.Fatalf("spf state = %q: a boolean from the old schema cannot tell an absent control from a failed lookup, so it must not be promoted to an assessment", got[0].SPF.State)
	}
	if got[0].SPF.State.Passing() {
		t.Fatal("a legacy row must never read as a control in place")
	}
	if assessed, total := got[0].Assessed(); assessed != 0 || total != 7 {
		t.Fatalf("assessed %d of %d, want 0 of 7", assessed, total)
	}
}

// TestAnOlderDatabaseGainsTheNewColumns.
//
// CREATE TABLE IF NOT EXISTS is a no-op against a database that already has
// the table, so without an explicit ALTER every existing installation would
// keep the old shape and fail on the first write — at run time, on a user's
// machine, rather than here.
func TestAnOlderDatabaseGainsTheNewColumns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Reconstruct the pre-migration shape, as an installation upgrading from
	// an earlier build would have it.
	if _, err := s.db.ExecContext(ctx, `
		DROP TABLE email_postures;
		CREATE TABLE email_postures (
			domain TEXT PRIMARY KEY,
			spf_valid INTEGER NOT NULL,
			dkim_found INTEGER NOT NULL,
			dmarc_policy TEXT NOT NULL,
			priority TEXT NOT NULL,
			last_checked DATETIME NOT NULL
		);`); err != nil {
		t.Fatalf("recreating the old schema: %v", err)
	}

	if err := s.migrate(ctx); err != nil {
		t.Fatalf("migrating an existing database: %v", err)
	}

	// Idempotent: opening the database twice must not fail on a column that
	// is already there.
	if err := s.migrate(ctx); err != nil {
		t.Fatalf("migrating twice: %v", err)
	}

	p := store.EmailPosture{
		Domain: "example.com",
		DMARC:  store.EmailControl{State: store.CoverageNotFound, Conclusive: true},
	}
	if err := s.SaveEmailPosture(ctx, &p); err != nil {
		t.Fatalf("writing to a migrated database: %v", err)
	}

	got, err := s.GetEmailPostures(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got[0].DMARC.State != store.CoverageNotFound {
		t.Fatalf("dmarc = %q after migration, want %q", got[0].DMARC.State, store.CoverageNotFound)
	}
}
