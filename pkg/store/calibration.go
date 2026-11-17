package store

import (
	"context"
	"time"
)

type CalibrationExperiment struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}

// CalibrationObservation is raw, append-only evidence from an isolated
// ambient-contact experiment. It has no asset relationship and cannot update
// a contact posterior.
type CalibrationObservation struct {
	ID                 string    `json:"id"`
	ExperimentID       string    `json:"experimentId"`
	EndpointID         string    `json:"endpointId"`
	ServiceClass       string    `json:"serviceClass"`
	Transport          string    `json:"transport"`
	Port               uint16    `json:"port"`
	ObservedAt         time.Time `json:"observedAt"`
	EventType          string    `json:"eventType"`
	SourceKey          string    `json:"sourceKey"`
	Accepted           bool      `json:"accepted"`
	ProtocolIdentified bool      `json:"protocolIdentified"`
}

type CalibrationReachability struct {
	ID           string    `json:"id"`
	ExperimentID string    `json:"experimentId"`
	EndpointID   string    `json:"endpointId"`
	ServiceClass string    `json:"serviceClass"`
	StartedAt    time.Time `json:"startedAt"`
	EndedAt      time.Time `json:"endedAt"`
}

type CalibrationStore interface {
	CreateCalibrationExperiment(context.Context, *CalibrationExperiment) error
	GetCalibrationExperiments(context.Context) ([]CalibrationExperiment, error)
	SaveCalibrationObservation(context.Context, *CalibrationObservation) error
	GetCalibrationObservations(context.Context, string) ([]CalibrationObservation, error)
	SaveCalibrationReachability(context.Context, *CalibrationReachability) error
	GetCalibrationReachability(context.Context, string) ([]CalibrationReachability, error)
}
