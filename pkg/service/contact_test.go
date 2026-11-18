package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestContactPanelEndToEndAndFactorRemoval(t *testing.T) {
	db, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "contact.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	svc, err := NewAssessmentService(db, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveAsset(ctx, &store.Asset{
		ID: "api.example.com", Type: store.AssetTypeSubdomain, Value: "api.example.com",
		Status: store.AssetStatusActive, DiscoverySource: "ct-log", Confidence: 1,
		FirstSeen: t0, LastSeen: t0,
	}); err != nil {
		t.Fatal(err)
	}

	// Before any service probe, DNS resolution alone yields reachability unknown (suppressed).
	unprobed, err := svc.ContactPanel(ctx, "api.example.com", nil, t0.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if unprobed.Reachability != contact.ReachabilityUnknown || len(unprobed.Estimates) != 1 || unprobed.Estimates[0].Available {
		t.Fatalf("expected DNS-only asset to suppress contact estimate: %+v", unprobed)
	}

	// Save a responding HTTPS observation and a coverage row.
	if err := db.SaveServiceObservation(ctx, &store.ServiceObservation{
		ID: "obs-1", AssetID: "api.example.com", Host: "api.example.com", Port: 443,
		Service: "https", Transport: "tcp", Protocol: "https", Layer: "http",
		State: "responding", Coverage: store.CoverageOK,
		Evidence: `{"http_status":401,"tls_version":"TLS 1.3"}`, Profile: "most-common",
		ObservedAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		ID: "cov-1", AssetID: "api.example.com", CheckID: "ct", State: store.CoverageOK, AssessedAt: t0,
	}); err != nil {
		t.Fatal(err)
	}

	panel, err := svc.ContactPanel(ctx, "api.example.com", nil, t0.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if panel.Reachability != contact.ReachabilityResponding || len(panel.Estimates) != 1 {
		t.Fatalf("unexpected contact panel: %+v", panel)
	}
	est := panel.Estimates[0]
	if !est.Available || !est.IsPrior || est.LambdaContact.Point <= 0 || est.PContact.Point <= 0 {
		t.Fatalf("expected non-zero prior estimate: %+v", est)
	}

	// Remove CT discoverability factor and confirm rate drops while auth barrier stays.
	recomputed, err := svc.ContactPanel(ctx, "api.example.com", []string{"discoverability:ct"}, t0.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !(recomputed.Estimates[0].LambdaContact.Point < est.LambdaContact.Point) {
		t.Fatalf("expected removing discoverability:ct to lower lambda: got %v vs %v",
			recomputed.Estimates[0].LambdaContact.Point, est.LambdaContact.Point)
	}

	// Portfolio aggregation reflects 1 uncovered instance, then 1 remediated after clean re-probe.
	aggs, err := svc.WeaknessAggregations(ctx, t0.Add(48*time.Hour))
	if err != nil || len(aggs) != 1 || aggs[0].UncoveredInstances != 1 || aggs[0].ResidualPContact.Point <= 0 {
		t.Fatalf("unexpected open weakness aggregations: %+v, %v", aggs, err)
	}

	if err := db.SaveServiceObservation(ctx, &store.ServiceObservation{
		ID: "obs-2", AssetID: "api.example.com", Host: "api.example.com", Port: 443,
		Service: "https", Transport: "tcp", Protocol: "https", Layer: "http",
		State: "not_responding", Coverage: store.CoverageOK,
		Evidence: `{}`, Profile: "most-common",
		ObservedAt: t0.Add(72 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	aggsClosed, err := svc.WeaknessAggregations(ctx, t0.Add(96*time.Hour))
	if err != nil || len(aggsClosed) != 1 || aggsClosed[0].ConfirmedRemediated != 1 || aggsClosed[0].ResidualPContact.Point != 0 {
		t.Fatalf("expected confirmed clean re-observation to reduce residual P(contact) to 0: %+v, %v", aggsClosed, err)
	}
}
