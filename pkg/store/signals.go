package store

import "time"

// CoverageState is the four-state assessment outcome adopted from vantage.
//
// The four states are kept distinct end to end. "No record published"
// (CoverageNotFound), "we did not look" (CoverageNotChecked) and "we looked
// and could not tell" (CoverageCheckFailed) are three different statements,
// and collapsing any of them into a boolean would let a reader conclude a
// control passed when in truth it was never assessed.
type CoverageState string

const (
	// CoverageOK means the check ran and the control is correctly configured.
	CoverageOK CoverageState = "ok"
	// CoverageNotFound means the check ran and the control is genuinely absent.
	CoverageNotFound CoverageState = "not_found"
	// CoverageNotChecked means the check was never run — excluded by policy,
	// out of scope, or not applicable. It is not evidence either way.
	CoverageNotChecked CoverageState = "not_checked"
	// CoverageCheckFailed means the check ran but could not reach a
	// conclusion — a resolver outage, a refused endpoint, a timeout.
	CoverageCheckFailed CoverageState = "check_failed"
)

// Valid reports whether the state is one of the four recognised values.
func (c CoverageState) Valid() bool {
	switch c {
	case CoverageOK, CoverageNotFound, CoverageNotChecked, CoverageCheckFailed:
		return true
	default:
		return false
	}
}

// Assessed reports whether an observation was actually made. Only CoverageOK
// and CoverageNotFound are assessments; the other two are absences of one.
//
// This is the only sanctioned reduction of the four states, and it never
// answers the question "did this control pass?" — see Passing.
func (c CoverageState) Assessed() bool {
	return c == CoverageOK || c == CoverageNotFound
}

// Passing reports whether the control was observed to be correctly configured.
// An unassessed control is never passing.
func (c CoverageState) Passing() bool {
	return c == CoverageOK
}

// SignalObservation is a single measured-state observation: an externally
// observable fact about the configuration of a defensive mechanism, carrying
// the identifier it was reported under and the state in which it was found.
type SignalObservation struct {
	ID       string `json:"id"`
	AssetID  string `json:"assetId"`
	SignalID string `json:"signalId"` // e.g. "SURF-SPF-005"
	CheckID  string `json:"checkId"`  // the vantage check that produced it

	State    CoverageState   `json:"state"`
	Severity FindingSeverity `json:"severity"`
	Evidence string          `json:"evidence"`

	// Mapped records whether SignalID resolved against the signal registry.
	// An unmapped identifier is retained rather than discarded, so that a
	// library upgrade adding a finding is visible instead of silently dropped.
	Mapped bool `json:"mapped"`

	// Provenance. Both versions are recorded on every observation so that a
	// change in interpretation can be told apart from a change in the domain.
	RegistryVersion string `json:"registryVersion"`
	LibraryVersion  string `json:"libraryVersion"`

	ObservedAt time.Time `json:"observedAt"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
}

// SignalRegistryEntry maps a finding identifier to what it bears on. The
// registry is versioned data, loaded from config/signals/vantage-<major>.json,
// and is consumed by Changes 007-009.
type SignalRegistryEntry struct {
	SignalID      string `json:"signalId"`
	Condition     string `json:"condition"`
	WeaknessClass string `json:"weaknessClass"`
	Scenario      string `json:"scenario"`
	Stage         string `json:"stage"`
	DedupGroup    string `json:"dedupGroup"`
	Control       string `json:"control"`

	// Direction states whether the signal raises or lowers the probability of
	// the scenario it bears on: "aggravating" or "mitigating".
	Direction string `json:"direction"`

	RegistryVersion string `json:"registryVersion"`
}

// AssessmentCoverage records, per asset and check, whether the check ran at
// all. It exists so that an aggregate can always be accompanied by a statement
// of how much of the intended assessment actually happened.
type AssessmentCoverage struct {
	ID      string `json:"id"`
	AssetID string `json:"assetId"`
	CheckID string `json:"checkId"`

	State CoverageState `json:"state"`

	// Reason names why a check is not_checked or check_failed — the excluding
	// policy, the out-of-scope reference, the unreachable endpoint. Naming the
	// reason is what makes a fail-closed exclusion actionable rather than
	// merely silent.
	Reason string `json:"reason,omitempty"`

	LibraryVersion string    `json:"libraryVersion"`
	AssessedAt     time.Time `json:"assessedAt"`
}

// AssessmentRun records how the last assessment of an asset ended, as distinct
// from what it found. Coverage says how much was established; this says
// whether the run that established it completed, was cut short, or was refused
// before anything was attempted.
//
// Without it a reader cannot distinguish a domain that was assessed cleanly
// from one that was never assessed at all: both present as an absence of
// adverse signals. Coverage alone does not close the gap, because a refused
// run writes no coverage either.
//
// It is stored per asset rather than appended per attempt. The question the
// read model asks is "how does this domain stand now", and a history nothing
// reads is a table that only grows.
type AssessmentRun struct {
	AssetID string `json:"assetId"`

	// Outcome mirrors the adapter's vocabulary: completed, partial, failed,
	// refused, cancelled. It is held as a string so that the store does not
	// depend on the scanner package.
	Outcome string `json:"outcome"`

	// Error names why, when the outcome is not completed.
	Error string `json:"error,omitempty"`

	// Profile records the breadth that was requested, so that a narrow run is
	// not later mistaken for a thorough one.
	Profile string `json:"profile,omitempty"`

	LibraryVersion string    `json:"libraryVersion,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	FinishedAt     time.Time `json:"finishedAt"`
}

// CoverageSummary aggregates assessment coverage for an asset or a scenario.
type CoverageSummary struct {
	Total        int `json:"total"`
	OK           int `json:"ok"`
	NotFound     int `json:"notFound"`
	NotChecked   int `json:"notChecked"`
	CheckFailed  int `json:"checkFailed"`
	AssessedOnly int `json:"assessedOnly"` // OK + NotFound
}

// Fraction is the proportion of intended checks that produced an assessment.
// It returns 0 when nothing was intended, which is honest: no assessment was
// made, so no coverage can be claimed.
func (c CoverageSummary) Fraction() float64 {
	if c.Total == 0 {
		return 0
	}
	return float64(c.AssessedOnly) / float64(c.Total)
}
