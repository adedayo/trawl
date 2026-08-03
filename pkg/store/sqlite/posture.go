package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) GetRegressions(ctx context.Context) ([]store.Regression, error) {
	query := `SELECT id, asset_id, attribute_type, previous_value, current_value, consecutive_fails, confirmed_at FROM regressions ORDER BY confirmed_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query regressions: %w", err)
	}
	defer rows.Close()

	var regressions []store.Regression
	for rows.Next() {
		var r store.Regression
		var confirmedAt string
		if err := rows.Scan(&r.ID, &r.AssetID, &r.AttributeType, &r.PreviousValue, &r.CurrentValue, &r.ConsecutiveFails, &confirmedAt); err != nil {
			return nil, fmt.Errorf("failed to scan regression: %w", err)
		}
		r.ConfirmedAt, _ = time.Parse(time.RFC3339, confirmedAt)
		regressions = append(regressions, r)
	}
	return regressions, nil
}

func (s *SQLiteStore) RecordPostureObservation(ctx context.Context, assetID string, attributeType string, value string) (*store.Regression, error) {
	nowStr := time.Now().Format(time.RFC3339)

	// Insert posture snapshot
	_, err := s.db.ExecContext(ctx, `INSERT INTO posture_snapshots (asset_id, attribute_type, value, observed_at) VALUES (?, ?, ?, ?)`,
		assetID, attributeType, value, nowStr)
	if err != nil {
		return nil, fmt.Errorf("failed to insert posture snapshot: %w", err)
	}

	// Fetch last 2 snapshots
	query := `SELECT value FROM posture_snapshots WHERE asset_id = ? AND attribute_type = ? ORDER BY id DESC LIMIT 2`
	rows, err := s.db.QueryContext(ctx, query, assetID, attributeType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent snapshots: %w", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err == nil {
			values = append(values, v)
		}
	}

	// If we have at least 2 consecutive observations and both differ from initial baseline
	if len(values) == 2 && values[0] != values[1] {
		regID := fmt.Sprintf("reg-%s-%s-%d", assetID, attributeType, time.Now().Unix())
		reg := &store.Regression{
			ID:               regID,
			AssetID:          assetID,
			AttributeType:    attributeType,
			PreviousValue:    values[1],
			CurrentValue:     values[0],
			ConsecutiveFails: 2,
			ConfirmedAt:      time.Now(),
		}

		insertReg := `
		INSERT INTO regressions (id, asset_id, attribute_type, previous_value, current_value, consecutive_fails, confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err := s.db.ExecContext(ctx, insertReg, reg.ID, reg.AssetID, reg.AttributeType, reg.PreviousValue, reg.CurrentValue, reg.ConsecutiveFails, nowStr)
		if err != nil {
			return nil, fmt.Errorf("failed to record confirmed regression: %w", err)
		}
		return reg, nil
	}

	return nil, nil
}

func (s *SQLiteStore) GetEmailPostures(ctx context.Context) ([]store.EmailPosture, error) {
	query := `SELECT domain, spf_valid, dkim_found, dmarc_policy, priority, last_checked FROM email_postures ORDER BY domain ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query email postures: %w", err)
	}
	defer rows.Close()

	var postures []store.EmailPosture
	for rows.Next() {
		var p store.EmailPosture
		var spfInt, dkimInt int
		var lastChecked string
		if err := rows.Scan(&p.Domain, &spfInt, &dkimInt, &p.DMARCPolicy, &p.Priority, &lastChecked); err != nil {
			return nil, fmt.Errorf("failed to scan email posture: %w", err)
		}
		p.SPFValid = spfInt == 1
		p.DKIMFound = dkimInt == 1
		p.LastChecked, _ = time.Parse(time.RFC3339, lastChecked)
		postures = append(postures, p)
	}
	return postures, nil
}

func (s *SQLiteStore) SaveEmailPosture(ctx context.Context, ep *store.EmailPosture) error {
	query := `
	INSERT INTO email_postures (domain, spf_valid, dkim_found, dmarc_policy, priority, last_checked)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(domain) DO UPDATE SET
		spf_valid = excluded.spf_valid,
		dkim_found = excluded.dkim_found,
		dmarc_policy = excluded.dmarc_policy,
		priority = excluded.priority,
		last_checked = excluded.last_checked
	`
	spfInt, dkimInt := 0, 0
	if ep.SPFValid {
		spfInt = 1
	}
	if ep.DKIMFound {
		dkimInt = 1
	}

	if ep.LastChecked.IsZero() {
		ep.LastChecked = time.Now()
	}

	_, err := s.db.ExecContext(ctx, query,
		ep.Domain,
		spfInt,
		dkimInt,
		ep.DMARCPolicy,
		ep.Priority,
		ep.LastChecked.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to save email posture: %w", err)
	}
	return nil
}
