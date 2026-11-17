package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/store"
)

// CalibrationIngestService is the narrow bridge from the collector contract
// to durable experiment evidence. It has no network access and cannot write
// assets, findings, or contact posteriors.
type CalibrationIngestService struct {
	store store.CalibrationStore
}

func newCalibrationPersistenceID() string {
	return uuid.NewString()
}

func NewCalibrationIngestService(s store.CalibrationStore) *CalibrationIngestService {
	return &CalibrationIngestService{store: s}
}

func (s *CalibrationIngestService) SaveObservation(ctx context.Context, observation contact.CalibrationObservation) error {
	if err := observation.Validate(); err != nil {
		return fmt.Errorf("calibration observation: %w", err)
	}
	return s.store.SaveCalibrationObservation(ctx, &store.CalibrationObservation{
		ID: observation.ID, ExperimentID: observation.ExperimentID,
		EndpointID: observation.EndpointID, ServiceClass: string(observation.ServiceClass),
		Transport: observation.Transport, Port: observation.Port,
		ObservedAt: observation.ObservedAt, EventType: string(observation.EventType),
		SourceKey: string(observation.SourceKey), Accepted: observation.Accepted,
		ProtocolIdentified: observation.ProtocolIdentified,
	})
}

func (s *CalibrationIngestService) SaveReachability(ctx context.Context, interval contact.ReachabilityInterval) error {
	if err := interval.Validate(); err != nil {
		return fmt.Errorf("calibration reachability: %w", err)
	}
	return s.store.SaveCalibrationReachability(ctx, &store.CalibrationReachability{
		ID: newCalibrationPersistenceID(), ExperimentID: interval.ExperimentID,
		EndpointID: interval.EndpointID, ServiceClass: string(interval.ServiceClass),
		StartedAt: interval.StartedAt, EndedAt: interval.EndedAt,
	})
}
