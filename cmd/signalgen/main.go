// Command signalgen generates Trawl's signal registry from vantage's finding
// catalogue.
//
// The registry maps every identifier vantage can raise onto what it bears on:
// the weakness class, the attack scenario, the kill-chain stage and the control
// it speaks to. Generating the skeleton means a vantage upgrade that adds a
// finding produces a visible, reviewable diff rather than an identifier that
// silently arrives at run time with nowhere to go.
//
// The classification is seeded from the catalogue's check and tags, and is
// then a reviewed artefact: a human corrects anything the heuristic got wrong,
// and the corrections survive regeneration because existing entries are
// preserved. Nothing here decides risk; it only decides what a signal is about.
//
// Usage:
//
//	go run ./cmd/signalgen -out config/signals/vantage-1.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// entry mirrors store.SignalRegistryEntry, kept as its own type so that the
// on-disk schema is deliberate rather than an accident of a Go struct.
type entry struct {
	SignalID      string `json:"signalId"`
	Condition     string `json:"condition"`
	WeaknessClass string `json:"weaknessClass"`
	Scenario      string `json:"scenario"`
	Stage         string `json:"stage"`
	DedupGroup    string `json:"dedupGroup"`
	Control       string `json:"control"`
	Direction     string `json:"direction"`
}

type registry struct {
	RegistryVersion string  `json:"registryVersion"`
	LibraryMajor    string  `json:"libraryMajor"`
	Entries         []entry `json:"entries"`
}

// classification is what a check's findings are about. Grouping by check
// rather than by identifier keeps the table short enough to review, and a
// check's findings do genuinely share a weakness class: they are different
// observations of the same mechanism.
type classification struct {
	weaknessClass string
	scenario      string
	stage         string
	control       string
}

var byCheck = map[string]classification{
	"spf":    {"email-authentication", "email-spoofing", "delivery", "spf"},
	"dmarc":  {"email-authentication", "email-spoofing", "delivery", "dmarc"},
	"dkim":   {"email-authentication", "email-spoofing", "delivery", "dkim"},
	"bimi":   {"email-authentication", "brand-impersonation", "delivery", "bimi"},
	"mx":     {"mail-transport", "email-interception", "delivery", "mx"},
	"mtasts": {"mail-transport", "email-interception", "delivery", "mta-sts"},
	"tlsrpt": {"mail-transport", "email-interception", "delivery", "tls-rpt"},

	"dnssec": {"domain-integrity", "dns-response-forgery", "reconnaissance", "dnssec"},
	"nssec":  {"domain-integrity", "zone-enumeration", "reconnaissance", "nsec3"},
	"caa":    {"certificate-issuance", "certificate-mis-issuance", "reconnaissance", "caa"},
	"ns":     {"delegation-integrity", "domain-hijack", "reconnaissance", "delegation"},
	"axfr":   {"delegation-integrity", "zone-disclosure", "reconnaissance", "zone-transfer"},
	"ptr":    {"delegation-integrity", "attribution-confusion", "reconnaissance", "ptr"},
	"wild":   {"dns-hygiene", "subdomain-takeover", "reconnaissance", "wildcard"},

	"tko": {"subdomain-takeover", "subdomain-takeover", "exploitation", "alias-hygiene"},
	"ct":  {"attack-surface-exposure", "forgotten-asset", "reconnaissance", "certificate-transparency"},
	"net": {"infrastructure-attribution", "jurisdiction-exposure", "reconnaissance", "hosting"},
}

// bySignal corrects individual findings whose check-level default is wrong.
//
// Classifying by check assumes a check's findings all bear on the same attack,
// and that assumption does not survive contact with the catalogue. The mx check
// raises both "this domain can be spoofed because it publishes no null MX" and
// "inbound mail depends on one operator" — a spoofing weakness and an
// availability observation, sharing nothing but the code that found them.
//
// This matters beyond a mislabel. Scenario counts are computed from control
// postures, and a posture turns deficient on any assessed advisory regardless
// of severity. A low resilience note filed under email-interception therefore
// contributes exactly as much to that scenario as a missing MTA-STS policy.
// Filing it under availability does not make it disappear; it makes it count
// towards the thing it is actually evidence of.
//
// The catalogue's own tags were the guide here: they are per-finding and
// hand-written, so where they say TagResilience and the check-level default
// says interception, the tag is the more considered of the two.
var bySignal = map[string]classification{
	// Mail reaching its destination at all, as distinct from reaching it
	// unread. An exchanger that does not resolve, or a single point of
	// failure, denies delivery; neither helps an interceptor.
	"SURF-MX-001": {"mail-delivery", "mail-delivery-failure", "delivery", "mx"},
	"SURF-MX-004": {"mail-delivery", "mail-delivery-failure", "delivery", "mx"},
	"SURF-MX-005": {"mail-delivery", "mail-delivery-failure", "delivery", "mx"},

	// A domain that sends no mail and publishes no null MX is a domain an
	// attacker can plausibly send as. That is spoofing, not interception.
	"SURF-MX-003": {"email-authentication", "email-spoofing", "delivery", "mx"},

	// Delegation resilience: one nameserver, one provider, one /24, or
	// disagreeing serials. These degrade availability of the zone. Hijack is
	// what the rest of the ns check is about, and conflating the two inflates
	// it.
	"SURF-NS-001": {"delegation-integrity", "dns-availability", "reconnaissance", "delegation"},
	"SURF-NS-002": {"delegation-integrity", "dns-availability", "reconnaissance", "delegation"},
	"SURF-NS-003": {"delegation-integrity", "dns-availability", "reconnaissance", "delegation"},
	"SURF-NS-008": {"delegation-integrity", "dns-availability", "reconnaissance", "delegation"},

	// A lame delegation and missing glue both describe resolution degrading:
	// the catalogue's own wording is "delayed or fail outright" and "depends
	// on cached data that will eventually expire". Neither hands anyone the
	// domain.
	"SURF-NS-005": {"delegation-integrity", "dns-availability", "reconnaissance", "delegation"},
	"SURF-NS-006": {"delegation-integrity", "dns-availability", "reconnaissance", "delegation"},

	// An openly recursive authoritative server exposes its cache to poisoning
	// and supplies amplification traffic. That is forged responses, not a
	// hijacked delegation.
	"SURF-NS-007": {"domain-integrity", "dns-response-forgery", "reconnaissance", "delegation"},

	// SURF-NS-004 is deliberately left under domain-hijack. A nameserver named
	// only by the parent "still receives queries even after the operator
	// believes they removed it" — an abandoned delegation an attacker can
	// claim, which is a hijack in the ordinary sense and not merely an
	// availability defect.

	// NSEC permits walking the zone and a low NSEC3 iteration count weakens
	// the same protection. Both disclose names; neither forges a response.
	// The zone-enumeration scenario already exists for the nssec check —
	// routing by check name was the only thing keeping these out of it.
	"SURF-DNSSEC-007": {"domain-integrity", "zone-enumeration", "reconnaissance", "dnssec"},
	"SURF-DNSSEC-008": {"domain-integrity", "zone-enumeration", "reconnaissance", "dnssec"},

	// A public name resolving into RFC 1918 space leaks internal addressing.
	// It says nothing about which jurisdiction anything sits in.
	"SURF-NET-002": {"attack-surface-exposure", "information-disclosure", "reconnaissance", "hosting"},
}

func main() {
	out := flag.String("out", "config/signals/vantage-1.json", "registry file to write")
	version := flag.String("version", "", "registry version to stamp (default: today)")
	check := flag.Bool("check", false, "verify the registry is complete without writing")
	flag.Parse()

	catalogue := vfinding.Catalogue()
	if len(catalogue) == 0 {
		fail("the vantage catalogue is empty; nothing to generate")
	}

	existing := load(*out)
	known := map[string]entry{}
	for _, e := range existing.Entries {
		known[e.SignalID] = e
	}

	var (
		result    []entry
		added     []string
		corrected []string
		unknown   []string
	)
	for _, c := range catalogue {
		override, overridden := bySignal[c.ID]

		// A reviewed entry is never overwritten. Regeneration must not quietly
		// revert a human's correction.
		//
		// An entry named in bySignal is the exception, because bySignal *is*
		// the reviewed correction — it lives in version control and changes
		// only by deliberate edit. Without this, adding an override would have
		// no effect on any identifier already in the registry, which is every
		// identifier we want to correct.
		if e, ok := known[c.ID]; ok {
			if overridden && !matches(e, override) {
				corrected = append(corrected, c.ID)
				e.WeaknessClass = override.weaknessClass
				e.Scenario = override.scenario
				e.Stage = override.stage
				e.Control = override.control
				e.DedupGroup = c.Check + ":" + override.weaknessClass
			}
			result = append(result, e)
			continue
		}
		cls, ok := byCheck[c.Check]
		if !ok {
			unknown = append(unknown, fmt.Sprintf("%s (check %q)", c.ID, c.Check))
			continue
		}
		// A per-signal correction wins over the check-level default, which is
		// only ever a heuristic.
		if overridden {
			cls = override
		}
		added = append(added, c.ID)
		result = append(result, entry{
			SignalID:      c.ID,
			Condition:     c.Title,
			WeaknessClass: cls.weaknessClass,
			Scenario:      cls.scenario,
			Stage:         cls.stage,
			// Findings from one check against one asset describe the same
			// weakness from different angles, so they share a dedup group and
			// are not counted as independent evidence.
			DedupGroup: c.Check + ":" + cls.weaknessClass,
			Control:    cls.control,
			// Every vantage finding reports a control that is absent, weak or
			// misconfigured, so all are aggravating. A mitigating signal would
			// have to come from an observation of a control working, which
			// this catalogue does not currently express.
			Direction: "aggravating",
		})
	}

	if len(unknown) > 0 {
		fail("no classification for check(s) behind these identifiers:\n  %s\n\n"+
			"Add the check to byCheck in cmd/signalgen. Guessing a weakness class "+
			"would put a finding under the wrong scenario, which is worse than failing here.",
			strings.Join(unknown, "\n  "))
	}

	sort.Slice(result, func(i, j int) bool { return result[i].SignalID < result[j].SignalID })

	if *check {
		if len(added) > 0 {
			fail("the registry is missing %d identifier(s) the library can raise:\n  %s\n\n"+
				"Run: go run ./cmd/signalgen", len(added), strings.Join(added, "\n  "))
		}
		if len(corrected) > 0 {
			fail("the registry disagrees with bySignal for %d identifier(s):\n  %s\n\n"+
				"Run: go run ./cmd/signalgen", len(corrected), strings.Join(corrected, "\n  "))
		}
		fmt.Printf("registry is complete: %d identifiers\n", len(result))
		return
	}

	reg := registry{
		RegistryVersion: *version,
		LibraryMajor:    "1",
		Entries:         result,
	}
	if reg.RegistryVersion == "" {
		reg.RegistryVersion = existing.RegistryVersion
		// A correction changes what a stored observation means, so it must
		// bump the version too. Observations record the version they were
		// interpreted under, and that is what lets a change in interpretation
		// be told apart from a change in the domain.
		if reg.RegistryVersion == "" || len(added) > 0 || len(corrected) > 0 {
			reg.RegistryVersion = nextVersion(existing.RegistryVersion)
		}
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		fail("encoding the registry: %v", err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		fail("writing %s: %v", *out, err)
	}
	fmt.Printf("wrote %s: %d identifiers (%d new, %d corrected)\n", *out, len(result), len(added), len(corrected))
}

func load(path string) registry {
	var r registry
	data, err := os.ReadFile(path)
	if err != nil {
		return r
	}
	if err := json.Unmarshal(data, &r); err != nil {
		fail("the existing registry at %s is not valid JSON: %v", path, err)
	}
	return r
}

// nextVersion bumps the registry version. The version is recorded on every
// observation, so that a change in how a signal is interpreted can be told
// apart from a change in the domain being observed.
func nextVersion(current string) string {
	var n int
	if _, err := fmt.Sscanf(current, "v%d", &n); err != nil {
		n = 0
	}
	return fmt.Sprintf("v%d", n+1)
}

// matches reports whether an existing entry already carries a classification.
func matches(e entry, c classification) bool {
	return e.WeaknessClass == c.weaknessClass &&
		e.Scenario == c.scenario &&
		e.Stage == c.stage &&
		e.Control == c.control
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "signalgen: "+format+"\n", args...)
	os.Exit(1)
}
