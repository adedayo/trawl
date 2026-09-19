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

// These tests drive persist directly. The decision under test is which of two
// recordings to make, and routing it through a live assessment would make the
// outcome depend on DNS rather than on the rule.

func suppressionFixture(t *testing.T) (*AssessmentService, store.Store, context.Context) {
	t.Helper()

	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "suppression.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	svc, err := NewAssessmentService(s, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	ctx := context.Background()
	if err := s.SaveAsset(ctx, &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com", Status: store.AssetStatusActive,
	}); err != nil {
		t.Fatalf("asset: %v", err)
	}
	return svc, s, ctx
}

// run persists one assessment: the given hosting, on the given basis.
func run(t *testing.T, svc *AssessmentService, ctx context.Context, provider, url string, fetched time.Time) {
	t.Helper()

	res := vadapter.Result{
		Coverage:             []store.AssessmentCoverage{{AssetID: "asset-1", CheckID: "net", State: store.CoverageOK}},
		AttributionAttempted: true,
		Attribution: []store.AssetAttribution{{
			AssetID: "asset-1", Host: "www.example.com", Address: "203.0.113.10",
			Provider: provider, ObservedAt: fetched,
		}},
		AttributionProvenance: []store.AttributionProvenance{{
			AssetID: "asset-1", Provider: "ranges", URL: url, FetchedAt: fetched,
		}},
	}
	if err := svc.persist(ctx, res); err != nil {
		t.Fatalf("persist: %v", err)
	}
}

// TestAHostThatMovesOnAnUnchangedBasisIsRaised is the signal the suppression
// must not swallow. Both runs attributed against the same provider data, so
// the difference is the estate's.
func TestAHostThatMovesOnAnUnchangedBasisIsRaised(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	monday := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	run(t, svc, ctx, "aws", "https://ranges.invalid/v1", monday)
	run(t, svc, ctx, "azure", "https://ranges.invalid/v1", monday.Add(24*time.Hour))

	regressions, err := s.GetRegressions(ctx)
	if err != nil {
		t.Fatalf("GetRegressions: %v", err)
	}
	if len(regressions) != 1 {
		t.Fatalf("a host moving provider on unchanged provider data is a real change and must be raised; got %d regressions", len(regressions))
	}
	if regressions[0].AttributeType != store.AttributionAttribute {
		t.Fatalf("raised under the wrong attribute: %q", regressions[0].AttributeType)
	}
}

// TestAChangeExplainedByNewProviderDataIsSuppressed is the noise. The ranges
// came from a different endpoint, so this evidence cannot tell a move from a
// republication — and a regression list of mostly noise is one an operator
// stops reading, taking the real moves with it.
func TestAChangeExplainedByNewProviderDataIsSuppressed(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	monday := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	run(t, svc, ctx, "aws", "https://ranges.invalid/v1", monday)
	run(t, svc, ctx, "azure", "https://mirror.invalid/v2", monday.Add(24*time.Hour))

	regressions, err := s.GetRegressions(ctx)
	if err != nil {
		t.Fatalf("GetRegressions: %v", err)
	}
	if len(regressions) != 0 {
		t.Fatalf("an attribution change across a changed basis must not be raised; got %d regressions", len(regressions))
	}
}

// TestSuppressionDoesNotMerelyDefer is the trap this design exists to avoid.
// If the suppressed run skipped its snapshot, the baseline would still hold
// the pre-refresh value and the very next run on a settled basis would raise
// the same change — suppression that postpones rather than suppresses.
func TestSuppressionDoesNotMerelyDefer(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	monday := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	run(t, svc, ctx, "aws", "https://ranges.invalid/v1", monday)
	// The basis changes and the attribution changes with it: suppressed.
	run(t, svc, ctx, "azure", "https://mirror.invalid/v2", monday.Add(24*time.Hour))
	// Same basis as the suppressed run, same hosting: nothing has moved.
	run(t, svc, ctx, "azure", "https://mirror.invalid/v2", monday.Add(48*time.Hour))

	regressions, err := s.GetRegressions(ctx)
	if err != nil {
		t.Fatalf("GetRegressions: %v", err)
	}
	if len(regressions) != 0 {
		t.Fatalf("the suppressed change must not resurface on the next run; got %d regressions", len(regressions))
	}
}

// TestSettledHostingRaisesNothing guards the common case: same basis, same
// hosting, no news.
func TestSettledHostingRaisesNothing(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	monday := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	run(t, svc, ctx, "aws", "https://ranges.invalid/v1", monday)
	run(t, svc, ctx, "aws", "https://ranges.invalid/v1", monday.Add(24*time.Hour))

	regressions, err := s.GetRegressions(ctx)
	if err != nil {
		t.Fatalf("GetRegressions: %v", err)
	}
	if len(regressions) != 0 {
		t.Fatalf("unchanged hosting on an unchanged basis is not a regression; got %d", len(regressions))
	}
}
