package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
)

type calibrationStoreStub struct {
	observation  *store.CalibrationObservation
	reachability *store.CalibrationReachability
}

func (s *calibrationStoreStub) CreateCalibrationExperiment(context.Context, *store.CalibrationExperiment) error {
	return nil
}

func (s *calibrationStoreStub) GetCalibrationExperiments(context.Context) ([]store.CalibrationExperiment, error) {
	return nil, nil
}

func (s *calibrationStoreStub) SaveCalibrationObservation(_ context.Context, observation *store.CalibrationObservation) error {
	s.observation = observation
	return nil
}

func (s *calibrationStoreStub) GetCalibrationObservations(context.Context, string) ([]store.CalibrationObservation, error) {
	return nil, nil
}

func (s *calibrationStoreStub) SaveCalibrationReachability(_ context.Context, reachability *store.CalibrationReachability) error {
	s.reachability = reachability
	return nil
}

func (s *calibrationStoreStub) GetCalibrationReachability(context.Context, string) ([]store.CalibrationReachability, error) {
	return nil, nil
}

func TestCalibrationIngestMapsValidatedRecords(t *testing.T) {
	collector, err := contact.NewCalibrationCollector("experiment-1", "endpoint-1", contact.ServiceClassSSH, "tcp", 22, "key")
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	observation, err := collector.Observe("source", contact.ConnectionAttempt, observedAt, true, false)
	if err != nil {
		t.Fatal(err)
	}
	interval, err := collector.Reachability(observedAt.Add(-24*time.Hour), observedAt)
	if err != nil {
		t.Fatal(err)
	}

	stub := &calibrationStoreStub{}
	ingest := service.NewCalibrationIngestService(stub)
	if err := ingest.SaveObservation(context.Background(), observation); err != nil {
		t.Fatal(err)
	}
	if err := ingest.SaveReachability(context.Background(), interval); err != nil {
		t.Fatal(err)
	}
	if stub.observation == nil || stub.observation.SourceKey == "source" || stub.reachability == nil || stub.reachability.ExperimentID != "experiment-1" {
		t.Fatalf("ingest mapping lost evidence boundaries: observation=%+v reachability=%+v", stub.observation, stub.reachability)
	}
}
