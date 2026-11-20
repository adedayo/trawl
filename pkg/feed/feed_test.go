package feed

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func epssAPIServer(t *testing.T, scores map[string]string, requests *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked := r.URL.Query().Get("cve")
		*requests = append(*requests, asked)
		var rows []string
		for _, cve := range strings.Split(asked, ",") {
			if score, ok := scores[cve]; ok {
				rows = append(rows, `{"cve":"`+cve+`","epss":"`+score+`","percentile":"0.5","date":"2026-11-19"}`)
			}
		}
		_, _ = w.Write([]byte(`{"status":"OK","status-code":200,"data":[` + strings.Join(rows, ",") + `]}`))
	}))
}

func TestFetchEPSSReferencedAsksOnlyAboutTheGivenCVEs(t *testing.T) {
	var requests []string
	server := epssAPIServer(t, map[string]string{"CVE-2026-0001": "0.84", "CVE-2026-0003": "0.01"}, &requests)
	defer server.Close()
	client, err := NewClient(server.Client(), map[Name]string{EPSSAPI: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	res, err := client.FetchEPSSReferenced(context.Background(),
		[]string{"cve-2026-0003", "CVE-2026-0001", "CVE-2026-0001", "CVE-2026-0002", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0] != "CVE-2026-0001,CVE-2026-0002,CVE-2026-0003" {
		t.Fatalf("expected one request naming the sorted distinct CVEs, got %v", requests)
	}
	snap := res.Snapshot
	if snap.Feed != EPSS || snap.RecordCount != 2 || *snap.Records["CVE-2026-0001"].EPSS != 0.84 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if _, scored := snap.Records["CVE-2026-0002"]; scored {
		t.Fatal("a CVE the API did not return has no score and must not be given one")
	}
	if !snap.Queried["CVE-2026-0002"] || snap.Queried["CVE-2026-9999"] {
		t.Fatalf("the snapshot must remember exactly which CVEs it asked about: %v", snap.Queried)
	}
}

func TestFetchEPSSReferencedBatchesAndFailsWholeOnAnyFailedBatch(t *testing.T) {
	var requests []string
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Query().Get("cve"))
		if fail && len(requests) == 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"OK","data":[]}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.Client(), map[Name]string{EPSSAPI: server.URL})

	var cves []string
	for i := 0; i < epssBatchSize+10; i++ {
		cves = append(cves, fmt.Sprintf("CVE-2026-%04d", i))
	}
	if _, err := client.FetchEPSSReferenced(context.Background(), cves); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || strings.Count(requests[0], ",") != epssBatchSize-1 {
		t.Fatalf("expected two batches, the first of %d: %d requests", epssBatchSize, len(requests))
	}

	requests, fail = nil, true
	if res, err := client.FetchEPSSReferenced(context.Background(), cves); err == nil || res != nil {
		t.Fatalf("a failed batch must fail the fetch without a snapshot: %+v, %v", res, err)
	}
}

func TestFetchEPSSReferencedSnapshotIdentityFollowsWhatWasLearned(t *testing.T) {
	var requests []string
	scores := map[string]string{"CVE-2026-0001": "0.84"}
	server := epssAPIServer(t, scores, &requests)
	defer server.Close()
	client, _ := NewClient(server.Client(), map[Name]string{EPSSAPI: server.URL})
	ctx := context.Background()

	a, _ := client.FetchEPSSReferenced(ctx, []string{"CVE-2026-0001"})
	b, _ := client.FetchEPSSReferenced(ctx, []string{"CVE-2026-0001"})
	if a.Snapshot.ID != b.Snapshot.ID {
		t.Fatal("the same scores for the same CVEs must be the same snapshot")
	}
	scores["CVE-2026-0001"] = "0.90"
	c, _ := client.FetchEPSSReferenced(ctx, []string{"CVE-2026-0001"})
	if c.Snapshot.ID == a.Snapshot.ID {
		t.Fatal("a changed score must be a new snapshot")
	}
	d, _ := client.FetchEPSSReferenced(ctx, []string{"CVE-2026-0001", "CVE-2026-0002"})
	if d.Snapshot.ID == c.Snapshot.ID {
		t.Fatal("asking about another CVE must be a new snapshot, because it covers more")
	}

	before := len(requests)
	e1, _ := client.FetchEPSSReferenced(ctx, nil)
	e2, _ := client.FetchEPSSReferenced(ctx, nil)
	if len(requests) != before {
		t.Fatal("with nothing to look up no request should be made")
	}
	if e1.Snapshot.ID != e2.Snapshot.ID || e1.Snapshot.RecordCount != 0 {
		t.Fatalf("nothing to look up is one stable empty snapshot: %s vs %s", e1.Snapshot.ID, e2.Snapshot.ID)
	}
}

func TestFetchEPSSReferencedIgnoresRowsItWasNotAskedFor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"OK","data":[` +
			`{"cve":"CVE-2026-0001","epss":"0.5","date":"d"},{"cve":"CVE-2026-7777","epss":"0.9","date":"d"}]}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.Client(), map[Name]string{EPSSAPI: server.URL})

	res, err := client.FetchEPSSReferenced(context.Background(), []string{"CVE-2026-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := res.Snapshot.Records["CVE-2026-7777"]; present || res.Snapshot.RecordCount != 1 {
		t.Fatalf("a score for a CVE outside the question must not enter the snapshot: %+v", res.Snapshot.Records)
	}
}

func TestFetchEPSSReferencedRejectsAnErrorStatusAndABadScore(t *testing.T) {
	for name, body := range map[string]string{
		"status": `{"status":"ERROR","data":[]}`,
		"score":  `{"status":"OK","data":[{"cve":"CVE-2026-0001","epss":"1.7","date":"d"}]}`,
		"json":   `not json`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		client, _ := NewClient(server.Client(), map[Name]string{EPSSAPI: server.URL})
		if res, err := client.FetchEPSSReferenced(context.Background(), []string{"CVE-2026-0001"}); err == nil || res != nil {
			t.Errorf("%s: expected a refusal, got %+v", name, res)
		}
		server.Close()
	}
}
