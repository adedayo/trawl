package vantage

import (
	"context"
	"strings"
	"testing"

	vfinding "github.com/adedayo/vantage/pkg/finding"
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
