package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/adedayo/trawl/pkg/store"
)

// SaveServiceObservation appends one Vantage reachability result. Existing
// rows are never replaced because transient exposure is evidence in itself.
func (s *SQLiteStore) SaveServiceObservation(ctx context.Context, observation *store.ServiceObservation) error {
	now := time.Now().UTC()
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = now
	}
	if observation.FirstSeen.IsZero() {
		observation.FirstSeen = observation.ObservedAt
	}
	if observation.LastSeen.IsZero() {
		observation.LastSeen = observation.ObservedAt
	}
	if observation.ID == "" {
		observation.ID = "svc-" + uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO service_observations (
			id, asset_id, host, port, service, transport, protocol, layer, state,
			coverage, evidence, profile, observed_at, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		observation.ID, observation.AssetID, observation.Host, observation.Port,
		observation.Service, observation.Transport, observation.Protocol,
		observation.Layer, observation.State, string(observation.Coverage),
		observation.Evidence, observation.Profile, observation.ObservedAt.Format(time.RFC3339Nano),
		observation.FirstSeen.Format(time.RFC3339Nano), observation.LastSeen.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("failed to save service observation: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetServiceObservations(ctx context.Context, assetID string) ([]store.ServiceObservation, error) {
	query := `SELECT id, asset_id, host, port, service, transport, protocol, layer,
		state, coverage, evidence, profile, observed_at, first_seen, last_seen
		FROM service_observations`
	args := []any{}
	if assetID != "" {
		query += ` WHERE asset_id = ?`
		args = append(args, assetID)
	}
	query += ` ORDER BY observed_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query service observations: %w", err)
	}
	defer rows.Close()

	var observations []store.ServiceObservation
	for rows.Next() {
		var observation store.ServiceObservation
		var coverage, observedAt, firstSeen, lastSeen string
		if err := rows.Scan(&observation.ID, &observation.AssetID, &observation.Host,
			&observation.Port, &observation.Service, &observation.Transport,
			&observation.Protocol, &observation.Layer, &observation.State,
			&coverage, &observation.Evidence, &observation.Profile,
			&observedAt, &firstSeen, &lastSeen); err != nil {
			return nil, fmt.Errorf("failed to scan service observation: %w", err)
		}
		observation.Coverage = store.CoverageState(coverage)
		observation.ObservedAt, _ = time.Parse(time.RFC3339Nano, observedAt)
		observation.FirstSeen, _ = time.Parse(time.RFC3339Nano, firstSeen)
		observation.LastSeen, _ = time.Parse(time.RFC3339Nano, lastSeen)
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading service observations: %w", err)
	}
	return observations, nil
}
