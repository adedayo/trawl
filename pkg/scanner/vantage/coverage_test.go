package vantage

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	vfinding "github.com/adedayo/vantage/pkg/finding"
	"github.com/adedayo/vantage/pkg/netattr"
	vobs "github.com/adedayo/vantage/pkg/observation"

	"github.com/adedayo/trawl/pkg/store"
)

func netObservation(failed, stale []string) *vfinding.Observation {
	return &vfinding.Observation{Network: &vobs.Network{
		Domain:        "example.com",
		FailedSources: failed,
		StaleSources:  stale,
	}}
}

// TestFailedSourcesDegradeAConclusiveCheck is the whole point of this file. An
// estate hosted entirely by a provider whose ranges failed to load would
// otherwise present as an estate using no third-party hosting: a clean result
// manufactured from missing evidence.
func TestFailedSourcesDegradeAConclusiveCheck(t *testing.T) {
	for _, conclusive := range []store.CoverageState{store.CoverageOK, store.CoverageNotFound} {
		state, reason := attributionGap(conclusive, "", netObservation([]string{"azure"}, nil))

		if state != store.CoverageCheckFailed {
			t.Fatalf("a %s check whose provider ranges could not be loaded must degrade to check_failed, got %s", conclusive, state)
		}
		if !strings.Contains(reason, "azure") {
			t.Fatalf("the reason must name the source that failed, so the gap can be acted on; got %q", reason)
		}
	}
}

// TestStaleSourcesAnnotateWithoutDegrading pins the other half. Stale data
// loaded and is usable — a prefix that moved last week is almost always still
// announced by the same operator. Degrading it would invent a coverage gap
// and suppress real findings every time a publisher had an outage.
func TestStaleSourcesAnnotateWithoutDegrading(t *testing.T) {
	state, reason := attributionGap(store.CoverageOK, "", netObservation(nil, []string{"gcp"}))

	if state != store.CoverageOK {
		t.Fatalf("stale provider data is usable and must not degrade the state, got %s", state)
	}
	if !strings.Contains(reason, "gcp") || !strings.Contains(reason, "not refreshed") {
		t.Fatalf("a stale basis must still be recorded for a reader comparing two runs; got %q", reason)
	}
}

// TestAnAlreadyFailedCheckKeepsItsOwnReason guards against the annotation
// displacing the more specific explanation of why the check stopped.
func TestAnAlreadyFailedCheckKeepsItsOwnReason(t *testing.T) {
	state, reason := attributionGap(
		store.CoverageCheckFailed,
		"dns_timeout: the resolver did not answer",
		netObservation([]string{"azure"}, nil),
	)

	if state != store.CoverageCheckFailed {
		t.Fatalf("state should be unchanged, got %s", state)
	}
	if !strings.HasPrefix(reason, "dns_timeout:") {
		t.Fatalf("the check's own error is the more specific reason and must stay first; got %q", reason)
	}
	if !strings.Contains(reason, "azure") {
		t.Fatalf("the provider gap must still be recorded; got %q", reason)
	}
}

// TestANotCheckedStateIsNotOverwritten keeps the four states distinct. A check
// that never ran says nothing about the asset, and relabelling it check_failed
// would claim it ran and could not tell.
func TestANotCheckedStateIsNotOverwritten(t *testing.T) {
	state, _ := attributionGap(store.CoverageNotChecked, "excluded by egress policy", netObservation([]string{"azure"}, nil))

	if state != store.CoverageNotChecked {
		t.Fatalf("a check that never ran must not be relabelled as one that ran and failed, got %s", state)
	}
}

// TestNoObservationChangesNothing keeps every other check unaffected. Only the
// network check carries a network observation; the rest must pass through
// untouched rather than acquiring an empty annotation.
func TestNoObservationChangesNothing(t *testing.T) {
	state, reason := attributionGap(store.CoverageOK, "", nil)
	if state != store.CoverageOK || reason != "" {
		t.Fatalf("a check with no observation must pass through unchanged, got %s / %q", state, reason)
	}

	state, reason = attributionGap(store.CoverageOK, "", &vfinding.Observation{})
	if state != store.CoverageOK || reason != "" {
		t.Fatalf("an observation carrying no network data must pass through unchanged, got %s / %q", state, reason)
	}

	state, reason = attributionGap(store.CoverageOK, "", netObservation(nil, nil))
	if state != store.CoverageOK || reason != "" {
		t.Fatalf("full provider coverage must leave no annotation at all, got %s / %q", state, reason)
	}
}

// TestSourcesAreOrderedAndDeduplicated stops the reason string churning. The
// order vantage loads providers in is not part of its contract, and a reason
// that reorders between runs reads as a change when nothing changed — which
// is how a reader learns to stop reading it.
func TestSourcesAreOrderedAndDeduplicated(t *testing.T) {
	_, first := attributionGap(store.CoverageOK, "", netObservation([]string{"gcp", "azure", "aws"}, nil))
	_, second := attributionGap(store.CoverageOK, "", netObservation([]string{"aws", "gcp", "azure", "aws"}, nil))

	if first != second {
		t.Fatalf("the same gap must render identically regardless of load order:\n  %q\n  %q", first, second)
	}
	if strings.Count(first, "aws") != 1 {
		t.Fatalf("a repeated source must be named once; got %q", first)
	}
}

// TestACoverageGapReachesTheStoreRecord runs the whole adapter, because the
// degrade is only worth anything if it survives translation. A helper that
// returns the right answer into a variable nobody reads is the failure this
// test exists to catch.
func TestACoverageGapReachesTheStoreRecord(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{
		{Check: "spf", Target: "example.com", State: vfinding.StateOK},
		{
			Check:  "net",
			Target: "example.com",
			State:  vfinding.StateOK,
			Observation: &vfinding.Observation{Network: &vobs.Network{
				Domain:        "example.com",
				FailedSources: []string{"azure"},
			}},
		},
	}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	byCheck := make(map[string]store.AssessmentCoverage, len(got.Coverage))
	for _, c := range got.Coverage {
		byCheck[c.CheckID] = c
	}

	if byCheck["net"].State != store.CoverageCheckFailed {
		t.Fatalf("the net check's coverage record must reach the store as check_failed, got %s", byCheck["net"].State)
	}
	if byCheck["net"].Reason == "" {
		t.Fatal("a degraded coverage record without a reason cannot be acted on and is indistinguishable from an adapter bug")
	}
	if byCheck["spf"].State != store.CoverageOK {
		t.Fatalf("an unrelated check must be untouched, got %s", byCheck["spf"].State)
	}
	if got.Outcome != OutcomePartial {
		t.Fatalf("an assessment carrying a coverage gap is partial, not completed; got %s", got.Outcome)
	}
}

// netCheckWithHosts builds a net check result carrying resolved hosts.
func netCheckWithHosts(hosts []vobs.NetworkHost, prov []netattr.SourceProvenance) vfinding.CheckResult {
	return vfinding.CheckResult{
		Check:  "net",
		Target: "example.com",
		State:  vfinding.StateOK,
		Observation: &vfinding.Observation{Network: &vobs.Network{
			Domain:     "example.com",
			Hosts:      hosts,
			Provenance: prov,
		}},
	}
}

// TestAttributionIsFlattenedPerAddress keeps a name's addresses apart. A name
// spread across two providers or two jurisdictions is a fact to show, and
// collapsing it to one winner would answer a data-residency question with
// whichever address happened to come back first.
func TestAttributionIsFlattenedPerAddress(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{netCheckWithHosts([]vobs.NetworkHost{{
		Host: "www.example.com",
		Role: "host",
		Attributions: []netattr.Attribution{
			{Address: netip.MustParseAddr("203.0.113.10"), Provider: "aws", Region: "eu-west-2", Jurisdiction: "GB", Source: "https://aws.invalid/ranges"},
			{Address: netip.MustParseAddr("198.51.100.7"), Provider: "gcp", Region: "europe-west2", Jurisdiction: "GB"},
		},
	}}, nil)}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if len(got.Attribution) != 2 {
		t.Fatalf("each address must become its own row; got %d", len(got.Attribution))
	}
	first := got.Attribution[0]
	if first.AssetID != "asset-1" || first.Host != "www.example.com" || first.Role != "host" {
		t.Fatalf("row is not attached to the asset and name that produced it: %+v", first)
	}
	if first.Address != "203.0.113.10" || first.Provider != "aws" || first.Region != "eu-west-2" || first.Jurisdiction != "GB" {
		t.Fatalf("attribution fields did not survive translation: %+v", first)
	}
	if first.Source == "" {
		t.Fatal("the citation must travel with the attribution, so a reader can check it rather than take it on trust")
	}
}

// TestAnUnmatchedAddressIsKept stops the silent drop. An address that matched
// no published range must still appear, because discarding it makes
// 'unattributed' indistinguishable from 'never looked up' — the exact silence
// this path exists to remove.
func TestAnUnmatchedAddressIsKept(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{netCheckWithHosts([]vobs.NetworkHost{{
		Host:         "self.example.com",
		Role:         "apex",
		Attributions: []netattr.Attribution{{Address: netip.MustParseAddr("192.0.2.5")}},
	}}, nil)}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if len(got.Attribution) != 1 {
		t.Fatalf("an unmatched address must be recorded, not dropped; got %d rows", len(got.Attribution))
	}
	if got.Attribution[0].Attributed() {
		t.Fatal("an address matching no published range must report itself as unattributed")
	}
}

// TestProvenanceTravelsWithTheAttribution pins the basis. Without it, a later
// run cannot tell a host that moved from range data that was merely
// refreshed, and every refresh reads as a change to the estate.
func TestProvenanceTravelsWithTheAttribution(t *testing.T) {
	fetched := time.Date(2026, 9, 18, 6, 0, 0, 0, time.UTC)
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{netCheckWithHosts(
		[]vobs.NetworkHost{{Host: "www.example.com", Attributions: []netattr.Attribution{{Address: netip.MustParseAddr("203.0.113.10"), Provider: "aws"}}}},
		[]netattr.SourceProvenance{{Provider: "aws", URL: "https://aws.invalid/ranges", Fetched: fetched}},
	)}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if len(got.AttributionProvenance) != 1 {
		t.Fatalf("provenance rows = %d, want 1", len(got.AttributionProvenance))
	}
	p := got.AttributionProvenance[0]
	if p.URL != "https://aws.invalid/ranges" {
		t.Fatalf("the endpoint must be recorded, since a fallback is a different basis and not the same data by another route: %+v", p)
	}
	if !p.FetchedAt.Equal(fetched) {
		t.Fatalf("the fetch time must be carried unrounded; got %s want %s", p.FetchedAt, fetched)
	}
}

// TestAttributionIsNotAttemptedWithoutAnObservation is what stops a run that
// never attributed from erasing the last run that did. An empty set is a
// claim — 'we looked and matched nothing' — and only a check that actually
// ran is entitled to make it.
func TestAttributionIsNotAttemptedWithoutAnObservation(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{
		{Check: "spf", Target: "example.com", State: vfinding.StateOK},
		{Check: "net", Target: "example.com", State: vfinding.StateNotChecked},
	}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if got.AttributionAttempted {
		t.Fatal("a net check that never ran produced no observation, so nothing may be asserted about the asset's hosting — writing an empty set would erase the previous run's attribution")
	}
	if len(got.Attribution) != 0 {
		t.Fatalf("no observation means no rows; got %d", len(got.Attribution))
	}
}

// TestAnEmptyObservationStillCountsAsAttempted is the other side of the same
// distinction: the check ran, resolved nothing, and that absence is a real
// result which must replace whatever was recorded before.
func TestAnEmptyObservationStillCountsAsAttempted(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{netCheckWithHosts(nil, nil)}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if !got.AttributionAttempted {
		t.Fatal("a check that ran and attributed nothing has observed something, and the store must be updated to say so")
	}
	if len(got.Attribution) != 0 {
		t.Fatalf("rows = %d, want none", len(got.Attribution))
	}
}

// TestARefetchOfTheSameEndpointIsTheSameBasis is what stops every provider
// republication reading as an estate change. Vantage owns the judgement; this
// asserts Trawl asks it the right question with its own types.
func TestARefetchOfTheSameEndpointIsTheSameBasis(t *testing.T) {
	monday := []store.AttributionProvenance{{Provider: "aws", URL: "https://aws.invalid/ranges", FetchedAt: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}}
	tuesday := []store.AttributionProvenance{{Provider: "aws", URL: "https://aws.invalid/ranges", FetchedAt: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)}}

	if !SameAttributionBasis(monday, tuesday) {
		t.Fatal("a re-fetch of the same endpoint is the same basis; treating it as a change would make every refresh look like a regression")
	}
}

// TestAFallbackEndpointIsADifferentBasis keeps the other direction honest. A
// fallback is different data, not the same data by another route, so an
// attribution difference across it cannot be attributed to the estate.
func TestAFallbackEndpointIsADifferentBasis(t *testing.T) {
	preferred := []store.AttributionProvenance{{Provider: "aws", URL: "https://aws.invalid/ranges"}}
	fallback := []store.AttributionProvenance{{Provider: "aws", URL: "https://mirror.invalid/ranges"}}

	if SameAttributionBasis(preferred, fallback) {
		t.Fatal("data from a different endpoint is a different basis")
	}
}

// TestAFirstRunHasNoBasisToCompareAgainst stops a baseline presenting as a
// regression. There is nothing to have moved from.
func TestAFirstRunHasNoBasisToCompareAgainst(t *testing.T) {
	current := []store.AttributionProvenance{{Provider: "aws", URL: "https://aws.invalid/ranges"}}

	if SameAttributionBasis(nil, current) {
		t.Fatal("no previous basis is not the same basis; the first attribution must be recorded as a baseline rather than compared against nothing")
	}
}
