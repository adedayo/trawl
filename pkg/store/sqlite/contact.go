package sqlite

import (
	"context"
	"fmt"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) SaveExposureHistory(ctx context.Context, history *store.AssetExposureHistory) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO asset_exposure_history (
			asset_id, service, first_observed, last_observed, still_exposed, left_censored,
			observed_duration_seconds, inferred_duration_seconds, blind_duration_seconds,
			expected_blind_seconds, worst_blind_seconds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(asset_id, service) DO UPDATE SET
			first_observed = excluded.first_observed, last_observed = excluded.last_observed,
			still_exposed = excluded.still_exposed, left_censored = excluded.left_censored,
			observed_duration_seconds = excluded.observed_duration_seconds,
			inferred_duration_seconds = excluded.inferred_duration_seconds,
			blind_duration_seconds = excluded.blind_duration_seconds,
			expected_blind_seconds = excluded.expected_blind_seconds,
			worst_blind_seconds = excluded.worst_blind_seconds`,
		history.AssetID, history.Service, history.FirstObserved, history.LastObserved,
		boolInt(history.StillExposed), boolInt(history.LeftCensored),
		history.ObservedDurationSeconds, history.InferredDurationSeconds,
		history.BlindDurationSeconds, history.ExpectedBlindSeconds, history.WorstBlindSeconds)
	if err != nil {
		return fmt.Errorf("failed to save exposure history: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetExposureHistory(ctx context.Context, assetID string) ([]store.AssetExposureHistory, error) {
	query := `SELECT asset_id, service, first_observed, last_observed, still_exposed, left_censored,
		observed_duration_seconds, inferred_duration_seconds, blind_duration_seconds,
		expected_blind_seconds, worst_blind_seconds FROM asset_exposure_history`
	args := []any{}
	if assetID != "" {
		query += ` WHERE asset_id = ?`
		args = append(args, assetID)
	}
	query += ` ORDER BY asset_id, service`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query exposure history: %w", err)
	}
	defer rows.Close()

	var histories []store.AssetExposureHistory
	for rows.Next() {
		var history store.AssetExposureHistory
		var stillExposed, leftCensored int
		if err := rows.Scan(&history.AssetID, &history.Service, &history.FirstObserved,
			&history.LastObserved, &stillExposed, &leftCensored,
			&history.ObservedDurationSeconds, &history.InferredDurationSeconds,
			&history.BlindDurationSeconds, &history.ExpectedBlindSeconds,
			&history.WorstBlindSeconds); err != nil {
			return nil, fmt.Errorf("failed to scan exposure history: %w", err)
		}
		history.StillExposed = stillExposed != 0
		history.LeftCensored = leftCensored != 0
		histories = append(histories, history)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading exposure history: %w", err)
	}
	return histories, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
