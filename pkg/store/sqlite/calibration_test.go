package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestCalibrationEvidenceRoundTripIsAppendOnly(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "calibration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	observedAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if err := s.SaveCalibrationObservation(ctx, &store.CalibrationObservation{
		ID: "observation-1", ExperimentID: "experiment-1", EndpointID: "endpoint-1",
		ServiceClass: "ssh", Transport: "tcp", Port: 22, ObservedAt: observedAt,
		EventType: "connection-attempt", SourceKey: "hmac-digest", Accepted: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCalibrationObservation(ctx, &store.CalibrationObservation{
		ID: "observation-2", ExperimentID: "experiment-1", EndpointID: "endpoint-1",
		ServiceClass: "ssh", Transport: "tcp", Port: 22, ObservedAt: observedAt.Add(time.Minute),
		EventType: "protocol-interaction", SourceKey: "hmac-digest", Accepted: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCalibrationReachability(ctx, &store.CalibrationReachability{
		ID: "interval-1", ExperimentID: "experiment-1", EndpointID: "endpoint-1",
		ServiceClass: "ssh", StartedAt: observedAt.Add(-24 * time.Hour), EndedAt: observedAt,
	}); err != nil {
		t.Fatal(err)
	}

	observations, err := s.GetCalibrationObservations(ctx, "experiment-1")
	if err != nil || len(observations) != 2 || observations[0].EventType != "connection-attempt" {
		t.Fatalf("observations = %+v, err = %v", observations, err)
	}
	intervals, err := s.GetCalibrationReachability(ctx, "experiment-1")
	if err != nil || len(intervals) != 1 || intervals[0].EndedAt != observedAt {
		t.Fatalf("intervals = %+v, err = %v", intervals, err)
	}
}
