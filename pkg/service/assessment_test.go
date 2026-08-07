package service_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// newAssessmentService wires the service against a real store rather than a
// stub. The behaviour under test is a join across four tables, and a stub that
// returns whatever the test hands it would only be asserting that the test set
// its own fixtures up correctly.
func newAssessmentService(t *testing.T) (*service.AssessmentService, store.Store, context.Context) {
	t.Helper()

	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "assessment.db"))
	if err != nil {
		t.Fatalf("Failed to initialise store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	svc, err := service.NewAssessmentService(s, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatalf("Failed to build assessment service: %v", err)
	}

	ctx := context.Background()
	if err := svc.SyncRegistry(ctx); err != nil {
		t.Fatalf("Failed to sync registry: %v", err)
	}
	return svc, s, ctx
}

func seedAsset(t *testing.T, s store.Store, ctx context.Context, id, domain string) {
	t.Helper()
	if err := s.SaveAsset(ctx, &store.Asset{
		ID:              id,
		Type:            store.AssetTypeDomain,
		Value:           domain,
		Status:          store.AssetStatusActive,
		DiscoverySource: "manual",
		Confidence:      1,
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}); err != nil {
		t.Fatalf("Failed to seed asset %s: %v", id, err)
	}
}

// A domain whose only record is a refusal must still appear in the portfolio.
// It is precisely the case an operator needs to see, and it carries neither
// coverage nor observations to be found by.
func TestViews_RefusedAssetIsNotOmitted(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)

	seedAsset(t, s, ctx, "asset-refused", "refused.example")
	if err := s.RecordAssessmentRun(ctx, &store.AssessmentRun{
		AssetID:    "asset-refused",
		Outcome:    "refused",
		Error:      "outside the authorised scope",
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to record run: %v", err)
	}

	views, err := svc.Views(ctx)
	if err != nil {
		t.Fatalf("Views failed: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("Expected the refused asset to appear, got %d views", len(views))
	}
	v := views[0]
	if v.Outcome != "refused" {
		t.Errorf("Expected outcome refused, got %q", v.Outcome)
	}
	if v.Error == "" {
		t.Error("Expected the refusal to name its reason")
	}
	if v.Domain != "refused.example" {
		t.Errorf("Expected the domain for display, got %q", v.Domain)
	}
	if v.Coverage.Total != 0 {
		t.Errorf("A refusal must claim no coverage, got %+v", v.Coverage)
	}
}

// The batch path and the single-asset path must agree. They are separate
// implementations of the same join, and a divergence would mean the portfolio
// list and the domain page disagreed about the same domain.
func TestViews_AgreesWithView(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)

	seedAsset(t, s, ctx, "asset-1", "one.example")
	seedAsset(t, s, ctx, "asset-2", "two.example")

	if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		AssetID:        "asset-1",
		CheckID:        "spf",
		State:          store.CoverageOK,
		LibraryVersion: "v1.1.0",
	}); err != nil {
		t.Fatalf("Failed to record coverage: %v", err)
	}
	if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		AssetID:        "asset-2",
		CheckID:        "dmarc",
		State:          store.CoverageNotChecked,
		Reason:         "excluded by egress policy",
		LibraryVersion: "v1.1.0",
	}); err != nil {
		t.Fatalf("Failed to record coverage: %v", err)
	}
	for _, id := range []string{"asset-1", "asset-2"} {
		if err := s.RecordAssessmentRun(ctx, &store.AssessmentRun{
			AssetID:    id,
			Outcome:    "completed",
			StartedAt:  time.Now(),
			FinishedAt: time.Now(),
		}); err != nil {
			t.Fatalf("Failed to record run for %s: %v", id, err)
		}
	}

	views, err := svc.Views(ctx)
	if err != nil {
		t.Fatalf("Views failed: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("Expected 2 views, got %d", len(views))
	}

	for _, batched := range views {
		single, err := svc.View(ctx, batched.AssetID)
		if err != nil {
			t.Fatalf("View(%s) failed: %v", batched.AssetID, err)
		}
		if single.Domain != batched.Domain {
			t.Errorf("%s: domain %q batched, %q single", batched.AssetID, batched.Domain, single.Domain)
		}
		if single.Outcome != batched.Outcome {
			t.Errorf("%s: outcome %q batched, %q single", batched.AssetID, batched.Outcome, single.Outcome)
		}
		if single.Coverage != batched.Coverage {
			t.Errorf("%s: coverage %+v batched, %+v single", batched.AssetID, batched.Coverage, single.Coverage)
		}
		if len(single.Controls) != len(batched.Controls) {
			t.Errorf("%s: %d controls batched, %d single",
				batched.AssetID, len(batched.Controls), len(single.Controls))
		}
	}
}

// Grouping in memory must not leak one asset's rows into another's view.
func TestViews_DoesNotCrossContaminateAssets(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)

	seedAsset(t, s, ctx, "asset-1", "one.example")
	seedAsset(t, s, ctx, "asset-2", "two.example")

	for _, check := range []string{"spf", "dmarc", "dkim"} {
		if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
			AssetID:        "asset-1",
			CheckID:        check,
			State:          store.CoverageOK,
			LibraryVersion: "v1.1.0",
		}); err != nil {
			t.Fatalf("Failed to record coverage: %v", err)
		}
	}
	if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		AssetID:        "asset-2",
		CheckID:        "spf",
		State:          store.CoverageOK,
		LibraryVersion: "v1.1.0",
	}); err != nil {
		t.Fatalf("Failed to record coverage: %v", err)
	}

	views, err := svc.Views(ctx)
	if err != nil {
		t.Fatalf("Views failed: %v", err)
	}

	totals := map[string]int{}
	for _, v := range views {
		totals[v.AssetID] = v.Coverage.Total
	}
	if totals["asset-1"] != 3 {
		t.Errorf("Expected 3 checks for asset-1, got %d", totals["asset-1"])
	}
	if totals["asset-2"] != 1 {
		t.Errorf("Expected 1 check for asset-2, got %d", totals["asset-2"])
	}
}

// An asset with rows but no run record predates the run table. Reporting that
// as completed would be a claim the store cannot support.
func TestView_MissingRunReportsUnknown(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)

	seedAsset(t, s, ctx, "asset-1", "one.example")
	if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		AssetID:        "asset-1",
		CheckID:        "spf",
		State:          store.CoverageOK,
		LibraryVersion: "v1.1.0",
	}); err != nil {
		t.Fatalf("Failed to record coverage: %v", err)
	}

	v, err := svc.View(ctx, "asset-1")
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
	if v.Outcome != "unknown" {
		t.Errorf("Expected an unrecorded run to read as unknown, got %q", v.Outcome)
	}
}

func TestViews_EmptyStoreYieldsNoViews(t *testing.T) {
	svc, _, ctx := newAssessmentService(t)

	views, err := svc.Views(ctx)
	if err != nil {
		t.Fatalf("Views failed: %v", err)
	}
	if len(views) != 0 {
		t.Errorf("Expected no views, got %d", len(views))
	}
}

// Every slice in the view model must marshal as an array, never as null.
//
// A control with nothing raised against it is the ordinary case, and a nil Go
// slice becomes JSON null. A frontend reading .length off null throws during
// render, and the failure presents as a blank panel rather than as an error
// anyone can trace back to the service — so this is asserted on the wire form,
// not on the Go value.
func TestView_SlicesAreNeverNullOnTheWire(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)

	seedAsset(t, s, ctx, "asset-1", "one.example")
	// Coverage for one check only. Every other control in the registry ends up
	// with no checks concluded and no advisories — precisely the shape that
	// produced nil slices.
	if err := s.RecordAssessmentCoverage(ctx, &store.AssessmentCoverage{
		AssetID:        "asset-1",
		CheckID:        "spf",
		State:          store.CoverageOK,
		LibraryVersion: "v1.1.0",
	}); err != nil {
		t.Fatalf("Failed to record coverage: %v", err)
	}

	v, err := svc.View(ctx, "asset-1")
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}

	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal view: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Failed to decode view: %v", err)
	}

	for _, key := range []string{"controls", "scenarios", "unmapped"} {
		if decoded[key] == nil {
			t.Errorf("%s marshalled as null; the view model promises an array", key)
		}
	}

	controls, _ := decoded["controls"].([]any)
	if len(controls) == 0 {
		t.Fatal("Expected the registry to yield controls")
	}
	for _, c := range controls {
		control, _ := c.(map[string]any)
		name, _ := control["control"].(string)
		for _, key := range []string{"checks", "signals"} {
			if control[key] == nil {
				t.Errorf("control %s: %s marshalled as null; the view model promises an array", name, key)
			}
		}
	}
}

// A signal must arrive at the UI carrying vantage's explanation of it, not
// just its title and its raw evidence keys.
//
// This is the difference between an operator reading "All mail exchangers
// operated by a single provider — mx.provider=mimecast.com" and being none the
// wiser, and reading why single-provider concentration is a resilience
// observation and what to do about it. The catalogue holds that prose; the
// signal registry does not, and the store does not. If this join is dropped,
// every advisory in the product becomes uninterpretable without reading
// vantage's source.
func TestViews_SignalsCarryCatalogueExplanation(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)

	seedAsset(t, s, ctx, "asset-mx", "mx.example")
	if err := s.SaveSignalObservation(ctx, &store.SignalObservation{
		AssetID:  "asset-mx",
		SignalID: "SURF-MX-005",
		CheckID:  "mx",
		State:    store.CoverageOK,
		Severity: store.SeverityLow,
		Evidence: "mx.provider=mimecast.com",
		Mapped:   true,
	}); err != nil {
		t.Fatalf("Failed to save observation: %v", err)
	}

	views, err := svc.Views(ctx)
	if err != nil {
		t.Fatalf("Views failed: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("Expected one view, got %d", len(views))
	}

	var found *service.SignalView
	for _, c := range views[0].Controls {
		for i := range c.Signals {
			if c.Signals[i].SignalID == "SURF-MX-005" {
				found = &c.Signals[i]
			}
		}
	}
	if found == nil {
		t.Fatal("Expected SURF-MX-005 to appear against a control")
	}
	if found.Description == "" {
		t.Error("Expected the catalogue description to reach the view; " +
			"without it the UI can only show evidence keys")
	}
	if found.Remediation == "" {
		t.Error("Expected the catalogue remediation to reach the view")
	}
	if len(found.References) == 0 {
		t.Error("Expected the catalogue references to reach the view")
	}
}
