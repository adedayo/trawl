package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"

	// Linked for its factory registration, so these tests exercise the same
	// selection path the binaries use.
	_ "github.com/adedayo/trawl/pkg/store/sqlite"
)

func newTestCore(t *testing.T) *Core {
	t.Helper()

	s, err := store.Open(t.TempDir() + "/trawl.db")
	if err != nil {
		t.Fatalf("failed to open the test store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	c, err := New(s, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatalf("failed to build the application layer: %v", err)
	}
	return c
}

// TestScopeFailsClosed is the test that matters most in this package.
//
// Every path into an assessment reads the scope, and a scope that reads as
// permissive when it should not is the difference between an authorised audit
// and an unauthorised scan of somebody else's infrastructure. Each case here
// is a way that record can be wrong.
func TestScopeFailsClosed(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		stored  string
		present bool
	}{
		{name: "no record at all"},
		{name: "empty record", stored: "", present: true},
		{name: "malformed JSON", stored: "{not json", present: true},
		{
			name:    "domains listed but never signed",
			stored:  `{"seedDomainsList":["example.com"],"isAuthorized":false}`,
			present: true,
		},
		{
			name:    "signed but naming no domains",
			stored:  `{"seedDomainsList":[],"isAuthorized":true}`,
			present: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCore(t)
			if tc.present {
				if err := c.SaveSetting(ctx, ScopeSettingsKey, tc.stored); err != nil {
					t.Fatalf("failed to seed the scope: %v", err)
				}
			}

			if got := c.Scope(ctx); got.Authorised() {
				t.Fatalf("scope %+v was treated as authorised; it must fail closed", got)
			}
		})
	}
}

// TestAssessDomainRefusesWithoutAuthorisation checks that the refusal is
// reported as a result rather than as an error.
//
// A refusal is a fact about the assessment, and the operator needs it recorded
// and rendered. Returning an error instead would let a caller log it and move
// on, leaving the UI unable to distinguish "we were not allowed to look" from
// "we looked and found nothing".
func TestAssessDomainRefusesWithoutAuthorisation(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)

	view, err := c.AssessDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("an unauthorised assessment must not error: %v", err)
	}
	if view.Outcome != "refused" {
		t.Fatalf("outcome = %q, want %q", view.Outcome, "refused")
	}
	if view.Error == "" {
		t.Fatal("a refusal must name its reason")
	}
}

// TestSaveScopeRoundTrips guards the record both transports share.
func TestSaveScopeRoundTrips(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)

	want := Scope{
		SeedDomainsList:    []string{"example.com"},
		ConsentedEndpoints: []string{"crt.sh"},
		IsAuthorized:       true,
		SignerName:         "A. Operator",
		SignerTitle:        "CISO",
		AuthorizationDate:  "2026-08-06",
	}
	if err := c.SaveScope(ctx, want); err != nil {
		t.Fatalf("failed to save the scope: %v", err)
	}

	got := c.Scope(ctx)
	if !got.Authorised() {
		t.Fatal("a signed scope naming a domain must be authorised")
	}
	if got.SignerName != want.SignerName || len(got.ConsentedEndpoints) != 1 {
		t.Fatalf("scope did not round-trip: %+v", got)
	}

	// The desktop build reads this key directly, so its shape is a contract
	// between the two deployments rather than an implementation detail.
	raw, err := c.Setting(ctx, ScopeSettingsKey)
	if err != nil {
		t.Fatalf("failed to read the scope setting: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("the stored scope is not valid JSON: %v", err)
	}
	if _, ok := decoded["seedDomainsList"]; !ok {
		t.Fatal("the stored scope must carry seedDomainsList")
	}
}

// TestAssessmentPersistsAgainstARegisteredAsset guards a referential
// constraint that is easy to violate and silent until run time.
//
// Observations and coverage carry a foreign key onto assets. An assessment
// that invented its own identifier for a domain would be rejected by the store
// — and, because scans run detached, rejected in a log line rather than in
// front of the operator who asked for it. This test exercises the persistence
// path end to end with a fake assessment result.
func TestAssessmentPersistsAgainstARegisteredAsset(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	s := c.Store()

	// Register the domain the way discovery would.
	asset := store.Asset{
		ID:              "example.com",
		Type:            store.AssetTypeDomain,
		Value:           "example.com",
		Status:          store.AssetStatusActive,
		DiscoverySource: "seed",
		Confidence:      1,
	}
	if err := s.SaveAsset(ctx, &asset); err != nil {
		t.Fatalf("failed to register the asset: %v", err)
	}

	cov := store.AssessmentCoverage{
		AssetID:        asset.ID,
		CheckID:        "spf",
		State:          store.CoverageOK,
		LibraryVersion: "v1.2.0",
	}
	if err := s.RecordAssessmentCoverage(ctx, &cov); err != nil {
		t.Fatalf("coverage must persist against a registered asset: %v", err)
	}

	obs := store.SignalObservation{
		AssetID:        asset.ID,
		SignalID:       "SURF-SPF-005",
		CheckID:        "spf",
		State:          store.CoverageOK,
		Severity:       store.SeverityMedium,
		Evidence:       "record=v=spf1 ~all",
		Mapped:         true,
		LibraryVersion: "v1.2.0",
	}
	if err := s.SaveSignalObservation(ctx, &obs); err != nil {
		t.Fatalf("observations must persist against a registered asset: %v", err)
	}

	// The read path must find it by domain, not by a synthesised identifier.
	view, err := c.Assessment(ctx, "example.com")
	if err != nil {
		t.Fatalf("failed to read the assessment: %v", err)
	}
	if view.Domain != "example.com" {
		t.Fatalf("Domain = %q, want the asset's value", view.Domain)
	}
	if view.Coverage.Total == 0 {
		t.Fatal("the stored coverage must appear in the view")
	}
}

// TestAssessmentOfUnknownDomainIsNotAnError checks that asking about a domain
// nobody has assessed reports exactly that, rather than failing or — worse —
// registering it as an asset as a side effect of being looked at.
func TestAssessmentOfUnknownDomainIsNotAnError(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)

	view, err := c.Assessment(ctx, "never-seen.example")
	if err != nil {
		t.Fatalf("an unknown domain must not error: %v", err)
	}
	if view.Outcome != "not_assessed" {
		t.Fatalf("outcome = %q, want %q", view.Outcome, "not_assessed")
	}

	assets, err := c.Assets(ctx, "")
	if err != nil {
		t.Fatalf("failed to list assets: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("reading an assessment must not create assets; found %d", len(assets))
	}
}

// TestStartSeedsRegistry checks that a fresh deployment can resolve signal
// identifiers. Without this, every observation would be recorded as unmapped
// and the UI would render a wall of bare identifiers.
func TestStartSeedsRegistry(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)

	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	entries, err := c.Store().GetSignalRegistry(ctx)
	if err != nil {
		t.Fatalf("failed to read the registry: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("Start must seed the signal registry")
	}

	var sample store.SignalRegistryEntry
	for _, e := range entries {
		if e.SignalID == "SURF-AXFR-001" {
			sample = e
		}
	}
	if sample.DedupGroup == "" || sample.Control == "" {
		t.Fatalf("a seeded entry must carry its mapping: %+v", sample)
	}
}
