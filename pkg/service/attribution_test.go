package service_test

import (
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

// TestAttributionReachesTheView is the end-to-end check that the enrichment is
// readable. Stored rows nobody assembles into the view are a migration with no
// feature attached.
func TestAttributionReachesTheView(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)
	seedAsset(t, s, ctx, "asset-1", "example.com")

	observed := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	if err := s.ReplaceAssetAttribution(ctx, "asset-1",
		[]store.AssetAttribution{{
			AssetID:      "asset-1",
			Host:         "www.example.com",
			Role:         "host",
			Address:      "203.0.113.10",
			Provider:     "aws",
			Region:       "eu-west-2",
			Jurisdiction: "GB",
			ObservedAt:   observed,
		}},
		[]store.AttributionProvenance{{
			AssetID:   "asset-1",
			Provider:  "aws",
			URL:       "https://aws.invalid/ranges",
			FetchedAt: observed.Add(-24 * time.Hour),
		}},
	); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	view, err := svc.View(ctx, "asset-1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}

	if len(view.Attribution) != 1 {
		t.Fatalf("attribution must reach the view; got %d rows", len(view.Attribution))
	}
	if view.Attribution[0].Jurisdiction != "GB" {
		t.Fatalf("jurisdiction did not survive: %+v", view.Attribution[0])
	}
	if len(view.AttributionProvenance) != 1 {
		t.Fatal("the basis must travel with the attribution; shown alone, attribution reads as current when it may be days old")
	}
}

// TestAViewWithoutAttributionCarriesEmptyLists keeps the transport honest. A
// null in the payload leaves a consumer to decide whether it means "none" or
// "not supplied", and those are different claims about the estate.
func TestAViewWithoutAttributionCarriesEmptyLists(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)
	seedAsset(t, s, ctx, "asset-1", "example.com")

	view, err := svc.View(ctx, "asset-1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if view.Attribution == nil || view.AttributionProvenance == nil {
		t.Fatal("attribution lists must be empty rather than nil, so the encoded payload says 'none' rather than 'null'")
	}
}

// TestThePortfolioViewKeepsAttributionWithItsOwnAsset guards the batch path,
// where everything is read in one query and grouped in memory. Attribution
// grouped onto the wrong asset would tell an operator a domain is hosted
// somewhere it is not.
func TestThePortfolioViewKeepsAttributionWithItsOwnAsset(t *testing.T) {
	svc, s, ctx := newAssessmentService(t)
	seedAsset(t, s, ctx, "asset-1", "example.com")
	seedAsset(t, s, ctx, "asset-2", "other.example")

	now := time.Now().UTC()
	if err := s.ReplaceAssetAttribution(ctx, "asset-1",
		[]store.AssetAttribution{{AssetID: "asset-1", Host: "www.example.com", Address: "203.0.113.1", Provider: "aws", ObservedAt: now}}, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}
	if err := s.ReplaceAssetAttribution(ctx, "asset-2",
		[]store.AssetAttribution{{AssetID: "asset-2", Host: "www.other.example", Address: "203.0.113.2", Provider: "gcp", ObservedAt: now}}, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	views, err := svc.Views(ctx)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}

	seen := 0
	for _, v := range views {
		for _, a := range v.Attribution {
			seen++
			if a.AssetID != v.AssetID {
				t.Fatalf("attribution for %s appeared under %s: an operator would be told a domain is hosted somewhere it is not", a.AssetID, v.AssetID)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("both assets' attribution must appear in the portfolio view; saw %d rows", seen)
	}
}
