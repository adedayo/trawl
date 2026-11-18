package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/store"
)

// AssetContactPanel holds the per-asset contact prior estimates and portfolio
// weakness-instance aggregations derived strictly from measured reachability,
// exposure history, and DNS/attribution observations.
type AssetContactPanel struct {
	AssetID        string                         `json:"assetId"`
	AssetValue     string                         `json:"assetValue"`
	Reachability   contact.ReachabilityState      `json:"reachability"`
	Estimates      []contact.ContactPriorEstimate `json:"estimates"`
	ScopeStatement string                         `json:"scopeStatement"`
}

var wafCDNProviders = map[string]bool{
	"cloudflare": true,
	"fastly":     true,
}

// ContactPanel computes the exposure-derived contact priors for one asset,
// applying any excludedFactors requested by the operator for one-step what-if
// recomputation.
func (svc *AssessmentService) ContactPanel(ctx context.Context, assetID string, excludedFactors []string, now time.Time) (AssetContactPanel, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	asset, err := svc.store.GetAssetByID(ctx, assetID)
	if err != nil {
		return AssetContactPanel{}, err
	}
	assetValue := assetID
	discoverySource := ""
	if asset != nil {
		assetValue = asset.Value
		discoverySource = asset.DiscoverySource
	}

	coverageRows, err := svc.store.GetAssessmentCoverage(ctx, assetID)
	if err != nil {
		return AssetContactPanel{}, err
	}
	signalRows, err := svc.store.GetSignalObservations(ctx, assetID)
	if err != nil {
		return AssetContactPanel{}, err
	}
	attributionRows, err := svc.store.GetAssetAttribution(ctx, assetID)
	if err != nil {
		return AssetContactPanel{}, err
	}
	serviceObs, err := svc.store.GetServiceObservations(ctx, assetID)
	if err != nil {
		return AssetContactPanel{}, err
	}

	excluded := make(map[string]bool, len(excludedFactors))
	for _, f := range excludedFactors {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			excluded[trimmed] = true
		}
	}

	return buildAssetContactPanel(assetID, assetValue, discoverySource, coverageRows, signalRows, attributionRows, serviceObs, excluded, now), nil
}

// WeaknessAggregations enumerates weakness instances across tracked inventory
// and aggregates heterogeneous contact priors alongside confirmed remediation
// coverage and residual exposure.
func (svc *AssessmentService) WeaknessAggregations(ctx context.Context, now time.Time) ([]contact.InstanceAggregation, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	assets, err := svc.store.GetAssets(ctx, "")
	if err != nil {
		return nil, err
	}
	coverageRows, err := svc.store.GetAssessmentCoverage(ctx, "")
	if err != nil {
		return nil, err
	}
	signalRows, err := svc.store.GetSignalObservations(ctx, "")
	if err != nil {
		return nil, err
	}
	attributionRows, err := svc.store.GetAssetAttribution(ctx, "")
	if err != nil {
		return nil, err
	}
	serviceObs, err := svc.store.GetServiceObservations(ctx, "")
	if err != nil {
		return nil, err
	}

	covByAsset := map[string][]store.AssessmentCoverage{}
	for _, c := range coverageRows {
		covByAsset[c.AssetID] = append(covByAsset[c.AssetID], c)
	}
	sigByAsset := map[string][]store.SignalObservation{}
	for _, o := range signalRows {
		sigByAsset[o.AssetID] = append(sigByAsset[o.AssetID], o)
	}
	attrByAsset := map[string][]store.AssetAttribution{}
	for _, a := range attributionRows {
		attrByAsset[a.AssetID] = append(attrByAsset[a.AssetID], a)
	}
	svcByAsset := map[string][]store.ServiceObservation{}
	for _, o := range serviceObs {
		svcByAsset[o.AssetID] = append(svcByAsset[o.AssetID], o)
	}

	byWeakness := map[string][]contact.WeaknessInstance{}
	for _, asset := range assets {
		obs := svcByAsset[asset.ID]
		if len(obs) == 0 {
			continue
		}
		grouped := groupServiceObservations(obs)
		keys := make([]string, 0, len(grouped))
		for k := range grouped {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		disc := deriveDiscoverabilityEvidence(asset.DiscoverySource, sigByAsset[asset.ID])
		covFig := deriveCoverageFigure(covByAsset[asset.ID], obs)

		for _, key := range keys {
			series := grouped[key]
			hist, reach, latest, everExposed := deriveServiceHistoryAndReachability(series, now, 24*time.Hour)
			if !everExposed {
				continue
			}
			svcClass := classifyFromObservation(latest)
			barriers := deriveBarrierEvidence(series, attrByAsset[asset.ID])
			remediated := !hist.StillExposed && reach == contact.ReachabilityNotResponding

			// For a confirmed-remediated instance we still evaluate the prior
			// that applied while it was reachable so AllInstances vs Residual
			// shows the exact reduction earned by closing it.
			evalReach := reach
			if remediated {
				evalReach = contact.ReachabilityResponding
			}
			est := contact.ComputeContactEstimate(contact.EstimateInput{
				AssetID:         asset.ID,
				ServiceID:       key,
				ServiceClass:    svcClass,
				WeaknessClass:   "service-exposure",
				Reachability:    evalReach,
				Discoverability: disc,
				Barriers:        barriers,
				History:         hist,
				Now:             now,
				Coverage:        covFig,
				ObservedAt:      latest.ObservedAt,
			})
			byWeakness["service-exposure"] = append(byWeakness["service-exposure"], contact.WeaknessInstance{
				InstanceID:          asset.ID + ":" + key,
				AssetID:             asset.ID,
				ServiceID:           key,
				ConfirmedRemediated: remediated,
				Estimate:            est,
			})
		}
	}

	classes := make([]string, 0, len(byWeakness))
	for k := range byWeakness {
		classes = append(classes, k)
	}
	sort.Strings(classes)
	out := make([]contact.InstanceAggregation, 0, len(classes))
	for _, cls := range classes {
		out = append(out, contact.AggregateInstances(cls, byWeakness[cls]))
	}
	return out, nil
}

func buildAssetContactPanel(
	assetID, assetValue, discoverySource string,
	coverageRows []store.AssessmentCoverage,
	signalRows []store.SignalObservation,
	attributionRows []store.AssetAttribution,
	serviceObs []store.ServiceObservation,
	excluded map[string]bool,
	now time.Time,
) AssetContactPanel {
	disc := deriveDiscoverabilityEvidence(discoverySource, signalRows)
	covFig := deriveCoverageFigure(coverageRows, serviceObs)

	var respondingCount, notRespondingCount, concludedCount int
	for _, o := range serviceObs {
		if !o.Coverage.Assessed() {
			continue
		}
		concludedCount++
		switch o.State {
		case "responding":
			respondingCount++
		case "not_responding":
			notRespondingCount++
		}
	}
	assetReach := contact.DeriveReachability(respondingCount, notRespondingCount, concludedCount)

	if len(serviceObs) == 0 || concludedCount == 0 {
		suppressed := contact.ComputeContactEstimate(contact.EstimateInput{
			AssetID:         assetID,
			ServiceID:       assetValue,
			ServiceClass:    contact.ServiceClassOtherTCP,
			WeaknessClass:   "service-exposure",
			Reachability:    contact.ReachabilityUnknown,
			Discoverability: disc,
			Now:             now,
			Coverage:        covFig,
			ExcludedFactors: excluded,
		})
		return AssetContactPanel{
			AssetID:        assetID,
			AssetValue:     assetValue,
			Reachability:   contact.ReachabilityUnknown,
			Estimates:      []contact.ContactPriorEstimate{suppressed},
			ScopeStatement: contact.AmbientScopeStatement,
		}
	}

	grouped := groupServiceObservations(serviceObs)
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	estimates := make([]contact.ContactPriorEstimate, 0, len(keys))
	for _, key := range keys {
		series := grouped[key]
		hist, reach, latest, everExposed := deriveServiceHistoryAndReachability(series, now, 24*time.Hour)
		if reach != contact.ReachabilityResponding && !everExposed {
			continue
		}
		est := contact.ComputeContactEstimate(contact.EstimateInput{
			AssetID:         assetID,
			ServiceID:       key,
			ServiceClass:    classifyFromObservation(latest),
			WeaknessClass:   "service-exposure",
			Reachability:    reach,
			Discoverability: disc,
			Barriers:        deriveBarrierEvidence(series, attributionRows),
			History:         hist,
			Now:             now,
			Coverage:        covFig,
			ExcludedFactors: excluded,
			ObservedAt:      latest.ObservedAt,
		})
		estimates = append(estimates, est)
	}

	if len(estimates) == 0 {
		// Probes concluded and every probed endpoint refused connections: R = 0.
		closed := contact.ComputeContactEstimate(contact.EstimateInput{
			AssetID:         assetID,
			ServiceID:       assetValue,
			ServiceClass:    contact.ServiceClassOtherTCP,
			WeaknessClass:   "service-exposure",
			Reachability:    contact.ReachabilityNotResponding,
			Discoverability: disc,
			Now:             now,
			Coverage:        covFig,
			ExcludedFactors: excluded,
		})
		estimates = append(estimates, closed)
	}

	return AssetContactPanel{
		AssetID:        assetID,
		AssetValue:     assetValue,
		Reachability:   assetReach,
		Estimates:      estimates,
		ScopeStatement: contact.AmbientScopeStatement,
	}
}

func groupServiceObservations(obs []store.ServiceObservation) map[string][]store.ServiceObservation {
	out := map[string][]store.ServiceObservation{}
	for _, o := range obs {
		key := serviceObservationKey(o)
		out[key] = append(out[key], o)
	}
	for k := range out {
		series := out[k]
		sort.SliceStable(series, func(i, j int) bool { return series[i].ObservedAt.Before(series[j].ObservedAt) })
		out[k] = series
	}
	return out
}

func deriveServiceHistoryAndReachability(series []store.ServiceObservation, now time.Time, cadence time.Duration) (contact.History, contact.ReachabilityState, store.ServiceObservation, bool) {
	histObs := make([]contact.Observation, 0, len(series))
	everExposed := false
	var latest store.ServiceObservation
	latestReach := contact.ReachabilityUnknown

	for _, o := range series {
		latest = o
		state := contact.Unknown
		successful := o.Coverage.Assessed()
		if o.State == "responding" && successful {
			state = contact.Exposed
			everExposed = true
			latestReach = contact.ReachabilityResponding
		} else if o.State == "not_responding" && successful {
			state = contact.Clear
			latestReach = contact.ReachabilityNotResponding
		}
		histObs = append(histObs, contact.Observation{
			At:         o.ObservedAt,
			State:      state,
			Successful: successful,
		})
	}
	hist, _ := contact.ComputeHistory(histObs, now, cadence)
	return hist, latestReach, latest, everExposed
}

func classifyFromObservation(o store.ServiceObservation) contact.ServiceClass {
	var port uint16
	if o.Port > 0 && o.Port <= 65535 {
		port = uint16(o.Port)
	}
	return contact.ClassifyService(o.Service, port, o.Protocol)
}

func deriveDiscoverabilityEvidence(discoverySource string, signals []store.SignalObservation) contact.DiscoverabilityEvidence {
	var ev contact.DiscoverabilityEvidence
	src := strings.ToLower(strings.TrimSpace(discoverySource))
	if strings.Contains(src, "ct") {
		ev.CTLogged = true
	}
	if strings.Contains(src, "subfinder") || strings.Contains(src, "passive") || strings.Contains(src, "hackertarget") || strings.Contains(src, "discovery") {
		ev.PassiveDNS = true
	}
	for _, sig := range signals {
		if !sig.State.Assessed() {
			continue
		}
		switch {
		case strings.HasPrefix(sig.SignalID, "SURF-CT-"):
			ev.CTLogged = true
		case strings.HasPrefix(sig.SignalID, "SURF-WILD-"):
			ev.WildcardZone = true
		case strings.HasPrefix(sig.SignalID, "SURF-AXFR-001"):
			ev.ZoneTransfer = true
		}
	}
	return ev
}

func deriveBarrierEvidence(series []store.ServiceObservation, attributions []store.AssetAttribution) contact.BarrierEvidence {
	var ev contact.BarrierEvidence
	for _, a := range attributions {
		if wafCDNProviders[strings.ToLower(strings.TrimSpace(a.Provider))] {
			ev.WAFOrCDN = true
		}
	}
	for _, o := range series {
		if o.Evidence == "" {
			continue
		}
		var parsed map[string]any
		if json.Unmarshal([]byte(o.Evidence), &parsed) != nil {
			continue
		}
		status := statusFromEvidence(parsed["http_status"])
		if status == 401 || status == 403 || status == 407 {
			ev.AuthRequired = true
		}
		server := strings.ToLower(fmt.Sprint(parsed["server"]))
		if strings.Contains(server, "cloudflare") || strings.Contains(server, "akamai") || strings.Contains(server, "fastly") || strings.Contains(server, "cloudfront") {
			ev.WAFOrCDN = true
		}
		if clientAuth, ok := parsed["client_auth_required"].(bool); ok && clientAuth {
			ev.MTLS = true
		}
	}
	return ev
}

func statusFromEvidence(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func deriveCoverageFigure(coverage []store.AssessmentCoverage, serviceObs []store.ServiceObservation) contact.CoverageFigure {
	var fig contact.CoverageFigure
	for _, c := range coverage {
		fig.ChecksApplicable++
		if c.State.Assessed() {
			fig.ChecksConcluded++
		}
	}
	if len(serviceObs) > 0 {
		fig.ChecksApplicable++
		for _, o := range serviceObs {
			if o.Coverage.Assessed() {
				fig.ChecksConcluded++
				break
			}
		}
	}
	if fig.ChecksApplicable == 0 {
		// The service probe itself is applicable even before it runs.
		fig.ChecksApplicable = 1
	}
	return fig
}
