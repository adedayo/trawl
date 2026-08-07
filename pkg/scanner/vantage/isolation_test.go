package vantage

import (
	"context"
	"sync"
	"testing"
)

// names returns every name the resolver was asked about. Concurrency tests need
// the identities, not just the count, because the question is not "how much
// traffic left" but "whose traffic left under whose consent".
func (c *countingResolver) names() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.queried))
	copy(out, c.queried)
	return out
}

// TestConcurrentScopesDoNotLeakAcrossAssessments is the property that makes it
// defensible to assess two customers' portfolios in one process. Each has its
// own guard over a shared transport; neither guard may be bypassed by the
// other's traffic, and no shared state may carry a name from one into the
// other. If this fails, multi-tenant operation is unsafe regardless of how
// correct the individual checks are.
func TestConcurrentScopesDoNotLeakAcrossAssessments(t *testing.T) {
	shared := &countingResolver{}

	scopeA := NewScope([]string{"alpha.example"}, nil)
	scopeB := NewScope([]string{"beta.example"}, nil)

	guardA := NewScopedResolver(shared, scopeA)
	guardB := NewScopedResolver(shared, scopeB)

	const rounds = 25

	var wg sync.WaitGroup
	wg.Add(2)

	// Each side attempts both its own name and the other's. Only its own may
	// reach the transport.
	go func() {
		defer wg.Done()
		for range rounds {
			_, _, _ = guardA.ExchangeFrom(context.Background(), "alpha.example", 16)
			_, _, _ = guardA.ExchangeFrom(context.Background(), "beta.example", 16)
		}
	}()
	go func() {
		defer wg.Done()
		for range rounds {
			_, _, _ = guardB.ExchangeFrom(context.Background(), "beta.example", 16)
			_, _, _ = guardB.ExchangeFrom(context.Background(), "alpha.example", 16)
		}
	}()
	wg.Wait()

	counts := map[string]int{}
	for _, n := range shared.names() {
		counts[n]++
	}

	// Exactly the permitted traffic, and nothing else. An unexpected key here
	// would mean a name escaped under the wrong authority.
	if len(counts) != 2 {
		t.Fatalf("resolver saw %d distinct names (%v), want exactly 2", len(counts), counts)
	}
	if counts["alpha.example"] != rounds {
		t.Errorf("alpha.example reached the transport %d times, want %d", counts["alpha.example"], rounds)
	}
	if counts["beta.example"] != rounds {
		t.Errorf("beta.example reached the transport %d times, want %d", counts["beta.example"], rounds)
	}

	// Each guard must have refused precisely the other's name, and recorded it.
	// A silent refusal would be as bad as a leak: the operator could not tell
	// the assessment was incomplete.
	assertDenials(t, "A", guardA.Denials(), rounds, "beta.example")
	assertDenials(t, "B", guardB.Denials(), rounds, "alpha.example")
}

func assertDenials(t *testing.T, label string, denials []Denial, want int, name string) {
	t.Helper()
	if len(denials) != want {
		t.Errorf("scope %s recorded %d denials, want %d", label, len(denials), want)
	}
	for _, d := range denials {
		if d.Name != name {
			t.Errorf("scope %s denied %q, want only %q", label, d.Name, name)
			return
		}
	}
}
