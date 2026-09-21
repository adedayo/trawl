package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestExposureHistoryRoundTripAndReplacement(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "contact.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.SaveAsset(ctx, &store.Asset{ID: "asset-1", Type: store.AssetTypeDomain,
		Value: "example.com", Status: store.AssetStatusActive, DiscoverySource: "test"}); err != nil {
		t.Fatal(err)
	}
	history := &store.AssetExposureHistory{
		AssetID: "asset-1", Service: "https:443", FirstObserved: "2026-01-01T00:00:00Z",
		LastObserved: "2026-01-02T00:00:00Z", StillExposed: true, LeftCensored: true,
		ObservedDurationSeconds: 3600, InferredDurationSeconds: 7200,
		BlindDurationSeconds: 1800, ExpectedBlindSeconds: 3600, WorstBlindSeconds: 7200,
	}
	if err := s.SaveExposureHistory(ctx, history); err != nil {
		t.Fatal(err)
	}
	history.InferredDurationSeconds = 9000
	if err := s.SaveExposureHistory(ctx, history); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetExposureHistory(ctx, "asset-1")
	if err != nil || len(got) != 1 || got[0].InferredDurationSeconds != 9000 || !got[0].StillExposed {
		t.Fatalf("unexpected exposure history: %+v, %v", got, err)
	}
}
