package contact

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// CalibrationLabel identifies the evidential standing of a contact estimate,
// satisfying Guardrail 6 (no unlabelled number) and Guardrail 8 (estimates stay
// available on class-level priors without blocking on telemetry).
type CalibrationLabel string

const (
	LabelIllustrative CalibrationLabel = "illustrative"
	LabelSourced      CalibrationLabel = "sourced"
	LabelCalibrated   CalibrationLabel = "calibrated"
)

// Interval carries a central value and its 90% uncertainty bounds. Bare scalar
// probabilities and rates are intentionally omitted from emitted estimate types.
type Interval struct {
	Point float64 `json:"point"`
	Low   float64 `json:"low"`
	High  float64 `json:"high"`
}

// ReachabilityState is the three-state gate on external contact. Unknown is
// never defaulted to NotResponding (which would falsely reassure with zero) or
// to Responding (which would fabricate exposure).
type ReachabilityState string

const (
	ReachabilityResponding    ReachabilityState = "responding"
	ReachabilityNotResponding ReachabilityState = "not_responding"
	ReachabilityUnknown       ReachabilityState = "unknown"
)

// DeriveReachability maps structured Vantage service probe outcomes to the
// contact model's reachability gate. DNS resolution without a concluded service
// probe remains ReachabilityUnknown, never Responding or NotResponding.
func DeriveReachability(respondingCount, notRespondingCount, totalConcluded int) ReachabilityState {
	if totalConcluded <= 0 {
		return ReachabilityUnknown
	}
	if respondingCount > 0 {
		return ReachabilityResponding
	}
	if notRespondingCount > 0 {
		return ReachabilityNotResponding
	}
	return ReachabilityUnknown
}

// DiscoverabilityEvidence names the externally observable signals that raise an
// asset's discoverability above pure address-space sweeps.
type DiscoverabilityEvidence struct {
	CTLogged     bool     `json:"ctLogged"`
	PassiveDNS   bool     `json:"passiveDns"`
	WildcardZone bool     `json:"wildcardZone"`
	ZoneTransfer bool     `json:"zoneTransfer"`
	SearchIndex  bool     `json:"searchIndex"`
	Sources      []string `json:"sources,omitempty"`
}

const (
	discoverabilityFloor = 1.0
	discoverabilityCap   = 3.0
)

// ComputeDiscoverability returns D >= 1, bounded by discoverabilityCap so that
// obscurity remains a weak defence (D >= 1 reflects indiscriminate IPv4 sweeps)
// while public enumeration raises the contact rate boundedly.
func ComputeDiscoverability(ev DiscoverabilityEvidence) (Interval, []FactorTrace) {
	point := discoverabilityFloor
	var traces []FactorTrace
	add := func(id string, delta float64, source string) {
		point += delta
		traces = append(traces, FactorTrace{
			ID:            id,
			Name:          "discoverability",
			Value:         delta,
			Source:        source,
			EvidenceClass: "measured-state",
			State:         "ok",
		})
	}
	if ev.CTLogged {
		add("discoverability:ct", 0.6, "vantage:ct")
	}
	if ev.PassiveDNS {
		add("discoverability:passive-dns", 0.35, "vantage:discovery")
	}
	if ev.WildcardZone {
		add("discoverability:wildcard", 0.25, "vantage:wild")
	}
	if ev.ZoneTransfer {
		add("discoverability:axfr", 1.2, "vantage:axfr")
	}
	if ev.SearchIndex {
		add("discoverability:index", 0.3, "search-index")
	}
	if point > discoverabilityCap {
		point = discoverabilityCap
	}
	low := math.Max(discoverabilityFloor, 1.0+(point-1.0)*0.7)
	high := math.Min(discoverabilityCap, 1.0+(point-1.0)*1.3)
	return Interval{Point: point, Low: low, High: high}, traces
}

// BarrierEvidence names external access controls observed in front of a
// responding service. Barriers attenuate opportunistic contact rates but never
// eliminate contact (B < 1 strictly).
type BarrierEvidence struct {
	AuthRequired bool `json:"authRequired"`
	Allowlist    bool `json:"allowlist"`
	MTLS         bool `json:"mtls"`
	WAFOrCDN     bool `json:"wafOrCdn"`
}

// MaxAccessBarrier bounds B strictly below 1 so credential-stuffing and edge
// probing remain priced as non-zero contact.
const MaxAccessBarrier = 0.85

// ComputeAccessBarrier returns B in [0, MaxAccessBarrier) and the factor traces
// behind it.
func ComputeAccessBarrier(ev BarrierEvidence) (Interval, []FactorTrace) {
	remaining := 1.0
	var traces []FactorTrace
	apply := func(id string, attenuation float64, source string) {
		remaining *= (1.0 - attenuation)
		traces = append(traces, FactorTrace{
			ID:            id,
			Name:          "access-barrier",
			Value:         attenuation,
			Source:        source,
			EvidenceClass: "measured-state",
			State:         "ok",
		})
	}
	if ev.WAFOrCDN {
		apply("barrier:waf-cdn", 0.35, "vantage:net")
	}
	if ev.AuthRequired {
		apply("barrier:auth", 0.25, "vantage:probe-http")
	}
	if ev.MTLS {
		apply("barrier:mtls", 0.45, "vantage:probe-tls")
	}
	if ev.Allowlist {
		apply("barrier:allowlist", 0.50, "declared-control")
	}
	b := 1.0 - remaining
	if b > MaxAccessBarrier {
		b = MaxAccessBarrier
	}
	low := math.Max(0, b*0.75)
	high := math.Min(MaxAccessBarrier, b*1.15)
	if b == 0 {
		low, high = 0, 0
	}
	return Interval{Point: b, Low: low, High: high}, traces
}

// AmbientPrior is the class-level opportunistic contact rate (attempts per
// reachable service-day) reviewed in ambient-rate-evidence.md.
type AmbientPrior struct {
	ServiceClass ServiceClass     `json:"serviceClass"`
	RatePerDay   Interval         `json:"ratePerDay"`
	Source       string           `json:"source"`
	RetrievedAt  string           `json:"retrievedAt"`
	Label        CalibrationLabel `json:"label"`
}

// stubAmbientPriors holds the literature-bounded class priors reviewed in
// openspec/changes/007-contact-probability/ambient-rate-evidence.md until
// Change 008's signed model pack loader supplies contactBaseRates directly.
var stubAmbientPriors = map[ServiceClass]AmbientPrior{
	ServiceClassWeb: {
		ServiceClass: ServiceClassWeb,
		RatePerDay:   Interval{Point: 12.0, Low: 2.0, High: 60.0},
		Source:       "Durumeric et al. 2014; Richter & Berger 2019; Hiesgen et al. 2022; Pauley et al. 2023 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassSSH: {
		ServiceClass: ServiceClassSSH,
		RatePerDay:   Interval{Point: 18.0, Low: 3.0, High: 90.0},
		Source:       "Durumeric et al. 2014; Richter & Berger 2019; Hiesgen et al. 2022 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassRDP: {
		ServiceClass: ServiceClassRDP,
		RatePerDay:   Interval{Point: 14.0, Low: 2.0, High: 70.0},
		Source:       "Durumeric et al. 2014; Richter & Berger 2019; Hiesgen et al. 2022 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassRemoteAdmin: {
		ServiceClass: ServiceClassRemoteAdmin,
		RatePerDay:   Interval{Point: 6.0, Low: 0.8, High: 30.0},
		Source:       "Richter & Berger 2019; Pauley et al. 2023 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassMail: {
		ServiceClass: ServiceClassMail,
		RatePerDay:   Interval{Point: 5.0, Low: 0.8, High: 25.0},
		Source:       "Durumeric et al. 2014; Richter & Berger 2019 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassDNS: {
		ServiceClass: ServiceClassDNS,
		RatePerDay:   Interval{Point: 4.0, Low: 0.5, High: 20.0},
		Source:       "Durumeric et al. 2014; Richter & Berger 2019 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassDatabase: {
		ServiceClass: ServiceClassDatabase,
		RatePerDay:   Interval{Point: 8.0, Low: 1.0, High: 45.0},
		Source:       "Richter & Berger 2019; Hiesgen et al. 2022; Pauley et al. 2023 (ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
	ServiceClassOtherTCP: {
		ServiceClass: ServiceClassOtherTCP,
		RatePerDay:   Interval{Point: 0.5, Low: 0.05, High: 4.0},
		Source:       "Horizontal IPv4 sweep fallback (Durumeric et al. 2014; Hiesgen et al. 2022; ambient-rate-evidence.md)",
		RetrievedAt:  "2026-11-18",
		Label:        LabelSourced,
	},
}

// LookupAmbientAttention resolves the ambient contact-rate prior for a service
// class, optionally from a caller-supplied table (Change 008 pack overrides)
// and otherwise from the reviewed class table. Unknown classes fall back to
// ServiceClassOtherTCP explicitly rather than inheriting a named service rate.
func LookupAmbientAttention(serviceClass ServiceClass, packTable map[ServiceClass]AmbientPrior) AmbientPrior {
	if !IsKnownServiceClass(serviceClass) {
		serviceClass = ServiceClassOtherTCP
	}
	if packTable != nil {
		if p, ok := packTable[serviceClass]; ok {
			return p
		}
	}
	if p, ok := stubAmbientPriors[serviceClass]; ok {
		return p
	}
	return stubAmbientPriors[ServiceClassOtherTCP]
}

// FactorTrace records one named input to a contact estimate so an operator can
// inspect, challenge, or exclude it and recompute in one step.
type FactorTrace struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	Source        string  `json:"source"`
	EvidenceClass string  `json:"evidenceClass"`
	ObservedAt    string  `json:"observedAt,omitempty"`
	State         string  `json:"state"`
}

// WindowSelection pairs an observation window in days with the threat-timeline
// rationale that selected it.
type WindowSelection struct {
	Days      float64 `json:"days"`
	Rationale string  `json:"rationale"`
}

// SelectWindow chooses the evaluation window for P(contact | T). When the
// asset has positive measured/inferred exposure history, time at risk drives
// the window (growing live for still-exposed services); otherwise the weakness
// class's threat timeline supplies the reference horizon.
func SelectWindow(weaknessClass string, timeAtRiskDays float64) WindowSelection {
	if timeAtRiskDays > 0 {
		return WindowSelection{
			Days:      timeAtRiskDays,
			Rationale: "measured and inferred exposure duration from Trawl observation history",
		}
	}
	switch strings.ToLower(strings.TrimSpace(weaknessClass)) {
	case "exposed-secret", "secret":
		return WindowSelection{Days: 1.0, Rationale: "class threat timeline: internet-exposed credential (24h horizon)"}
	case "subdomain-takeover":
		return WindowSelection{Days: 7.0, Rationale: "class threat timeline: claimable external alias or dangling cloud address (7d horizon)"}
	case "service-exposure", "internet-facing-service", "":
		return WindowSelection{Days: 30.0, Rationale: "class threat timeline: internet-facing service (30d horizon)"}
	default:
		return WindowSelection{Days: 30.0, Rationale: fmt.Sprintf("class threat timeline: %s (30d reference horizon)", weaknessClass)}
	}
}

// CoverageFigure accompanies every emitted contact estimate (Guardrail 5 & 6).
type CoverageFigure struct {
	ChecksConcluded  int `json:"checksConcluded"`
	ChecksApplicable int `json:"checksApplicable"`
}

// TimeAtRiskSummary reports observed, inferred and blind intervals separately
// without collapsing them into a single "measured" scalar.
type TimeAtRiskSummary struct {
	ObservedDays         float64 `json:"observedDays"`
	InferredDays         float64 `json:"inferredDays"`
	EstimatedBlindDays   float64 `json:"estimatedBlindDays"`
	ExpectedDetectionDay float64 `json:"expectedDetectionDays"`
	LeftCensored         bool    `json:"leftCensored"`
	StillExposed         bool    `json:"stillExposed"`
}

// EstimateInput gathers the inputs to ComputeContactEstimate. ExcludedFactors
// enables one-step factor-removal what-if recomputation without mutating the
// underlying evidence.
type EstimateInput struct {
	AssetID         string
	ServiceID       string
	ServiceClass    ServiceClass
	WeaknessClass   string
	Reachability    ReachabilityState
	Discoverability DiscoverabilityEvidence
	Barriers        BarrierEvidence
	History         History
	Now             time.Time
	Coverage        CoverageFigure
	PackPriors      map[ServiceClass]AmbientPrior
	CalibratedPrior *AmbientPrior
	ExcludedFactors map[string]bool
	ObservedAt      time.Time
}

// AmbientScopeStatement is the mandatory scope disclosure attached to every
// emitted prior so no surface can present opportunistic scan rates as targeted
// adversary likelihood.
const AmbientScopeStatement = "Prices ambient, opportunistic internet contact only; deliberately targeted adversary selection is excluded."

// ContactPriorEstimate is the exposure-derived contact prior. Its type and
// IsPrior marker structurally distinguish it from any telemetry-derived contact
// posterior (Guardrail 7).
type ContactPriorEstimate struct {
	AssetID         string            `json:"assetId"`
	ServiceID       string            `json:"serviceId"`
	ServiceClass    ServiceClass      `json:"serviceClass"`
	WeaknessClass   string            `json:"weaknessClass"`
	Available       bool              `json:"available"`
	SuppressedWhy   string            `json:"suppressedReason,omitempty"`
	LambdaContact   Interval          `json:"lambdaContact"`
	WindowDays      float64           `json:"windowDays"`
	WindowRationale string            `json:"windowRationale"`
	PContact        Interval          `json:"pContact"`
	TimeAtRisk      TimeAtRiskSummary `json:"timeAtRisk"`
	Factors         []FactorTrace     `json:"factors"`
	ExcludedFactors []string          `json:"excludedFactors,omitempty"`
	Coverage        CoverageFigure    `json:"coverage"`
	Label           CalibrationLabel  `json:"label"`
	ScopeStatement  string            `json:"scopeStatement"`
	IsPrior         bool              `json:"isPrior"`
}

// ComputeContactEstimate evaluates the Change 007 rate model as a pure
// function. When reachability is unknown, Available is false and no rate or
// probability is fabricated.
func ComputeContactEstimate(in EstimateInput) ContactPriorEstimate {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	live := in.History.LiveTimeAtRisk(now)
	tar := TimeAtRiskSummary{
		ObservedDays:         live.ObservedDuration.Hours() / 24.0,
		InferredDays:         live.InferredDuration.Hours() / 24.0,
		EstimatedBlindDays:   live.BlindDuration.Hours() / 24.0,
		ExpectedDetectionDay: live.ExpectedBlindTime.Hours() / 24.0,
		LeftCensored:         live.LeftCensored,
		StillExposed:         live.StillExposed,
	}

	var excludedList []string
	for k, v := range in.ExcludedFactors {
		if v {
			excludedList = append(excludedList, k)
		}
	}
	sort.Strings(excludedList)

	observedStamp := ""
	if !in.ObservedAt.IsZero() {
		observedStamp = in.ObservedAt.UTC().Format(time.RFC3339)
	}

	if in.Reachability == ReachabilityUnknown || in.Reachability == "" {
		return ContactPriorEstimate{
			AssetID:         in.AssetID,
			ServiceID:       in.ServiceID,
			ServiceClass:    in.ServiceClass,
			WeaknessClass:   in.WeaknessClass,
			Available:       false,
			SuppressedWhy:   "reachability undetermined: service probe has not reached a conclusion",
			TimeAtRisk:      tar,
			Factors:         []FactorTrace{{ID: "reachability", Name: "reachability", Value: 0, Source: "vantage:probe", EvidenceClass: "measured-state", ObservedAt: observedStamp, State: "unknown"}},
			ExcludedFactors: excludedList,
			Coverage:        in.Coverage,
			Label:           LabelSourced,
			ScopeStatement:  AmbientScopeStatement,
			IsPrior:         true,
		}
	}

	prior := LookupAmbientAttention(in.ServiceClass, in.PackPriors)
	if in.CalibratedPrior != nil {
		prior = *in.CalibratedPrior
		prior.Label = LabelCalibrated
	}

	discEv := filterDiscoverability(in.Discoverability, in.ExcludedFactors)
	barEv := filterBarriers(in.Barriers, in.ExcludedFactors)

	disc, discTraces := ComputeDiscoverability(discEv)
	bar, barTraces := ComputeAccessBarrier(barEv)

	awareDays := live.TotalAwareDays()
	win := SelectWindow(in.WeaknessClass, awareDays)

	if in.Reachability == ReachabilityNotResponding {
		factors := []FactorTrace{
			{ID: "reachability", Name: "reachability", Value: 0, Source: "vantage:probe", EvidenceClass: "measured-state", ObservedAt: observedStamp, State: "not_responding"},
		}
		return ContactPriorEstimate{
			AssetID:         in.AssetID,
			ServiceID:       in.ServiceID,
			ServiceClass:    prior.ServiceClass,
			WeaknessClass:   in.WeaknessClass,
			Available:       true,
			LambdaContact:   Interval{Point: 0, Low: 0, High: 0},
			WindowDays:      win.Days,
			WindowRationale: win.Rationale,
			PContact:        Interval{Point: 0, Low: 0, High: 0},
			TimeAtRisk:      tar,
			Factors:         factors,
			ExcludedFactors: excludedList,
			Coverage:        in.Coverage,
			Label:           prior.Label,
			ScopeStatement:  AmbientScopeStatement,
			IsPrior:         true,
		}
	}

	// R = 1:
	// lambda_contact = lambda_ambient * D * (1 - B)
	// Interval propagation pairs lower ambient/discoverability with higher
	// barrier attenuation for Low, and higher ambient/discoverability with
	// lower barrier attenuation for High.
	lambdaPoint := prior.RatePerDay.Point * disc.Point * (1.0 - bar.Point)
	lambdaLow := prior.RatePerDay.Low * disc.Low * (1.0 - bar.High)
	lambdaHigh := prior.RatePerDay.High * disc.High * (1.0 - bar.Low)

	lambda := Interval{
		Point: lambdaPoint,
		Low:   lambdaLow,
		High:  lambdaHigh,
	}
	pContact := Interval{
		Point: 1.0 - expNeg(lambda.Point*win.Days),
		Low:   1.0 - expNeg(lambda.Low*win.Days),
		High:  1.0 - expNeg(lambda.High*win.Days),
	}

	factors := []FactorTrace{
		{ID: "reachability", Name: "reachability", Value: 1, Source: "vantage:probe", EvidenceClass: "measured-state", ObservedAt: observedStamp, State: "ok"},
		{ID: "ambient:" + string(prior.ServiceClass), Name: "ambient-attention", Value: prior.RatePerDay.Point, Source: prior.Source, EvidenceClass: string(prior.Label), ObservedAt: prior.RetrievedAt, State: "ok"},
	}
	for i := range discTraces {
		discTraces[i].ObservedAt = observedStamp
	}
	for i := range barTraces {
		barTraces[i].ObservedAt = observedStamp
	}
	factors = append(factors, discTraces...)
	factors = append(factors, barTraces...)
	factors = append(factors, FactorTrace{
		ID:            "time-at-risk",
		Name:          "time-at-risk",
		Value:         win.Days,
		Source:        "trawl:exposure-history",
		EvidenceClass: "measurement",
		ObservedAt:    observedStamp,
		State:         "ok",
	})

	return ContactPriorEstimate{
		AssetID:         in.AssetID,
		ServiceID:       in.ServiceID,
		ServiceClass:    prior.ServiceClass,
		WeaknessClass:   in.WeaknessClass,
		Available:       true,
		LambdaContact:   lambda,
		WindowDays:      win.Days,
		WindowRationale: win.Rationale,
		PContact:        pContact,
		TimeAtRisk:      tar,
		Factors:         factors,
		ExcludedFactors: excludedList,
		Coverage:        in.Coverage,
		Label:           prior.Label,
		ScopeStatement:  AmbientScopeStatement,
		IsPrior:         true,
	}
}

func filterDiscoverability(ev DiscoverabilityEvidence, excluded map[string]bool) DiscoverabilityEvidence {
	if len(excluded) == 0 {
		return ev
	}
	out := ev
	if excluded["discoverability:ct"] {
		out.CTLogged = false
	}
	if excluded["discoverability:passive-dns"] {
		out.PassiveDNS = false
	}
	if excluded["discoverability:wildcard"] {
		out.WildcardZone = false
	}
	if excluded["discoverability:axfr"] {
		out.ZoneTransfer = false
	}
	if excluded["discoverability:index"] {
		out.SearchIndex = false
	}
	return out
}

func filterBarriers(ev BarrierEvidence, excluded map[string]bool) BarrierEvidence {
	if len(excluded) == 0 {
		return ev
	}
	out := ev
	if excluded["barrier:waf-cdn"] {
		out.WAFOrCDN = false
	}
	if excluded["barrier:auth"] {
		out.AuthRequired = false
	}
	if excluded["barrier:mtls"] {
		out.MTLS = false
	}
	if excluded["barrier:allowlist"] {
		out.Allowlist = false
	}
	return out
}

// RecomputeWithoutFactor recomputes an EstimateInput after adding factorID to
// ExcludedFactors without mutating the caller's map or any other factor input.
func RecomputeWithoutFactor(in EstimateInput, factorID string) ContactPriorEstimate {
	next := make(map[string]bool, len(in.ExcludedFactors)+1)
	for k, v := range in.ExcludedFactors {
		next[k] = v
	}
	if factorID != "" {
		next[factorID] = true
	}
	in.ExcludedFactors = next
	return ComputeContactEstimate(in)
}
