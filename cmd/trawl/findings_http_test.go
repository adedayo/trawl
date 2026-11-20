package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/core"
	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func findingsMux(t *testing.T) (*http.ServeMux, store.Store) {
	t.Helper()
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := event.NewMemoryBus()
	c, err := core.New(s, bus, signals.RegistryJSON())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv := &server{core: c, store: s, bus: bus}
	mux := http.NewServeMux()
	srv.routes(mux)
	return mux, s
}

func get(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// The finding routes are served over HTTP exactly as over Wails: the literal
// paths beat {key}, a key with every awkward character in its source row round
// trips through the path, and an unknown or malformed key is a 404 rather than
// an empty 200.
func TestTheFindingRoutesServeTheDrillDownOverHTTP(t *testing.T) {
	mux, s := findingsMux(t)
	ctx := context.Background()

	if err := s.SaveAsset(ctx, &store.Asset{ID: "a1", Type: store.AssetTypeDomain, Value: "example.com",
		Status: store.AssetStatusActive, DiscoverySource: "t", Confidence: 1}); err != nil {
		t.Fatal(err)
	}
	// An identifier with a slash, colons and an at-sign, as service findings have.
	id := "service:a1:host.example.com:443:tls/1:exposed"
	run := store.WithRun(ctx, store.RunRef{ID: "run-1", Kind: store.RunServiceProbe})
	if err := s.SaveFinding(run, &store.Finding{ID: id, AssetID: "a1", Title: "Externally reachable https service",
		Severity: store.SeverityMedium, Priority: "medium", Category: "service-exposure"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordScanRun(ctx, &store.ScanRun{ID: "run-1", Kind: store.RunServiceProbe, AssetID: "a1",
		FinishedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	rec := get(t, mux, "/api/v1/findings/index?status=active")
	if rec.Code != 200 {
		t.Fatalf("index: %d %s", rec.Code, rec.Body)
	}
	var list []service.FindingSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("index body: %v %s", err, rec.Body)
	}

	rec = get(t, mux, "/api/v1/findings/"+url.PathEscape(list[0].Key))
	if rec.Code != 200 {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body)
	}
	var d service.FindingDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Key != list[0].Key || len(d.Timeline) != 1 || d.Timeline[0].RunID != "run-1" {
		t.Errorf("detail = %+v", d)
	}
	if d.Explanation.Version == "" || d.Explanation.WhyItMatters == "" {
		t.Errorf("the executive explanation is missing: %+v", d.Explanation)
	}

	for _, path := range []string{"/api/v1/findings/nonsense", "/api/v1/findings/" + store.FindingKey(store.SourceFinding, "nope")} {
		if rec := get(t, mux, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, rec.Code)
		}
	}

	if rec := get(t, mux, "/api/v1/findings/changes"); rec.Code != 200 {
		t.Errorf("changes: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, mux, "/api/v1/findings/changes?to=run-404"); rec.Code != http.StatusBadRequest {
		t.Errorf("changes to an unknown run = %d, want 400", rec.Code)
	}
	if rec := get(t, mux, "/api/v1/scans"); rec.Code != 200 {
		t.Errorf("scans: %d", rec.Code)
	}
	if rec := get(t, mux, "/api/v1/history/summary"); rec.Code != 200 {
		t.Errorf("history: %d", rec.Code)
	}
	if rec := get(t, mux, "/api/v1/history/summary?since=yesterday"); rec.Code != http.StatusBadRequest {
		t.Errorf("history with a bad time = %d, want 400", rec.Code)
	}
}
