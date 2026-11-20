package service

import (
	"github.com/adedayo/trawl/pkg/narrative"
	"github.com/adedayo/trawl/pkg/store"
)

// This file is the wire contract for Change 025's read model. The Angular
// frontend mirrors these types in app/src/app/models/types.ts, so a change here
// is a change there.

// Finding sources. A finding's source says which table holds its current state.
const (
	SourceSignal    = "signal"    // vantage observation (email, DNS, PKI, ...)
	SourceFinding   = "finding"   // CVE, service-exposure and template findings
	SourceSecret    = "secret"    // repository secret
	SourceCandidate = "candidate" // unconfirmed software-matched CVE
)

// FindingFilter narrows the unified list. Empty fields do not filter.
type FindingFilter struct {
	// Status is "open", "resolved" or "reopened". "active" means open+reopened.
	Status   string `json:"status,omitempty"`
	Severity string `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	AssetID  string `json:"assetId,omitempty"`
	// ChangedSince is an RFC3339 time. Only findings with an event at or after
	// it are returned.
	ChangedSince string `json:"changedSince,omitempty"`
}

// FindingSummary is one row of the unified findings list.
type FindingSummary struct {
	// Key is the stable, URL-safe address of the finding.
	Key        string `json:"key"`
	Source     string `json:"source"`
	AssetID    string `json:"assetId"`
	AssetValue string `json:"assetValue"`
	Title      string `json:"title"`

	Severity store.FindingSeverity `json:"severity"`
	Priority string                `json:"priority"`
	// Status is open, resolved or reopened.
	Status string `json:"status"`

	// NotReassessed is true when the finding is open only because the check
	// that raises it could not conclude on the latest run. It must be rendered
	// distinctly from an open finding that was re-assessed and still present.
	NotReassessed       bool   `json:"notReassessed"`
	NotReassessedReason string `json:"notReassessedReason,omitempty"`

	// SignalID and SecretID name the source row for findings of those sources,
	// so another tab can find the finding that corresponds to a row it shows
	// without parsing the opaque key.
	SignalID string `json:"signalId,omitempty"`
	SecretID string `json:"secretId,omitempty"`

	Category string `json:"category"` // check family, control or finding category
	CVE      string `json:"cve,omitempty"`
	KEV      bool   `json:"kev"`

	// Unconfirmed is true for software-matched candidates: the software version
	// appears to fall in an affected range, but nothing has shown the flaw is
	// present or reachable.
	Unconfirmed    bool   `json:"unconfirmed,omitempty"`
	SeveritySource string `json:"severitySource,omitempty"`

	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
	ResolvedAt string `json:"resolvedAt,omitempty"`
	// AgeDays is whole days from first seen to now (or to resolution).
	AgeDays int `json:"ageDays"`
	// LastChange is the time of the most recent event; LastChangeType its type.
	LastChange     string `json:"lastChange"`
	LastChangeType string `json:"lastChangeType"`
}

// FindingEventView is one entry in a finding's timeline.
type FindingEventView struct {
	ID   string `json:"id"`
	Type string `json:"type"` // baseline, first_seen, changed, priority_changed, resolved, reopened
	At   string `json:"at"`

	RunID string `json:"runId,omitempty"`
	// Cause is "estate", "library", "feed" or "baseline". Library means the
	// scanner library changed between observations; it is not a change in the
	// estate.
	Cause string `json:"cause"`

	PrevSeverity string `json:"prevSeverity,omitempty"`
	Severity     string `json:"severity,omitempty"`
	PrevPriority string `json:"prevPriority,omitempty"`
	Priority     string `json:"priority,omitempty"`

	LibraryVersion  string `json:"libraryVersion,omitempty"`
	RegistryVersion string `json:"registryVersion,omitempty"`
	// SnapshotID cites the feed snapshot behind a feed-caused change.
	SnapshotID string `json:"snapshotId,omitempty"`
	Note       string `json:"note,omitempty"`
}

// FindingProvenance says what the finding's current state rests on.
type FindingProvenance struct {
	RunID           string               `json:"runId,omitempty"`
	LibraryVersion  string               `json:"libraryVersion,omitempty"`
	RegistryVersion string               `json:"registryVersion,omitempty"`
	FeedSnapshots   []store.FeedSnapshot `json:"feedSnapshots,omitempty"`
}

// FindingDetail is everything the drill-down view shows.
type FindingDetail struct {
	FindingSummary

	Description string `json:"description,omitempty"`
	// Detail is the occurrence-specific sentence; Evidence the measured value.
	Detail   string `json:"detail,omitempty"`
	Evidence string `json:"evidence,omitempty"`

	CheckID     string              `json:"checkId,omitempty"`
	CheckState  store.CoverageState `json:"checkState,omitempty"`
	CheckReason string              `json:"checkReason,omitempty"`

	// Candidate software-match evidence (populated when Source == "candidate").
	ObservedString   string `json:"observedString,omitempty"`
	MatchedVendor    string `json:"matchedVendor,omitempty"`
	MatchedProduct   string `json:"matchedProduct,omitempty"`
	MatchedVersion   string `json:"matchedVersion,omitempty"`
	LookupSnapshotID string `json:"lookupSnapshotId,omitempty"`
	LookupAt         string `json:"lookupAt,omitempty"`

	Explanation narrative.Explanation `json:"explanation"`
	// NarrativeVersion duplicates Explanation.Version so a viewer can say which
	// wording it was shown without unpacking the explanation.
	NarrativeVersion string `json:"narrativeVersion"`

	Timeline    []FindingEventView `json:"timeline"`
	Regressions []store.Regression `json:"regressions"`
	Provenance  FindingProvenance  `json:"provenance"`
}

// ScanRunView is one run in the ledger, with the coverage of what it attempted.
type ScanRunView struct {
	ID               string                `json:"id"`
	Kind             string                `json:"kind"` // assessment, service-probe, ingest-scan, secret-scan, feed-refresh
	AssetID          string                `json:"assetId,omitempty"`
	AssetValue       string                `json:"assetValue,omitempty"`
	Profile          string                `json:"profile,omitempty"`
	Outcome          string                `json:"outcome"`
	LibraryVersion   string                `json:"libraryVersion,omitempty"`
	RegistryVersion  string                `json:"registryVersion,omitempty"`
	StartedAt        string                `json:"startedAt"`
	FinishedAt       string                `json:"finishedAt"`
	Coverage         store.CoverageSummary `json:"coverage"`
	CoverageFraction float64               `json:"coverageFraction"`
}

// HistoryPoint is one run's summary: the estate's open findings by severity at
// the moment the run finished, the transitions the run caused, and the coverage
// of the run. Trend lines are drawn from these and only these; there is no
// interpolation between runs.
type HistoryPoint struct {
	Run      ScanRunView `json:"run"`
	At       string      `json:"at"`
	Critical int         `json:"critical"`
	High     int         `json:"high"`
	Medium   int         `json:"medium"`
	Low      int         `json:"low"`
	Info     int         `json:"info"`
	Open     int         `json:"open"`
	Resolved int         `json:"resolved"`
	// New and ResolvedInRun count transitions this run caused.
	New           int `json:"new"`
	ResolvedInRun int `json:"resolvedInRun"`
}

// FindingChange is one finding that moved between two points in time.
type FindingChange struct {
	Finding FindingSummary   `json:"finding"`
	Event   FindingEventView `json:"event"`
}

// ChangeSet is the difference between two runs (or between a run and now).
type ChangeSet struct {
	From *HistoryPoint `json:"from,omitempty"`
	To   *HistoryPoint `json:"to,omitempty"`

	New      []FindingChange `json:"new"`
	Resolved []FindingChange `json:"resolved"`
	Worsened []FindingChange `json:"worsened"`
	Improved []FindingChange `json:"improved"`

	// Note states, in words, anything that limits how the comparison should be
	// read, most importantly a drop in coverage between the two runs.
	Note string `json:"note,omitempty"`
}
