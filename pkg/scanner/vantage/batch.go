package vantage

import (
	"context"
	"sort"
	"sync"

	vantage "github.com/adedayo/vantage/pkg"
	vaudit "github.com/adedayo/vantage/pkg/audit"
)

// DefaultConcurrency bounds how many assets are assessed at once.
//
// It is deliberately modest. Each assessment issues DNS queries against
// infrastructure belonging to somebody else, and a scan that looks like a
// flood is both discourteous and likely to be rate-limited into producing
// wrong answers — a false negative caused by the tool's own impatience.
const DefaultConcurrency = 4

// BatchRequest is an assessment of many assets under one authorisation.
//
// The scope belongs to the batch rather than to each request because that is
// what it means: an operator authorises a portfolio, and every assessment in
// the batch is bounded by that same authorisation.
type BatchRequest struct {
	// Requests are the individual assessments.
	Requests []Request
	// Concurrency bounds how many run at once. Zero uses DefaultConcurrency.
	Concurrency int
}

// BatchResult pairs a request with its outcome, preserving input order.
type BatchResult struct {
	Request Request
	Result  Result
	Err     error
}

// AssessBatch runs many assessments through a bounded worker pool.
//
// Results are returned in request order regardless of completion order, so
// that output is deterministic and diffable between runs. A failure never
// aborts the batch: one unreachable domain must not deny the operator the
// results for the other ninety-nine.
func (a *Adapter) AssessBatch(ctx context.Context, batch BatchRequest) []BatchResult {
	concurrency := batch.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	if concurrency > len(batch.Requests) {
		concurrency = len(batch.Requests)
	}

	out := make([]BatchResult, len(batch.Requests))
	sem := make(chan struct{}, max(concurrency, 1))
	var wg sync.WaitGroup

	for i, req := range batch.Requests {
		wg.Add(1)
		go func(i int, req Request) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				// Cancellation is recorded rather than dropped. A caller must
				// be able to tell an assessment that was abandoned from one
				// that ran and found nothing.
				out[i] = BatchResult{
					Request: req,
					Result:  Result{Outcome: OutcomeCancelled, Err: ctx.Err()},
					Err:     ctx.Err(),
				}
				return
			}

			res, err := a.Assess(ctx, req)
			out[i] = BatchResult{Request: req, Result: res, Err: err}
		}(i, req)
	}
	wg.Wait()
	return out
}

// CoverageOf summarises how many assessments in a batch reached a conclusion.
//
// Every aggregate over a batch must be accompanied by one of these. Reporting
// "no exposure found across 100 domains" when 40 were refused is not a summary
// but a misstatement.
func CoverageOf(results []BatchResult) map[Outcome]int {
	out := map[Outcome]int{}
	for _, r := range results {
		out[r.Result.Outcome]++
	}
	return out
}

// OutcomesInOrder returns the distinct outcomes present, sorted, for stable
// reporting.
func OutcomesInOrder(counts map[Outcome]int) []Outcome {
	out := make([]Outcome, 0, len(counts))
	for o := range counts {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// NewScopedAssessor builds an assessor whose every egress is bounded by scope.
//
// This is the constructor an embedding caller should use. Both boundaries are
// wrapped together because guarding only one leaves the other open: a scope
// that refuses DNS but permits HTTP would still disclose the portfolio to a
// third party through the MTA-STS or takeover path.
func NewScopedAssessor(
	resolver vantage.Resolver,
	scope *Scope,
	opts ...vaudit.Option,
) (vaudit.Assessor, *ScopedResolver, *ScopedHTTPClient, error) {
	guardedDNS := NewScopedResolver(resolver, scope)
	guardedHTTP := NewScopedHTTPClient(nil, scope)

	opts = append([]vaudit.Option{vaudit.WithHTTPClient(guardedHTTP)}, opts...)
	a, err := vaudit.NewAssessor(guardedDNS, opts...)
	if err != nil {
		return nil, nil, nil, err
	}
	return a, guardedDNS, guardedHTTP, nil
}
