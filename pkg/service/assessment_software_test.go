package service_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	vobs "github.com/adedayo/vantage/pkg/observation"
	vprobe "github.com/adedayo/vantage/pkg/probe"

	"github.com/adedayo/trawl/config/signals"
	"github.com/adedayo/trawl/pkg/event"
	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/service"
	"github.com/adedayo/trawl/pkg/software"
	"github.com/adedayo/trawl/pkg/store"
	"github.com/adedayo/trawl/pkg/store/sqlite"
)

type stubServiceProber struct {
	serverHeader string
}

func (p stubServiceProber) Probe(_ context.Context, req vprobe.Request) vobs.ServiceObservation {
	return vobs.ServiceObservation{
		Host:         req.Target.Host,
		Port:         req.Target.Port,
		Service:      "https",
		Transport:    "tcp",
		Protocol:     "https",
		Layer:        req.Layer,
		State:        vobs.ServiceResponding,
		ProbeProfile: "most-common",
		ObservedAt:   time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC),
		Evidence: vobs.ServiceEvidence{
			ResponseClass: "http_response",
			HTTPStatus:    200,
			Server:        p.serverHeader,
		},
	}
}

func (p stubServiceProber) ProbeMany(ctx context.Context, reqs []vprobe.Request) []vobs.ServiceObservation {
	out := make([]vobs.ServiceObservation, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, p.Probe(ctx, r))
	}
	return out
}

func TestProbeServicesPopulatesSoftwareInventoryFromServerHeader(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "probe_sw.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	svc, err := service.NewAssessmentService(s, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatalf("NewAssessmentService: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	if err := s.SaveAsset(ctx, &store.Asset{
		ID: "asset-1", Type: store.AssetTypeSubdomain, Value: "web.example.com",
		Status: store.AssetStatusActive, DiscoverySource: "subfinder", Confidence: 1,
		FirstSeen: now, LastSeen: now,
	}); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}

	custom := []vprobe.Request{{
		Target: vprobe.Target{
			Host: "web.example.com",
			Port: 443,
		},
		Layer: vobs.ServiceLayerHTTP,
	}}

	_, err = svc.ProbeServices(ctx, stubServiceProber{serverHeader: "Apache/2.4.49 (Ubuntu) OpenSSL/1.1.1k"}, vadapter.ServiceRequest{
		AssetID: "asset-1",
		Host:    "web.example.com",
		Custom:  custom,
	})
	if err != nil {
		t.Fatalf("ProbeServices: %v", err)
	}

	cov, err := s.ComputeSoftwareCoverage(ctx, "asset-1")
	if err != nil {
		t.Fatalf("ComputeSoftwareCoverage: %v", err)
	}
	if cov.State != store.SoftwareCoverageIdentified || cov.Identified != 2 || cov.VersionUnknown != 1 {
		t.Fatalf("coverage = %+v, want identified=2 (apache, openssl) versionUnknown=1 (ubuntu)", cov)
	}

	items, err := s.GetAssetSoftware(ctx, "asset-1")
	if err != nil {
		t.Fatalf("GetAssetSoftware: %v", err)
	}
	var sawApache, sawOpenSSL bool
	for _, it := range items {
		if it.Vendor == "apache" && it.Product == "http_server" && it.Version == "2.4.49" && it.Source == "vantage-probe" {
			sawApache = true
		}
		if it.Vendor == "openssl" && it.Product == "openssl" && it.Version == "1.1.1k" && it.Source == "vantage-probe" {
			sawOpenSSL = true
		}
	}
	if !sawApache || !sawOpenSSL {
		t.Fatalf("expected apache 2.4.49 and openssl 1.1.1k from vantage-probe, got %+v", items)
	}
}

type stubMultiServiceProber struct {
	obs []vobs.ServiceObservation
}

func (p stubMultiServiceProber) Probe(_ context.Context, _ vprobe.Request) vobs.ServiceObservation {
	return p.obs[0]
}

func (p stubMultiServiceProber) ProbeMany(_ context.Context, _ []vprobe.Request) []vobs.ServiceObservation {
	return p.obs
}

func TestProbeServicesRunsHTTPFingerprinterAndRecordsSilentNonHTTPServices(t *testing.T) {
	s, err := sqlite.NewSQLiteStore(filepath.Join(t.TempDir(), "probe_fp.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	svc, err := service.NewAssessmentService(s, event.NewMemoryBus(), signals.RegistryJSON())
	if err != nil {
		t.Fatalf("NewAssessmentService: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	if err := s.SaveAsset(ctx, &store.Asset{
		ID: "asset-fp", Type: store.AssetTypeSubdomain, Value: "portal.example.com",
		Status: store.AssetStatusActive, DiscoverySource: "subfinder", Confidence: 1,
		FirstSeen: now, LastSeen: now,
	}); err != nil {
		t.Fatalf("SaveAsset: %v", err)
	}

	svc.SetHTTPFingerprinter(func(_ context.Context, _ *vadapter.Scope, scheme, host string, port uint16) ([]software.ParsedTech, error) {
		if scheme != "https" || host != "portal.example.com" || port != 443 {
			t.Errorf("unexpected fingerprint target %s://%s:%d", scheme, host, port)
		}
		return []software.ParsedTech{
			{Raw: "meta generator: WordPress 6.5.2", Name: "WordPress", Version: "6.5.2", VersionKnown: true},
		}, nil
	})

	prober := stubMultiServiceProber{obs: []vobs.ServiceObservation{
		{
			Host: "portal.example.com", Port: 443, Service: "https", Transport: "tcp",
			Protocol: "https", Layer: vobs.ServiceLayerHTTP, State: vobs.ServiceResponding,
			ProbeProfile: "most-common", ObservedAt: now,
			Evidence: vobs.ServiceEvidence{ResponseClass: "http_response", HTTPStatus: 200},
		},
		{
			Host: "portal.example.com", Port: 5432, Service: "postgresql", Transport: "tcp",
			Protocol: "postgresql", Layer: vobs.ServiceLayerTCP, State: vobs.ServiceResponding,
			ProbeProfile: "most-common", ObservedAt: now,
			Evidence: vobs.ServiceEvidence{ResponseClass: "tcp_connect"},
		},
	}}

	custom := []vprobe.Request{
		{Target: vprobe.Target{Host: "portal.example.com", Port: 443}, Layer: vobs.ServiceLayerHTTP},
		{Target: vprobe.Target{Host: "portal.example.com", Port: 5432}, Layer: vobs.ServiceLayerTCP},
	}

	if _, err := svc.ProbeServices(ctx, prober, vadapter.ServiceRequest{
		AssetID: "asset-fp", Host: "portal.example.com", Custom: custom,
	}); err != nil {
		t.Fatalf("ProbeServices: %v", err)
	}

	items, err := s.GetAssetSoftware(ctx, "asset-fp")
	if err != nil {
		t.Fatalf("GetAssetSoftware: %v", err)
	}
	var sawWP, sawPG bool
	for _, it := range items {
		if it.Product == "wordpress" && it.Version == "6.5.2" && it.Source == "http-fingerprint" {
			sawWP = true
		}
		if it.ObservedName == "postgresql" && it.VersionState == store.SoftwareVersionUnknown && it.Source == "vantage-probe" {
			sawPG = true
		}
	}
	if !sawWP || !sawPG {
		t.Fatalf("expected WordPress 6.5.2 (http-fingerprint) and postgresql (version unknown), got %+v", items)
	}
}
