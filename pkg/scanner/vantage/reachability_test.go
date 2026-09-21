package vantage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	vobs "github.com/adedayo/vantage/pkg/observation"
	vprobe "github.com/adedayo/vantage/pkg/probe"

	"github.com/adedayo/trawl/pkg/store"
)

type fakeServiceProber struct {
	requests []vprobe.Request
	results  []vobs.ServiceObservation
}

func (f *fakeServiceProber) Probe(context.Context, vprobe.Request) vobs.ServiceObservation {
	return vobs.ServiceObservation{State: vobs.ServiceUnknown}
}

func (f *fakeServiceProber) ProbeMany(_ context.Context, requests []vprobe.Request) []vobs.ServiceObservation {
	f.requests = append([]vprobe.Request(nil), requests...)
	return f.results
}

var _ vprobe.Prober = (*fakeServiceProber)(nil)

func TestProbeServicesUsesMostCommonProfileAndPreservesCoverage(t *testing.T) {
	when := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	fake := &fakeServiceProber{results: make([]vobs.ServiceObservation, 10)}
	for i := range fake.results {
		state := vobs.ServiceNotResponding
		if i == 0 {
			state = vobs.ServiceResponding
		}
		if i == 2 {
			state = vobs.ServiceUnknown
		}
		fake.results[i] = vobs.ServiceObservation{
			Host: "asset.example.test", Port: uint16(443 + i),
			Service: "service", Protocol: "tcp", Layer: vobs.ServiceLayerTCP,
			State: state, ProbeProfile: "most-common", ObservedAt: when,
			Evidence: vobs.ServiceEvidence{ResponseClass: "test"},
		}
	}

	observations, err := ProbeServices(context.Background(), fake, ServiceRequest{
		AssetID: "asset-1", Host: "asset.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != 10 || len(observations) != 10 {
		t.Fatalf("requests = %d, observations = %d", len(fake.requests), len(observations))
	}
	if observations[0].Coverage != store.CoverageOK || observations[1].Coverage != store.CoverageOK {
		t.Fatalf("settled coverage = %q/%q", observations[0].Coverage, observations[1].Coverage)
	}
	if observations[2].Coverage != store.CoverageCheckFailed {
		t.Fatalf("unknown coverage = %q, want check_failed", observations[2].Coverage)
	}
	if observations[0].ObservedAt != when || observations[0].AssetID != "asset-1" {
		t.Fatalf("provenance was not preserved: %+v", observations[0])
	}
}

func TestProbeServicesRequiresExplicitCustomRequests(t *testing.T) {
	fake := &fakeServiceProber{}
	_, err := ProbeServices(context.Background(), fake, ServiceRequest{
		AssetID: "asset-1", Host: "asset.example.test", Profile: vprobe.DiscoveryCustom,
	})
	if err == nil {
		t.Fatal("custom profile without requests should fail closed")
	}
	if len(fake.requests) != 0 {
		t.Fatalf("custom profile submitted %d requests", len(fake.requests))
	}
}

func TestServiceAdvisoriesFlagMissingHSTSAndSensitiveExposure(t *testing.T) {
	evidence, err := json.Marshal(map[string]any{
		"http_status": 200, "tls_version": "TLS 1.3", "certificate_expiry": "2026-12-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	observations := []store.ServiceObservation{
		{AssetID: "asset-1", Host: "api.example.test", Port: 443, Service: "https", Protocol: "https", Layer: "http", State: "responding", Coverage: store.CoverageOK, Evidence: string(evidence), ObservedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)},
		{AssetID: "asset-1", Host: "db.example.test", Port: 6379, Service: "redis", Protocol: "redis", Layer: "tcp", State: "responding", Coverage: store.CoverageOK, Evidence: "{}", ObservedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)},
	}
	var findings []store.Finding
	if err := ServiceAdvisories(context.Background(), func(finding *store.Finding) error {
		findings = append(findings, *finding)
		return nil
	}, observations, time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(findings) != 3 {
		t.Fatalf("findings = %+v, want exposure findings plus HSTS advisory", findings)
	}
	if findings[0].Category != "service-exposure" && findings[1].Category != "service-exposure" {
		t.Fatalf("missing service exposure findings: %+v", findings)
	}
}
