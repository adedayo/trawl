package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

// SaveFeedSnapshot stores a snapshot and, when it carries them, its entries.
// Both are written in one transaction, and the entries of any earlier snapshot
// of the same feed are dropped in it: the metadata of every snapshot is kept
// for provenance, but only the catalogue in force is kept in full.
func (s *SQLiteStore) SaveFeedSnapshot(ctx context.Context, snapshot *store.FeedSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin feed snapshot save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO feed_snapshots (id, feed, source_url, retrieved_at, content_digest, record_count)
		VALUES (?, ?, ?, ?, ?, ?)`, snapshot.ID, snapshot.Feed, snapshot.SourceURL,
		snapshot.RetrievedAt.Format(time.RFC3339), snapshot.ContentDigest, snapshot.RecordCount); err != nil {
		return fmt.Errorf("failed to save feed snapshot: %w", err)
	}
	if len(snapshot.Records) > 0 {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM feed_records WHERE snapshot_id IN
				(SELECT id FROM feed_snapshots WHERE feed = ? AND id <> ?)`, snapshot.Feed, snapshot.ID); err != nil {
			return fmt.Errorf("failed to retire earlier feed records: %w", err)
		}
		if err := insertFeedRecords(ctx, tx, snapshot.ID, snapshot.Records); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit feed snapshot: %w", err)
	}
	return nil
}

func insertFeedRecords(ctx context.Context, tx *sql.Tx, snapshotID string, records []store.FeedRecord) error {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO feed_records (snapshot_id, cve, kev_listed, epss) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare feed records: %w", err)
	}
	defer stmt.Close()
	for _, r := range records {
		var kev, epss any
		if r.KEVListed != nil {
			kev = *r.KEVListed
		}
		if r.EPSS != nil {
			epss = *r.EPSS
		}
		if _, err := stmt.ExecContext(ctx, snapshotID, r.CVE, kev, epss); err != nil {
			return fmt.Errorf("failed to save feed record %s: %w", r.CVE, err)
		}
	}
	return nil
}

// SaveFeedRecords backfills the entries of a snapshot stored before entries
// were kept, retiring those of the feed's earlier snapshots as a new snapshot
// would. It refuses a snapshot that is not the feed's latest, so it cannot
// resurrect a catalogue that has since been superseded.
func (s *SQLiteStore) SaveFeedRecords(ctx context.Context, snapshotID string, records []store.FeedRecord) error {
	latest, err := s.GetFeedSnapshot(ctx, snapshotID)
	if err != nil {
		return err
	}
	if latest == nil {
		return fmt.Errorf("feed snapshot %s is not stored", snapshotID)
	}
	current, err := s.GetLatestFeedSnapshot(ctx, latest.Feed)
	if err != nil {
		return err
	}
	if current == nil || current.ID != snapshotID {
		return fmt.Errorf("feed snapshot %s is not the latest %s snapshot", snapshotID, latest.Feed)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin feed records save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM feed_records WHERE snapshot_id IN
			(SELECT id FROM feed_snapshots WHERE feed = ? AND id <> ?)`, latest.Feed, snapshotID); err != nil {
		return fmt.Errorf("failed to retire earlier feed records: %w", err)
	}
	if err := insertFeedRecords(ctx, tx, snapshotID, records); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetFeedRecords(ctx context.Context, snapshotID string) ([]store.FeedRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cve, kev_listed, epss FROM feed_records WHERE snapshot_id = ? ORDER BY cve`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to read feed records: %w", err)
	}
	defer rows.Close()
	var records []store.FeedRecord
	for rows.Next() {
		var r store.FeedRecord
		var kev sql.NullBool
		var epss sql.NullFloat64
		if err := rows.Scan(&r.CVE, &kev, &epss); err != nil {
			return nil, fmt.Errorf("failed to scan feed record: %w", err)
		}
		if kev.Valid {
			v := kev.Bool
			r.KEVListed = &v
		}
		if epss.Valid {
			v := epss.Float64
			r.EPSS = &v
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading feed records: %w", err)
	}
	return records, nil
}

func (s *SQLiteStore) RecordFeedCheck(ctx context.Context, feed string, checkedAt time.Time, snapshotID string) error {
	var snap any
	if snapshotID != "" {
		snap = snapshotID
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO feed_checks (feed, checked_at, snapshot_id) VALUES (?, ?, ?)
		ON CONFLICT(feed) DO UPDATE SET checked_at = excluded.checked_at, snapshot_id = excluded.snapshot_id`,
		feed, formatTS(checkedAt), snap)
	if err != nil {
		return fmt.Errorf("failed to record feed check: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetFeedSnapshot(ctx context.Context, id string) (*store.FeedSnapshot, error) {
	var snapshot store.FeedSnapshot
	var retrievedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, feed, source_url, retrieved_at, content_digest, record_count
		FROM feed_snapshots WHERE id = ?`, id).Scan(&snapshot.ID, &snapshot.Feed,
		&snapshot.SourceURL, &retrievedAt, &snapshot.ContentDigest, &snapshot.RecordCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get feed snapshot: %w", err)
	}
	snapshot.RetrievedAt, _ = time.Parse(time.RFC3339, retrievedAt)
	return &snapshot, nil
}

func (s *SQLiteStore) GetLatestFeedSnapshot(ctx context.Context, feed string) (*store.FeedSnapshot, error) {
	var snapshot store.FeedSnapshot
	var retrievedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, feed, source_url, retrieved_at, content_digest, record_count
		FROM feed_snapshots WHERE feed = ? ORDER BY retrieved_at DESC LIMIT 1`, feed).
		Scan(&snapshot.ID, &snapshot.Feed, &snapshot.SourceURL, &retrievedAt,
			&snapshot.ContentDigest, &snapshot.RecordCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest feed snapshot: %w", err)
	}
	snapshot.RetrievedAt, _ = time.Parse(time.RFC3339, retrievedAt)
	return &snapshot, nil
}

func (s *SQLiteStore) RecordFeedFailure(ctx context.Context, feed string, failedAt time.Time, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO feed_failures (feed, failed_at, error)
		VALUES (?, ?, ?)
		ON CONFLICT(feed) DO UPDATE SET failed_at = excluded.failed_at, error = excluded.error`,
		feed, failedAt.UTC().Format(time.RFC3339), reason)
	if err != nil {
		return fmt.Errorf("failed to record feed failure: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ClearFeedFailure(ctx context.Context, feed string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM feed_failures WHERE feed = ?`, feed)
	if err != nil {
		return fmt.Errorf("failed to clear feed failure: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetFeedStatuses(ctx context.Context) ([]store.FeedStatus, error) {
	feeds := []string{store.FeedKEV, store.FeedEPSS, store.FeedNVD}
	statuses := make([]store.FeedStatus, 0, len(feeds))
	for _, feedName := range feeds {
		snapshot, err := s.GetLatestFeedSnapshot(ctx, feedName)
		if err != nil {
			return nil, err
		}
		status := store.FeedStatus{
			Feed:     feedName,
			Snapshot: snapshot,
		}
		if snapshot != nil {
			var stored int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM feed_records WHERE snapshot_id = ?`, snapshot.ID).Scan(&stored); err != nil {
				return nil, fmt.Errorf("failed to count feed records for %s: %w", feedName, err)
			}
			status.RecordsStored = stored > 0
		}
		var checkedAt sql.NullString
		if err := s.db.QueryRowContext(ctx, `SELECT checked_at FROM feed_checks WHERE feed = ?`, feedName).Scan(&checkedAt); err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to read feed check for %s: %w", feedName, err)
		}
		if status.LastCheckedAt, err = parseTSPtr("feed check", checkedAt); err != nil {
			return nil, err
		}
		var failedAt string
		var reason string
		err = s.db.QueryRowContext(ctx, `SELECT failed_at, error FROM feed_failures WHERE feed = ?`, feedName).
			Scan(&failedAt, &reason)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get feed failure for %s: %w", feedName, err)
		}
		if err == nil {
			if parsed, parseErr := time.Parse(time.RFC3339, failedAt); parseErr == nil {
				status.LastFailureAt = &parsed
			}
			status.LastError = reason
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (s *SQLiteStore) SaveFindingEnrichment(ctx context.Context, enrichment *store.FindingEnrichment) error {
	var epss any = enrichment.EPSS
	var kev any
	if enrichment.KEVListed != nil {
		kev = *enrichment.KEVListed
	}
	var snapshotID any
	if enrichment.SnapshotID != "" {
		snapshotID = enrichment.SnapshotID
	}
	var checkedAt any
	if enrichment.CheckedAt != nil {
		checkedAt = enrichment.CheckedAt.Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO finding_enrichments
			(finding_id, feed, cve, state, snapshot_id, epss, kev_listed, checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(finding_id, feed) DO UPDATE SET
			cve = excluded.cve, state = excluded.state, snapshot_id = excluded.snapshot_id,
			epss = excluded.epss, kev_listed = excluded.kev_listed, checked_at = excluded.checked_at`,
		enrichment.FindingID, enrichment.Feed, enrichment.CVE, enrichment.State,
		snapshotID, epss, kev, checkedAt)
	if err != nil {
		return fmt.Errorf("failed to save finding enrichment: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetFindingEnrichments(ctx context.Context, findingID string) ([]store.FindingEnrichment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT finding_id, feed, cve, state, COALESCE(snapshot_id, ''), epss,
		       kev_listed, checked_at
		FROM finding_enrichments WHERE finding_id = ? ORDER BY feed`, findingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get finding enrichments: %w", err)
	}
	defer rows.Close()

	var enrichments []store.FindingEnrichment
	for rows.Next() {
		var enrichment store.FindingEnrichment
		var epss sql.NullFloat64
		var kev sql.NullBool
		var checkedAt sql.NullString
		if err := rows.Scan(&enrichment.FindingID, &enrichment.Feed, &enrichment.CVE,
			&enrichment.State, &enrichment.SnapshotID, &epss, &kev, &checkedAt); err != nil {
			return nil, fmt.Errorf("failed to scan finding enrichment: %w", err)
		}
		if epss.Valid {
			value := epss.Float64
			enrichment.EPSS = &value
		}
		if kev.Valid {
			value := kev.Bool
			enrichment.KEVListed = &value
		}
		if checkedAt.Valid {
			value, _ := time.Parse(time.RFC3339, checkedAt.String)
			enrichment.CheckedAt = &value
		}
		enrichments = append(enrichments, enrichment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading finding enrichments: %w", err)
	}
	rows.Close()

	for i := range enrichments {
		if enrichments[i].SnapshotID != "" {
			enrichments[i].Snapshot, err = s.GetFeedSnapshot(ctx, enrichments[i].SnapshotID)
			if err != nil {
				return nil, err
			}
		}
	}
	return enrichments, nil
}

// UpdateFindingCatalogue applies a feed refresh to a finding's EPSS, KEV and
// priority. A priority that moves is recorded as an event naming the feed (and
// the snapshot, when the run carries one) as its cause, so "became critical
// because it was added to KEV" is readable from the timeline rather than
// inferred.
func (s *SQLiteStore) UpdateFindingCatalogue(ctx context.Context, findingID string, epss float64, kev bool, priority string, updateEPSS, updateKEV bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin finding catalogue update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var assetID, prevPriority, severity string
	err = tx.QueryRowContext(ctx, `SELECT asset_id, priority, severity FROM findings WHERE id = ?`, findingID).
		Scan(&assetID, &prevPriority, &severity)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read finding %s: %w", findingID, err)
	}
	found := err == nil

	query := `UPDATE findings SET epss = CASE WHEN ? THEN ? ELSE epss END, kev_listed = CASE WHEN ? THEN ? ELSE kev_listed END, priority = ? WHERE id = ?`
	if _, err := tx.ExecContext(ctx, query, updateEPSS, epss, updateKEV, kev, priority, findingID); err != nil {
		return fmt.Errorf("failed to update finding catalogue values: %w", err)
	}

	if found && prevPriority != priority {
		if store.RunFrom(ctx).Cause == "" {
			ctx = store.WithRun(ctx, withCause(store.RunFrom(ctx), store.CauseFeed))
		}
		e := s.newEvent(ctx, store.SourceFinding, findingID, assetID, store.EventPriorityChanged)
		e.Severity = store.FindingSeverity(severity)
		e.PrevPriority, e.Priority = prevPriority, priority
		if err := appendEvent(ctx, tx, &e); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit finding catalogue update: %w", err)
	}
	return nil
}

func withCause(ref store.RunRef, cause string) store.RunRef {
	ref.Cause = cause
	return ref
}

func (s *SQLiteStore) RecordFeedRegression(ctx context.Context, assetID, feed, cve, previous, current string) (*store.Regression, error) {
	now := time.Now().UTC()
	reg := &store.Regression{
		ID:               fmt.Sprintf("reg-feed-%s-%d", cve, now.UnixNano()),
		AssetID:          assetID,
		AttributeType:    "catalogue:" + feed + ":" + cve,
		PreviousValue:    previous,
		CurrentValue:     current,
		ConsecutiveFails: 1,
		ConfirmedAt:      now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO regressions
			(id, asset_id, attribute_type, previous_value, current_value, consecutive_fails, confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, reg.ID, reg.AssetID, reg.AttributeType,
		reg.PreviousValue, reg.CurrentValue, reg.ConsecutiveFails, now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to record feed regression: %w", err)
	}
	return reg, nil
}
