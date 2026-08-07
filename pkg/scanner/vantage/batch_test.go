package vantage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	vaudit "github.com/adedayo/vantage/pkg/audit"
	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// scriptedAssessor stands in for vantage, recording how many assessments were
// in flight at once so the batch's restraint can be measured rather than
// assumed.
type scriptedAssessor struct {
	mu       sync.Mutex
	inFlight int
	peak     int
	calls    atomic.Int64

	respond func(ctx context.Context, target string) (*vfinding.Result, error)
}

func (s *scriptedAssessor) Catalogue(context.Context) (vaudit.Capabilities, error) {
	return vaudit.Capabilities{}, nil
}

func (s *scriptedAssessor) Assess(ctx context.Context, req vaudit.Request) (*vfinding.Result, error) {
	s.calls.Add(1)

	s.mu.Lock()
	s.inFlight++
	if s.inFlight > s.peak {
		s.peak = s.inFlight
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.inFlight--
		s.mu.Unlock()
	}()

	target := ""
	if len(req.Targets) > 0 {
		target = req.Targets[0]
	}
	if s.respond != nil {
		return s.respond(ctx, target)
	}
	return vfinding.NewResult("vantage", "1.0.0"), nil
}

var _ vaudit.Assessor = (*scriptedAssessor)(nil)

func (s *scriptedAssessor) peakConcurrency() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peak
}

func batchOf(targets ...string) BatchRequest {
	reqs := make([]Request, 0, len(targets))
	for _, t := range targets {
		reqs = append(reqs, Request{AssetID: t, Domain: t})
	}
	return BatchRequest{Requests: reqs}
}

func newBatchAdapter(t *testing.T, s *scriptedAssessor) *Adapter {
	t.Helper()
	a, err := New(s)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

// TestBatchPreservesRequestOrder matters because reports are compared between
// runs. If ordering tracked completion time, every scan would produce a
// different diff and genuine change would be lost in the noise.
func TestBatchPreservesRequestOrder(t *testing.T) {
	// Deliberately make later targets finish first.
	delays := map[string]time.Duration{
		"a.example": 30 * time.Millisecond,
		"b.example": 20 * time.Millisecond,
		"c.example": 10 * time.Millisecond,
		"d.example": 0,
	}
	assessor := &scriptedAssessor{
		respond: func(_ context.Context, target string) (*vfinding.Result, error) {
			time.Sleep(delays[target])
			return vfinding.NewResult("vantage", "1.0.0"), nil
		},
	}

	results := newBatchAdapter(t, assessor).AssessBatch(context.Background(),
		batchOf("a.example", "b.example", "c.example", "d.example"))

	want := []string{"a.example", "b.example", "c.example", "d.example"}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}
	for i, w := range want {
		if results[i].Request.Domain != w {
			t.Errorf("position %d is %q, want %q", i, results[i].Request.Domain, w)
		}
	}
}

// TestBatchRespectsConcurrencyBound guards the courtesy contract: an assessment
// must not become something that looks, to the party being assessed, like an
// attack.
func TestBatchRespectsConcurrencyBound(t *testing.T) {
	assessor := &scriptedAssessor{
		respond: func(context.Context, string) (*vfinding.Result, error) {
			time.Sleep(20 * time.Millisecond)
			return vfinding.NewResult("vantage", "1.0.0"), nil
		},
	}

	batch := batchOf("a.example", "b.example", "c.example", "d.example",
		"e.example", "f.example", "g.example", "h.example")
	batch.Concurrency = 2

	newBatchAdapter(t, assessor).AssessBatch(context.Background(), batch)

	if peak := assessor.peakConcurrency(); peak > 2 {
		t.Errorf("peak concurrency was %d, want at most 2", peak)
	}
}

// TestBatchDefaultsToBoundedConcurrency ensures an unset bound is never
// unbounded. A caller who forgets to configure this should get restraint, not
// a flood.
func TestBatchDefaultsToBoundedConcurrency(t *testing.T) {
	assessor := &scriptedAssessor{
		respond: func(context.Context, string) (*vfinding.Result, error) {
			time.Sleep(20 * time.Millisecond)
			return vfinding.NewResult("vantage", "1.0.0"), nil
		},
	}

	targets := make([]string, 0, 20)
	for i := range 20 {
		targets = append(targets, string(rune('a'+i))+".example")
	}

	newBatchAdapter(t, assessor).AssessBatch(context.Background(), batchOf(targets...))

	if peak := assessor.peakConcurrency(); peak > DefaultConcurrency {
		t.Errorf("peak concurrency was %d, want at most %d", peak, DefaultConcurrency)
	}
}

// TestBatchFailureDoesNotAbortRemainder: one unreachable domain must not deny
// the operator results for the other ninety-nine.
func TestBatchFailureDoesNotAbortRemainder(t *testing.T) {
	boom := errors.New("resolver unreachable")
	assessor := &scriptedAssessor{
		respond: func(_ context.Context, target string) (*vfinding.Result, error) {
			if target == "b.example" {
				return nil, boom
			}
			return vfinding.NewResult("vantage", "1.0.0"), nil
		},
	}

	results := newBatchAdapter(t, assessor).AssessBatch(context.Background(),
		batchOf("a.example", "b.example", "c.example"))

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if results[1].Err == nil {
		t.Error("expected the failing target to carry an error")
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Error("a single failure must not contaminate its neighbours")
	}
	if assessor.calls.Load() != 3 {
		t.Errorf("assessor called %d times, want 3", assessor.calls.Load())
	}
}

// TestCancelledBatchRecordsCancellation: an abandoned assessment must be
// distinguishable from one that ran and found nothing. Conflating the two would
// let an interrupted scan read as a clean bill of health.
func TestCancelledBatchRecordsCancellation(t *testing.T) {
	release := make(chan struct{})
	assessor := &scriptedAssessor{
		respond: func(ctx context.Context, _ string) (*vfinding.Result, error) {
			<-release
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return vfinding.NewResult("vantage", "1.0.0"), nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	batch := batchOf("a.example", "b.example", "c.example", "d.example",
		"e.example", "f.example")
	batch.Concurrency = 1

	adapter := newBatchAdapter(t, assessor)
	done := make(chan []BatchResult, 1)
	go func() { done <- adapter.AssessBatch(ctx, batch) }()

	// Let the first assessment begin, then withdraw consent.
	time.Sleep(20 * time.Millisecond)
	cancel()
	close(release)

	results := <-done

	if len(results) != 6 {
		t.Fatalf("got %d results, want 6 - cancellation must not drop entries", len(results))
	}
	for i, r := range results {
		if r.Result.Outcome == "" {
			t.Errorf("entry %d carries no outcome; an abandoned assessment must still be accounted for", i)
		}
	}
}

// TestCoverageOfCountsEveryOutcome: an aggregate that omits refusals would
// report "no exposure found" over targets that were never examined.
func TestCoverageOfCountsEveryOutcome(t *testing.T) {
	results := []BatchResult{
		{Result: Result{Outcome: OutcomeCompleted}},
		{Result: Result{Outcome: OutcomeCompleted}},
		{Result: Result{Outcome: OutcomeRefused}},
		{Result: Result{Outcome: OutcomeFailed}},
		{Result: Result{Outcome: OutcomeCancelled}},
	}

	counts := CoverageOf(results)

	var total int
	for _, n := range counts {
		total += n
	}
	if total != len(results) {
		t.Fatalf("coverage accounts for %d of %d results", total, len(results))
	}
	if counts[OutcomeCompleted] != 2 {
		t.Errorf("completed count is %d, want 2", counts[OutcomeCompleted])
	}
	if counts[OutcomeRefused] != 1 {
		t.Errorf("refused count is %d, want 1", counts[OutcomeRefused])
	}
}

// TestEmptyBatchIsHarmless: a bounded pool sized from an empty slice is a
// classic deadlock, so it is worth pinning.
func TestEmptyBatchIsHarmless(t *testing.T) {
	results := newBatchAdapter(t, &scriptedAssessor{}).
		AssessBatch(context.Background(), BatchRequest{})
	if len(results) != 0 {
		t.Errorf("got %d results for an empty batch, want 0", len(results))
	}
}
