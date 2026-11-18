package contact

import (
	"math"
	"testing"
	"time"
)

func TestReachabilityGatesAndSuppressesUnknown(t *testing.T) {
	if got := DeriveReachability(0, 0, 0); got != ReachabilityUnknown {
		t.Fatalf("DNS-only / no concluded probes must be unknown, got %q", got)
	}
	if got := DeriveReachability(0, 2, 2); got != ReachabilityNotResponding {
		t.Fatalf("concluded non-responding probes must be not_responding, got %q", got)
	}
	if got := DeriveReachability(1, 3, 4); got != ReachabilityResponding {
		t.Fatalf("any responding service sets R=1, got %q", got)
	}

	unknownEst := ComputeContactEstimate(EstimateInput{
		AssetID:      "asset-1",
		ServiceID:    "https@example.com:443/http",
		ServiceClass: ServiceClassWeb,
		Reachability: ReachabilityUnknown,
	})
	if unknownEst.Available {
		t.Fatalf("reachability unknown must suppress estimate rather than default to zero or nominal: %+v", unknownEst)
	}
	if unknownEst.SuppressedWhy == "" || !unknownEst.IsPrior {
		t.Fatalf("suppressed estimate must carry reason and IsPrior=true: %+v", unknownEst)
	}

	closedEst := ComputeContactEstimate(EstimateInput{
		AssetID:      "asset-1",
		ServiceID:    "https@example.com:443/http",
		ServiceClass: ServiceClassWeb,
		Reachability: ReachabilityNotResponding,
	})
	if !closedEst.Available || closedEst.LambdaContact.Point != 0 || closedEst.PContact.Point != 0 {
		t.Fatalf("confirmed not_responding service must produce available zero contact rate: %+v", closedEst)
	}
}

func TestDiscoverabilityHasFloorAndCap(t *testing.T) {
	base, traces := ComputeDiscoverability(DiscoverabilityEvidence{})
	if base.Point != 1.0 || len(traces) != 0 {
		t.Fatalf("undiscovered host must retain sweep floor D=1.0, got %+v (%v)", base, traces)
	}

	all, traces := ComputeDiscoverability(DiscoverabilityEvidence{
		CTLogged: true, PassiveDNS: true, WildcardZone: true, ZoneTransfer: true, SearchIndex: true,
	})
	if all.Point > discoverabilityCap || all.High > discoverabilityCap {
		t.Fatalf("discoverability exceeded cap %v: %+v", discoverabilityCap, all)
	}
	if all.Point <= 1.0 || len(traces) != 5 {
		t.Fatalf("expected 5 discoverability traces above floor: %+v (%d)", all, len(traces))
	}
}

func TestAccessBarrierAttenuatesStrictlyBelowOne(t *testing.T) {
	none, _ := ComputeAccessBarrier(BarrierEvidence{})
	if none.Point != 0 {
		t.Fatalf("no barriers must yield B=0, got %+v", none)
	}

	heavy, _ := ComputeAccessBarrier(BarrierEvidence{
		AuthRequired: true, Allowlist: true, MTLS: true, WAFOrCDN: true,
	})
	if heavy.Point <= 0 || heavy.Point >= 1.0 || heavy.High >= 1.0 {
		t.Fatalf("access barrier must remain strictly in (0, 1): %+v", heavy)
	}
}

func TestUnremediatedExposurePContactStrictlyIncreasesOverWallClockTime(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	hist, err := ComputeHistory([]Observation{
		{At: t0, State: Exposed, Successful: true},
	}, t0.Add(12*time.Hour), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	input := EstimateInput{
		AssetID:         "asset-1",
		ServiceID:       "ssh@host.example.com:22/tcp",
		ServiceClass:    ServiceClassSSH,
		WeaknessClass:   "service-exposure",
		Reachability:    ReachabilityResponding,
		Discoverability: DiscoverabilityEvidence{CTLogged: true},
		History:         hist,
		Now:             t0.Add(12 * time.Hour),
		Coverage:        CoverageFigure{ChecksConcluded: 4, ChecksApplicable: 5},
	}
	early := ComputeContactEstimate(input)
	input.Now = t0.Add(72 * time.Hour)
	later := ComputeContactEstimate(input)

	if !early.Available || !later.Available {
		t.Fatalf("estimates must be available: early=%+v later=%+v", early, later)
	}
	if !(later.PContact.Point > early.PContact.Point) {
		t.Fatalf("unremediated exposure P(contact) must strictly increase over wall-clock time: early=%v later=%v",
			early.PContact.Point, later.PContact.Point)
	}
	if !later.TimeAtRisk.LeftCensored || later.ScopeStatement != AmbientScopeStatement || !later.IsPrior {
		t.Fatalf("missing censoring, ambient scope statement, or IsPrior marker: %+v", later)
	}
}

func TestFactorRemovalRecomputesWithoutTouchingOtherFactors(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	hist, _ := ComputeHistory([]Observation{
		{At: t0, State: Exposed, Successful: true},
		{At: t0.Add(24 * time.Hour), State: Exposed, Successful: true},
	}, t0.Add(24*time.Hour), 24*time.Hour)

	in := EstimateInput{
		AssetID:         "asset-1",
		ServiceID:       "https@app.example.com:443/http",
		ServiceClass:    ServiceClassWeb,
		WeaknessClass:   "service-exposure",
		Reachability:    ReachabilityResponding,
		Discoverability: DiscoverabilityEvidence{CTLogged: true, WildcardZone: true},
		Barriers:        BarrierEvidence{WAFOrCDN: true},
		History:         hist,
		Now:             t0.Add(24 * time.Hour),
		Coverage:        CoverageFigure{ChecksConcluded: 10, ChecksApplicable: 10},
	}
	withCT := ComputeContactEstimate(in)
	withoutCT := RecomputeWithoutFactor(in, "discoverability:ct")

	if !(withoutCT.LambdaContact.Point < withCT.LambdaContact.Point) {
		t.Fatalf("removing discoverability:ct must lower lambda_contact: before=%v after=%v",
			withCT.LambdaContact.Point, withoutCT.LambdaContact.Point)
	}
	// Barrier and wildcard traces must remain untouched.
	var sawWildcard, sawWAF bool
	for _, f := range withoutCT.Factors {
		if f.ID == "discoverability:ct" {
			t.Fatalf("excluded factor discoverability:ct still present in traces")
		}
		if f.ID == "discoverability:wildcard" && f.Value == 0.25 {
			sawWildcard = true
		}
		if f.ID == "barrier:waf-cdn" && f.Value == 0.35 {
			sawWAF = true
		}
	}
	if !sawWildcard || !sawWAF {
		t.Fatalf("other factors were altered when removing CT: %+v", withoutCT.Factors)
	}
}

func TestHeterogeneousAndHomogeneousInstanceAggregationAndResidual(t *testing.T) {
	singleP := 0.20
	est := ContactPriorEstimate{
		Available: true,
		PContact:  Interval{Point: singleP, Low: 0.05, High: 0.50},
		Coverage:  CoverageFigure{ChecksConcluded: 1, ChecksApplicable: 1},
		Label:     LabelSourced,
		IsPrior:   true,
	}

	instances := make([]WeaknessInstance, 15)
	for i := range instances {
		instances[i] = WeaknessInstance{
			InstanceID:          "inst",
			ConfirmedRemediated: i < 3, // 3 of 15 confirmed remediated
			Estimate:            est,
		}
	}
	agg := AggregateInstances("subdomain-takeover", instances)
	wantAll := 1.0 - math.Pow(1.0-singleP, 15)
	wantResidual := 1.0 - math.Pow(1.0-singleP, 12)

	if !agg.Available || agg.TotalInstances != 15 || agg.ConfirmedRemediated != 3 || agg.UncoveredInstances != 12 {
		t.Fatalf("unexpected counts: %+v", agg)
	}
	if math.Abs(agg.AllInstancesPContact.Point-wantAll) > 1e-9 {
		t.Fatalf("homogeneous all-instance reduction mismatch: got %v, want %v", agg.AllInstancesPContact.Point, wantAll)
	}
	if math.Abs(agg.ResidualPContact.Point-wantResidual) > 1e-9 {
		t.Fatalf("residual 12-instance reduction mismatch: got %v, want %v", agg.ResidualPContact.Point, wantResidual)
	}
}

func TestCadenceEvaluationAndLatencySeparation(t *testing.T) {
	cmp, err := EvaluateCadenceChange(90*24*time.Hour, 24*time.Hour, 4.0, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.BlindTimeReduction <= 0 || cmp.ContactProbReduction <= 0 || cmp.ExpectedBlindDaysSavedYr <= 0 {
		t.Fatalf("moving from quarterly to daily must reduce blind time and contact probability: %+v", cmp)
	}

	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	hist, _ := ComputeHistory([]Observation{{At: t0, State: Exposed, Successful: true}}, t0.Add(30*24*time.Hour), time.Hour)
	split := ComputeLatencySplit(hist, t0.Add(30*24*time.Hour))
	if split.ExpectedDetectionLatency != 30*time.Minute || split.RemediationLatency != 30*24*time.Hour {
		t.Fatalf("detection latency and remediation latency must stay separate: %+v", split)
	}
}
