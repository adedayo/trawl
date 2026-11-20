package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetFindings(ctx context.Context, assetID string) ([]store.Finding, error) {
	query := `SELECT id, asset_id, title, COALESCE(description,''), severity, priority, COALESCE(cve,''), epss, kev_listed, COALESCE(status,'open'), category, COALESCE(proof,''), COALESCE(ai_annotation,''), first_seen, last_seen, resolved_at FROM findings`
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
		var resolvedAt sql.NullString
		if err := rows.Scan(&f.ID, &f.AssetID, &f.Title, &f.Description, &f.Severity, &f.Priority, &f.CVE, &f.EPSS, &kevInt, &f.Status, &f.Category, &f.Proof, &f.AIAnnotation, &firstSeen, &lastSeen, &resolvedAt); err != nil {
			return nil, fmt.Errorf("failed to scan finding: %w", err)
		}
		f.KEVListed = kevInt == 1
		if f.FirstSeen, err = parseTS("finding first_seen", firstSeen); err != nil {
			return nil, err
		}
		if f.LastSeen, err = parseTS("finding last_seen", lastSeen); err != nil {
			return nil, err
		}
		if f.ResolvedAt, err = parseTSPtr("finding resolved_at", resolvedAt); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read findings: %w", err)
	}
	// Enrichments are read after the first cursor is closed: a second query
	// issued while it is open would hold a second connection for the whole
	// listing.
	rows.Close()
	for i := range findings {
		findings[i].Enrichments, err = s.GetFindingEnrichments(ctx, findings[i].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query finding enrichments: %w", err)
		}
	}

	return findings, nil
}

// SaveFinding upserts a finding and, in the same transaction, records the
// event the write amounts to: first seen, a change in severity or priority,
// resolution, or reopening. This is the one write path, so the history and the
// current state cannot disagree.
//
// A finding that was resolved and is saved again as open is reopened: the
// stored status becomes "reopened", which stays until it is resolved again.
func (s *SQLiteStore) SaveFinding(ctx context.Context, finding *store.Finding) error {
	now := s.now()
	if finding.FirstSeen.IsZero() {
		finding.FirstSeen = now
	}
	if finding.Status == "" {
		finding.Status = store.FindingOpen
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin finding save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prevStatus, prevSeverity, prevPriority string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(status,'open'), severity, priority FROM findings WHERE id = ?`, finding.ID).
		Scan(&prevStatus, &prevSeverity, &prevPriority)
	exists := err == nil
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read finding %s: %w", finding.ID, err)
	}

	var evType store.EventType
	var resolvedAt any
	switch {
	case !exists:
		evType = store.EventFirstSeen
	case prevStatus == string(store.FindingResolved) && finding.Status != store.FindingResolved:
		evType = store.EventReopened
		finding.Status = store.FindingReopened
	case prevStatus != string(store.FindingResolved) && finding.Status == store.FindingResolved:
		evType = store.EventResolved
	case prevSeverity != string(finding.Severity):
		evType = store.EventChanged
	case prevPriority != finding.Priority && prevPriority != "" && finding.Priority != "":
		evType = store.EventPriorityChanged
	}
	// A finding already reopened stays reopened until it is resolved again; a
	// later save as plain open must not quietly clear that.
	if exists && prevStatus == string(store.FindingReopened) && finding.Status == store.FindingOpen {
		finding.Status = store.FindingReopened
	}
	if finding.Status == store.FindingResolved {
		resolvedAt = formatTS(now)
	}

	query := `
	INSERT INTO findings (id, asset_id, title, description, severity, priority, cve, epss, kev_listed, status, category, proof, ai_annotation, first_seen, last_seen, resolved_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		severity = excluded.severity,
		priority = excluded.priority,
		epss = excluded.epss,
		kev_listed = excluded.kev_listed,
		status = excluded.status,
		ai_annotation = excluded.ai_annotation,
		last_seen = excluded.last_seen,
		resolved_at = excluded.resolved_at
	`
	kevInt := 0
	if finding.KEVListed {
		kevInt = 1
	}
	if _, err := tx.ExecContext(ctx, query,
		finding.ID,
		finding.AssetID,
		finding.Title,
		finding.Description,
		finding.Severity,
		finding.Priority,
		finding.CVE,
		finding.EPSS,
		kevInt,
		finding.Status,
		finding.Category,
		finding.Proof,
		finding.AIAnnotation,
		formatTS(finding.FirstSeen),
		formatTS(now),
		resolvedAt,
	); err != nil {
		return fmt.Errorf("failed to save finding: %w", err)
	}

	for _, feed := range []string{store.FeedKEV, store.FeedEPSS, store.FeedNVD} {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO finding_enrichments (finding_id, feed, cve, state)
			VALUES (?, ?, ?, ?)`, finding.ID, feed, finding.CVE, store.EnrichmentNotChecked); err != nil {
			return fmt.Errorf("failed to initialize finding enrichment: %w", err)
		}
	}

	if evType != "" {
		e := s.newEvent(ctx, store.SourceFinding, finding.ID, finding.AssetID, evType)
		e.Severity = finding.Severity
		e.Priority = finding.Priority
		if exists {
			e.PrevSeverity = store.FindingSeverity(prevSeverity)
			e.PrevPriority = prevPriority
		}
		if err := appendEvent(ctx, tx, &e); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit finding %s: %w", finding.ID, err)
	}
	return nil
}
