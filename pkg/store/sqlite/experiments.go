package sqlite

import (
	"context"
	"fmt"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) CreateCalibrationExperiment(ctx context.Context, experiment *store.CalibrationExperiment) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO calibration_experiments (id, label, created_at) VALUES (?, ?, ?)`, experiment.ID, experiment.Label, experiment.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create calibration experiment: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetCalibrationExperiments(ctx context.Context) ([]store.CalibrationExperiment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, label, created_at FROM calibration_experiments ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to query calibration experiments: %w", err)
	}
	defer rows.Close()
	experiments := []store.CalibrationExperiment{}
	for rows.Next() {
		var experiment store.CalibrationExperiment
		if err := rows.Scan(&experiment.ID, &experiment.Label, &experiment.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan calibration experiment: %w", err)
		}
		experiments = append(experiments, experiment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading calibration experiments: %w", err)
	}
	return experiments, nil
}
