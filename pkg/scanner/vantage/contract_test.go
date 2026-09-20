package vantage

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	vfinding "github.com/adedayo/vantage/pkg/finding"
	"github.com/adedayo/vantage/pkg/netattr"
	vobs "github.com/adedayo/vantage/pkg/observation"
)

// This file is a contract test for the structured-observation surface Trawl
// consumes from vantage. It asserts nothing about Trawl's own behaviour.
//
// Its job is to fail the build when an upstream release changes the shape or
// meaning of what Trawl reads, rather than letting the change surface as a
// wrong answer during a customer's scan. Enrichment reads facts, and a fact
// that quietly stops arriving reads as "nothing observed" — the reassuring
// direction, which is exactly the failure this pins down.

// TestSchemaVersionCarriesStructuredObservations fails if the library is
// downgraded below the release that introduced CheckResult.Observation.
//
// The version is compared numerically rather than by string equality, so a
// later additive release does not fail: pinning to an exact string would make
// every routine upgrade look like a contract break and train a reader to edit
// the number without reading it.
func TestSchemaVersionCarriesStructuredObservations(t *testing.T) {
	var major, minor int
	if _, err := fmt.Sscanf(vfinding.SchemaVersion, "%d.%d", &major, &minor); err != nil {
		t.Fatalf("finding.SchemaVersion %q is not MAJOR.MINOR: %v", vfinding.SchemaVersion, err)
	}
	if major != 1 {
		t.Fatalf("finding schema major version is %d, not 1: the result shape has changed incompatibly and the adapter must be reviewed, not re-pinned", major)
	}
	if minor < 2 {
		t.Fatalf("finding schema is %s; structured email-authentication posture arrived in 1.2, so this build reads DMARC tags and DKIM selector facts that the pinned library does not emit", vfinding.SchemaVersion)
	}
}

// TestEmailObservationFieldsTrawlReadsStillExist names every field the
// email-authentication path depends on, in Go rather than in prose.
//
// The capability's requirements are satisfied by reading these. If one is
// renamed the build stops here rather than in a scan, and if one changes
// meaning the value assertions below catch it — which is the more dangerous
// change, because it keeps compiling.
func TestEmailObservationFieldsTrawlReadsStillExist(t *testing.T) {
	email := vobs.Email{
		Domain: "example.com",
		SPF: &vobs.SPF{
			Presence:            vobs.PresencePublished,
			Record:              "v=spf1 include:_spf.example.net +all",
			AllMechanism:        "+",
			Valid:               true,
			Lookups:             3,
			LookupLimitExceeded: false,
			SendsMail:           true,
		},
		DKIM: &vobs.DKIM{
			SelectorsExamined: []string{"default", "google"},
			SelectorsFound:    []string{"google"},
			UsableKeys:        1,
			Probed:            true,
		},
		DMARC: &vobs.DMARC{
			Presence:           vobs.PresencePublished,
			Policy:             "reject",
			SubdomainPolicy:    "none",
			Percent:            40,
			AlignmentSPF:       "s",
			AlignmentDKIM:      "r",
			AggregateReporting: true,
			RecordCount:        1,
			Valid:              true,
		},
		Adjacent: &vobs.AdjacentRecord{
			Kind:     "mtasts",
			Presence: vobs.PresencePublished,
			Detail:   "testing",
		},
	}

	result := vfinding.CheckResult{
		Check:       "dmarc",
		Target:      "example.com",
		State:       vfinding.StateOK,
		Observation: &vfinding.Observation{Email: &email},
	}

	got := result.Observation.Email
	if got == nil || got.SPF == nil || got.DKIM == nil || got.DMARC == nil || got.Adjacent == nil {
		t.Fatal("every part of the email observation must survive the round trip; a nil part reads as a control that was never assessed")
	}
	if got.SPF.AllMechanism != "+" {
		t.Fatalf("the all-mechanism qualifier is what distinguishes a policy that authorises the internet from one that denies it; got %q", got.SPF.AllMechanism)
	}
	if got.DMARC.Percent != 40 || got.DMARC.SubdomainPolicy != "none" {
		t.Fatalf("deterministic severity is a function of these tags, so each must arrive as data; got pct=%d sp=%q", got.DMARC.Percent, got.DMARC.SubdomainPolicy)
	}
}

// TestPartialEnforcementIsNotReportedAsEnforcement pins the judgement Trawl
// relies on the library to make.
//
// p=reject at pct=40 instructs receivers to reject two in five spoofed
// messages. A reader told "policy: reject" concludes the route is closed. If
// upstream ever relaxed this to test the policy alone, Trawl would start
// reporting partially enforced domains as protected — silently, and in the
// flattering direction.
func TestPartialEnforcementIsNotReportedAsEnforcement(t *testing.T) {
	partial := vobs.DMARC{Policy: "reject", Percent: 40}
	if partial.Enforcing() {
		t.Fatal("a policy applied to part of the mail is partial enforcement; reporting it as enforcement tells a reader a spoofing route is closed when it is open three times in five")
	}

	full := vobs.DMARC{Policy: "reject", Percent: 100}
	if !full.Enforcing() {
		t.Fatal("a reject policy at full percentage is enforcement; failing to credit it would send an operator to fix what is already correct")
	}

	monitor := vobs.DMARC{Policy: "none", Percent: 100}
	if monitor.Enforcing() {
		t.Fatal("monitoring observes spoofed mail; it does not stop it")
	}
}

// TestProbedDKIMAbsenceStaysInconclusive pins the claim the capability spec
// forbids us from making.
//
// Selectors are not enumerable from DNS. Probing the common list and finding
// nothing says nothing about a domain signing with a tenant-specific
// selector, and a CISO told "DKIM is missing" commissions work already done.
func TestProbedDKIMAbsenceStaysInconclusive(t *testing.T) {
	probed := vobs.DKIM{SelectorsExamined: []string{"default", "google"}, Probed: true}
	if probed.Conclusive() {
		t.Fatal("guessing selectors and finding none proves nothing; recording it as an absence would be a plain falsehood")
	}

	named := vobs.DKIM{SelectorsExamined: []string{"acme2026"}}
	if !named.Conclusive() {
		t.Fatal("selectors the operator named are evidence about their own deployment, so their absence is an answer")
	}
}

// TestPresenceKeepsThreeStates. A control that is absent is a decision
// somebody made; a control we could not look for is a gap in our evidence.
// Collapsed into a boolean, an outage presents as a clean bill of health.
func TestPresenceKeepsThreeStates(t *testing.T) {
	if !vobs.PresenceAbsent.Settled() || !vobs.PresencePublished.Settled() {
		t.Fatal("an answered question is settled, whichever way it went")
	}
	if vobs.PresenceUndetermined.Settled() {
		t.Fatal("an unanswered lookup settles nothing, and must never be read as an absent control")
	}
}

// TestObservationFieldsTrawlReadsStillExist names, in Go rather than in prose,
// every field the enrichment path depends on. Removing or renaming one stops
// this file compiling.
//
// Values are asserted as well as names, because a field that survives a
// rename but changes meaning is the more dangerous change: it keeps building.
func TestObservationFieldsTrawlReadsStillExist(t *testing.T) {
	fetched := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)

	network := vobs.Network{
		Domain: "example.com",
		Hosts: []vobs.NetworkHost{{
			Host: "www.example.com",
			Role: "host",
			Attributions: []netattr.Attribution{{
				Address:      netip.MustParseAddr("203.0.113.10"),
				Provider:     "aws",
				Source:       "https://ip-ranges.amazonaws.com/ip-ranges.json",
				Region:       "eu-west-2",
				Jurisdiction: "GB",
			}},
		}},
		Estate:                map[string]bool{"aws": true},
		ExpectedJurisdictions: []string{"GB"},
		FailedSources:         []string{"azure"},
		StaleSources:          []string{"gcp"},
		Provenance: []netattr.SourceProvenance{{
			Provider: "aws",
			URL:      "https://ip-ranges.amazonaws.com/ip-ranges.json",
			Fetched:  fetched,
		}},
	}

	result := vfinding.CheckResult{
		Check:       "net",
		Target:      "example.com",
		State:       vfinding.StateOK,
		Observation: &vfinding.Observation{Network: &network},
	}

	if result.Observation == nil || result.Observation.Network == nil {
		t.Fatal("CheckResult.Observation must carry the network observation; without it the only route to attribution is parsing rendered records")
	}
	got := result.Observation.Network
	if len(got.Hosts) != 1 || got.Hosts[0].Host != "www.example.com" {
		t.Fatalf("network hosts did not survive the round trip: %+v", got.Hosts)
	}
	attr := got.Hosts[0].Attributions[0]
	if attr.Provider != "aws" || attr.Region != "eu-west-2" || attr.Jurisdiction != "GB" {
		t.Fatalf("attribution fields changed shape: %+v", attr)
	}
	if len(got.FailedSources) != 1 || len(got.StaleSources) != 1 {
		t.Fatal("FailedSources and StaleSources must remain distinguishable: a source that could not be loaded is a coverage gap, while a stale one is usable data on an unrefreshed basis, and collapsing them makes an unattributed asset read as clean")
	}
}

// TestObservationIsAbsentRatherThanEmpty pins the pointer.
//
// A value type could not distinguish "this check reported no observation"
// from "this check observed an empty estate". The first is a gap in what we
// know; the second is a claim about the domain. An enrichment path that
// cannot tell them apart writes the claim.
func TestObservationIsAbsentRatherThanEmpty(t *testing.T) {
	var absent vfinding.CheckResult
	if absent.Observation != nil {
		t.Fatal("a CheckResult with no observation must leave the field nil")
	}

	empty := vfinding.CheckResult{Observation: &vfinding.Observation{}}
	if empty.Observation == nil {
		t.Fatal("an observation carrying no network or CT data is still an observation")
	}
	if empty.Observation.Network != nil || empty.Observation.CT != nil {
		t.Fatal("an empty observation must not fabricate sub-observations")
	}
}

// TestUndeterminedKeepsCTResolutionThreeValued pins the three-state result
// that discovery depends on.
//
// Resolves and NXDOMAIN are not negations of each other: a lookup that failed
// leaves both false and licenses no conclusion. If this ever collapses to a
// boolean, a name we could not check would enter the inventory as a name that
// does not exist.
func TestUndeterminedKeepsCTResolutionThreeValued(t *testing.T) {
	live := vobs.CTHost{Host: "a.example.com", Resolves: true}
	gone := vobs.CTHost{Host: "b.example.com", NXDOMAIN: true}
	unknown := vobs.CTHost{Host: "c.example.com"}

	if live.Undetermined() {
		t.Fatal("a resolving name is determined")
	}
	if gone.Undetermined() {
		t.Fatal("a definitive NXDOMAIN is determined")
	}
	if !unknown.Undetermined() {
		t.Fatal("a failed lookup must report as undetermined, not as absence; discovery would otherwise record a name we could not check as one that does not exist")
	}
}

// TestSameBasisIgnoresFetchTimeOnly pins the drift-suppression primitive.
//
// Trawl uses this to answer "did the host move, or did our provider data
// refresh?" before raising a regression. If it started comparing fetch times,
// every refresh would look like a change and the regression signal would be
// noise; if it stopped comparing URLs, a genuine change of basis would be
// suppressed.
func TestSameBasisIgnoresFetchTimeOnly(t *testing.T) {
	monday := []netattr.SourceProvenance{{Provider: "aws", URL: "https://example.invalid/ranges", Fetched: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}}
	tuesday := []netattr.SourceProvenance{{Provider: "aws", URL: "https://example.invalid/ranges", Fetched: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)}}
	fallback := []netattr.SourceProvenance{{Provider: "aws", URL: "https://mirror.invalid/ranges", Fetched: monday[0].Fetched}}

	if !vobs.SameBasis(monday, tuesday) {
		t.Fatal("a re-fetch of the same endpoint is the same basis; treating it as a change would make every refresh look like a regression")
	}
	if vobs.SameBasis(monday, fallback) {
		t.Fatal("data from a different endpoint is a different basis and must not be suppressed")
	}
}

// TestAgeReportsAbsenceRatherThanZero pins absence being distinguishable from
// freshness.
//
// A zero duration means "fetched just now", which is the strongest possible
// claim. Returning it for missing provenance would report the least evidence
// as the best.
func TestAgeReportsAbsenceRatherThanZero(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

	if _, ok := vobs.Age(nil, now); ok {
		t.Fatal("missing provenance has no age; reporting zero would present the absence of evidence as freshly fetched data")
	}

	aged, ok := vobs.Age([]netattr.SourceProvenance{
		{Provider: "aws", Fetched: now.Add(-48 * time.Hour)},
		{Provider: "gcp", Fetched: now.Add(-2 * time.Hour)},
	}, now)
	if !ok {
		t.Fatal("present provenance must report an age")
	}
	if aged != 48*time.Hour {
		t.Fatalf("age must be that of the oldest entry, so a single fresh source cannot vouch for a stale one; got %s", aged)
	}
}
