package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestServiceObservationsAreAppendOnlyAndOrdered(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "services.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.SaveAsset(ctx, &store.Asset{ID: "asset-1", Type: store.AssetTypeDomain,
		Value: "example.com", Status: store.AssetStatusActive, DiscoverySource: "test"}); err != nil {
		t.Fatal(err)
	}
	first := &store.ServiceObservation{
		AssetID: "asset-1", Host: "example.com", Port: 443, Service: "https",
		Transport: "tcp", Protocol: "https", Layer: "http", State: "responding",
		Coverage: store.CoverageOK, Profile: "most-common",
		ObservedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), Evidence: "status=200",
	}
	second := *first
	second.ObservedAt = first.ObservedAt.Add(time.Hour)
	second.State = "not_responding"
	second.Evidence = "tcp_refused"
	if err := s.SaveServiceObservation(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveServiceObservation(ctx, &second); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetServiceObservations(ctx, "asset-1")
	if err != nil || len(got) != 2 {
		t.Fatalf("observations = %+v, err = %v", got, err)
	}
	if got[0].State != "responding" || got[1].State != "not_responding" {
		t.Fatalf("observation history was not preserved: %+v", got)
	}
}
