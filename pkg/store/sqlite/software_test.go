package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

func TestSoftwareInventorySupersedingAndCoverage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "software.db")
	s, err := sqlite.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return t0 })
	ctx := context.Background()

	asset := &store.Asset{
		ID: "asset-1", Type: store.AssetTypeSubdomain, Value: "app.example.com",
		Status: store.AssetStatusActive, DiscoverySource: "httpx", Confidence: 1.0,
		FirstSeen: t0, LastSeen: t0,
	}
	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}

	// Initially not assessed, with an explicit reason explaining why.
	cov, err := s.ComputeSoftwareCoverage(ctx, asset.ID)
	if err != nil {
		t.Fatalf("ComputeSoftwareCoverage: %v", err)
	}
	if cov.State != store.SoftwareCoverageNotAssessed {
		t.Fatalf("initial coverage = %q, want not_assessed", cov.State)
	}
	if cov.Reason == "" {
		t.Fatalf("expected non-empty reason when software is not_assessed")
	}

	// Only unmapped software -> seen_unmapped.
	if err := s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: asset.ID, ObservedName: "Cloudflare", MappingState: store.SoftwareUnmapped,
		VersionState: store.SoftwareVersionUnknown, Source: "httpx", RawString: "Cloudflare",
	}); err != nil {
		t.Fatalf("SaveAssetSoftware unmapped: %v", err)
	}
	cov, err = s.ComputeSoftwareCoverage(ctx, asset.ID)
	if err != nil {
		t.Fatalf("ComputeSoftwareCoverage: %v", err)
	}
	if cov.State != store.SoftwareCoverageSeenUnmapped || cov.Unmapped != 1 {
		t.Fatalf("unmapped coverage = %+v, want seen_unmapped with Unmapped=1", cov)
	}

	// Mapped software without version -> version_unknown.
	if err := s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: asset.ID, ObservedName: "WordPress", CPEPart: "a", Vendor: "wordpress", Product: "wordpress",
		MappingState: store.SoftwareMapped, MappingVersion: "1.0.0",
		VersionState: store.SoftwareVersionUnknown, Source: "httpx", RawString: "WordPress",
	}); err != nil {
		t.Fatalf("SaveAssetSoftware version unknown: %v", err)
	}
	cov, err = s.ComputeSoftwareCoverage(ctx, asset.ID)
	if err != nil {
		t.Fatalf("ComputeSoftwareCoverage: %v", err)
	}
	if cov.State != store.SoftwareCoverageVersionUnknown || cov.VersionUnknown != 1 {
		t.Fatalf("version unknown coverage = %+v, want version_unknown", cov)
	}

	// Mapped software with version -> identified.
	if err := s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: asset.ID, ObservedName: "Nginx", CPEPart: "a", Vendor: "f5", Product: "nginx",
		MappingState: store.SoftwareMapped, MappingVersion: "1.0.0",
		Version: "1.18.0", VersionState: store.SoftwareVersionKnown, Source: "httpx", RawString: "Nginx:1.18.0",
	}); err != nil {
		t.Fatalf("SaveAssetSoftware nginx 1.18.0: %v", err)
	}
	cov, err = s.ComputeSoftwareCoverage(ctx, asset.ID)
	if err != nil {
		t.Fatalf("ComputeSoftwareCoverage: %v", err)
	}
	if cov.State != store.SoftwareCoverageIdentified || cov.Identified != 1 {
		t.Fatalf("identified coverage = %+v, want identified with Identified=1", cov)
	}

	// Upgrade nginx 1.18.0 -> 1.24.0: 1.18.0 becomes superseded, 1.24.0 is current, both remain readable.
	t1 := t0.Add(24 * time.Hour)
	s.SetClock(func() time.Time { return t1 })
	if err := s.SaveAssetSoftware(ctx, &store.AssetSoftware{
		AssetID: asset.ID, ObservedName: "Nginx", CPEPart: "a", Vendor: "f5", Product: "nginx",
		MappingState: store.SoftwareMapped, MappingVersion: "1.0.0",
		Version: "1.24.0", VersionState: store.SoftwareVersionKnown, Source: "httpx", RawString: "Nginx:1.24.0",
	}); err != nil {
		t.Fatalf("SaveAssetSoftware nginx 1.24.0: %v", err)
	}

	items, err := s.GetAssetSoftware(ctx, asset.ID)
	if err != nil {
		t.Fatalf("GetAssetSoftware: %v", err)
	}
	var current124, superseded118 bool
	for _, it := range items {
		if it.Product == "nginx" && it.Version == "1.24.0" && it.Status == store.SoftwareCurrent {
			current124 = true
		}
		if it.Product == "nginx" && it.Version == "1.18.0" && it.Status == store.SoftwareSuperseded && it.SupersededAt != nil {
			superseded118 = true
		}
	}
	if !current124 || !superseded118 {
		t.Fatalf("expected nginx 1.24.0 current and 1.18.0 superseded, got %+v", items)
	}
}

func TestCandidateFindingSingleWritePathEvents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "candidates.db")
	s, err := sqlite.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return t0 })
	ctx := context.Background()

	asset := &store.Asset{
		ID: "asset-1", Type: store.AssetTypeSubdomain, Value: "app.example.com",
		Status: store.AssetStatusActive, DiscoverySource: "httpx", Confidence: 1.0,
		FirstSeen: t0, LastSeen: t0,
	}
	if err := s.SaveAsset(ctx, asset); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}

	cf := &store.CandidateFinding{
		AssetID: asset.ID, SoftwareID: "sw:1", Vendor: "f5", Product: "nginx", Version: "1.18.0",
		RawString: "Nginx:1.18.0", CVE: "CVE-2021-23017", Title: "CVE-2021-23017 (nginx 1.18.0)",
		Severity: store.SeverityHigh, Priority: "high", Status: store.FindingOpen,
		LookupSnapshotID: "snap-nvd-1", LookupAt: t0,
	}
	if err := s.SaveCandidateFinding(ctx, cf); err != nil {
		t.Fatalf("SaveCandidateFinding: %v", err)
	}

	// Resolve candidate.
	s.SetClock(func() time.Time { return t0.Add(time.Hour) })
	cf.Status = store.FindingResolved
	if err := s.SaveCandidateFinding(ctx, cf); err != nil {
		t.Fatalf("SaveCandidateFinding resolve: %v", err)
	}

	// Reopen candidate.
	s.SetClock(func() time.Time { return t0.Add(2 * time.Hour) })
	cf.Status = store.FindingOpen
	if err := s.SaveCandidateFinding(ctx, cf); err != nil {
		t.Fatalf("SaveCandidateFinding reopen: %v", err)
	}

	key := store.FindingKey(store.SourceCandidate, cf.ID)
	evs, err := s.GetFindingEvents(ctx, store.EventQuery{Key: key})
	if err != nil {
		t.Fatalf("GetFindingEvents: %v", err)
	}
	if len(evs) != 3 {
		t.Fatalf("got %d events, want 3 (first_seen, resolved, reopened): %+v", len(evs), evs)
	}
	if evs[0].Type != store.EventFirstSeen || evs[1].Type != store.EventResolved || evs[2].Type != store.EventReopened {
		t.Fatalf("event types = [%s, %s, %s], want [first_seen, resolved, reopened]", evs[0].Type, evs[1].Type, evs[2].Type)
	}
}

func TestSoftwareNotAssessedReasons(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "reasons.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

	saveAsset := func(id string, typ store.AssetType, val string) {
		if err := s.SaveAsset(ctx, &store.Asset{
			ID: id, Type: typ, Value: val, Status: store.AssetStatusActive,
			DiscoverySource: "test", Confidence: 1, FirstSeen: now, LastSeen: now,
		}); err != nil {
			t.Fatalf("SaveAsset %s: %v", id, err)
		}
	}

	// 1. Repository asset
	saveAsset("repo-1", store.AssetTypeRepository, "https://github.com/example/repo")
	cov, err := s.ComputeSoftwareCoverage(ctx, "repo-1")
	if err != nil || cov.Reason == "" {
		t.Fatalf("repo coverage = %+v, err=%v", cov, err)
	}

	// 2. Probed, none responded (ports closed/refused)
	saveAsset("closed-1", store.AssetTypeSubdomain, "closed.example.com")
	_ = s.SaveServiceObservation(ctx, &store.ServiceObservation{
		ID: "o1", AssetID: "closed-1", Host: "closed.example.com", Port: 443,
		Service: "https", Transport: "tcp", Protocol: "https", Layer: "http",
		State: "not_responding", Coverage: store.CoverageOK, ObservedAt: now,
	})
	cov, err = s.ComputeSoftwareCoverage(ctx, "closed-1")
	if err != nil || cov.State != store.SoftwareCoverageNotAssessed || cov.Reason == "" {
		t.Fatalf("closed coverage = %+v, err=%v", cov, err)
	}
	if want := "none responded"; !containsStr(cov.Reason, want) {
		t.Errorf("closed reason = %q, want substring %q", cov.Reason, want)
	}

	// 3. Probed, inconclusive (timeout)
	saveAsset("timeout-1", store.AssetTypeSubdomain, "fw.example.com")
	_ = s.SaveServiceObservation(ctx, &store.ServiceObservation{
		ID: "o2", AssetID: "timeout-1", Host: "fw.example.com", Port: 443,
		Service: "https", Transport: "tcp", Protocol: "https", Layer: "http",
		State: "unknown", Coverage: store.CoverageCheckFailed, Evidence: `{"error":"i/o timeout"}`, ObservedAt: now,
	})
	cov, err = s.ComputeSoftwareCoverage(ctx, "timeout-1")
	if err != nil || !containsStr(cov.Reason, "i/o timeout") {
		t.Errorf("timeout reason = %q, err=%v, want i/o timeout", cov.Reason, err)
	}

	// 4. Responded, but disclosed no software headers/banners
	saveAsset("silent-1", store.AssetTypeSubdomain, "silent.example.com")
	_ = s.SaveServiceObservation(ctx, &store.ServiceObservation{
		ID: "o3", AssetID: "silent-1", Host: "silent.example.com", Port: 443,
		Service: "https", Transport: "tcp", Protocol: "https", Layer: "http",
		State: "responding", Coverage: store.CoverageOK, ObservedAt: now,
	})
	cov, err = s.ComputeSoftwareCoverage(ctx, "silent-1")
	if err != nil || !containsStr(cov.Reason, "disclosed no software") {
		t.Errorf("silent reason = %q, err=%v, want disclosed no software", cov.Reason, err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
