package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

func TestFetchNVDBySoftwareSendsOnlyProductAndVersionAndFailsClosed(t *testing.T) {
	var seenQueries []string
	var seenAPIKey string
	failSecond := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQueries = append(seenQueries, r.URL.RawQuery)
		seenAPIKey = r.Header.Get("apiKey")

		// Egress invariant: only virtualMatchString, resultsPerPage, startIndex.
		for k := range r.URL.Query() {
			if k != "virtualMatchString" && k != "resultsPerPage" && k != "startIndex" {
				t.Errorf("unexpected query param %q sent to NVD", k)
			}
		}
		if strings.Contains(r.URL.RawQuery, "example.com") || strings.Contains(r.URL.RawQuery, "asset-") {
			t.Errorf("asset identifier leaked in NVD query: %s", r.URL.RawQuery)
		}

		vms := r.URL.Query().Get("virtualMatchString")
		if failSecond && strings.Contains(vms, "apache") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if strings.Contains(vms, "nginx") {
			_, _ = w.Write([]byte(`{
				"resultsPerPage": 2000,
				"startIndex": 0,
				"totalResults": 2,
				"vulnerabilities": [
					{
						"cve": {
							"id": "CVE-2021-23017",
							"published": "2021-06-01T13:15:07.853",
							"descriptions": [{"lang": "en", "value": "1-byte memory overwrite in nginx resolver."}],
							"metrics": {"cvssMetricV31": [{"type": "Primary", "cvssData": {"baseSeverity": "HIGH"}}]},
							"configurations": [{"nodes": [{"cpeMatch": [{"vulnerable": true, "criteria": "cpe:2.3:a:f5:nginx:*:*:*:*:*:*:*:*"}]}]}]
						}
					},
					{
						"cve": {
							"id": "CVE-2099-0001",
							"published": "2025-01-01T00:00:00.000",
							"descriptions": [{"lang": "en", "value": "Unrelated plugin running on nginx."}],
							"metrics": {"cvssMetricV31": [{"type": "Primary", "cvssData": {"baseSeverity": "CRITICAL"}}]},
							"configurations": [{"nodes": [{"cpeMatch": [{"vulnerable": false, "criteria": "cpe:2.3:a:f5:nginx:*:*:*:*:*:*:*:*"}]}]}]
						}
					}
				]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{"resultsPerPage": 2000, "startIndex": 0, "totalResults": 0, "vulnerabilities": []}`))
	}))
	defer srv.Close()

	client, err := NewClient(srv.Client(), map[Name]string{NVD: srv.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.SetNVDAPIKey("test-key-123")
	client.SetNVDRateInterval(0)

	// Duplicate queries must be deduplicated into one request.
	queries := []SoftwareQuery{
		{CPEPart: "a", Vendor: "f5", Product: "nginx", Version: "1.18.0"},
		{CPEPart: "a", Vendor: "f5", Product: "nginx", Version: "1.18.0"},
		{CPEPart: "a", Vendor: "apache", Product: "http_server", Version: "2.4.58"},
	}

	bundle, err := client.FetchNVDBySoftware(context.Background(), queries)
	if err != nil {
		t.Fatalf("FetchNVDBySoftware: %v", err)
	}
	if len(seenQueries) != 2 {
		t.Fatalf("got %d HTTP requests, want 2 deduplicated requests", len(seenQueries))
	}
	if seenAPIKey != "test-key-123" {
		t.Errorf("apiKey header = %q, want test-key-123", seenAPIKey)
	}

	nginxCVEs := bundle.ByKey["a:f5:nginx:1.18.0"]
	if len(nginxCVEs) != 1 || nginxCVEs[0].CVE != "CVE-2021-23017" || nginxCVEs[0].Severity != store.SeverityHigh {
		t.Fatalf("nginx CVEs = %+v, want only CVE-2021-23017 (HIGH), excluding vulnerable:false CVE-2099-0001", nginxCVEs)
	}
	if bundle.Snapshot == nil || !bundle.Snapshot.Queried["CVE-2021-23017"] {
		t.Fatalf("snapshot missing provenance or Queried set: %+v", bundle.Snapshot)
	}

	// Fail-closed: if one request fails, the whole lookup fails and returns nil.
	failSecond = true
	failedBundle, err := client.FetchNVDBySoftware(context.Background(), queries)
	if err == nil {
		t.Fatal("expected error when one NVD request fails")
	}
	if failedBundle != nil {
		t.Fatalf("expected nil bundle on partial failure, got %+v", failedBundle)
	}
}
