package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseCatalogueFormats(t *testing.T) {
	kev, err := Parse(KEV, "file:///kev.json", time.Unix(1, 0), []byte(`{"vulnerabilities":[{"cveID":"CVE-2026-0001"}]}`))
	if err != nil || kev.RecordCount != 1 || kev.Records["CVE-2026-0001"].KEVListed == nil {
		t.Fatalf("KEV parse failed: %+v, %v", kev, err)
	}

	epss, err := Parse(EPSS, "file:///epss.csv", time.Unix(1, 0), []byte("cve,epss,percentile\nCVE-2026-0001,0.73,0.91\n"))
	if err != nil || epss.Records["CVE-2026-0001"].EPSS == nil || *epss.Records["CVE-2026-0001"].EPSS != 0.73 {
		t.Fatalf("EPSS parse failed: %+v, %v", epss, err)
	}

	nvd, err := Parse(NVD, "file:///nvd.json", time.Unix(1, 0), []byte(`{"vulnerabilities":[{"cve":{"id":"CVE-2026-0001"}}]}`))
	if err != nil || nvd.RecordCount != 1 {
		t.Fatalf("NVD parse failed: %+v, %v", nvd, err)
	}
	if kev.ID == epss.ID || kev.ContentDigest == "" {
		t.Fatal("snapshots must be content-addressed")
	}
}

func TestClientUsesConfiguredEndpointAndConditionalRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(`{"vulnerabilities":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(nil, map[Name]string{KEV: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.Fetch(context.Background(), KEV)
	if err != nil || first.Snapshot == nil {
		t.Fatalf("first fetch failed: %+v, %v", first, err)
	}
	second, err := client.Fetch(context.Background(), KEV)
	if err != nil || !second.NotModified || requests.Load() != 2 {
		t.Fatalf("conditional fetch failed: %+v, requests=%d, err=%v", second, requests.Load(), err)
	}
	if _, err := client.Fetch(context.Background(), EPSS); err == nil {
		t.Fatal("unconfigured feed endpoint was accepted")
	}
}
