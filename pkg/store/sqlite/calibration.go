package sqlite

import (
	"context"
	"fmt"

	"github.com/adedayo/trawl/pkg/store"
)

func (s *SQLiteStore) SaveCalibrationObservation(ctx context.Context, observation *store.CalibrationObservation) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO calibration_observations (
			id, experiment_id, endpoint_id, service_class, transport, port, observed_at,
			event_type, source_key, accepted, protocol_identified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		observation.ID, observation.ExperimentID, observation.EndpointID,
		observation.ServiceClass, observation.Transport, observation.Port,
		observation.ObservedAt, observation.EventType, observation.SourceKey,
		boolInt(observation.Accepted), boolInt(observation.ProtocolIdentified))
	if err != nil {
		return fmt.Errorf("failed to save calibration observation: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetCalibrationObservations(ctx context.Context, experimentID string) ([]store.CalibrationObservation, error) {
	query := `SELECT id, experiment_id, endpoint_id, service_class, transport, port,
		observed_at, event_type, source_key, accepted, protocol_identified
		FROM calibration_observations`
	args := []any{}
	if experimentID != "" {
		query += ` WHERE experiment_id = ?`
		args = append(args, experimentID)
	}
	query += ` ORDER BY observed_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query calibration observations: %w", err)
	}
	defer rows.Close()

	observations := []store.CalibrationObservation{}
	for rows.Next() {
		var observation store.CalibrationObservation
		var accepted, protocolIdentified int
		if err := rows.Scan(&observation.ID, &observation.ExperimentID, &observation.EndpointID,
			&observation.ServiceClass, &observation.Transport, &observation.Port,
			&observation.ObservedAt, &observation.EventType, &observation.SourceKey,
			&accepted, &protocolIdentified); err != nil {
			return nil, fmt.Errorf("failed to scan calibration observation: %w", err)
		}
		observation.Accepted = accepted != 0
		observation.ProtocolIdentified = protocolIdentified != 0
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading calibration observations: %w", err)
	}
	return observations, nil
}

func (s *SQLiteStore) SaveCalibrationReachability(ctx context.Context, reachability *store.CalibrationReachability) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO calibration_reachability (
			id, experiment_id, endpoint_id, service_class, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		reachability.ID, reachability.ExperimentID, reachability.EndpointID,
		reachability.ServiceClass, reachability.StartedAt, reachability.EndedAt)
	if err != nil {
		return fmt.Errorf("failed to save calibration reachability: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetCalibrationReachability(ctx context.Context, experimentID string) ([]store.CalibrationReachability, error) {
	query := `SELECT id, experiment_id, endpoint_id, service_class, started_at, ended_at
		FROM calibration_reachability`
	args := []any{}
	if experimentID != "" {
		query += ` WHERE experiment_id = ?`
		args = append(args, experimentID)
	}
	query += ` ORDER BY started_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query calibration reachability: %w", err)
	}
	defer rows.Close()

	intervals := []store.CalibrationReachability{}
	for rows.Next() {
		var interval store.CalibrationReachability
		if err := rows.Scan(&interval.ID, &interval.ExperimentID, &interval.EndpointID,
			&interval.ServiceClass, &interval.StartedAt, &interval.EndedAt); err != nil {
			return nil, fmt.Errorf("failed to scan calibration reachability: %w", err)
		}
		intervals = append(intervals, interval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading calibration reachability: %w", err)
	}
	return intervals, nil
}
