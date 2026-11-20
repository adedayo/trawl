package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Finding sources. A finding's source names the table that holds its current
// state; its history lives in the event log whatever the source.
const (
	SourceSignal    = "signal"    // vantage observation
	SourceFinding   = "finding"   // CVE, service-exposure and template findings
	SourceSecret    = "secret"    // repository secret
	SourceCandidate = "candidate" // unconfirmed software-matched CVE
)

// FindingKey is the stable, URL-safe address of a finding.
//
// It is derived from what the finding asserts, which for each source is already
// the identity of its row: a signal observation is one (asset, signal), a
// finding or secret carries a content-derived identifier. Encoding rather than
// hashing keeps the key reversible, so resolving it needs no index.
func FindingKey(source, rowID string) string {
	return source + "." + base64.RawURLEncoding.EncodeToString([]byte(rowID))
}

// ParseFindingKey reverses FindingKey.
func ParseFindingKey(key string) (source, rowID string, err error) {
	source, enc, ok := strings.Cut(key, ".")
	if !ok || enc == "" {
		return "", "", fmt.Errorf("store: %q is not a finding key", key)
	}
	switch source {
	case SourceSignal, SourceFinding, SourceSecret, SourceCandidate:
	default:
		return "", "", fmt.Errorf("store: %q names no known finding source", key)
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", "", fmt.Errorf("store: %q is not a finding key: %w", key, err)
	}
	return source, string(raw), nil
}

// CandidateFindingID is the stable identity of an unconfirmed software-matched
// candidate vulnerability: the same asset, mapped product, version and CVE.
// It deliberately ignores raw observed string formatting, observation times,
// lookup snapshot IDs, and severity/priority updates so enrichment changes
// append events to the same candidate rather than minting a new one.
func CandidateFindingID(assetID, vendor, product, version, cve string) string {
	return "candidate:" + shortDigest(assetID, vendor, product, version, strings.ToUpper(strings.TrimSpace(cve)))
}

// ScanFindingID is the identity of a template (nuclei) finding: the same
// template matching the same place on the same asset is the same finding on
// every scan. It ignores the matched response body and the time, which change
// between runs without the finding changing.
func ScanFindingID(assetID, templateID, matchedAt string) string {
	return "scan:" + shortDigest(assetID, templateID, matchedAt)
}

// SecretFindingID is the identity of an exposed secret: the same secret in the
// same file of the same repository under the same rule. The line number is not
// part of it, because history rewrites and unrelated edits move a secret
// without making it a different secret.
func SecretFindingID(repoURL, ruleID, filePath, redactedRef string) string {
	return "secret:" + shortDigest(repoURL, ruleID, filePath, redactedRef)
}

func shortDigest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}

// SecretSeverity rates a secret. A secret verified as live is critical; one
// that is merely present is high, because an unverified key is still a
// credential somebody wrote down in public.
func SecretSeverity(verified bool) FindingSeverity {
	if verified {
		return SeverityCritical
	}
	return SeverityHigh
}

// EventType names a transition in a finding's life.
type EventType string

const (
	// EventBaseline is the single event given to a finding recorded before
	// history existed. It asserts only that the finding was known by its
	// first-seen time; it implies nothing earlier.
	EventBaseline        EventType = "baseline"
	EventFirstSeen       EventType = "first_seen"
	EventChanged         EventType = "changed"
	EventPriorityChanged EventType = "priority_changed"
	EventResolved        EventType = "resolved"
	EventReopened        EventType = "reopened"
)

// Causes of a recorded change. They exist so that "the estate changed" can be
// told apart from "the scanner or a feed changed underneath it".
const (
	CauseEstate   = "estate"
	CauseLibrary  = "library"
	CauseFeed     = "feed"
	CauseBaseline = "baseline"
)

// FindingEvent is one append-only entry in a finding's history.
type FindingEvent struct {
	ID      string    `json:"id"`
	Key     string    `json:"key"`
	Source  string    `json:"source"`
	RowID   string    `json:"rowId"`
	AssetID string    `json:"assetId"`
	Type    EventType `json:"type"`
	At      time.Time `json:"at"`
	RunID   string    `json:"runId,omitempty"`
	Cause   string    `json:"cause"`

	PrevSeverity FindingSeverity `json:"prevSeverity,omitempty"`
	Severity     FindingSeverity `json:"severity,omitempty"`
	PrevPriority string          `json:"prevPriority,omitempty"`
	Priority     string          `json:"priority,omitempty"`

	LibraryVersion  string `json:"libraryVersion,omitempty"`
	RegistryVersion string `json:"registryVersion,omitempty"`
	// SnapshotID cites the feed snapshot behind a feed-caused change.
	SnapshotID string `json:"snapshotId,omitempty"`
	Note       string `json:"note,omitempty"`
}

// EventQuery selects events. Zero fields do not filter. Results are in the
// order they were recorded.
type EventQuery struct {
	Key   string
	RunID string
	// After and Until bound the event time: After < at <= Until.
	After time.Time
	Until time.Time
	// Since bounds the event time: at >= Since.
	Since time.Time
}

// RunRef identifies the run a write belongs to. It travels in the context so
// that the single write path (SaveFinding, SaveSignalObservation, ...) can
// stamp the event it records without every caller threading it through.
type RunRef struct {
	ID   string
	Kind string
	// Cause overrides the default "estate" for changes made under this run. A
	// feed refresh sets CauseFeed and the snapshot it applied.
	Cause      string
	SnapshotID string
}

type runRefKey struct{}

// WithRun returns a context carrying the run the writes beneath it belong to.
func WithRun(ctx context.Context, ref RunRef) context.Context {
	return context.WithValue(ctx, runRefKey{}, ref)
}

// RunFrom returns the run carried by ctx, or the zero RunRef.
func RunFrom(ctx context.Context) RunRef {
	ref, _ := ctx.Value(runRefKey{}).(RunRef)
	return ref
}

// Run kinds.
const (
	RunAssessment       = "assessment"
	RunServiceProbe     = "service-probe"
	RunIngestScan       = "ingest-scan"
	RunSecretScan       = "secret-scan"
	RunFeedRefresh      = "feed-refresh"
	RunCandidateRefresh = "candidate-refresh"
)

// ScanRun is one entry in the run ledger. The ledger is append-only: the
// latest-only assessment_runs record answers "how does this domain stand now",
// this answers "what has been run".
type ScanRun struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	AssetID         string          `json:"assetId,omitempty"`
	Profile         string          `json:"profile,omitempty"`
	Outcome         string          `json:"outcome"`
	Error           string          `json:"error,omitempty"`
	LibraryVersion  string          `json:"libraryVersion,omitempty"`
	RegistryVersion string          `json:"registryVersion,omitempty"`
	StartedAt       time.Time       `json:"startedAt"`
	FinishedAt      time.Time       `json:"finishedAt"`
	Coverage        CoverageSummary `json:"coverage"`
}

// ScanSummary is the estate at the moment a run finished: open findings by
// severity, and the transitions the run itself caused. It is derived data,
// which is why it may be pruned while the events it was derived from may not.
type ScanSummary struct {
	RunID    string    `json:"runId"`
	At       time.Time `json:"at"`
	Critical int       `json:"critical"`
	High     int       `json:"high"`
	Medium   int       `json:"medium"`
	Low      int       `json:"low"`
	Info     int       `json:"info"`
	Open     int       `json:"open"`
	Resolved int       `json:"resolved"`
	// New and ResolvedInRun count the transitions this run caused.
	New           int `json:"new"`
	ResolvedInRun int `json:"resolvedInRun"`
}

// HistorySummaryRetentionKey is the setting naming how many days of run
// summaries to keep. Absent, empty or zero keeps them indefinitely. Events are
// never pruned.
const HistorySummaryRetentionKey = "history_summary_retention_days"

// HistoryStore records and reads the append-only history.
//
// There is deliberately no operation that updates or deletes an individual
// event. Events leave the store only when the asset they concern is removed or
// the operator erases discovered data, both of which discard the record
// entirely by the operator's decision. TestHistoryStoreOffersNoMutation holds
// that line.
type HistoryStore interface {
	// RecordScanRun appends a run and, in the same transaction, the summary of
	// the estate as it stands once the run's writes have landed.
	RecordScanRun(ctx context.Context, run *ScanRun) error
	// GetScanRuns lists runs, newest first. A limit of zero means all.
	GetScanRuns(ctx context.Context, limit int) ([]ScanRun, error)
	// GetScanSummaries lists summaries from since, oldest first.
	GetScanSummaries(ctx context.Context, since time.Time) ([]ScanSummary, error)
	// GetFindingEvents reads events in the order they were recorded.
	GetFindingEvents(ctx context.Context, q EventQuery) ([]FindingEvent, error)
	// PruneScanSummaries drops summaries older than before. It never touches
	// events or runs.
	PruneScanSummaries(ctx context.Context, before time.Time) (int, error)

	// ResolveSignalObservations marks open observations of the given checks on
	// an asset as resolved, except those whose signal is in raised. The caller
	// passes only checks that reached a conclusion: resolving on silence from a
	// check that did not run would turn an outage into a fix.
	ResolveSignalObservations(ctx context.Context, assetID string, concludedChecks []string, raised map[string]bool) (int, error)
	// ResolveSecretFindings marks open secrets of a repository as resolved,
	// except those in seen. Call it only after a scan that completed.
	ResolveSecretFindings(ctx context.Context, repoURL string, seen map[string]bool) (int, error)
}
