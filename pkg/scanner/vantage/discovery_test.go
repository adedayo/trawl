package vantage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	vfinding "github.com/adedayo/vantage/pkg/finding"
	vobs "github.com/adedayo/vantage/pkg/observation"

	"github.com/adedayo/trawl/pkg/store"
)

func ctObservation(hosts ...vobs.CTHost) *vfinding.Observation {
	return &vfinding.Observation{CT: &vobs.CT{Domain: "example.com", Source: "crt.sh", Hosts: hosts}}
}

func valuesOf(assets []store.Asset) []string {
	out := make([]string, 0, len(assets))
	for _, a := range assets {
		out = append(out, a.Value)
	}
	return out
}

// TestAResolvingNameBecomesAnAsset is the path the phase exists for: a name
// the operator never entered, disclosed by a log and confirmed by a resolver,
// arriving in the inventory with the provenance that explains it.
func TestAResolvingNameBecomesAnAsset(t *testing.T) {
	at := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	scope := NewScope([]string{"example.com"}, nil)

	assets, _ := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "vpn.example.com", Resolves: true, Issuer: "Let's Encrypt", Expiry: "2026-12-01"},
	), scope, at)

	if len(assets) != 1 {
		t.Fatalf("a resolving in-scope name must enter the inventory; got %v", valuesOf(assets))
	}
	a := assets[0]
	if a.Value != "vpn.example.com" || a.ID != "vpn.example.com" {
		t.Fatalf("the name must be recorded as itself; got %q/%q", a.ID, a.Value)
	}
	if a.Type != store.AssetTypeSubdomain {
		t.Fatalf("a discovered host is a subdomain; got %q", a.Type)
	}
	if a.DiscoverySource != "ct-log" {
		t.Fatalf("provenance decides what an operator does with a surprise; got %q", a.DiscoverySource)
	}
	if a.Status != store.AssetStatusActive || a.Confidence != 1 {
		t.Fatalf("a logged and resolved name is not a guess; got status %q confidence %v", a.Status, a.Confidence)
	}

	var meta map[string]string
	if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
		t.Fatalf("the metadata must be readable: %v", err)
	}
	if meta["issuer"] != "Let's Encrypt" || meta["certificateExpiry"] != "2026-12-01" || meta["log"] != "crt.sh" {
		t.Fatalf("the certificate that disclosed the name must be recorded; got %v", meta)
	}
}

// TestAnUndeterminedNameIsNeitherAdmittedNorDismissed is the rule the whole
// three-state contract exists to protect.
//
// A failed lookup is not an absence. Admitting the name invents an asset out
// of a resolver outage; dropping it silently loses a real one. It does
// neither: the name stays out of the inventory and is reported as unsettled,
// which is the only account the evidence supports.
func TestAnUndeterminedNameIsNeitherAdmittedNorDismissed(t *testing.T) {
	scope := NewScope([]string{"example.com"}, nil)

	assets, undetermined := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "maybe.example.com"},
		vobs.CTHost{Host: "gone.example.com", NXDOMAIN: true},
	), scope, time.Now())

	if len(assets) != 0 {
		t.Fatalf("neither an unresolvable nor a withdrawn name may be invented as an asset; got %v", valuesOf(assets))
	}
	if len(undetermined) != 1 || undetermined[0] != "maybe.example.com" {
		t.Fatalf("an unsettled name must be reported, not swallowed; got %v", undetermined)
	}
}

// TestADefinitiveNXDOMAINIsNotReportedAsAGap separates the two negatives.
//
// A definitive denial is an answer, and reporting it alongside the lookups
// that failed would make every expired certificate read as a coverage gap —
// so the gap list would be permanently non-empty and mean nothing.
func TestADefinitiveNXDOMAINIsNotReportedAsAGap(t *testing.T) {
	scope := NewScope([]string{"example.com"}, nil)

	_, undetermined := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "retired.example.com", NXDOMAIN: true},
	), scope, time.Now())

	if len(undetermined) != 0 {
		t.Fatalf("a definitive absence is an answer, not a gap; got %v", undetermined)
	}
}

// TestANameOutsideScopeIsRefused guards the direction the transport cannot.
//
// A shared certificate names every tenant on it. Admitting those names would
// put a third party's infrastructure in this operator's inventory, and the
// next scheduled run would assess it — the authorisation defeated by a write
// rather than by a query.
func TestANameOutsideScopeIsRefused(t *testing.T) {
	scope := NewScope([]string{"example.com"}, nil)

	assets, undetermined := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "tenant.other-company.test", Resolves: true},
		vobs.CTHost{Host: "unknown.other-company.test"},
		vobs.CTHost{Host: "ok.example.com", Resolves: true},
	), scope, time.Now())

	if v := valuesOf(assets); len(v) != 1 || v[0] != "ok.example.com" {
		t.Fatalf("only authorised names may be recorded; got %v", v)
	}
	if len(undetermined) != 0 {
		t.Fatalf("an out-of-scope name is not our gap to report; got %v", undetermined)
	}
}

// TestNilScopeDiscoversNothing pins the fail-closed default. A caller who
// forgets the scope must discover nothing rather than everything.
func TestNilScopeDiscoversNothing(t *testing.T) {
	assets, _ := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "vpn.example.com", Resolves: true},
	), nil, time.Now())

	if len(assets) != 0 {
		t.Fatalf("an absent authorisation must admit nothing; got %v", valuesOf(assets))
	}
}

// TestTheApexAndWildcardsAreNotDiscoveries keeps the inventory to things that
// exist and can be scanned.
//
// The apex is already an asset with provenance of its own, and re-recording
// it as a CT discovery would overwrite what the operator entered. A wildcard
// is a certificate's coverage, not a host; there is nothing at "*.example.com"
// to assess.
func TestTheApexAndWildcardsAreNotDiscoveries(t *testing.T) {
	scope := NewScope([]string{"example.com"}, nil)

	assets, _ := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "Example.com.", Resolves: true},
		vobs.CTHost{Host: "*.example.com", Resolves: true},
		vobs.CTHost{Host: "real.example.com", Resolves: true},
	), scope, time.Now())

	if v := valuesOf(assets); len(v) != 1 || v[0] != "real.example.com" {
		t.Fatalf("the apex and wildcard identities are not discoveries; got %v", v)
	}
}

// TestRepeatedCertificationYieldsOneAsset. A name is commonly certified many
// times over; counting certificates as assets would inflate the inventory.
func TestRepeatedCertificationYieldsOneAsset(t *testing.T) {
	scope := NewScope([]string{"example.com"}, nil)

	assets, _ := discoveredHosts("example.com", ctObservation(
		vobs.CTHost{Host: "www.example.com", Resolves: true, Issuer: "A"},
		vobs.CTHost{Host: "WWW.example.com.", Resolves: true, Issuer: "B"},
	), scope, time.Now())

	if len(assets) != 1 {
		t.Fatalf("one name is one asset however often it was certified; got %v", valuesOf(assets))
	}
}

// TestTheGapIsNamedOnCoverageWithoutDegradingIt.
//
// The CT check concluded what the logs hold, and it concluded that whether or
// not every disclosed name could afterwards be resolved. Degrading it would
// discard a finding that was genuinely established. But an inventory quietly
// short of names reads as complete, so the shortfall is stated on the record
// that accompanies it.
func TestTheGapIsNamedOnCoverageWithoutDegradingIt(t *testing.T) {
	obs := ctObservation(vobs.CTHost{Host: "maybe.example.com"})
	obs.CT.Discovered = 40

	state, reason := discoveryGap(store.CoverageOK, "", obs, []string{"maybe.example.com"})

	if state != store.CoverageOK {
		t.Fatalf("an unresolvable name is not a failure of the certificate check; got %q", state)
	}
	if !strings.Contains(reason, "maybe.example.com") {
		t.Fatalf("the unsettled name must be named so it can be re-checked; got %q", reason)
	}
	if !strings.Contains(reason, "39") {
		t.Fatalf("names disclosed beyond the enumeration bound must be counted, or the inventory reads as complete; got %q", reason)
	}
}

// TestTheCheckOwnErrorStaysFirst. The reason that explains why a check failed
// is more specific than a note about enumeration, and a reader who sees only
// the note learns about the logs and not about the failure.
func TestTheCheckOwnErrorStaysFirst(t *testing.T) {
	_, reason := discoveryGap(store.CoverageCheckFailed, "the log service was unreachable",
		ctObservation(vobs.CTHost{Host: "maybe.example.com"}), []string{"maybe.example.com"})

	if !strings.HasPrefix(reason, "the log service was unreachable") {
		t.Fatalf("the check's own explanation must not be displaced; got %q", reason)
	}
}

// TestNoObservationMakesNoClaim. A check that never ran produces no
// observation, and an empty discovery set written on its behalf would assert
// that the logs hold nothing.
func TestNoObservationMakesNoClaim(t *testing.T) {
	scope := NewScope([]string{"example.com"}, nil)

	if assets, un := discoveredHosts("example.com", nil, scope, time.Now()); assets != nil || un != nil {
		t.Fatal("an absent observation must yield no discovery at all")
	}
	if assets, _ := discoveredHosts("example.com", &vfinding.Observation{}, scope, time.Now()); assets != nil {
		t.Fatal("an observation with no CT data says nothing about the logs")
	}

	state, reason := discoveryGap(store.CoverageOK, "untouched", nil, nil)
	if state != store.CoverageOK || reason != "untouched" {
		t.Fatalf("coverage must pass through unchanged; got %q/%q", state, reason)
	}
}
