package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

// ErrInvalidCoverageState is returned when a state outside the four-state
// model is presented for persistence. The store refuses it rather than
// coercing it, because an unrecognised state silently stored as one of the
// four would be indistinguishable from a genuine assessment.
var ErrInvalidCoverageState = errors.New("invalid coverage state")

// SaveSignalObservation upserts an observation keyed on (asset, signal),
// preserving the original first_seen, and records in the same transaction the
// event the write amounts to.
//
// An observation that was resolved and is raised again is reopened. A change in
// severity is an event whose cause is the scanner library when the library
// version differs from the one that recorded the previous severity, and the
// estate otherwise, so an upgrade that re-rates a finding is not reported as
// the estate having changed.
func (s *SQLiteStore) SaveSignalObservation(ctx context.Context, obs *store.SignalObservation) error {
	if !obs.State.Valid() {
		return fmt.Errorf("%w: %q for signal %s", ErrInvalidCoverageState, obs.State, obs.SignalID)
	}

	now := s.now()
	if obs.ObservedAt.IsZero() {
		obs.ObservedAt = now
	}
	if obs.FirstSeen.IsZero() {
		obs.FirstSeen = now
	}
	if obs.LastSeen.IsZero() {
		obs.LastSeen = now
	}
	if obs.ID == "" {
		obs.ID = fmt.Sprintf("sig-%s-%s", obs.AssetID, obs.SignalID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin observation save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prevID, prevStatus, prevSeverity, prevLibrary string
	err = tx.QueryRowContext(ctx,
		`SELECT id, COALESCE(status,'open'), severity, COALESCE(library_version,'') FROM signal_observations WHERE asset_id = ? AND signal_id = ?`,
		obs.AssetID, obs.SignalID).Scan(&prevID, &prevStatus, &prevSeverity, &prevLibrary)
	exists := err == nil
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read observation %s: %w", obs.SignalID, err)
	}
	// The row keeps the identifier it was first written under, so its events
	// stay attached to it whatever the caller supplied this time.
	if exists {
		obs.ID = prevID
	}

	var evType store.EventType
	status := store.FindingOpen
	switch {
	case !exists:
		evType = store.EventFirstSeen
	case prevStatus == string(store.FindingResolved):
		evType = store.EventReopened
		status = store.FindingReopened
	case prevSeverity != string(obs.Severity):
		evType = store.EventChanged
		status = store.FindingStatus(prevStatus)
	default:
		status = store.FindingStatus(prevStatus)
	}
	obs.Status = status
	obs.ResolvedAt = nil
	assessed := now
	obs.LastAssessedAt = &assessed

	query := `
	INSERT INTO signal_observations (
		id, asset_id, signal_id, check_id, state, severity, evidence, detail, mapped,
		registry_version, library_version, observed_at, first_seen, last_seen,
		status, resolved_at, last_assessed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)
	ON CONFLICT(asset_id, signal_id) DO UPDATE SET
		check_id = excluded.check_id,
		state = excluded.state,
		severity = excluded.severity,
		evidence = excluded.evidence,
		detail = excluded.detail,
		mapped = excluded.mapped,
		registry_version = excluded.registry_version,
		library_version = excluded.library_version,
		observed_at = excluded.observed_at,
		last_seen = excluded.last_seen,
		status = excluded.status,
		resolved_at = NULL,
		last_assessed_at = excluded.last_assessed_at
	`
	if _, err := tx.ExecContext(ctx, query,
		obs.ID, obs.AssetID, obs.SignalID, obs.CheckID, string(obs.State), string(obs.Severity),
		obs.Evidence, obs.Detail, obs.Mapped, obs.RegistryVersion, obs.LibraryVersion,
		formatTS(obs.ObservedAt), formatTS(obs.FirstSeen), formatTS(obs.LastSeen),
		string(status), formatTS(assessed),
	); err != nil {
		return fmt.Errorf("failed to save signal observation %s: %w", obs.SignalID, err)
	}

	if evType != "" {
		e := s.newEvent(ctx, store.SourceSignal, obs.ID, obs.AssetID, evType)
		e.Severity = obs.Severity
		e.LibraryVersion, e.RegistryVersion = obs.LibraryVersion, obs.RegistryVersion
		if exists {
			e.PrevSeverity = store.FindingSeverity(prevSeverity)
			if evType == store.EventChanged && e.Cause == store.CauseEstate &&
				prevLibrary != "" && obs.LibraryVersion != "" && prevLibrary != obs.LibraryVersion {
				e.Cause = store.CauseLibrary
				e.Note = "Scanner library changed from " + prevLibrary + " to " + obs.LibraryVersion + "."
			}
		}
		if err := appendEvent(ctx, tx, &e); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit signal observation %s: %w", obs.SignalID, err)
	}
	return nil
}

// GetSignalObservations returns the live observations: those not resolved.
func (s *SQLiteStore) GetSignalObservations(ctx context.Context, assetID string) ([]store.SignalObservation, error) {
	return s.querySignalObservations(ctx, assetID, true)
}

// GetAllSignalObservations returns every observation, resolved ones included.
func (s *SQLiteStore) GetAllSignalObservations(ctx context.Context, assetID string) ([]store.SignalObservation, error) {
	return s.querySignalObservations(ctx, assetID, false)
}

func (s *SQLiteStore) querySignalObservations(ctx context.Context, assetID string, liveOnly bool) ([]store.SignalObservation, error) {
	query := `
	SELECT id, asset_id, signal_id, check_id, state, severity, evidence, detail, mapped,
	       registry_version, library_version, observed_at, first_seen, last_seen,
	       COALESCE(status,'open'), resolved_at, last_assessed_at
	FROM signal_observations`
	var where []string
	args := []any{}
	if assetID != "" {
		where = append(where, `asset_id = ?`)
		args = append(args, assetID)
	}
	if liveOnly {
		where = append(where, `COALESCE(status,'open') != 'resolved'`)
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY signal_id`

	rows, err := s.reader().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query signal observations: %w", err)
	}
	defer rows.Close()

	observations := []store.SignalObservation{}
	for rows.Next() {
		var o store.SignalObservation
		var state, severity, observedAt, firstSeen, lastSeen, status string
		// Nullable: rows written before the column existed carry NULL, which
		// is the same absence as a signal that names no particular item.
		var evidence, detail, resolvedAt, assessedAt sql.NullString
		if err := rows.Scan(&o.ID, &o.AssetID, &o.SignalID, &o.CheckID, &state, &severity, &evidence,
			&detail, &o.Mapped, &o.RegistryVersion, &o.LibraryVersion, &observedAt, &firstSeen, &lastSeen,
			&status, &resolvedAt, &assessedAt); err != nil {
			return nil, fmt.Errorf("failed to scan signal observation: %w", err)
		}
		o.State = store.CoverageState(state)
		o.Severity = store.FindingSeverity(severity)
		o.Status = store.FindingStatus(status)
		o.Evidence = evidence.String
		o.Detail = detail.String
		if o.ObservedAt, err = parseTS("observation observed_at", observedAt); err != nil {
			return nil, err
		}
		if o.FirstSeen, err = parseTS("observation first_seen", firstSeen); err != nil {
			return nil, err
		}
		if o.LastSeen, err = parseTS("observation last_seen", lastSeen); err != nil {
			return nil, err
		}
		if o.ResolvedAt, err = parseTSPtr("observation resolved_at", resolvedAt); err != nil {
			return nil, err
		}
		if o.LastAssessedAt, err = parseTSPtr("observation last_assessed_at", assessedAt); err != nil {
			return nil, err
		}
		observations = append(observations, o)
	}
	return observations, rows.Err()
}

// ReplaceSignalRegistry swaps the registry contents in a single transaction.
// Loading is all-or-nothing so that no observation can be mapped against a
// partially applied registry version.
func (s *SQLiteStore) ReplaceSignalRegistry(ctx context.Context, entries []store.SignalRegistryEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin registry transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM signal_registry`); err != nil {
		return fmt.Errorf("failed to clear signal registry: %w", err)
	}

	insert := `
	INSERT INTO signal_registry (
		signal_id, condition, weakness_class, scenario, stage, dedup_group,
		control, direction, registry_version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, e := range entries {
		if e.SignalID == "" {
			return errors.New("signal registry entry has empty signalId")
		}
		if _, err := tx.ExecContext(ctx, insert, e.SignalID, e.Condition, e.WeaknessClass,
			e.Scenario, e.Stage, e.DedupGroup, e.Control, e.Direction, e.RegistryVersion); err != nil {
			return fmt.Errorf("failed to insert registry entry %s: %w", e.SignalID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit signal registry: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSignalRegistry(ctx context.Context) ([]store.SignalRegistryEntry, error) {
	query := `
	SELECT signal_id, condition, weakness_class, scenario, stage, dedup_group,
	       control, direction, registry_version
	FROM signal_registry ORDER BY signal_id`
	rows, err := s.reader().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query signal registry: %w", err)
	}
	defer rows.Close()

	entries := []store.SignalRegistryEntry{}
	for rows.Next() {
		var e store.SignalRegistryEntry
		if err := rows.Scan(&e.SignalID, &e.Condition, &e.WeaknessClass, &e.Scenario,
			&e.Stage, &e.DedupGroup, &e.Control, &e.Direction, &e.RegistryVersion); err != nil {
			return nil, fmt.Errorf("failed to scan registry entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetSignalRegistryEntry returns (nil, nil) when the identifier is unmapped —
// an unmapped signal is an expected outcome of a library upgrade, not an error.
func (s *SQLiteStore) GetSignalRegistryEntry(ctx context.Context, signalID string) (*store.SignalRegistryEntry, error) {
	query := `
	SELECT signal_id, condition, weakness_class, scenario, stage, dedup_group,
	       control, direction, registry_version
	FROM signal_registry WHERE signal_id = ?`
	var e store.SignalRegistryEntry
	err := s.reader().QueryRowContext(ctx, query, signalID).Scan(&e.SignalID, &e.Condition,
		&e.WeaknessClass, &e.Scenario, &e.Stage, &e.DedupGroup, &e.Control, &e.Direction, &e.RegistryVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry entry %s: %w", signalID, err)
	}
	return &e, nil
}

func (s *SQLiteStore) RecordAssessmentCoverage(ctx context.Context, cov *store.AssessmentCoverage) error {
	if !cov.State.Valid() {
		return fmt.Errorf("%w: %q for check %s", ErrInvalidCoverageState, cov.State, cov.CheckID)
	}
	if cov.AssessedAt.IsZero() {
		cov.AssessedAt = time.Now()
	}
	if cov.ID == "" {
		cov.ID = fmt.Sprintf("cov-%s-%s", cov.AssetID, cov.CheckID)
	}

	query := `
	INSERT INTO assessment_coverage (id, asset_id, check_id, state, reason, library_version, assessed_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(asset_id, check_id) DO UPDATE SET
		state = excluded.state,
		reason = excluded.reason,
		library_version = excluded.library_version,
		assessed_at = excluded.assessed_at`
	_, err := s.db.ExecContext(ctx, query, cov.ID, cov.AssetID, cov.CheckID, string(cov.State),
		cov.Reason, cov.LibraryVersion, cov.AssessedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to record assessment coverage for %s: %w", cov.CheckID, err)
	}
	return nil
}

func (s *SQLiteStore) GetAssessmentCoverage(ctx context.Context, assetID string) ([]store.AssessmentCoverage, error) {
	query := `SELECT id, asset_id, check_id, state, reason, library_version, assessed_at FROM assessment_coverage`
	args := []any{}
	if assetID != "" {
		query += ` WHERE asset_id = ?`
		args = append(args, assetID)
	}
	query += ` ORDER BY check_id`

	rows, err := s.reader().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query assessment coverage: %w", err)
	}
	defer rows.Close()

	coverage := []store.AssessmentCoverage{}
	for rows.Next() {
		var c store.AssessmentCoverage
		var state, assessedAt string
		var reason sql.NullString
		if err := rows.Scan(&c.ID, &c.AssetID, &c.CheckID, &state, &reason, &c.LibraryVersion, &assessedAt); err != nil {
			return nil, fmt.Errorf("failed to scan assessment coverage: %w", err)
		}
		c.State = store.CoverageState(state)
		c.Reason = reason.String
		c.AssessedAt, _ = time.Parse(time.RFC3339, assessedAt)
		coverage = append(coverage, c)
	}
	return coverage, rows.Err()
}

// RecordAssessmentRun upserts the latest run for an asset.
//
// An empty outcome is rejected. The whole purpose of the row is to say how the
// run ended, and a blank verdict would read as "unknown" while occupying the
// slot where a real outcome belongs — worse than no row at all.
func (s *SQLiteStore) RecordAssessmentRun(ctx context.Context, run *store.AssessmentRun) error {
	if run.AssetID == "" {
		return fmt.Errorf("assessment run requires an asset id")
	}
	if run.Outcome == "" {
		return fmt.Errorf("assessment run for %s requires an outcome", run.AssetID)
	}
	if run.FinishedAt.IsZero() {
		run.FinishedAt = time.Now()
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = run.FinishedAt
	}

	query := `
	INSERT INTO assessment_runs (asset_id, outcome, error, profile, library_version, started_at, finished_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(asset_id) DO UPDATE SET
		outcome = excluded.outcome,
		error = excluded.error,
		profile = excluded.profile,
		library_version = excluded.library_version,
		started_at = excluded.started_at,
		finished_at = excluded.finished_at`
	_, err := s.db.ExecContext(ctx, query, run.AssetID, run.Outcome, run.Error, run.Profile,
		run.LibraryVersion, run.StartedAt.Format(time.RFC3339), run.FinishedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to record assessment run for %s: %w", run.AssetID, err)
	}
	return nil
}

// GetAssessmentRuns returns the latest run per asset, or for one asset when an
// identifier is given. An asset with no run yields no row rather than an
// error: never having been assessed is an ordinary state, not a fault.
func (s *SQLiteStore) GetAssessmentRuns(ctx context.Context, assetID string) ([]store.AssessmentRun, error) {
	query := `SELECT asset_id, outcome, error, profile, library_version, started_at, finished_at FROM assessment_runs`
	args := []any{}
	if assetID != "" {
		query += ` WHERE asset_id = ?`
		args = append(args, assetID)
	}
	query += ` ORDER BY asset_id`

	rows, err := s.reader().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query assessment runs: %w", err)
	}
	defer rows.Close()

	runs := []store.AssessmentRun{}
	for rows.Next() {
		var r store.AssessmentRun
		var startedAt, finishedAt string
		if err := rows.Scan(&r.AssetID, &r.Outcome, &r.Error, &r.Profile,
			&r.LibraryVersion, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("failed to scan assessment run: %w", err)
		}
		r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		r.FinishedAt, _ = time.Parse(time.RFC3339, finishedAt)
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// ComputeCoverage aggregates coverage for an asset. Each state is counted
// separately; the summary never reduces to a pass/fail figure.
func (s *SQLiteStore) ComputeCoverage(ctx context.Context, assetID string) (store.CoverageSummary, error) {
	var summary store.CoverageSummary

	rows, err := s.db.QueryContext(ctx,
		`SELECT state, COUNT(*) FROM assessment_coverage WHERE asset_id = ? GROUP BY state`, assetID)
	if err != nil {
		return summary, fmt.Errorf("failed to compute coverage: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return summary, fmt.Errorf("failed to scan coverage tally: %w", err)
		}
		switch store.CoverageState(state) {
		case store.CoverageOK:
			summary.OK = count
		case store.CoverageNotFound:
			summary.NotFound = count
		case store.CoverageNotChecked:
			summary.NotChecked = count
		case store.CoverageCheckFailed:
			summary.CheckFailed = count
		}
		summary.Total += count
	}
	summary.AssessedOnly = summary.OK + summary.NotFound
	return summary, rows.Err()
}
