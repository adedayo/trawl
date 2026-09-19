package vantage

import (
	"fmt"
	"sort"
	"strings"

	vfinding "github.com/adedayo/vantage/pkg/finding"

	"github.com/adedayo/trawl/pkg/store"
)

// attributionGap degrades a check's coverage to account for provider data
// that could not be loaded.
//
// The network check attributes an address by looking it up in ranges
// published by the providers themselves. When a publication cannot be
// fetched, every address that provider announces reads as unattributed — and
// "we could not look" is then indistinguishable from "there is nothing there".
// Left alone, an estate sitting entirely in a provider whose ranges failed to
// load presents as an estate using no third-party hosting at all: a clean
// result manufactured from a gap in evidence.
//
// So a conclusive state with a failed source is not conclusive. It becomes
// check_failed, which is Trawl's way of saying the check ran and could not
// tell, and the reason names the sources so the gap can be acted on.
//
// A stale source is different and is deliberately not degraded. The data
// loaded, from a cache entry older than its lifetime, because the endpoint
// was unreachable; a prefix that moved last week is almost always still
// announced by the same operator. The attribution remains usable, so the
// state stands and the reason records the basis was not refreshed — enough
// for a reader comparing two runs, without inventing a coverage gap that
// would suppress a real finding.
func attributionGap(state store.CoverageState, reason string, obs *vfinding.Observation) (store.CoverageState, string) {
	if obs == nil || obs.Network == nil {
		return state, reason
	}

	failed := normaliseSources(obs.Network.FailedSources)
	stale := normaliseSources(obs.Network.StaleSources)
	if len(failed) == 0 && len(stale) == 0 {
		return state, reason
	}

	notes := make([]string, 0, 2)
	if len(failed) > 0 {
		notes = append(notes, fmt.Sprintf(
			"provider ranges unavailable for %s: addresses they announce cannot be attributed and must not be read as unattributed",
			strings.Join(failed, ", ")))
	}
	if len(stale) > 0 {
		notes = append(notes, fmt.Sprintf(
			"provider ranges served stale for %s: attribution is usable but the basis was not refreshed",
			strings.Join(stale, ", ")))
	}

	// Only a state that currently claims a conclusion can be degraded. A
	// check that already failed or was never run says nothing about the
	// asset, and overwriting its state would discard the more specific reason
	// it already carries.
	if len(failed) > 0 && state.Assessed() {
		state = store.CoverageCheckFailed
	}

	return state, joinReasons(reason, notes...)
}

// normaliseSources sorts and de-duplicates source names.
//
// The order vantage loads providers in is not part of its contract, and a
// reason string that reorders between runs reads as a change when nothing
// changed — which is how a reader learns to stop reading it.
func normaliseSources(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// joinReasons appends notes to an existing reason without losing it.
//
// The check's own error is the more specific explanation and stays first. A
// coverage record carrying only the later note would tell a reader about the
// provider data and not about the failure that actually stopped the check.
func joinReasons(reason string, notes ...string) string {
	parts := make([]string, 0, len(notes)+1)
	if strings.TrimSpace(reason) != "" {
		parts = append(parts, reason)
	}
	for _, n := range notes {
		if strings.TrimSpace(n) != "" {
			parts = append(parts, n)
		}
	}
	return strings.Join(parts, "; ")
}
