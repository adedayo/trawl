package store

import (
	"context"
	"time"
)

// ServiceObservation is one timestamped reachability result from Vantage.
// Rows are append-only so a short-lived exposure remains visible after it
// closes; exposure history is derived separately from this evidence.
type ServiceObservation struct {
	ID         string        `json:"id"`
	AssetID    string        `json:"assetId"`
	Host       string        `json:"host"`
	Port       uint16        `json:"port"`
	Service    string        `json:"service"`
	Transport  string        `json:"transport"`
	Protocol   string        `json:"protocol"`
	Layer      string        `json:"layer"`
	State      string        `json:"state"`
	Coverage   CoverageState `json:"coverage"`
	Evidence   string        `json:"evidence,omitempty"`
	Profile    string        `json:"profile"`
	ObservedAt time.Time     `json:"observedAt"`
	FirstSeen  time.Time     `json:"firstSeen"`
	LastSeen   time.Time     `json:"lastSeen"`
}

// ServiceObservationStore persists raw service evidence independently of
// derived exposure history.
type ServiceObservationStore interface {
	SaveServiceObservation(context.Context, *ServiceObservation) error
	GetServiceObservations(context.Context, string) ([]ServiceObservation, error)
}
