package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestCalibrationExperimentsRoundTrip(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "experiments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	experiment := &store.CalibrationExperiment{ID: "experiment-1", Label: "SSH September", CreatedAt: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)}
	if err := s.CreateCalibrationExperiment(context.Background(), experiment); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCalibrationExperiments(context.Background())
	if err != nil || len(got) != 1 || got[0].ID != experiment.ID || got[0].Label != experiment.Label {
		t.Fatalf("experiments=%+v err=%v", got, err)
	}
}
