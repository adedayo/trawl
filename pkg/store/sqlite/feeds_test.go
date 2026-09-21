package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestFeedSnapshotsAndEnrichmentsRoundTrip(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	retrievedAt := time.Date(2026, 9, 21, 4, 0, 0, 0, time.UTC)
	snapshot := &store.FeedSnapshot{
		ID: "kev-2026-09-21", Feed: "cisa-kev", SourceURL: "file:///tmp/kev.json",
		RetrievedAt: retrievedAt, ContentDigest: "sha256:test", RecordCount: 1,
	}
	if err := s.SaveFeedSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	gotSnapshot, err := s.GetFeedSnapshot(ctx, snapshot.ID)
	if err != nil || gotSnapshot == nil || !gotSnapshot.RetrievedAt.Equal(retrievedAt) {
		t.Fatalf("snapshot round trip failed: got=%+v err=%v", gotSnapshot, err)
	}

	kev := true
	checkedAt := retrievedAt.Add(time.Minute)
	enrichment := &store.FindingEnrichment{
		FindingID: "finding-1", Feed: "cisa-kev", CVE: "CVE-2026-0001",
		State: store.EnrichmentOK, SnapshotID: snapshot.ID, KEVListed: &kev,
		CheckedAt: &checkedAt,
	}
	if err := s.SaveFindingEnrichment(ctx, enrichment); err == nil {
		t.Fatal("expected foreign-key failure for an unknown finding")
	}

	asset := &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "test",
		FirstSeen: retrievedAt, LastSeen: retrievedAt,
	}
	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFinding(ctx, &store.Finding{
		ID: "finding-1", AssetID: asset.ID, Title: "test finding",
		Severity: store.SeverityHigh, Priority: "high", CVE: enrichment.CVE,
		Category: "test", FirstSeen: retrievedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFindingEnrichment(ctx, enrichment); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFindingEnrichments(ctx, enrichment.FindingID)
	if err != nil || len(got) != 3 {
		t.Fatalf("enrichment round trip failed: got=%+v err=%v", got, err)
	}
	var found bool
	for _, item := range got {
		if item.Feed == store.FeedKEV {
			found = item.State == store.EnrichmentOK && item.SnapshotID == snapshot.ID &&
				item.KEVListed != nil && *item.KEVListed
		}
	}
	if !found {
		t.Fatalf("unexpected KEV enrichment: %+v", got)
	}
}
