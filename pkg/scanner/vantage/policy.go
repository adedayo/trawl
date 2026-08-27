package vantage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	vaudit "github.com/adedayo/vantage/pkg/audit"

	"github.com/adedayo/trawl/pkg/store"
)

// EgressClass is a category of network contact a check may require.
//
// Deployment policy is stated over these classes rather than over check names,
// and the difference is what happens on upgrade. A configuration listing
// forbidden check names is a deny list: a check added upstream is absent from
// it and therefore runs. A configuration listing permitted egress classes is
// an allow list over a closed vocabulary: a new check declares the classes it
// needs, and if any of them was not permitted it is excluded without anyone
// having to notice it was added.
type EgressClass string

const (
	// ClassResolver is an ordinary query through a recursive resolver.
	ClassResolver EgressClass = "resolver"
	// ClassTargetNameservers is a query addressed directly to the target's own
	// authoritative servers, bypassing the recursive resolver. It is visible
	// to the target in a way an ordinary query is not.
	ClassTargetNameservers EgressClass = "target-nameservers"
	// ClassTargetHTTPS is an HTTPS request to the target's own infrastructure,
	// such as MTA-STS policy retrieval.
	ClassTargetHTTPS EgressClass = "target-https"
	// ClassThirdParty is contact with a service that is neither the
	// operator's nor the target's. Permitting the class is necessary but not
	// sufficient: each service must also be consented to by name.
	ClassThirdParty EgressClass = "third-party"
	// ClassIntrusive is contact beyond an ordinary query — a zone transfer
	// request, and anything similar added later. Nothing in vantage is
	// destructive; the class marks what an operator would want to agree to
	// before it happens.
	ClassIntrusive EgressClass = "intrusive"
	// ClassOffline is no network contact at all. It is a class rather than an
	// absence so that "this check touches nothing" is something a check states
	// and the policy schema recognises, instead of being the accidental
	// meaning of a check that declared nothing.
	ClassOffline EgressClass = "offline"
)

// EgressClasses returns every class the policy schema recognises, sorted.
//
// The egress-conformance check compares this vocabulary against what the
// library's checks actually declare. A class vantage declares that does not
// appear here fails the build, which is the only way an upgrade introducing an
// unrecognised kind of contact can be prevented from being silently permitted
// or, just as bad, silently dropped.
func EgressClasses() []EgressClass {
	return []EgressClass{
		ClassIntrusive,
		ClassOffline,
		ClassResolver,
		ClassTargetHTTPS,
		ClassTargetNameservers,
		ClassThirdParty,
	}
}

// Known reports whether the class is part of the policy schema.
func (c EgressClass) Known() bool {
	for _, k := range EgressClasses() {
		if c == k {
			return true
		}
	}
	return false
}

// EgressPolicy is a deployment's statement of what contact it permits.
//
// It deliberately contains no check names. The set of checks to run is derived
// from it by filtering the library's catalogue, so policy and mechanism share
// one definition and cannot drift apart.
type EgressPolicy struct {
	// AllowedClasses are the egress classes this deployment permits. A check
	// requiring any class not listed here is excluded.
	AllowedClasses []EgressClass `json:"allowedClasses"`

	// ConsentedServices are the third-party services the operator has agreed
	// to disclose assessed names to. Permitting ClassThirdParty without naming
	// a service permits nothing: consent is per service, so that a check added
	// later which contacts something new cannot be mistaken for one already
	// agreed to.
	ConsentedServices []vaudit.ThirdPartyService `json:"consentedServices"`
}

// DefaultEgressPolicy is what a deployment gets when it says nothing.
//
// Ordinary resolution, HTTPS to the target's own infrastructure, and checks
// needing no network. Not direct contact with the target's nameservers, not
// anything intrusive, and no third party — those are the classes where a
// reasonable operator would want to have been asked.
func DefaultEgressPolicy() EgressPolicy {
	return EgressPolicy{
		AllowedClasses: []EgressClass{ClassOffline, ClassResolver, ClassTargetHTTPS},
	}
}

// Validate reports whether the policy is expressed in the schema's vocabulary.
//
// An unrecognised class or service is an error rather than something ignored.
// A typo in a configuration file that silently narrows assessment would
// present as a clean domain, which is the failure this whole model exists to
// prevent.
func (p EgressPolicy) Validate() error {
	for _, c := range p.AllowedClasses {
		if !c.Known() {
			return fmt.Errorf("vantage: unrecognised egress class %q; known classes are %s",
				c, joinClasses(EgressClasses()))
		}
	}
	for _, s := range p.ConsentedServices {
		if !s.Known() {
			return fmt.Errorf("vantage: unrecognised third-party service %q", s)
		}
	}
	return nil
}

func (p EgressPolicy) allows(c EgressClass) bool {
	for _, a := range p.AllowedClasses {
		if a == c {
			return true
		}
	}
	return false
}

func (p EgressPolicy) consents(s vaudit.ThirdPartyService) bool {
	for _, c := range p.ConsentedServices {
		if c == s {
			return true
		}
	}
	return false
}

// ClassesOf reports the egress classes a declared profile requires.
//
// An undeclared profile returns nothing, which excludes the check: a check
// that forgot to say what it touches is treated as unpermitted rather than
// unrestricted.
func ClassesOf(e vaudit.EgressProfile) []EgressClass {
	if !e.Declared() {
		return nil
	}
	var out []EgressClass
	if e.Offline {
		out = append(out, ClassOffline)
	}
	if e.Resolver {
		out = append(out, ClassResolver)
	}
	if e.TargetNameservers {
		out = append(out, ClassTargetNameservers)
	}
	if e.TargetHTTPS {
		out = append(out, ClassTargetHTTPS)
	}
	if len(e.ThirdParty) > 0 {
		out = append(out, ClassThirdParty)
	}
	if e.Intrusive {
		out = append(out, ClassIntrusive)
	}
	return out
}

// Exclusion records a check the policy refused, and why.
//
// The reason names the excluding class or service in operator-facing terms.
// An exclusion nobody can act on is only marginally better than a check that
// silently did not run.
type Exclusion struct {
	CheckID string `json:"checkId"`
	Reason  string `json:"reason"`
}

// Permits reports whether the policy allows a check to run, and if not, why.
func (p EgressPolicy) Permits(d vaudit.Description) (bool, string) {
	if !d.Egress.Declared() {
		return false, fmt.Sprintf(
			"check %q declares no egress profile, so what it would contact is unknown", d.Name)
	}

	for _, c := range ClassesOf(d.Egress) {
		if !c.Known() {
			return false, fmt.Sprintf(
				"check %q requires egress class %q, which this build's policy schema does not recognise",
				d.Name, c)
		}
		if !p.allows(c) {
			return false, fmt.Sprintf(
				"check %q requires egress class %q, which this deployment has not permitted",
				d.Name, c)
		}
	}

	// Class permission is not service consent. A deployment may be happy to
	// contact third parties in general and still not want this portfolio's
	// names disclosed to a particular one.
	for _, s := range d.Egress.ThirdParty {
		if !p.consents(s) {
			return false, fmt.Sprintf(
				"check %q contacts third-party service %q (%s), which this deployment has not consented to",
				d.Name, s, strings.Join(s.Endpoints(), ", "))
		}
	}

	return true, ""
}

// Selection is the outcome of filtering a catalogue through a policy: the
// checks to request, the checks refused, and the transport permissions those
// requested checks imply.
type Selection struct {
	// Checks are the check identifiers to request, sorted.
	Checks []string `json:"checks"`
	// Excluded are the checks the policy refused, sorted by check, each with
	// the reason it was refused.
	Excluded []Exclusion `json:"excluded"`
	// Services are the third-party services the selected checks actually need,
	// sorted. This is derived from the selected checks rather than copied from
	// the policy's consent list, so consenting to a service the requested
	// checks do not use grants the transport nothing.
	Services []vaudit.ThirdPartyService `json:"services"`
	// TargetNameservers reports whether any selected check addresses the
	// target's authoritative servers directly.
	TargetNameservers bool `json:"targetNameservers"`
	// TargetHTTPS reports whether any selected check makes HTTPS requests to
	// target infrastructure.
	TargetHTTPS bool `json:"targetHttps"`
}

// SelectChecks filters a capability manifest through a policy.
//
// Every check in the catalogue appears in exactly one of Checks or Excluded,
// so the result accounts for the whole catalogue and a check cannot go missing
// between the two.
func SelectChecks(caps vaudit.Capabilities, p EgressPolicy) (Selection, error) {
	if err := p.Validate(); err != nil {
		return Selection{}, err
	}

	var sel Selection
	services := map[vaudit.ThirdPartyService]bool{}

	for _, c := range caps.Checks {
		ok, reason := p.Permits(c.Description)
		if !ok {
			sel.Excluded = append(sel.Excluded, Exclusion{CheckID: c.Name, Reason: reason})
			continue
		}
		sel.Checks = append(sel.Checks, c.Name)
		if c.Egress.TargetNameservers {
			sel.TargetNameservers = true
		}
		if c.Egress.TargetHTTPS {
			sel.TargetHTTPS = true
		}
		for _, s := range c.Egress.ThirdParty {
			services[s] = true
		}
	}

	for s := range services {
		sel.Services = append(sel.Services, s)
	}

	sort.Strings(sel.Checks)
	sort.Slice(sel.Excluded, func(i, j int) bool { return sel.Excluded[i].CheckID < sel.Excluded[j].CheckID })
	sort.Slice(sel.Services, func(i, j int) bool { return sel.Services[i] < sel.Services[j] })
	return sel, nil
}

// EndpointAllowlist returns the hostnames the transport should permit besides
// in-scope targets, derived from the profiles of the selected checks.
//
// Derivation from the selection rather than from the policy is deliberate: the
// transport's permission set should be the smallest one that lets the
// requested work happen. If the checks that use a consented service are all
// excluded for some other reason, the endpoint is not reachable either.
func (s Selection) EndpointAllowlist() []string {
	var out []string
	for _, svc := range s.Services {
		out = append(out, svc.Endpoints()...)
	}
	sort.Strings(out)
	return out
}

// NewScope builds the transport scope implied by this selection over the
// authorised domains.
func (s Selection) NewScope(domains []string) *Scope {
	return NewScope(domains, s.EndpointAllowlist())
}

// ExclusionCoverage renders the refused checks as coverage records.
//
// This is the fail-closed path made legible. A check the policy excluded did
// not run, so it is not_checked — never absent, and never anything that could
// be read as a pass. The excluding reason travels with it so the operator can
// see what a policy change would buy them.
func (s Selection) ExclusionCoverage(assetID, libraryVersion string, at time.Time) []store.AssessmentCoverage {
	out := make([]store.AssessmentCoverage, 0, len(s.Excluded))
	for _, e := range s.Excluded {
		out = append(out, store.AssessmentCoverage{
			ID:             uuid.NewString(),
			AssetID:        assetID,
			CheckID:        e.CheckID,
			State:          store.CoverageNotChecked,
			Reason:         e.Reason,
			LibraryVersion: libraryVersion,
			AssessedAt:     at,
		})
	}
	return out
}

func joinClasses(cs []EgressClass) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = string(c)
	}
	return strings.Join(parts, ", ")
}
