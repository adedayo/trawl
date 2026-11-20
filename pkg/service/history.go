package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/store"
)

// newRunID names a run. The prefix says what it is wherever it turns up in a
// log or an event.
func newRunID() string { return "run-" + uuid.NewString() }

// assessedAssetID returns the asset an assessment result concerns.
func assessedAssetID(res vadapter.Result) string {
	if len(res.Coverage) > 0 {
		return res.Coverage[0].AssetID
	}
	if len(res.Observations) > 0 {
		return res.Observations[0].AssetID
	}
	return ""
}

// ConcludedChecks lists the checks of an assessment that reached a conclusion
// and may therefore resolve what they previously raised.
//
// Only an assessed state counts. not_checked and check_failed say nothing about
// the asset, and resolving on them would turn an outage into a fix, which is
// the failure the four-state model exists to prevent.
//
// DKIM is a further exception: selectors cannot be enumerated from DNS, so
// probing the usual ones and finding none establishes nothing about a domain
// that signs with a selector nobody guessed. Its absence is never conclusive
// unless the posture says it was.
func ConcludedChecks(res vadapter.Result) []string {
	dkimConclusive := res.EmailPosture == nil || res.EmailPosture.DKIM.Conclusive
	var out []string
	for _, c := range res.Coverage {
		if !c.State.Assessed() {
			continue
		}
		if c.CheckID == "dkim" && !dkimConclusive {
			continue
		}
		out = append(out, c.CheckID)
	}
	return out
}

// RaisedSignals is the set of signal identifiers the assessment raised.
func RaisedSignals(res vadapter.Result) map[string]bool {
	out := make(map[string]bool, len(res.Observations))
	for _, o := range res.Observations {
		out[o.SignalID] = true
	}
	return out
}

// recordScanRun appends the run to the ledger, with the coverage of what it
// attempted. kind defaults to an assessment.
func (svc *AssessmentService) recordScanRun(ctx context.Context, runID string, run store.AssessmentRun, coverage []store.AssessmentCoverage, kind ...string) error {
	k := store.RunAssessment
	if len(kind) > 0 && kind[0] != "" {
		k = kind[0]
	}
	if err := svc.store.RecordScanRun(ctx, &store.ScanRun{
		ID:             runID,
		Kind:           k,
		AssetID:        run.AssetID,
		Profile:        run.Profile,
		Outcome:        run.Outcome,
		Error:          run.Error,
		LibraryVersion: run.LibraryVersion,
		StartedAt:      run.StartedAt,
		FinishedAt:     run.FinishedAt,
		Coverage:       SummariseCoverage(coverage),
	}); err != nil {
		return fmt.Errorf("assessment: recording the run in the ledger: %w", err)
	}
	return nil
}

// SummariseCoverage counts a run's coverage records by state.
func SummariseCoverage(coverage []store.AssessmentCoverage) store.CoverageSummary {
	var sum store.CoverageSummary
	for _, c := range coverage {
		sum.Total++
		switch c.State {
		case store.CoverageOK:
			sum.OK++
			sum.AssessedOnly++
		case store.CoverageNotFound:
			sum.NotFound++
			sum.AssessedOnly++
		case store.CoverageCheckFailed:
			sum.CheckFailed++
		default:
			sum.NotChecked++
		}
	}
	return sum
}
