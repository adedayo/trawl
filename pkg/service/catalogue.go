package service

import (
	"sync"

	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// The finding catalogue is vantage's version-controlled explanation of every
// identifier it can raise: what the observation means, what to do about it, and
// which standard says so. Trawl's own signal registry answers a different
// question — which weakness class and attack scenario a signal belongs to — so
// the two are complementary rather than redundant, and neither substitutes for
// the other.
//
// The catalogue is read here rather than copied into the store because it is
// static library text. Persisting it would freeze prose that a vantage upgrade
// is entitled to correct, and would leave stored rows disagreeing with the
// library actually running.

var (
	catalogueOnce  sync.Once
	catalogueIndex map[string]vfinding.Entry
)

// catalogueEntry returns vantage's definition of a finding identifier.
//
// A miss is ordinary, not exceptional: Trawl may be running against a library
// older or newer than the one a stored observation came from, so an identifier
// with no definition simply carries no explanatory text. The caller shows what
// it does have rather than failing.
func catalogueEntry(signalID string) (vfinding.Entry, bool) {
	catalogueOnce.Do(func() {
		entries := vfinding.Catalogue()
		catalogueIndex = make(map[string]vfinding.Entry, len(entries))
		for _, e := range entries {
			catalogueIndex[e.ID] = e
		}
	})
	e, ok := catalogueIndex[signalID]
	return e, ok
}
