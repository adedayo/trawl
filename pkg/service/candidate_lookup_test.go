package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/feed"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestCandidateLookupCachingResolutionAndKEVEPSS(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "cand_svc.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return t0 })
	ctx := context.Background()

	// Two assets running the same nginx 1.18.0 -> NVD must be asked only ONCE.
	for _, a := range []store.Asset{
		{ID: "asset-1", Type: store.AssetTypeSubdomain, Value: "a.example.com", Status: store.AssetStatusActive, DiscoverySource: "httpx", Confidence: 1, FirstSeen: t0, LastSeen: t0},
		{ID: "asset-2", Type: store.AssetTypeSubdomain, Value: "b.example.com", Status: store.AssetStatusActive, DiscoverySource: "httpx", Confidence: 1, FirstSeen: t0, LastSeen: t0},
	} {
		a := a
		if err := s.SaveAsset(ctx, &a); err != nil {
			t.Fatalf("SaveAsset %s: %v", a.ID, err)
		}
		if err := s.SaveAssetSoftware(ctx, &store.AssetSoftware{
			AssetID: a.ID, ObservedName: "Nginx", CPEPart: "a", Vendor: "f5", Product: "nginx",
			MappingState: store.SoftwareMapped, MappingVersion: "1.0.0",
			Version: "1.18.0", VersionState: store.SoftwareVersionKnown, Source: "httpx", RawString: "Nginx:1.18.0",
		}); err != nil {
			t.Fatalf("SaveAssetSoftware: %v", err)
		}
	}
	// Also add an unversioned WordPress and unmapped Cloudflare: neither may be queried.
	_ = s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: "asset-1", ObservedName: "WordPress", CPEPart: "a", Vendor: "wordpress", Product: "wordpress",
		MappingState: store.SoftwareMapped, MappingVersion: "1.0.0",
		VersionState: store.SoftwareVersionUnknown, Source: "httpx", RawString: "WordPress",
	})
	_ = s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: "asset-1", ObservedName: "Cloudflare",
		MappingState: store.SoftwareUnmapped, VersionState: store.SoftwareVersionUnknown,
		Source: "httpx", RawString: "Cloudflare",
	})

	nvdCalls := 0
	nvdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nvdCalls++
		vms := r.URL.Query().Get("virtualMatchString")
		if strings.Contains(vms, "wordpress") || strings.Contains(vms, "cloudflare") {
			t.Errorf("unversioned or unmapped software was queried: %s", vms)
		}
		if strings.Contains(vms, "1.18.0") {
			_, _ = w.Write([]byte(`{
				"resultsPerPage": 2000, "startIndex": 0, "totalResults": 1,
				"vulnerabilities": [{
					"cve": {
						"id": "CVE-2021-23017",
						"descriptions": [{"lang": "en", "value": "nginx resolver 1-byte overwrite"}],
						"metrics": {"cvssMetricV31": [{"type": "Primary", "cvssData": {"baseSeverity": "HIGH"}}]}
					}
				}]
			}`))
			return
		}
		// 1.24.0 has no matching CVEs in this test.
		_, _ = w.Write([]byte(`{"resultsPerPage": 2000, "startIndex": 0, "totalResults": 0, "vulnerabilities": []}`))
	}))
	defer nvdSrv.Close()

	client, err := feed.NewClient(nvdSrv.Client(), map[feed.Name]string{feed.NVD: nvdSrv.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.SetNVDRateInterval(0)

	svc := service.NewCandidateLookupService(s)
	sum1, err := svc.RefreshCandidates(ctx, client, false)
	if err != nil {
		t.Fatalf("RefreshCandidates 1: %v", err)
	}
	if nvdCalls != 1 {
		t.Fatalf("nvdCalls = %d, want 1 (deduplicated across 2 assets)", nvdCalls)
	}
	if sum1.Queried != 1 || sum1.Candidates != 2 || sum1.SkippedNoVer != 1 || sum1.SkippedUnmap != 1 {
		t.Fatalf("sum1 = %+v", sum1)
	}

	// Second refresh without changes -> 0 NVD requests (cache hit).
	sum2, err := svc.RefreshCandidates(ctx, client, false)
	if err != nil {
		t.Fatalf("RefreshCandidates 2: %v", err)
	}
	if nvdCalls != 1 || sum2.CacheHits != 1 || sum2.Queried != 0 {
		t.Fatalf("expected cache hit with 0 NVD calls, got nvdCalls=%d sum2=%+v", nvdCalls, sum2)
	}

	// Apply KEV snapshot containing CVE-2021-23017 -> priority elevates to critical!
	kevTrue := true
	feedSvc := service.NewFeedIngestService(s)
	if _, err := feedSvc.ApplySnapshot(ctx, &feed.Snapshot{
		ID: "kev-snap-1", Feed: feed.KEV, SourceURL: "https://example.test/kev",
		RetrievedAt: t0.Add(time.Hour), ContentDigest: "sha256:abc", RecordCount: 1,
		Records: map[string]feed.Record{"CVE-2021-23017": {CVE: "CVE-2021-23017", KEVListed: &kevTrue}},
	}, 0.5); err != nil {
		t.Fatalf("ApplySnapshot KEV: %v", err)
	}

	cands, err := s.GetCandidateFindings(ctx, "asset-1")
	if err != nil || len(cands) != 1 {
		t.Fatalf("GetCandidateFindings asset-1: %v, len=%d", err, len(cands))
	}
	if !cands[0].KEVListed || cands[0].Priority != "critical" {
		t.Fatalf("KEV candidate = %+v, want KEVListed=true Priority=critical", cands[0])
	}

	// Upgrade asset-1 from nginx 1.18.0 -> 1.24.0 and refresh -> candidate resolves!
	s.SetClock(func() time.Time { return t0.Add(2 * time.Hour) })
	if err := s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: "asset-1", ObservedName: "Nginx", CPEPart: "a", Vendor: "f5", Product: "nginx",
		MappingState: store.SoftwareMapped, MappingVersion: "1.0.0",
		Version: "1.24.0", VersionState: store.SoftwareVersionKnown, Source: "httpx", RawString: "Nginx:1.24.0",
	}); err != nil {
		t.Fatalf("SaveAssetSoftware 1.24.0: %v", err)
	}
	sum3, err := svc.RefreshCandidates(ctx, client, false)
	if err != nil {
		t.Fatalf("RefreshCandidates 3: %v", err)
	}
	if sum3.Resolved != 1 {
		t.Fatalf("sum3.Resolved = %d, want 1", sum3.Resolved)
	}
	candsAfter, _ := s.GetCandidateFindings(ctx, "asset-1")
	if len(candsAfter) != 1 || candsAfter[0].Status != store.FindingResolved {
		t.Fatalf("expected asset-1 candidate resolved, got %+v", candsAfter)
	}
}
