package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetFindings(ctx context.Context, assetID string) ([]store.Finding, error) {
	query := `SELECT id, asset_id, title, COALESCE(description,''), severity, priority, COALESCE(cve,''), epss, kev_listed, category, COALESCE(proof,''), COALESCE(ai_annotation,''), first_seen, last_seen FROM findings`
	var rows *sql.Rows
	var err error

	if assetID != "" {
		query += ` WHERE asset_id = ? ORDER BY last_seen DESC`
		rows, err = s.db.QueryContext(ctx, query, assetID)
	} else {
		query += ` ORDER BY last_seen DESC`
		rows, err = s.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query findings: %w", err)
	}
	defer rows.Close()

	findings := []store.Finding{}
	for rows.Next() {
		var f store.Finding
		var kevInt int
		var firstSeen, lastSeen string
		if err := rows.Scan(&f.ID, &f.AssetID, &f.Title, &f.Description, &f.Severity, &f.Priority, &f.CVE, &f.EPSS, &kevInt, &f.Category, &f.Proof, &f.AIAnnotation, &firstSeen, &lastSeen); err != nil {
			return nil, fmt.Errorf("failed to scan finding: %w", err)
		}
		f.KEVListed = kevInt == 1
		f.FirstSeen, _ = time.Parse(time.RFC3339, firstSeen)
		f.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		findings = append(findings, f)
	}

	return findings, nil
}

func (s *SQLiteStore) SaveFinding(ctx context.Context, finding *store.Finding) error {
	query := `
	INSERT INTO findings (id, asset_id, title, description, severity, priority, cve, epss, kev_listed, category, proof, ai_annotation, first_seen, last_seen)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		severity = excluded.severity,
		priority = excluded.priority,
		epss = excluded.epss,
		kev_listed = excluded.kev_listed,
		ai_annotation = excluded.ai_annotation,
		last_seen = excluded.last_seen
	`
	kevInt := 0
	if finding.KEVListed {
		kevInt = 1
	}

	nowStr := time.Now().Format(time.RFC3339)
	if finding.FirstSeen.IsZero() {
		finding.FirstSeen = time.Now()
	}

	_, err := s.db.ExecContext(ctx, query,
		finding.ID,
		finding.AssetID,
		finding.Title,
		finding.Description,
		finding.Severity,
		finding.Priority,
		finding.CVE,
		finding.EPSS,
		kevInt,
		finding.Category,
		finding.Proof,
		finding.AIAnnotation,
		finding.FirstSeen.Format(time.RFC3339),
		nowStr,
	)
	if err != nil {
		return fmt.Errorf("failed to save finding: %w", err)
	}
	return nil
}
