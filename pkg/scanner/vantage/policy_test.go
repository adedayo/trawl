package vantage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vaudit "github.com/adedayo/vantage/pkg/audit"
	vfinding "github.com/adedayo/vantage/pkg/finding"

	"github.com/adedayo/trawl/pkg/store"
)

// TestEgressConformance is the required CI check for Phase 7.
//
// It asserts two things about the linked library, both of which are properties
// of the upgrade rather than of any particular run: every check states what it
// touches, and everything it states is expressible in the policy schema. The
// second is the one that earns its keep. A vantage release introducing a new
// kind of contact — a class this build's policy vocabulary has no word for —
// would otherwise be handled by whichever branch happened to catch it, and the
// two possible accidents are equally bad: silently permitted, or silently
// dropped so that a check never runs and nobody is told why.
func TestEgressConformance(t *testing.T) {
	for _, d := range vaudit.Descriptions() {
		if !d.Egress.Declared() {
			t.Errorf("check %q declares no egress profile; "+
				"an undeclared profile cannot be reasoned about by policy", d.Name)
			continue
		}

		classes := ClassesOf(d.Egress)
		if len(classes) == 0 {
			t.Errorf("check %q declares a profile that implies no egress class", d.Name)
		}
		for _, c := range classes {
			if !c.Known() {
				t.Errorf("check %q requires egress class %q, which the policy schema "+
					"does not recognise; add it to EgressClasses and decide whether "+
					"DefaultEgressPolicy should permit it", d.Name, c)
			}
		}

		for _, s := range d.Egress.ThirdParty {
			if !s.Known() {
				t.Errorf("check %q names third-party service %q, which is not a "+
					"declared service", d.Name, s)
			}
			if len(s.Endpoints()) == 0 {
				t.Errorf("third-party service %q names no endpoints, so consent to it "+
					"cannot be enforced at the transport", s)
			}
		}
	}
}

// TestEveryCatalogueClassIsDecidedByDefaultPolicy asserts the default policy
// reaches a decision on every class the library can require — permitting it or
// not, but never being silent about it.
func TestEveryCatalogueClassIsDecidedByDefaultPolicy(t *testing.T) {
	p := DefaultEgressPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("the default policy does not satisfy its own schema: %v", err)
	}

	seen := map[EgressClass]bool{}
	for _, d := range vaudit.Descriptions() {
		for _, c := range ClassesOf(d.Egress) {
			seen[c] = true
		}
	}
	for c := range seen {
		if !c.Known() {
			t.Errorf("class %q is required by the catalogue but unknown to the schema", c)
		}
	}
}

// TestDefaultPolicyWithholdsIntrusiveAndThirdPartyChecks runs the real
// catalogue through the default policy and asserts nothing that a reasonable
// operator would expect to be asked about slipped through.
func TestDefaultPolicyWithholdsIntrusiveAndThirdPartyChecks(t *testing.T) {
	caps := realCapabilities(t)

	sel, err := SelectChecks(caps, DefaultEgressPolicy())
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}
	if len(sel.Checks) == 0 {
		t.Fatal("the default policy admits no checks at all, which would make Trawl useless out of the box")
	}

	byName := map[string]vaudit.EgressProfile{}
	for _, c := range caps.Checks {
		byName[c.Name] = c.Egress
	}
	for _, name := range sel.Checks {
		e := byName[name]
		if e.Intrusive {
			t.Errorf("check %q is intrusive but was selected under the default policy", name)
		}
		if len(e.ThirdParty) > 0 {
			t.Errorf("check %q contacts %v but was selected with no service consented", name, e.ThirdParty)
		}
		if e.TargetNameservers {
			t.Errorf("check %q addresses target nameservers directly but was selected "+
				"under the default policy", name)
		}
	}

	if len(sel.Services) != 0 {
		t.Errorf("selection needs services %v under a policy consenting to none", sel.Services)
	}
	if allow := sel.EndpointAllowlist(); len(allow) != 0 {
		t.Errorf("transport allowlist is %v under a policy consenting to no service", allow)
	}
	if sel.TargetNameservers {
		t.Error("selection claims it needs direct nameserver contact under the default policy")
	}
}

// TestAddedIntrusiveCheckIsExcludedUnderDefaultPolicy is the upgrade
// simulation that the whole design exists for.
//
// A future vantage release adds a check that goes beyond an ordinary query.
// Nobody in Trawl has heard of it, no deny list names it, and no test was
// written for it. It must still not run.
func TestAddedIntrusiveCheckIsExcludedUnderDefaultPolicy(t *testing.T) {
	caps := capsOf(
		check("spf", vaudit.EgressProfile{Resolver: true}),
		check("zone-walk", vaudit.EgressProfile{Resolver: true, TargetNameservers: true, Intrusive: true}),
	)

	sel, err := SelectChecks(caps, DefaultEgressPolicy())
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}

	if contains(sel.Checks, "zone-walk") {
		t.Fatal("an intrusive check added upstream was selected under the default policy")
	}
	if !contains(sel.Checks, "spf") {
		t.Fatal("an ordinary resolver check was excluded, so the exclusion is not discriminating")
	}

	reason := reasonForCheck(sel, "zone-walk")
	if reason == "" {
		t.Fatal("the excluded check has no recorded reason, so an operator cannot act on it")
	}
	// The reason must name the excluding class, not merely report a refusal.
	// "This did not run" is not actionable; "this needs a class you have not
	// permitted" tells the operator exactly what decision is theirs to make.
	if !strings.Contains(reason, string(ClassTargetNameservers)) &&
		!strings.Contains(reason, string(ClassIntrusive)) {
		t.Errorf("exclusion reason %q names neither the intrusive nor the "+
			"target-nameservers class", reason)
	}
}

// TestAddedThirdPartyDependencyIsExcludedAndEndpointNamed covers the other way
// an upgrade can widen behaviour: a check that reaches a service the operator
// has not agreed to disclose their portfolio to.
func TestAddedThirdPartyDependencyIsExcludedAndEndpointNamed(t *testing.T) {
	caps := capsOf(
		check("spf", vaudit.EgressProfile{Resolver: true}),
		check("ct", vaudit.EgressProfile{
			Resolver:   true,
			ThirdParty: []vaudit.ThirdPartyService{vaudit.ServiceCertSpotter},
		}),
	)

	// A policy that permits third-party contact in general but has consented
	// to no service. This is the interesting case: permitting the class must
	// not be read as blanket consent, or "third party" becomes one decision
	// covering every service vantage ever adds.
	policy := DefaultEgressPolicy()
	policy.AllowedClasses = append(policy.AllowedClasses, ClassThirdParty)

	sel, err := SelectChecks(caps, policy)
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}
	if contains(sel.Checks, "ct") {
		t.Fatal("a check contacting an unconsented service was selected")
	}

	reason := reasonForCheck(sel, "ct")
	if !strings.Contains(reason, string(vaudit.ServiceCertSpotter)) {
		t.Errorf("reason %q does not name the service", reason)
	}
	// Naming the endpoint, not just the service, is what lets an operator
	// decide. "certspotter" is a brand; the hostname is the thing their egress
	// policy and their disclosure assessment actually concern.
	for _, ep := range vaudit.ServiceCertSpotter.Endpoints() {
		if !strings.Contains(reason, ep) {
			t.Errorf("reason %q does not name endpoint %q", reason, ep)
		}
	}
}

// TestConsentAdmitsTheCheckAndItsEndpoint is the positive counterpart: once an
// operator consents, the check runs and the transport learns the endpoint.
func TestConsentAdmitsTheCheckAndItsEndpoint(t *testing.T) {
	caps := capsOf(check("ct", vaudit.EgressProfile{
		Resolver:   true,
		ThirdParty: []vaudit.ThirdPartyService{vaudit.ServiceCertSpotter},
	}))

	policy := EgressPolicy{
		AllowedClasses:    []EgressClass{ClassResolver, ClassThirdParty},
		ConsentedServices: []vaudit.ThirdPartyService{vaudit.ServiceCertSpotter},
	}

	sel, err := SelectChecks(caps, policy)
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}
	if !contains(sel.Checks, "ct") {
		t.Fatalf("consented check was excluded: %v", sel.Excluded)
	}

	scope := sel.NewScope([]string{"example.com"})
	for _, ep := range vaudit.ServiceCertSpotter.Endpoints() {
		if !scope.PermitsEndpoint(ep) {
			t.Errorf("transport refuses %q for a consented, selected check", ep)
		}
	}
}

// TestTransportPermissionsFollowSelectionNotConsent asserts the transport is
// given the narrowest permission set that lets the selected work happen.
//
// Consent is a ceiling, not a grant. If the checks that would have used a
// service are excluded for some unrelated reason, the endpoint stays
// unreachable — otherwise a stale line in a config file keeps an egress path
// open long after the last thing that needed it stopped running.
func TestTransportPermissionsFollowSelectionNotConsent(t *testing.T) {
	caps := capsOf(check("ct", vaudit.EgressProfile{
		Resolver:   true,
		Intrusive:  true, // excluded for a reason unrelated to the service
		ThirdParty: []vaudit.ThirdPartyService{vaudit.ServiceCertSpotter},
	}))

	policy := EgressPolicy{
		AllowedClasses:    []EgressClass{ClassResolver, ClassThirdParty},
		ConsentedServices: []vaudit.ThirdPartyService{vaudit.ServiceCertSpotter},
	}

	sel, err := SelectChecks(caps, policy)
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}
	if len(sel.Checks) != 0 {
		t.Fatalf("intrusive check selected: %v", sel.Checks)
	}
	if allow := sel.EndpointAllowlist(); len(allow) != 0 {
		t.Errorf("endpoint allowlist %v is derived from consent rather than from "+
			"the checks actually selected", allow)
	}

	scope := sel.NewScope([]string{"example.com"})
	for _, ep := range vaudit.ServiceCertSpotter.Endpoints() {
		if scope.PermitsEndpoint(ep) {
			t.Errorf("transport permits %q though no selected check needs it", ep)
		}
	}
}

// TestUndeclaredProfileIsRefused asserts a check that forgot to declare its
// egress is treated as unpermitted rather than unrestricted.
func TestUndeclaredProfileIsRefused(t *testing.T) {
	caps := capsOf(check("mystery", vaudit.EgressProfile{}))

	sel, err := SelectChecks(caps, EgressPolicy{AllowedClasses: EgressClasses()})
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}
	if len(sel.Checks) != 0 {
		t.Fatal("a check declaring no egress ran under a policy permitting every class; " +
			"an undeclared profile must not be read as an empty one")
	}
	if r := reasonForCheck(sel, "mystery"); !strings.Contains(r, "declares no egress") {
		t.Errorf("reason %q does not explain that the declaration is missing", r)
	}
}

// TestSelectionAccountsForEveryCheck asserts nothing is lost between the two
// output lists. A check that appears in neither has silently vanished, which
// is the failure mode this design is meant to make impossible.
func TestSelectionAccountsForEveryCheck(t *testing.T) {
	caps := realCapabilities(t)

	sel, err := SelectChecks(caps, DefaultEgressPolicy())
	if err != nil {
		t.Fatalf("SelectChecks: %v", err)
	}
	if got, want := len(sel.Checks)+len(sel.Excluded), len(caps.Checks); got != want {
		t.Errorf("selection accounts for %d checks, catalogue has %d", got, want)
	}
}

func TestPolicyValidationRejectsUnknownVocabulary(t *testing.T) {
	cases := []struct {
		name   string
		policy EgressPolicy
		want   string
	}{
		{
			name:   "unknown class",
			policy: EgressPolicy{AllowedClasses: []EgressClass{"carrier-pigeon"}},
			want:   "carrier-pigeon",
		},
		{
			name: "unknown service",
			policy: EgressPolicy{
				AllowedClasses:    []EgressClass{ClassThirdParty},
				ConsentedServices: []vaudit.ThirdPartyService{"someones-blog"},
			},
			want: "someones-blog",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.Validate()
			if err == nil {
				t.Fatal("validation accepted a policy it cannot enforce; a typo that " +
					"silently narrows assessment presents as a clean domain")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name the offending term %q", err, tc.want)
			}
		})
	}
}

// TestAdapterAppliesPolicyOverCallerChecks asserts the policy is not something
// a caller can widen by asking for more.
func TestAdapterAppliesPolicyOverCallerChecks(t *testing.T) {
	f := &fakeAssessor{
		caps: capsOf(
			check("spf", vaudit.EgressProfile{Resolver: true}),
			check("axfr", vaudit.EgressProfile{TargetNameservers: true, Intrusive: true}),
		),
		result: &vfinding.Result{
			Checks: []vfinding.CheckResult{{Check: "spf", State: vfinding.StateOK}},
		},
	}
	a := newAdapter(t, f, WithEgressPolicy(DefaultEgressPolicy()), WithLibraryVersion("v1.3.0"))

	res, err := a.Assess(context.Background(), Request{
		AssetID: "asset-1",
		Domain:  "example.com",
		Checks:  []string{"spf", "axfr"}, // the caller asks for the intrusive one
	})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	if contains(f.seen.Selection.Only, "axfr") {
		t.Fatal("the caller's check list overrode deployment policy; a policy a caller " +
			"can widen is not a policy")
	}

	// The excluded check must be present as coverage, not absent. Absence is
	// indistinguishable from a check that does not exist, and both read as
	// "nothing to worry about here".
	cov, ok := coverageFor(res.Coverage, "axfr")
	if !ok {
		t.Fatal("the excluded check produced no coverage record")
	}
	if cov.State != store.CoverageNotChecked {
		t.Errorf("excluded check recorded as %q, want %q", cov.State, store.CoverageNotChecked)
	}
	if cov.Reason == "" {
		t.Error("excluded check recorded without a reason")
	}

	// Withholding checks means the run is not complete, whatever the checks
	// that did run reported.
	if res.Outcome != OutcomePartial {
		t.Errorf("outcome is %q; a run with withheld checks is not a completed one", res.Outcome)
	}
}

// TestPolicyExcludingEverythingRefusesRatherThanReportingSilence asserts an
// over-restrictive policy fails loudly instead of producing an empty, clean
// looking result.
func TestPolicyExcludingEverythingRefusesRatherThanReportingSilence(t *testing.T) {
	f := &fakeAssessor{
		caps:   capsOf(check("spf", vaudit.EgressProfile{Resolver: true})),
		result: &vfinding.Result{},
	}
	a := newAdapter(t, f, WithEgressPolicy(EgressPolicy{AllowedClasses: []EgressClass{ClassOffline}}),
		WithLibraryVersion("v1.3.0"))

	res, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err == nil {
		t.Fatal("an assessment that could run nothing returned no error")
	}
	if res.Outcome != OutcomeRefused {
		t.Errorf("outcome is %q, want %q", res.Outcome, OutcomeRefused)
	}
	if len(res.Observations) != 0 {
		t.Error("a refused assessment produced observations")
	}
	if f.seen.Targets != nil {
		t.Error("the assessor was invoked despite every check being excluded")
	}
	if len(res.Coverage) == 0 {
		t.Fatal("a refused assessment recorded no coverage, so it is indistinguishable " +
			"from never having been attempted")
	}
	for _, c := range res.Coverage {
		if c.State.Passing() {
			t.Errorf("check %q reads as passing after a refused assessment", c.CheckID)
		}
	}
}

// TestAdapterRejectsAnUnenforceablePolicy asserts construction fails rather
// than deferring the problem to the first scan.
func TestAdapterRejectsAnUnenforceablePolicy(t *testing.T) {
	_, err := New(&fakeAssessor{}, WithEgressPolicy(EgressPolicy{
		AllowedClasses: []EgressClass{"telepathy"},
	}))
	if err == nil {
		t.Fatal("an adapter was built over a policy it cannot enforce")
	}
}

// TestExampleConfigPolicyIsEnforceable asserts the shipped example
// configuration expresses a policy this build can actually enforce, and does
// it in the schema's vocabulary rather than by naming checks.
//
// The example is what operators copy. A policy in it that fails validation, or
// that reaches for a check name, propagates into every deployment derived from
// it.
func TestExampleConfigPolicyIsEnforceable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "example.json"))
	if err != nil {
		t.Fatalf("reading example config: %v", err)
	}

	var cfg struct {
		EgressPolicy *EgressPolicy `json:"egressPolicy"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing example config: %v", err)
	}
	if cfg.EgressPolicy == nil {
		t.Fatal("the example configuration declares no egress policy, so an operator " +
			"copying it has no statement of what the deployment may contact")
	}
	if err := cfg.EgressPolicy.Validate(); err != nil {
		t.Fatalf("the example configuration's policy is unenforceable: %v", err)
	}

	// The example must not have drifted into naming checks. That is the whole
	// distinction: a class list is an allow list over a closed vocabulary, and
	// a check list is a deny list that a new check walks straight past.
	var loose map[string]any
	if err := json.Unmarshal(raw, &loose); err != nil {
		t.Fatalf("parsing example config: %v", err)
	}
	if policy, ok := loose["egressPolicy"].(map[string]any); ok {
		for key := range policy {
			if key != "allowedClasses" && key != "consentedServices" {
				t.Errorf("egressPolicy carries unrecognised key %q; policy is stated over "+
					"egress classes and consented services only", key)
			}
		}
	}
}

// --- helpers ---

func realCapabilities(t *testing.T) vaudit.Capabilities {
	t.Helper()
	var caps vaudit.Capabilities
	for _, d := range vaudit.Descriptions() {
		caps.Checks = append(caps.Checks, vaudit.CheckCapability{Description: d})
	}
	if len(caps.Checks) == 0 {
		t.Fatal("the linked library declares no checks")
	}
	return caps
}

func check(name string, e vaudit.EgressProfile) vaudit.CheckCapability {
	return vaudit.CheckCapability{
		Description: vaudit.Description{Name: name, Egress: e},
	}
}

func capsOf(checks ...vaudit.CheckCapability) vaudit.Capabilities {
	return vaudit.Capabilities{Checks: checks}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func reasonForCheck(s Selection, name string) string {
	for _, e := range s.Excluded {
		if e.CheckID == name {
			return e.Reason
		}
	}
	return ""
}

func coverageFor(cov []store.AssessmentCoverage, check string) (store.AssessmentCoverage, bool) {
	for _, c := range cov {
		if c.CheckID == check {
			return c, true
		}
	}
	return store.AssessmentCoverage{}, false
}
