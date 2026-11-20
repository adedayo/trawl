package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/adedayo/trawl/pkg/store"
)

// tsLayout is the layout every history timestamp is written in: UTC, with a
// fixed-width fractional part. Fixed width is what makes the strings sort the
// same way the instants do; RFC3339Nano trims trailing zeros, and "…01Z" then
// sorts after "…01.5Z" although it is earlier.
const tsLayout = "2006-01-02T15:04:05.000000000Z"

func formatTS(t time.Time) string { return t.UTC().Format(tsLayout) }

// parseTS reads a stored timestamp. A value that cannot be read is an error
// rather than a silent zero time: a finding that appears to have been first
// seen in the year 1 is a finding whose age nobody can trust.
func parseTS(field, v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("unreadable %s timestamp %q: %w", field, v, err)
	}
	return t.UTC(), nil
}

func parseTSPtr(field string, v sql.NullString) (*time.Time, error) {
	if !v.Valid || v.String == "" {
		return nil, nil
	}
	t, err := parseTS(field, v.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// execer is the part of *sql.DB and *sql.Tx the event writer needs, so that the
// same code serves a standalone write and one inside a larger transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// newEvent starts an event for a finding, stamped with the run and cause the
// context carries. Cause defaults to the estate: a change nobody attributed
// elsewhere is a change the estate made.
func (s *SQLiteStore) newEvent(ctx context.Context, source, rowID, assetID string, typ store.EventType) store.FindingEvent {
	run := store.RunFrom(ctx)
	cause := run.Cause
	if cause == "" {
		cause = store.CauseEstate
	}
	return store.FindingEvent{
		ID:         "evt-" + uuid.NewString(),
		Key:        store.FindingKey(source, rowID),
		Source:     source,
		RowID:      rowID,
		AssetID:    assetID,
		Type:       typ,
		At:         s.now(),
		RunID:      run.ID,
		Cause:      cause,
		SnapshotID: run.SnapshotID,
	}
}

// appendEvent is the only statement in the engine that writes finding_events.
func appendEvent(ctx context.Context, ex execer, e *store.FindingEvent) error {
	_, err := ex.ExecContext(ctx, `
	INSERT INTO finding_events (
		id, finding_key, source, row_id, asset_id, type, at, run_id, cause,
		prev_severity, severity, prev_priority, priority,
		library_version, registry_version, snapshot_id, note
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Key, e.Source, e.RowID, e.AssetID, string(e.Type), formatTS(e.At), e.RunID, e.Cause,
		string(e.PrevSeverity), string(e.Severity), e.PrevPriority, e.Priority,
		e.LibraryVersion, e.RegistryVersion, e.SnapshotID, e.Note)
	if err != nil {
		return fmt.Errorf("failed to record %s event for %s: %w", e.Type, e.Key, err)
	}
	return nil
}

// baselineNote is attached to the single event given to a finding recorded
// before history existed.
const baselineNote = "Recorded before history began. Earlier change is not known."

// seedBaselineEvents gives every finding that has no events one event, at its
// recorded first-seen time, labelled as a baseline. It reconstructs nothing:
// the event asserts only that the finding was known by then.
//
// It is idempotent, so it is safe to run on every open. A finding gains a real
// first_seen event only if it is deleted and observed again.
func (s *SQLiteStore) seedBaselineEvents(ctx context.Context, tx *sql.Tx) error {
	type seed struct {
		source, table, cols string
		build               func(rows *sql.Rows) (store.FindingEvent, error)
	}
	seeds := []seed{
		{store.SourceSignal, "signal_observations",
			"id, asset_id, severity, COALESCE(library_version,''), COALESCE(registry_version,''), first_seen",
			func(r *sql.Rows) (store.FindingEvent, error) {
				var id, asset, sev, lib, reg, first string
				if err := r.Scan(&id, &asset, &sev, &lib, &reg, &first); err != nil {
					return store.FindingEvent{}, err
				}
				at, err := parseTS("first_seen", first)
				if err != nil {
					return store.FindingEvent{}, err
				}
				return store.FindingEvent{RowID: id, AssetID: asset, At: at,
					Severity: store.FindingSeverity(sev), LibraryVersion: lib, RegistryVersion: reg}, nil
			}},
		{store.SourceFinding, "findings",
			"id, asset_id, severity, priority, first_seen",
			func(r *sql.Rows) (store.FindingEvent, error) {
				var id, asset, sev, prio, first string
				if err := r.Scan(&id, &asset, &sev, &prio, &first); err != nil {
					return store.FindingEvent{}, err
				}
				at, err := parseTS("first_seen", first)
				if err != nil {
					return store.FindingEvent{}, err
				}
				return store.FindingEvent{RowID: id, AssetID: asset, At: at,
					Severity: store.FindingSeverity(sev), Priority: prio}, nil
			}},
		{store.SourceSecret, "secret_findings",
			"id, asset_id, verified, first_seen",
			func(r *sql.Rows) (store.FindingEvent, error) {
				var id, asset, first string
				var verified int
				if err := r.Scan(&id, &asset, &verified, &first); err != nil {
					return store.FindingEvent{}, err
				}
				at, err := parseTS("first_seen", first)
				if err != nil {
					return store.FindingEvent{}, err
				}
				sev := store.SecretSeverity(verified == 1)
				return store.FindingEvent{RowID: id, AssetID: asset, At: at,
					Severity: sev, Priority: string(sev)}, nil
			}},
	}

	for _, sd := range seeds {
		rows, err := tx.QueryContext(ctx, fmt.Sprintf(
			`SELECT %s FROM %s t WHERE NOT EXISTS (SELECT 1 FROM finding_events e WHERE e.source = ? AND e.row_id = t.id)`,
			sd.cols, sd.table), sd.source)
		if err != nil {
			return fmt.Errorf("reading %s for baseline events: %w", sd.table, err)
		}
		var pending []store.FindingEvent
		for rows.Next() {
			e, err := sd.build(rows)
			if err != nil {
				rows.Close()
				return fmt.Errorf("reading %s for baseline events: %w", sd.table, err)
			}
			pending = append(pending, e)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		for _, e := range pending {
			e.Source = sd.source
			e.Key = store.FindingKey(sd.source, e.RowID)
			e.ID = "baseline-" + e.Key
			e.Type = store.EventBaseline
			e.Cause = store.CauseBaseline
			e.Note = baselineNote
			if err := appendEvent(ctx, tx, &e); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetFindingEvents reads events in the order they were recorded.
func (s *SQLiteStore) GetFindingEvents(ctx context.Context, q store.EventQuery) ([]store.FindingEvent, error) {
	query := `SELECT id, finding_key, source, row_id, asset_id, type, at, run_id, cause,
		prev_severity, severity, prev_priority, priority,
		library_version, registry_version, snapshot_id, note
		FROM finding_events`
	var where []string
	var args []any
	if q.Key != "" {
		where = append(where, "finding_key = ?")
		args = append(args, q.Key)
	}
	if q.RunID != "" {
		where = append(where, "run_id = ?")
		args = append(args, q.RunID)
	}
	if !q.After.IsZero() {
		where = append(where, "at > ?")
		args = append(args, formatTS(q.After))
	}
	if !q.Until.IsZero() {
		where = append(where, "at <= ?")
		args = append(args, formatTS(q.Until))
	}
	if !q.Since.IsZero() {
		where = append(where, "at >= ?")
		args = append(args, formatTS(q.Since))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY seq"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query finding events: %w", err)
	}
	defer rows.Close()

	out := []store.FindingEvent{}
	for rows.Next() {
		var e store.FindingEvent
		var typ, at, prevSev, sev string
		if err := rows.Scan(&e.ID, &e.Key, &e.Source, &e.RowID, &e.AssetID, &typ, &at, &e.RunID, &e.Cause,
			&prevSev, &sev, &e.PrevPriority, &e.Priority,
			&e.LibraryVersion, &e.RegistryVersion, &e.SnapshotID, &e.Note); err != nil {
			return nil, fmt.Errorf("failed to scan finding event: %w", err)
		}
		e.Type = store.EventType(typ)
		e.PrevSeverity = store.FindingSeverity(prevSev)
		e.Severity = store.FindingSeverity(sev)
		if e.At, err = parseTS("event", at); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RecordScanRun appends a run and the summary of the estate once the run's
// writes have landed, in one transaction.
func (s *SQLiteStore) RecordScanRun(ctx context.Context, run *store.ScanRun) error {
	if run.ID == "" {
		run.ID = "run-" + uuid.NewString()
	}
	if run.FinishedAt.IsZero() {
		run.FinishedAt = s.now()
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = run.FinishedAt
	}
	if run.Outcome == "" {
		run.Outcome = "completed"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin run record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
	INSERT INTO scan_runs (id, kind, asset_id, profile, outcome, error, library_version, registry_version,
		started_at, finished_at, cov_total, cov_ok, cov_not_found, cov_not_checked, cov_check_failed)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.Kind, run.AssetID, run.Profile, run.Outcome, run.Error, run.LibraryVersion, run.RegistryVersion,
		formatTS(run.StartedAt), formatTS(run.FinishedAt),
		run.Coverage.Total, run.Coverage.OK, run.Coverage.NotFound, run.Coverage.NotChecked, run.Coverage.CheckFailed); err != nil {
		return fmt.Errorf("failed to record scan run %s: %w", run.ID, err)
	}

	sum := store.ScanSummary{RunID: run.ID, At: run.FinishedAt}

	// Open findings by rank. Priority is the deterministic ranking where one
	// exists; a signal observation is rated by its severity, and a secret by
	// whether it was verified live. Observations recorded at a state that is
	// not an assessment are not findings and are not counted.
	rows, err := tx.QueryContext(ctx, `
	SELECT sev, COUNT(*) FROM (
		SELECT COALESCE(NULLIF(priority,''), severity) AS sev FROM findings WHERE COALESCE(status,'open') != 'resolved'
		UNION ALL
		SELECT severity FROM signal_observations
			WHERE COALESCE(status,'open') != 'resolved' AND state IN ('ok','not_found')
		UNION ALL
		SELECT CASE verified WHEN 1 THEN 'critical' ELSE 'high' END FROM secret_findings WHERE COALESCE(status,'open') != 'resolved'
	) GROUP BY sev`)
	if err != nil {
		return fmt.Errorf("failed to summarise open findings: %w", err)
	}
	for rows.Next() {
		var sev string
		var n int
		if err := rows.Scan(&sev, &n); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan open-finding summary: %w", err)
		}
		sum.Open += n
		switch store.FindingSeverity(sev) {
		case store.SeverityCritical:
			sum.Critical += n
		case store.SeverityHigh:
			sum.High += n
		case store.SeverityMedium:
			sum.Medium += n
		case store.SeverityLow:
			sum.Low += n
		default:
			sum.Info += n
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if err := tx.QueryRowContext(ctx, `
	SELECT
		(SELECT COUNT(*) FROM findings WHERE status = 'resolved') +
		(SELECT COUNT(*) FROM signal_observations WHERE status = 'resolved') +
		(SELECT COUNT(*) FROM secret_findings WHERE status = 'resolved')`).Scan(&sum.Resolved); err != nil {
		return fmt.Errorf("failed to count resolved findings: %w", err)
	}

	evRows, err := tx.QueryContext(ctx, `SELECT type, COUNT(*) FROM finding_events WHERE run_id = ? GROUP BY type`, run.ID)
	if err != nil {
		return fmt.Errorf("failed to count run transitions: %w", err)
	}
	for evRows.Next() {
		var typ string
		var n int
		if err := evRows.Scan(&typ, &n); err != nil {
			evRows.Close()
			return err
		}
		switch store.EventType(typ) {
		case store.EventFirstSeen, store.EventReopened:
			sum.New += n
		case store.EventResolved:
			sum.ResolvedInRun += n
		}
	}
	if err := evRows.Err(); err != nil {
		evRows.Close()
		return err
	}
	evRows.Close()

	if _, err := tx.ExecContext(ctx, `
	INSERT INTO scan_summaries (run_id, at, critical, high, medium, low, info, open_total, resolved_total, new_count, resolved_count)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sum.RunID, formatTS(sum.At), sum.Critical, sum.High, sum.Medium, sum.Low, sum.Info,
		sum.Open, sum.Resolved, sum.New, sum.ResolvedInRun); err != nil {
		return fmt.Errorf("failed to record run summary %s: %w", run.ID, err)
	}

	// Retention applies to summaries only. Events are the record; summaries are
	// a chart derived from them.
	var raw sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, store.HistorySummaryRetentionKey).Scan(&raw); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read summary retention: %w", err)
	}
	if days, convErr := strconv.Atoi(strings.TrimSpace(raw.String)); convErr == nil && days > 0 {
		cutoff := run.FinishedAt.AddDate(0, 0, -days)
		if _, err := tx.ExecContext(ctx, `DELETE FROM scan_summaries WHERE at < ?`, formatTS(cutoff)); err != nil {
			return fmt.Errorf("failed to prune run summaries: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit scan run %s: %w", run.ID, err)
	}
	return nil
}

// GetScanRuns lists runs, newest first.
func (s *SQLiteStore) GetScanRuns(ctx context.Context, limit int) ([]store.ScanRun, error) {
	query := `SELECT id, kind, asset_id, profile, outcome, error, library_version, registry_version,
		started_at, finished_at, cov_total, cov_ok, cov_not_found, cov_not_checked, cov_check_failed
		FROM scan_runs ORDER BY finished_at DESC, rowid DESC`
	var args []any
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query scan runs: %w", err)
	}
	defer rows.Close()

	out := []store.ScanRun{}
	for rows.Next() {
		var r store.ScanRun
		var started, finished string
		if err := rows.Scan(&r.ID, &r.Kind, &r.AssetID, &r.Profile, &r.Outcome, &r.Error, &r.LibraryVersion, &r.RegistryVersion,
			&started, &finished, &r.Coverage.Total, &r.Coverage.OK, &r.Coverage.NotFound,
			&r.Coverage.NotChecked, &r.Coverage.CheckFailed); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		r.Coverage.AssessedOnly = r.Coverage.OK + r.Coverage.NotFound
		if r.StartedAt, err = parseTS("run start", started); err != nil {
			return nil, err
		}
		if r.FinishedAt, err = parseTS("run finish", finished); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetScanSummaries lists summaries from since, oldest first.
func (s *SQLiteStore) GetScanSummaries(ctx context.Context, since time.Time) ([]store.ScanSummary, error) {
	query := `SELECT run_id, at, critical, high, medium, low, info, open_total, resolved_total, new_count, resolved_count FROM scan_summaries`
	var args []any
	if !since.IsZero() {
		query += " WHERE at >= ?"
		args = append(args, formatTS(since))
	}
	query += " ORDER BY at, rowid"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query scan summaries: %w", err)
	}
	defer rows.Close()

	out := []store.ScanSummary{}
	for rows.Next() {
		var m store.ScanSummary
		var at string
		if err := rows.Scan(&m.RunID, &at, &m.Critical, &m.High, &m.Medium, &m.Low, &m.Info,
			&m.Open, &m.Resolved, &m.New, &m.ResolvedInRun); err != nil {
			return nil, fmt.Errorf("failed to scan run summary: %w", err)
		}
		if m.At, err = parseTS("summary", at); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PruneScanSummaries drops summaries older than before. Events and runs are
// untouched.
func (s *SQLiteStore) PruneScanSummaries(ctx context.Context, before time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM scan_summaries WHERE at < ?`, formatTS(before))
	if err != nil {
		return 0, fmt.Errorf("failed to prune run summaries: %w", err)
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// ResolveSignalObservations resolves open observations of concluded checks that
// the latest run did not raise, and notes the assessment on the ones it did.
func (s *SQLiteStore) ResolveSignalObservations(ctx context.Context, assetID string, concludedChecks []string, raised map[string]bool) (int, error) {
	if len(concludedChecks) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(concludedChecks)), ",")
	args := []any{assetID}
	for _, c := range concludedChecks {
		args = append(args, c)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin resolution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
	SELECT id, signal_id, severity, COALESCE(library_version,''), COALESCE(registry_version,'')
	FROM signal_observations
	WHERE asset_id = ? AND check_id IN (`+placeholders+`) AND COALESCE(status,'open') != 'resolved'`, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to read open observations: %w", err)
	}
	type open struct{ id, signal, sev, lib, reg string }
	var opens []open
	for rows.Next() {
		var o open
		if err := rows.Scan(&o.id, &o.signal, &o.sev, &o.lib, &o.reg); err != nil {
			rows.Close()
			return 0, err
		}
		opens = append(opens, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	now := formatTS(s.now())
	resolved := 0
	for _, o := range opens {
		if raised[o.signal] {
			if _, err := tx.ExecContext(ctx, `UPDATE signal_observations SET last_assessed_at = ? WHERE id = ?`, now, o.id); err != nil {
				return 0, fmt.Errorf("failed to note assessment of %s: %w", o.signal, err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE signal_observations SET status = 'resolved', resolved_at = ?, last_assessed_at = ? WHERE id = ?`,
			now, now, o.id); err != nil {
			return 0, fmt.Errorf("failed to resolve %s: %w", o.signal, err)
		}
		e := s.newEvent(ctx, store.SourceSignal, o.id, assetID, store.EventResolved)
		e.Severity = store.FindingSeverity(o.sev)
		e.LibraryVersion, e.RegistryVersion = o.lib, o.reg
		if err := appendEvent(ctx, tx, &e); err != nil {
			return 0, err
		}
		resolved++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit resolution: %w", err)
	}
	return resolved, nil
}

// ResolveSecretFindings resolves open secrets of a repository that a completed
// scan did not find.
func (s *SQLiteStore) ResolveSecretFindings(ctx context.Context, repoURL string, seen map[string]bool) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin secret resolution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
	SELECT id, asset_id, verified FROM secret_findings
	WHERE repo_url = ? AND COALESCE(status,'open') != 'resolved'`, repoURL)
	if err != nil {
		return 0, fmt.Errorf("failed to read open secrets: %w", err)
	}
	type open struct {
		id, asset string
		verified  bool
	}
	var opens []open
	for rows.Next() {
		var o open
		var v int
		if err := rows.Scan(&o.id, &o.asset, &v); err != nil {
			rows.Close()
			return 0, err
		}
		o.verified = v == 1
		opens = append(opens, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	now := formatTS(s.now())
	resolved := 0
	for _, o := range opens {
		if seen[o.id] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE secret_findings SET status = 'resolved', resolved_at = ? WHERE id = ?`, now, o.id); err != nil {
			return 0, fmt.Errorf("failed to resolve secret %s: %w", o.id, err)
		}
		e := s.newEvent(ctx, store.SourceSecret, o.id, o.asset, store.EventResolved)
		e.Severity = store.SecretSeverity(o.verified)
		e.Priority = string(e.Severity)
		if err := appendEvent(ctx, tx, &e); err != nil {
			return 0, err
		}
		resolved++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit secret resolution: %w", err)
	}
	return resolved, nil
}
