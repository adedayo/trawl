package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

func post(t *testing.T, mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

// Refreshing one feed fetches that feed alone, stores what it fetched, and the
// status read back says when the source was last consulted. Asking again is the
// operator's "refresh if necessary": the second answer is a later check of the
// same catalogue, not a second snapshot.
func TestRefreshingOneFeedStoresItAndReportsWhenItWasLastChecked(t *testing.T) {
	mux, _ := findingsMux(t)

	fetched := 0
	kev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched++
		_, _ = w.Write([]byte(`{"vulnerabilities":[{"cveID":"CVE-2026-0001"},{"cveID":"CVE-2026-0002"}]}`))
	}))
	defer kev.Close()
	body := `{"feeds":["cisa-kev"],"endpoints":{"cisa-kev":"` + kev.URL + `"}}`

	var first, second struct {
		Feeds []store.FeedStatus `json:"feeds"`
		Error string             `json:"error"`
	}
	if rec := post(t, mux, "/api/v1/feeds/refresh", body); rec.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	} else if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}

	byFeed := map[string]store.FeedStatus{}
	for _, s := range first.Feeds {
		byFeed[s.Feed] = s
	}
	kevStatus := byFeed["cisa-kev"]
	if kevStatus.Snapshot == nil || kevStatus.Snapshot.RecordCount != 2 || !kevStatus.RecordsStored || kevStatus.LastCheckedAt == nil {
		t.Fatalf("the refreshed feed should be stored, with its entries and a last-checked time: %+v", kevStatus)
	}
	if byFeed["epss"].Snapshot != nil || byFeed["nvd"].Snapshot != nil {
		t.Fatalf("a single-feed refresh must not touch the others: %+v", byFeed)
	}

	if rec := post(t, mux, "/api/v1/feeds/refresh", body); rec.Code != http.StatusOK {
		t.Fatalf("second refresh: %d %s", rec.Code, rec.Body)
	} else if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if fetched != 2 {
		t.Fatalf("each refresh should consult the source, fetched %d times", fetched)
	}
	again := second.Feeds[0]
	if again.Snapshot == nil || again.Snapshot.ID != kevStatus.Snapshot.ID {
		t.Fatalf("unchanged content must stay one snapshot: %+v", again.Snapshot)
	}
	if again.LastCheckedAt == nil || again.LastCheckedAt.Before(*kevStatus.LastCheckedAt) {
		t.Fatalf("the second refresh should move the last-checked time forward: %v -> %v",
			kevStatus.LastCheckedAt, again.LastCheckedAt)
	}
}

func TestRefreshingAnUnknownFeedIsRefused(t *testing.T) {
	mux, _ := findingsMux(t)
	if rec := post(t, mux, "/api/v1/feeds/refresh", `{"feeds":["cisa-kve"]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a mistyped feed should be a 400, got %d %s", rec.Code, rec.Body)
	}
}

// Refreshing EPSS reaches FIRST's per-CVE API for the CVEs on stored findings
// and for nothing else, and works when only that endpoint is configured.
func TestRefreshingEPSSAsksOnlyAboutTheCVEsOnStoredFindings(t *testing.T) {
	mux, s := findingsMux(t)
	ctx := context.Background()
	if err := s.SaveAsset(ctx, &store.Asset{ID: "a1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "t", Confidence: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFinding(ctx, &store.Finding{ID: "f1", AssetID: "a1", Title: "t", Severity: store.SeverityHigh,
		CVE: "CVE-2026-0001", Category: "vulnerability"}); err != nil {
		t.Fatal(err)
	}

	var asked []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("cve"))
		_, _ = w.Write([]byte(`{"status":"OK","data":[{"cve":"CVE-2026-0001","epss":"0.71","date":"2026-11-19"}]}`))
	}))
	defer api.Close()

	rec := post(t, mux, "/api/v1/feeds/refresh",
		`{"feeds":["epss"],"endpoints":{"epss-api":"`+api.URL+`"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	}
	if len(asked) != 1 || asked[0] != "CVE-2026-0001" {
		t.Fatalf("only the stored finding's CVE should be requested: %v", asked)
	}
	var out struct {
		Feeds []store.FeedStatus `json:"feeds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range out.Feeds {
		if f.Feed == "epss" && (f.Snapshot == nil || f.Snapshot.RecordCount != 1) {
			t.Fatalf("EPSS should hold the one score fetched: %+v", f)
		}
	}
	findings, _ := s.GetFindings(ctx, "a1")
	if len(findings) != 1 || findings[0].EPSS != 0.71 {
		t.Fatalf("the finding should carry its score: %+v", findings)
	}
}
