package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

func attributionFixture(t *testing.T) (*SQLiteStore, string, context.Context) {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()

	asset := &store.Asset{
		ID:     "asset-1",
		Type:   store.AssetTypeDomain,
		Value:  "example.com",
		Status: store.AssetStatusActive,
	}
	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}
	return s, asset.ID, ctx
}

func row(assetID, host, address, provider string) store.AssetAttribution {
	return store.AssetAttribution{
		AssetID:    assetID,
		Host:       host,
		Role:       "host",
		Address:    address,
		Provider:   provider,
		ObservedAt: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
	}
}

// TestAttributionIsReplacedNotMerged is the reason this is a replace and not
// an upsert. Addresses the estate has stopped using must not linger beside
// ones it currently uses: an operator reading the provider list would
// otherwise be shown the union of every run ever performed.
func TestAttributionIsReplacedNotMerged(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	first := []store.AssetAttribution{
		row(assetID, "www.example.com", "203.0.113.10", "aws"),
		row(assetID, "old.example.com", "198.51.100.5", "azure"),
	}
	if err := s.ReplaceAssetAttribution(ctx, assetID, first, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	second := []store.AssetAttribution{row(assetID, "www.example.com", "203.0.113.10", "aws")}
	if err := s.ReplaceAssetAttribution(ctx, assetID, second, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	got, err := s.GetAssetAttribution(ctx, assetID)
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("an address the estate no longer uses must not survive the replacement; got %d rows", len(got))
	}
	if got[0].Host != "www.example.com" {
		t.Fatalf("wrong row survived: %+v", got[0])
	}
}

// TestReplacingWithNothingRecordsThatNothingWasAttributed pins the meaning of
// an empty write. It is a claim — "we looked and matched nothing" — which is
// why the service only makes it when the check actually ran.
func TestReplacingWithNothingRecordsThatNothingWasAttributed(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	if err := s.ReplaceAssetAttribution(ctx, assetID, []store.AssetAttribution{row(assetID, "www.example.com", "203.0.113.10", "aws")}, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}
	if err := s.ReplaceAssetAttribution(ctx, assetID, nil, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	got, err := s.GetAssetAttribution(ctx, assetID)
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("an empty replacement must clear the previous snapshot; got %d rows", len(got))
	}
	if got == nil {
		t.Fatal("the result must be an empty slice, never nil: a consumer reading null cannot tell 'none' from 'not supplied'")
	}
}

// TestSeveralAddressesForOneNameAllSurvive keeps the disagreement visible. A
// name balanced across two providers is a fact an operator needs, not a
// discrepancy to be reduced to whichever address was returned first.
func TestSeveralAddressesForOneNameAllSurvive(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	rows := []store.AssetAttribution{
		row(assetID, "www.example.com", "203.0.113.10", "aws"),
		row(assetID, "www.example.com", "198.51.100.7", "gcp"),
	}
	if err := s.ReplaceAssetAttribution(ctx, assetID, rows, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	got, err := s.GetAssetAttribution(ctx, assetID)
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("both addresses of a name must be kept; got %d", len(got))
	}
}

// TestUnattributedAddressesSortLast keeps the settled attributions in front.
// An empty provider is the row a reader should trust least, since it may mean
// the address matched nothing or that the ranges never loaded.
func TestUnattributedAddressesSortLast(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	rows := []store.AssetAttribution{
		row(assetID, "a.example.com", "203.0.113.1", ""),
		row(assetID, "b.example.com", "203.0.113.2", "aws"),
	}
	if err := s.ReplaceAssetAttribution(ctx, assetID, rows, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	got, err := s.GetAssetAttribution(ctx, assetID)
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(got) != 2 || got[0].Provider != "aws" {
		t.Fatalf("attributed addresses must lead; got %+v", got)
	}
	if got[1].Attributed() {
		t.Fatal("the unattributed row must report itself as such")
	}
}

// TestProvenanceIsStoredOldestFirst puts the weakest basis where a reader
// meets it. A list led by this morning's fetch invites them to stop reading
// and conclude the whole attribution is fresh.
func TestProvenanceIsStoredOldestFirst(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	prov := []store.AttributionProvenance{
		{AssetID: assetID, Provider: "gcp", URL: "https://gcp.invalid/ranges", FetchedAt: now.Add(-1 * time.Hour)},
		{AssetID: assetID, Provider: "aws", URL: "https://aws.invalid/ranges", FetchedAt: now.Add(-72 * time.Hour)},
	}
	if err := s.ReplaceAssetAttribution(ctx, assetID, nil, prov); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	got, err := s.GetAttributionProvenance(ctx, assetID)
	if err != nil {
		t.Fatalf("GetAttributionProvenance: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("provenance rows = %d, want 2", len(got))
	}
	if got[0].Provider != "aws" {
		t.Fatalf("the oldest basis must come first; got %+v", got)
	}
}

// TestEveryAssetIsReadInOneQuery pins the portfolio convention: an empty asset
// id means every asset, as it does for coverage, so a portfolio view reads
// once rather than once per domain.
func TestEveryAssetIsReadInOneQuery(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	other := &store.Asset{ID: "asset-2", Type: store.AssetTypeDomain, Value: "other.example", Status: store.AssetStatusActive}
	if err := s.SaveAsset(ctx, other); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}
	if err := s.ReplaceAssetAttribution(ctx, assetID, []store.AssetAttribution{row(assetID, "a.example.com", "203.0.113.1", "aws")}, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}
	if err := s.ReplaceAssetAttribution(ctx, other.ID, []store.AssetAttribution{row(other.ID, "b.other.example", "203.0.113.2", "gcp")}, nil); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	all, err := s.GetAssetAttribution(ctx, "")
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("an empty asset id must return every asset's attribution; got %d", len(all))
	}

	one, err := s.GetAssetAttribution(ctx, assetID)
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(one) != 1 {
		t.Fatalf("a named asset must return only its own; got %d", len(one))
	}
}

// TestErasingDiscoveredDataClearsAttribution keeps the start-over promise
// honest. Attribution is something the engine observed about the estate, not
// configuration the operator set, so it must go with everything else.
func TestErasingDiscoveredDataClearsAttribution(t *testing.T) {
	s, assetID, ctx := attributionFixture(t)

	if err := s.ReplaceAssetAttribution(ctx, assetID,
		[]store.AssetAttribution{row(assetID, "www.example.com", "203.0.113.10", "aws")},
		[]store.AttributionProvenance{{AssetID: assetID, Provider: "aws", URL: "https://aws.invalid/ranges", FetchedAt: time.Now().UTC()}},
	); err != nil {
		t.Fatalf("ReplaceAssetAttribution: %v", err)
	}

	if err := s.EraseDiscoveredData(ctx); err != nil {
		t.Fatalf("EraseDiscoveredData: %v", err)
	}

	rows, err := s.GetAssetAttribution(ctx, "")
	if err != nil {
		t.Fatalf("GetAssetAttribution: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("attribution survived an erase; got %d rows", len(rows))
	}
	prov, err := s.GetAttributionProvenance(ctx, "")
	if err != nil {
		t.Fatalf("GetAttributionProvenance: %v", err)
	}
	if len(prov) != 0 {
		t.Fatalf("attribution provenance survived an erase; got %d rows", len(prov))
	}
}
