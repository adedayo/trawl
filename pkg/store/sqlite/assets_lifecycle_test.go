package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func lifecycleStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "assets.db"))
	if err != nil {
		t.Fatalf("Failed to open the store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedAsset(t *testing.T, s *sqlite.SQLiteStore, ctx context.Context, id, value string, status store.AssetStatus) {
	t.Helper()
	now := time.Now()
	if err := s.SaveAsset(ctx, &store.Asset{
		ID:              id,
		Type:            store.AssetTypeDomain,
		Value:           value,
		Status:          status,
		DiscoverySource: "manual",
		Confidence:      1,
		FirstSeen:       now,
		LastSeen:        now,
	}); err != nil {
		t.Fatalf("Failed to seed asset %s: %v", value, err)
	}
}

func TestDeleteAsset(t *testing.T) {
	ctx := context.Background()

	t.Run("it removes the asset and everything hanging off it", func(t *testing.T) {
		s := lifecycleStore(t)
		now := time.Now()
		seedAsset(t, s, ctx, "asset-1", "example.com", store.AssetStatusActive)

		if err := s.SaveFinding(ctx, &store.Finding{
			ID: "finding-1", AssetID: "asset-1", Title: "open port",
			Severity: "high", Priority: "high", Category: "network",
			FirstSeen: now, LastSeen: now,
		}); err != nil {
			t.Fatalf("Failed to seed the finding: %v", err)
		}
		if err := s.SaveSignalObservation(ctx, &store.SignalObservation{
			ID: "obs-1", AssetID: "asset-1", SignalID: "SURF-SPF-001", CheckID: "spf",
			State: "ok", Severity: "medium",
			ObservedAt: now, FirstSeen: now, LastSeen: now,
		}); err != nil {
			t.Fatalf("Failed to seed the observation: %v", err)
		}
		if err := s.RecordAssessmentRun(ctx, &store.AssessmentRun{
			AssetID: "asset-1", Outcome: "completed",
			StartedAt: now, FinishedAt: now,
		}); err != nil {
			t.Fatalf("Failed to seed the assessment run: %v", err)
		}

		if err := s.DeleteAsset(ctx, "asset-1"); err != nil {
			t.Fatalf("Failed to delete the asset: %v", err)
		}

		assets, err := s.GetAssets(ctx, "")
		if err != nil {
			t.Fatalf("Failed to read the assets back: %v", err)
		}
		if len(assets) != 0 {
			t.Errorf("The asset survived deletion: %d rows remain", len(assets))
		}

		findings, err := s.GetFindings(ctx, "")
		if err != nil {
			t.Fatalf("Failed to read the findings back: %v", err)
		}
		for _, f := range findings {
			if f.AssetID == "asset-1" {
				t.Errorf("A finding outlived its asset: %s", f.ID)
			}
		}

		obs, err := s.GetSignalObservations(ctx, "asset-1")
		if err != nil {
			t.Fatalf("Failed to read the observations back: %v", err)
		}
		if len(obs) != 0 {
			t.Errorf("%d observations outlived their asset", len(obs))
		}

		runs, err := s.GetAssessmentRuns(ctx, "")
		if err != nil {
			t.Fatalf("Failed to read the assessment runs back: %v", err)
		}
		if len(runs) != 0 {
			t.Errorf("%d assessment runs outlived their asset", len(runs))
		}
	})

	t.Run("it reports an asset that is not there", func(t *testing.T) {
		s := lifecycleStore(t)

		if err := s.DeleteAsset(ctx, "no-such-asset"); err == nil {
			t.Fatal("Deleting a missing asset succeeded; the operator would be told something was removed when nothing was")
		}
	})
}
