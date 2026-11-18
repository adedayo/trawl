package feed

import (
	"bytes"
	"compress/gzip"
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

func TestParseEPSSSupportsGzipAndHeaderComments(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte("#model_version:v2025.03.14,score_date:2026-10-01T00:00:00+0000\ncve,epss,percentile\nCVE-2026-1234,0.84,0.99\n"))
	_ = gz.Close()

	epss, err := Parse(EPSS, "https://epss.empiricalsecurity.com/epss_scores-current.csv.gz", time.Unix(1, 0), buf.Bytes())
	if err != nil {
		t.Fatalf("Parse gzipped EPSS failed: %v", err)
	}
	if epss.RecordCount != 1 || epss.Records["CVE-2026-1234"].EPSS == nil || *epss.Records["CVE-2026-1234"].EPSS != 0.84 {
		t.Fatalf("unexpected gzipped EPSS snapshot: %+v", epss)
	}
}

func TestFetchNVDReferencedQueriesOnlyDistinctReferencedCVEsAndFailsOnPartialError(t *testing.T) {
	var queried []string
	failSecond := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cve := r.URL.Query().Get("cveId")
		queried = append(queried, cve)
		if failSecond && cve == "CVE-2026-0002" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"vulnerabilities":[{"cve":{"id":"` + cve + `"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), map[Name]string{NVD: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	res, err := client.FetchNVDReferenced(context.Background(), []string{"cve-2026-0002", "CVE-2026-0001", "CVE-2026-0001", ""})
	if err != nil {
		t.Fatalf("FetchNVDReferenced: %v", err)
	}
	if len(queried) != 2 || queried[0] != "CVE-2026-0001" || queried[1] != "CVE-2026-0002" {
		t.Fatalf("expected sorted distinct referenced CVEs only, queried=%v", queried)
	}
	if res.Snapshot == nil || res.Snapshot.RecordCount != 2 {
		t.Fatalf("unexpected NVD snapshot: %+v", res.Snapshot)
	}

	failSecond = true
	if partial, err := client.FetchNVDReferenced(context.Background(), []string{"CVE-2026-0001", "CVE-2026-0002"}); err == nil || partial != nil {
		t.Fatalf("expected partial NVD fetch to fail without returning a snapshot, got %+v, %v", partial, err)
	}
}
