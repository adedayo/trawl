package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

func TestCorrelateHTTPXPopulatesSoftwareInventoryAndSupersedes(t *testing.T) {
	c := authorisedCore(t, []string{"example.com"}, nil)
	s := c.Store()
	ctx := context.Background()

	raw1 := []byte(`{
		"jobRunId": "job-1",
		"httpx": [{
			"url": "https://app.example.com",
			"host": "app.example.com",
			"status_code": 200,
			"tech": ["Nginx:1.18.0", "WordPress", "Cloudflare"]
		}]
	}`)
	if _, err := c.CorrelateIngest(ctx, IngestScan, raw1); err != nil {
		t.Fatalf("CorrelateIngest 1: %v", err)
	}

	assets, err := s.GetAssets(ctx, store.AssetStatusActive)
	if err != nil || len(assets) != 1 {
		t.Fatalf("GetAssets: %v, len=%d", err, len(assets))
	}
	asset := assets[0]

	// Compatibility: free-text metadata still carries tech.
	var meta map[string]any
	if err := json.Unmarshal([]byte(asset.Metadata), &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if _, ok := meta["tech"]; !ok {
		t.Fatal("asset.Metadata lost tech field")
	}

	cov, err := s.ComputeSoftwareCoverage(ctx, asset.ID)
	if err != nil {
		t.Fatalf("ComputeSoftwareCoverage: %v", err)
	}
	if cov.State != store.SoftwareCoverageIdentified || cov.Identified != 1 || cov.VersionUnknown != 1 || cov.Unmapped != 1 {
		t.Fatalf("coverage = %+v, want identified=1 versionUnknown=1 unmapped=1", cov)
	}

	// Second scan upgrades Nginx 1.18.0 -> 1.24.0.
	raw2 := []byte(`{
		"jobRunId": "job-2",
		"httpx": [{
			"url": "https://app.example.com",
			"host": "app.example.com",
			"status_code": 200,
			"tech": ["Nginx:1.24.0"]
		}]
	}`)
	if _, err := c.CorrelateIngest(ctx, IngestScan, raw2); err != nil {
		t.Fatalf("CorrelateIngest 2: %v", err)
	}

	sws, err := s.GetAssetSoftware(ctx, asset.ID)
	if err != nil {
		t.Fatalf("GetAssetSoftware: %v", err)
	}
	var foundCurrent124, foundSuperseded118 bool
	for _, sw := range sws {
		if sw.Vendor == "f5" && sw.Product == "nginx" && sw.Version == "1.24.0" && sw.Status == store.SoftwareCurrent {
			foundCurrent124 = true
		}
		if sw.Vendor == "f5" && sw.Product == "nginx" && sw.Version == "1.18.0" && sw.Status == store.SoftwareSuperseded {
			foundSuperseded118 = true
		}
	}
	if !foundCurrent124 || !foundSuperseded118 {
		t.Fatalf("expected nginx 1.24.0 current and 1.18.0 superseded, got %+v", sws)
	}
}

func TestHostBelongsToDomain(t *testing.T) {
	if !hostBelongsToDomain("example.com", "example.com") {
		t.Error("apex domain must match itself")
	}
	if !hostBelongsToDomain("API.Example.com.", "example.com") {
		t.Error("subdomain must match parent domain case-insensitively")
	}
	if hostBelongsToDomain("notexample.com", "example.com") {
		t.Error("unrelated suffix domain must not match")
	}
	if hostBelongsToDomain("other.org", "example.com") {
		t.Error("different domain must not match")
	}
}
