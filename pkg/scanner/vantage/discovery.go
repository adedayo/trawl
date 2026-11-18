package vantage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	vfinding "github.com/adedayo/vantage/pkg/finding"

	"github.com/adedayo/trawl/pkg/store"
)

// ctLogSource is the discovery source recorded against a name that reached
// the inventory by way of Certificate Transparency.
//
// It is recorded rather than folded into "assessment" because provenance
// decides what an operator does with a surprise. A name they do not recognise
// means one thing if we resolved it ourselves and another if a certificate
// authority published it — the second says somebody in the organisation
// obtained a certificate for it, which is a lead rather than a mystery.
const ctLogSource = "ct-log"

// discoveredHosts turns certificate-transparency and multi-source external
// discovery observations into inventory assets, and reports the names whose
// existence could not be established.
//
// Three states arrive and three states leave. A name that resolves is an
// asset. A name that returned a definitive NXDOMAIN ("absent" in
// Observation.Discovery) is not: certificates and passive host records outlive
// the things they were issued for, and admitting every expired hostname would
// grow the inventory monotonically with names that no longer exist. A name
// that could neither be resolved nor definitively denied is neither — it is
// returned separately, so that the gap is reported instead of being resolved
// by assumption in whichever direction happens to be convenient.
//
// Every name is filtered through the authorised scope. A certificate or
// passive host dataset can disclose hosts belonging to other parties;
// admitting those would put somebody else's infrastructure into this
// operator's inventory, and the next scheduled scan would assess it. Scope is
// the same guard the transport uses, so a name that could not have been
// queried cannot be recorded either.
func discoveredHosts(domain string, obs *vfinding.Observation, scope *Scope, at time.Time) (assets []store.Asset, undetermined []string) {
	if obs == nil || (obs.CT == nil && obs.Discovery == nil) {
		return nil, nil
	}

	apex := normalise(domain)
	seen := make(map[string]bool)
	unknown := make(map[string]bool)

	if obs.CT != nil {
		for _, h := range obs.CT.Hosts {
			name := normalise(h.Host)

			// A wildcard identity is a certificate's coverage, not a host. There
			// is nothing at "*.example.com" to scan, and recording one would put
			// an unqueryable name into the inventory permanently.
			if name == "" || strings.Contains(name, "*") {
				continue
			}
			// The apex is already the asset this assessment is attached to.
			// Re-recording it as a CT discovery would overwrite the provenance of
			// the asset the operator themselves entered.
			if name == apex {
				continue
			}
			if !scope.PermitsTarget(name) {
				continue
			}

			if h.Undetermined() {
				unknown[name] = true
				continue
			}
			if !h.Resolves {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			assets = append(assets, store.Asset{
				ID:              name,
				Type:            store.AssetTypeSubdomain,
				Value:           name,
				Status:          store.AssetStatusActive,
				DiscoverySource: ctLogSource,
				Confidence:      1,
				FirstSeen:       at,
				LastSeen:        at,
				Metadata:        ctMetadata(obs.CT.Source, h.Issuer, h.Expiry),
			})
		}
	}

	if obs.Discovery != nil {
		for _, h := range obs.Discovery.Hosts {
			name := normalise(h.Name)
			if name == "" || strings.Contains(name, "*") || name == apex {
				continue
			}
			if !scope.PermitsTarget(name) {
				continue
			}
			switch h.Status {
			case "unknown":
				unknown[name] = true
				continue
			case "resolves":
			default:
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			sourceLabel := discoverySourceLabel(h.Sources)
			assets = append(assets, store.Asset{
				ID:              name,
				Type:            store.AssetTypeSubdomain,
				Value:           name,
				Status:          store.AssetStatusActive,
				DiscoverySource: sourceLabel,
				Confidence:      1,
				FirstSeen:       at,
				LastSeen:        at,
				Metadata:        externalDiscoveryMetadata(h.Sources),
			})
		}
	}

	sort.Slice(assets, func(i, j int) bool { return assets[i].Value < assets[j].Value })
	return assets, sortedKeys(unknown)
}

func discoverySourceLabel(sources []string) string {
	if len(sources) == 0 {
		return "vantage-discovery"
	}
	hasCT := false
	hasPassive := false
	hasActive := false
	for _, s := range sources {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "certspotter", "crt.sh", "crtsh":
			hasCT = true
		case "hackertarget":
			hasPassive = true
		case "active":
			hasActive = true
		}
	}
	switch {
	case hasCT && !hasPassive && !hasActive:
		return ctLogSource
	case hasActive && !hasCT && !hasPassive:
		return "active-discovery"
	case hasPassive && !hasCT:
		return "passive-discovery"
	default:
		return "vantage-discovery"
	}
}

func externalDiscoveryMetadata(sources []string) string {
	payload := map[string]any{
		"source":  "vantage-discovery",
		"sources": sources,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(b)
}

// ctMetadata records the log service and the certificate that disclosed a
// name, so an operator meeting an unfamiliar asset can see who certified it
// and when that certificate lapses.
//
// Marshalling cannot fail for a map of strings, so the error is discarded
// rather than propagated into a signature that would then have to be handled
// at every call site for a case that does not occur.
func ctMetadata(source, issuer, expiry string) string {
	fields := map[string]string{"source": ctLogSource}
	if source != "" {
		fields["log"] = source
	}
	if issuer != "" {
		fields["issuer"] = issuer
	}
	if expiry != "" {
		fields["certificateExpiry"] = expiry
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	return string(b)
}

// discoveryGap annotates a check's coverage with what enumeration disclosed
// but could not settle.
//
// Two gaps are reported, and neither degrades the state. The CT check's
// conclusion is about what the logs hold, and it reached that conclusion
// whether or not every disclosed name could afterwards be resolved; marking
// it check_failed would discard a finding that was genuinely established.
// But an inventory that is quietly short of names is an inventory an operator
// will read as complete, so the shortfall is named on the record that
// accompanies it.
//
//   - undetermined names: disclosed by a log, and the resolver neither
//     confirmed nor denied them. They are not in the inventory and their
//     absence means nothing.
//   - unresolved names: disclosed, but enumeration stopped at its bound
//     before reaching them. The logs held more than was examined.
func discoveryGap(state store.CoverageState, reason string, obs *vfinding.Observation, undetermined []string) (store.CoverageState, string) {
	if obs == nil || (obs.CT == nil && obs.Discovery == nil) {
		return state, reason
	}

	var notes []string
	if obs.CT != nil {
		if len(undetermined) > 0 {
			notes = append(notes, fmt.Sprintf(
				"%d name(s) disclosed by certificate transparency could not be resolved or denied and are absent from the inventory: %s",
				len(undetermined), strings.Join(undetermined, ", ")))
		}
		if extra := obs.CT.Discovered - len(obs.CT.Hosts); extra > 0 {
			notes = append(notes, fmt.Sprintf(
				"%d further name(s) were disclosed but not resolved, because enumeration stopped at its bound",
				extra))
		}
	}
	if obs.Discovery != nil {
		if obs.CT == nil && len(undetermined) > 0 {
			notes = append(notes, fmt.Sprintf(
				"%d name(s) disclosed by external discovery could not be resolved or denied and are absent from the inventory: %s",
				len(undetermined), strings.Join(undetermined, ", ")))
		}
		if obs.Discovery.Omitted > 0 {
			notes = append(notes, fmt.Sprintf(
				"%d candidate name(s) were omitted at the discovery validation bound",
				obs.Discovery.Omitted))
		}
		if obs.Discovery.ActiveSuppressed {
			notes = append(notes, "active DNS candidate generation was suppressed due to wildcard or inconclusive wildcard probe")
		}
		for _, src := range obs.Discovery.Sources {
			if src.Error != "" {
				notes = append(notes, fmt.Sprintf("source %s failed: %s", src.Name, src.Error))
			}
			notes = append(notes, src.Warnings...)
		}
		notes = append(notes, obs.Discovery.Warnings...)
	}

	return state, joinReasons(reason, notes...)
}

func sortedKeys(in map[string]bool) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
