package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetSecretFindings(ctx context.Context, repoURL string) ([]store.SecretFinding, error) {
	query := `SELECT id, asset_id, repo_url, rule_id, secret_type, redacted_ref, file_path, start_line, verified, is_reused, first_seen FROM secret_findings`
	var rows *sql.Rows
	var err error

	if repoURL != "" {
		query += ` WHERE repo_url = ? ORDER BY first_seen DESC`
		rows, err = s.db.QueryContext(ctx, query, repoURL)
	} else {
		query += ` ORDER BY first_seen DESC`
		rows, err = s.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query secret findings: %w", err)
	}
	defer rows.Close()

	var sfs []store.SecretFinding
	for rows.Next() {
		var sf store.SecretFinding
		var verifiedInt, isReusedInt int
		var firstSeen string
		if err := rows.Scan(&sf.ID, &sf.AssetID, &sf.RepoURL, &sf.RuleID, &sf.SecretType, &sf.RedactedRef, &sf.FilePath, &sf.StartLine, &verifiedInt, &isReusedInt, &firstSeen); err != nil {
			return nil, fmt.Errorf("failed to scan secret finding: %w", err)
		}
		sf.Verified = verifiedInt == 1
		sf.IsReused = isReusedInt == 1
		sf.FirstSeen, _ = time.Parse(time.RFC3339, firstSeen)
		sfs = append(sfs, sf)
	}

	return sfs, nil
}

func (s *SQLiteStore) SaveSecretFinding(ctx context.Context, sf *store.SecretFinding) error {
	query := `
	INSERT INTO secret_findings (id, asset_id, repo_url, rule_id, secret_type, redacted_ref, file_path, start_line, verified, is_reused, first_seen)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		verified = excluded.verified,
		is_reused = excluded.is_reused
	`
	vInt, rInt := 0, 0
	if sf.Verified {
		vInt = 1
	}
	if sf.IsReused {
		rInt = 1
	}

	if sf.FirstSeen.IsZero() {
		sf.FirstSeen = time.Now()
	}

	_, err := s.db.ExecContext(ctx, query,
		sf.ID,
		sf.AssetID,
		sf.RepoURL,
		sf.RuleID,
		sf.SecretType,
		sf.RedactedRef,
		sf.FilePath,
		sf.StartLine,
		vInt,
		rInt,
		sf.FirstSeen.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to save secret finding: %w", err)
	}
	return nil
}
