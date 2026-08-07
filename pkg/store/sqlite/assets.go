package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetAssets(ctx context.Context, status store.AssetStatus) ([]store.Asset, error) {
	query := `SELECT id, type, value, status, discovery_source, confidence, first_seen, last_seen, COALESCE(metadata, '') FROM assets`
	var rows *sql.Rows
	var err error

	if status != "" {
		query += ` WHERE status = ? ORDER BY last_seen DESC`
		rows, err = s.db.QueryContext(ctx, query, status)
	} else {
		query += ` ORDER BY last_seen DESC`
		rows, err = s.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query assets: %w", err)
	}
	defer rows.Close()

	assets := []store.Asset{}
	for rows.Next() {
		var a store.Asset
		var firstSeen, lastSeen string
		if err := rows.Scan(&a.ID, &a.Type, &a.Value, &a.Status, &a.DiscoverySource, &a.Confidence, &firstSeen, &lastSeen, &a.Metadata); err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		a.FirstSeen, _ = time.Parse(time.RFC3339, firstSeen)
		a.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		assets = append(assets, a)
	}

	return assets, nil
}

func (s *SQLiteStore) GetAssetByID(ctx context.Context, id string) (*store.Asset, error) {
	query := `SELECT id, type, value, status, discovery_source, confidence, first_seen, last_seen, COALESCE(metadata, '') FROM assets WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var a store.Asset
	var firstSeen, lastSeen string
	if err := row.Scan(&a.ID, &a.Type, &a.Value, &a.Status, &a.DiscoverySource, &a.Confidence, &firstSeen, &lastSeen, &a.Metadata); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan asset by ID: %w", err)
	}
	a.FirstSeen, _ = time.Parse(time.RFC3339, firstSeen)
	a.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	return &a, nil
}

func (s *SQLiteStore) SaveAsset(ctx context.Context, asset *store.Asset) error {
	query := `
	INSERT INTO assets (id, type, value, status, discovery_source, confidence, first_seen, last_seen, metadata)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(value) DO UPDATE SET
		status = excluded.status,
		confidence = excluded.confidence,
		last_seen = excluded.last_seen,
		metadata = excluded.metadata
	RETURNING id
	`
	nowStr := time.Now().Format(time.RFC3339)
	if asset.FirstSeen.IsZero() {
		asset.FirstSeen = time.Now()
	}
	if asset.LastSeen.IsZero() {
		asset.LastSeen = time.Now()
	}

	err := s.db.QueryRowContext(ctx, query,
		asset.ID,
		asset.Type,
		asset.Value,
		asset.Status,
		asset.DiscoverySource,
		asset.Confidence,
		asset.FirstSeen.Format(time.RFC3339),
		nowStr,
		asset.Metadata,
	).Scan(&asset.ID)
	if err != nil {
		return fmt.Errorf("failed to save asset: %w", err)
	}
	return nil
}

// DeleteAsset removes an asset and, by cascade, everything recorded against it.
//
// Foreign keys are enforced explicitly for this statement rather than assumed.
// If the PRAGMA is off, the delete would succeed and leave findings and
// observations orphaned — pointing at an asset that no longer exists, and
// counted by any query that does not join.
func (s *SQLiteStore) DeleteAsset(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin asset delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM assets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete asset %s: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm the deletion of asset %s: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("no such asset: %s", id)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit the deletion of asset %s: %w", id, err)
	}
	return nil
}
