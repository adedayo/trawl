package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetSecretFindings(ctx context.Context, repoURL string) ([]store.SecretFinding, error) {
	query := `SELECT id, asset_id, repo_url, rule_id, secret_type, redacted_ref, file_path, start_line, verified, is_reused, first_seen, last_seen, COALESCE(status,'open'), resolved_at FROM secret_findings`
	var rows *sql.Rows
	var err error

	if repoURL != "" {
		query += ` WHERE repo_url = ? ORDER BY first_seen DESC`
		rows, err = s.reader().QueryContext(ctx, query, repoURL)
	} else {
		query += ` ORDER BY first_seen DESC`
		rows, err = s.reader().QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query secret findings: %w", err)
	}
	defer rows.Close()

	sfs := []store.SecretFinding{}
	for rows.Next() {
		var sf store.SecretFinding
		var verifiedInt, isReusedInt int
		var firstSeen string
		var lastSeen, resolvedAt sql.NullString
		if err := rows.Scan(&sf.ID, &sf.AssetID, &sf.RepoURL, &sf.RuleID, &sf.SecretType, &sf.RedactedRef, &sf.FilePath, &sf.StartLine, &verifiedInt, &isReusedInt, &firstSeen, &lastSeen, &sf.Status, &resolvedAt); err != nil {
			return nil, fmt.Errorf("failed to scan secret finding: %w", err)
		}
		sf.Verified = verifiedInt == 1
		sf.IsReused = isReusedInt == 1
		if sf.FirstSeen, err = parseTS("secret first_seen", firstSeen); err != nil {
			return nil, err
		}
		// Rows written before last_seen existed fall back to first_seen: the
		// only thing known is that it was seen then.
		sf.LastSeen = sf.FirstSeen
		if lastSeen.Valid && lastSeen.String != "" {
			if sf.LastSeen, err = parseTS("secret last_seen", lastSeen.String); err != nil {
				return nil, err
			}
		}
		if sf.ResolvedAt, err = parseTSPtr("secret resolved_at", resolvedAt); err != nil {
			return nil, err
		}
		sfs = append(sfs, sf)
	}

	return sfs, rows.Err()
}

// SaveSecretFinding upserts a secret and records the event the write amounts
// to, in the same transaction: first seen, a change in whether it verified as
// live, or reopening after it had resolved.
func (s *SQLiteStore) SaveSecretFinding(ctx context.Context, sf *store.SecretFinding) error {
	now := s.now()
	if sf.FirstSeen.IsZero() {
		sf.FirstSeen = now
	}
	vInt, rInt := 0, 0
	if sf.Verified {
		vInt = 1
	}
	if sf.IsReused {
		rInt = 1
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin secret save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prevStatus string
	var prevVerified int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(status,'open'), verified FROM secret_findings WHERE id = ?`, sf.ID).
		Scan(&prevStatus, &prevVerified)
	exists := err == nil
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read secret finding %s: %w", sf.ID, err)
	}

	var evType store.EventType
	status := store.FindingOpen
	switch {
	case !exists:
		evType = store.EventFirstSeen
	case prevStatus == string(store.FindingResolved):
		evType = store.EventReopened
		status = store.FindingReopened
	case prevVerified != vInt:
		evType = store.EventChanged
		status = store.FindingStatus(prevStatus)
	default:
		status = store.FindingStatus(prevStatus)
	}
	sf.Status = status

	_, err = tx.ExecContext(ctx, `
	INSERT INTO secret_findings (id, asset_id, repo_url, rule_id, secret_type, redacted_ref, file_path, start_line, verified, is_reused, first_seen, last_seen, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		start_line = excluded.start_line,
		verified = excluded.verified,
		is_reused = excluded.is_reused,
		last_seen = excluded.last_seen,
		status = excluded.status,
		resolved_at = NULL
	`,
		sf.ID, sf.AssetID, sf.RepoURL, sf.RuleID, sf.SecretType, sf.RedactedRef, sf.FilePath, sf.StartLine,
		vInt, rInt, formatTS(sf.FirstSeen), formatTS(now), string(status))
	if err != nil {
		return fmt.Errorf("failed to save secret finding: %w", err)
	}

	if evType != "" {
		e := s.newEvent(ctx, store.SourceSecret, sf.ID, sf.AssetID, evType)
		e.Severity = store.SecretSeverity(sf.Verified)
		e.Priority = string(e.Severity)
		if exists {
			e.PrevSeverity = store.SecretSeverity(prevVerified == 1)
			e.PrevPriority = string(e.PrevSeverity)
		}
		if err := appendEvent(ctx, tx, &e); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit secret finding %s: %w", sf.ID, err)
	}
	return nil
}
