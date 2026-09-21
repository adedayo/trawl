package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestApplySnapshotReevaluatesFindingWithoutTouchingLastSeen(t *testing.T) {
	db, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "feed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	observedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveAsset(ctx, &store.Asset{
		ID: "asset-1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "test",
		FirstSeen: observedAt, LastSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFinding(ctx, &store.Finding{
		ID: "finding-1", AssetID: "asset-1", Title: "CVE finding",
		CVE: "CVE-2026-0001", Category: "vulnerability",
		FirstSeen: observedAt,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := db.GetFindings(ctx, "asset-1")
	if err != nil || len(before) != 1 {
		t.Fatalf("baseline finding: %+v, %v", before, err)
	}

	listed := true
	snapshot := &feed.Snapshot{
		ID: "cisa-kev-test", Feed: feed.KEV, SourceURL: "file:///kev.json",
		RetrievedAt: observedAt.Add(time.Hour), ContentDigest: "sha256:test",
		RecordCount: 1, Records: map[string]feed.Record{
			"CVE-2026-0001": {CVE: "CVE-2026-0001", KEVListed: &listed},
		},
	}
	regressions, err := NewFeedIngestService(db).ApplySnapshot(ctx, snapshot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(regressions) != 1 || regressions[0].AttributeType != "catalogue:cisa-kev:CVE-2026-0001" {
		t.Fatalf("unexpected regressions: %+v", regressions)
	}

	findings, err := db.GetFindings(ctx, "asset-1")
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings: %+v, %v", findings, err)
	}
	if !findings[0].KEVListed || !findings[0].LastSeen.Equal(before[0].LastSeen) {
		t.Fatalf("catalogue update changed finding incorrectly: %+v", findings[0])
	}
	if len(findings[0].Enrichments) != 3 {
		t.Fatalf("expected three enrichment states, got %+v", findings[0].Enrichments)
	}
	for _, enrichment := range findings[0].Enrichments {
		if enrichment.Feed == string(feed.KEV) && (enrichment.State != store.EnrichmentOK || enrichment.Snapshot == nil) {
			t.Fatalf("missing KEV provenance: %+v", enrichment)
		}
	}
}
