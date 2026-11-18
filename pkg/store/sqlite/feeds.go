package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) SaveFeedSnapshot(ctx context.Context, snapshot *store.FeedSnapshot) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO feed_snapshots (id, feed, source_url, retrieved_at, content_digest, record_count)
		VALUES (?, ?, ?, ?, ?, ?)`, snapshot.ID, snapshot.Feed, snapshot.SourceURL,
		snapshot.RetrievedAt.Format(time.RFC3339), snapshot.ContentDigest, snapshot.RecordCount)
	if err != nil {
		return fmt.Errorf("failed to save feed snapshot: %w", err)
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
		if enrichment.SnapshotID != "" {
			enrichment.Snapshot, err = s.GetFeedSnapshot(ctx, enrichment.SnapshotID)
			if err != nil {
				return nil, err
			}
		}
		enrichments = append(enrichments, enrichment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading finding enrichments: %w", err)
	}
	return enrichments, nil
}

func (s *SQLiteStore) UpdateFindingCatalogue(ctx context.Context, findingID string, epss float64, kev bool, priority string, updateEPSS, updateKEV bool) error {
	query := `UPDATE findings SET epss = CASE WHEN ? THEN ? ELSE epss END, kev_listed = CASE WHEN ? THEN ? ELSE kev_listed END, priority = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, updateEPSS, epss, updateKEV, kev, priority, findingID)
	if err != nil {
		return fmt.Errorf("failed to update finding catalogue values: %w", err)
	}
	return nil
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
