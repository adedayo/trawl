package vantage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	vantage "github.com/adedayo/vantage/pkg"
	vaudit "github.com/adedayo/vantage/pkg/audit"
	vfinding "github.com/adedayo/vantage/pkg/finding"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
)

// fakeAssessor stands in for vantage so these tests never touch the network.
// It satisfies vaudit.Assessor, so if that interface changes shape this file
// stops compiling — which is the point of a contract test.
type fakeAssessor struct {
	result *vfinding.Result
	caps   vaudit.Capabilities
	err    error
	seen   vaudit.Request
	emit   []vaudit.Progress
}

func (f *fakeAssessor) Catalogue(context.Context) (vaudit.Capabilities, error) {
	return f.caps, f.err
}

func (f *fakeAssessor) Assess(_ context.Context, req vaudit.Request) (*vfinding.Result, error) {
	f.seen = req
	for _, p := range f.emit {
		if req.Observer != nil {
			req.Observer(p)
		}
	}
	return f.result, f.err
}

var _ vaudit.Assessor = (*fakeAssessor)(nil)

func fixedClock() func() time.Time {
	t := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func newAdapter(t *testing.T, f *fakeAssessor, opts ...Option) *Adapter {
	t.Helper()
	opts = append([]Option{WithClock(fixedClock())}, opts...)
	a, err := New(f, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

// TestCoverageStatesSurviveTranslation is the central guarantee of this
// adapter. All four vantage states must arrive in the store as four distinct
// states; any collapse here would let a check that never ran be read as a
// control that passed.
func TestCoverageStatesSurviveTranslation(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{
		{Check: "spf", Target: "example.com", State: vfinding.StateOK},
		{Check: "dmarc", Target: "example.com", State: vfinding.StateNotFound},
		{Check: "dkim", Target: "example.com", State: vfinding.StateNotChecked},
		{Check: "caa", Target: "example.com", State: vfinding.StateCheckFailed},
	}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "asset-1", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}

	want := map[string]store.CoverageState{
		"spf":   store.CoverageOK,
		"dmarc": store.CoverageNotFound,
		"dkim":  store.CoverageNotChecked,
		"caa":   store.CoverageCheckFailed,
	}
	if len(got.Coverage) != len(want) {
		t.Fatalf("coverage records = %d, want %d", len(got.Coverage), len(want))
	}
	for _, c := range got.Coverage {
		if c.State != want[c.CheckID] {
			t.Errorf("check %q state = %q, want %q", c.CheckID, c.State, want[c.CheckID])
		}
		if !c.State.Valid() {
			t.Errorf("check %q produced an invalid state %q", c.CheckID, c.State)
		}
		if c.AssetID != "asset-1" {
			t.Errorf("check %q lost its asset association", c.CheckID)
		}
	}
}

// TestUnknownStateDegradesToCheckFailed pins the fail-closed direction of the
// mapping. If vantage ever adds a fifth state, Trawl must read it as "we could
// not tell" rather than as a passing control.
func TestUnknownStateDegradesToCheckFailed(t *testing.T) {
	if got := coverageState(vfinding.State("something-new")); got != store.CoverageCheckFailed {
		t.Fatalf("unknown state mapped to %q, want %q", got, store.CoverageCheckFailed)
	}
	if got := coverageState(""); got != store.CoverageCheckFailed {
		t.Fatalf("empty state mapped to %q, want %q", got, store.CoverageCheckFailed)
	}
}

// TestCheckFailedNeverReadsAsPassing is the propagation guard required by the
// change's exit criteria.
func TestCheckFailedNeverReadsAsPassing(t *testing.T) {
	for _, s := range []store.CoverageState{
		store.CoverageCheckFailed, store.CoverageNotChecked,
	} {
		if s.Passing() {
			t.Errorf("%q must never report as passing", s)
		}
		if s.Assessed() {
			t.Errorf("%q must never report as assessed", s)
		}
	}
}

// TestNonConclusiveCoverageNamesAReason ensures a gap can be acted on. A
// coverage record that says "could not tell" without saying why is
// indistinguishable from a bug in this adapter.
func TestNonConclusiveCoverageNamesAReason(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{
		{Check: "caa", Target: "example.com", State: vfinding.StateCheckFailed},
	}
	res.Errors = []vfinding.CheckError{{
		Check: "caa", Target: "example.com",
		Code: vfinding.ErrCodeResolverUnreachable, Message: "no resolver answered",
	}}

	a := newAdapter(t, &fakeAssessor{result: res})
	got, err := a.Assess(context.Background(), Request{AssetID: "a", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if got.Coverage[0].Reason == "" {
		t.Fatal("a check_failed coverage record must name why")
	}
	if got.Outcome != OutcomePartial {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomePartial)
	}
}

// TestUnmappedSignalsAreRetained guards against silent loss. A vantage upgrade
// that adds a finding must show up as an unmapped observation, not vanish.
func TestUnmappedSignalsAreRetained(t *testing.T) {
	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{{Check: "spf", Target: "example.com", State: vfinding.StateOK}}
	res.Findings = []vfinding.Finding{
		{ID: "SURF-SPF-001", Check: "spf", Target: "example.com", Severity: vfinding.SeverityHigh},
		{ID: "SURF-BRAND-NEW-999", Check: "spf", Target: "example.com", Severity: vfinding.SeverityLow},
	}

	reg := stubRegistry{version: "2026.1", known: map[string]string{"SURF-SPF-001": "2026.1"}}
	a := newAdapter(t, &fakeAssessor{result: res}, WithRegistry(reg))

	got, err := a.Assess(context.Background(), Request{AssetID: "a", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if len(got.Observations) != 2 {
		t.Fatalf("observations = %d, want 2 (the unmapped one must be retained)", len(got.Observations))
	}

	byID := map[string]store.SignalObservation{}
	for _, o := range got.Observations {
		byID[o.SignalID] = o
	}
	if !byID["SURF-SPF-001"].Mapped {
		t.Error("a known identifier must be marked mapped")
	}
	if byID["SURF-BRAND-NEW-999"].Mapped {
		t.Error("an unknown identifier must be marked unmapped, not mapped")
	}
	// Provenance must be present either way, so a later reader can tell which
	// registry was consulted when the decision was made.
	for id, o := range byID {
		if o.RegistryVersion == "" {
			t.Errorf("%s: registry version not recorded", id)
		}
		if o.LibraryVersion != "1.0.0" {
			t.Errorf("%s: library version = %q, want 1.0.0", id, o.LibraryVersion)
		}
	}
}

// TestOutcomeClassification pins the distinction between "found nothing" and
// "never looked", at the level of the whole assessment.
func TestOutcomeClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Outcome
	}{
		{"clean", nil, OutcomeCompleted},
		{"cancelled", context.Canceled, OutcomeCancelled},
		{"refused by scope", vantage.ErrOutOfScope, OutcomeRefused},
		{"wrapped refusal", errors.New("x"), OutcomeFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyOutcome(c.err); got != c.want {
				t.Fatalf("outcome = %q, want %q", got, c.want)
			}
		})
	}
}

// TestFailedAssessmentStillReportsAResult ensures a caller can record that an
// attempt was made. Returning nothing would be indistinguishable from never
// having tried.
func TestFailedAssessmentStillReportsAResult(t *testing.T) {
	a := newAdapter(t, &fakeAssessor{err: vantage.ErrOutOfScope})
	got, err := a.Assess(context.Background(), Request{AssetID: "a", Domain: "example.com"})
	if err == nil {
		t.Fatal("expected the refusal to propagate")
	}
	if got.Outcome != OutcomeRefused {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeRefused)
	}
}

// TestSeverityMappingIsTotal guards the other silent-downgrade risk: an
// unrecognised severity must not become critical, nor be dropped.
func TestSeverityMappingIsTotal(t *testing.T) {
	want := map[vfinding.Severity]store.FindingSeverity{
		vfinding.SeverityCritical: store.SeverityCritical,
		vfinding.SeverityHigh:     store.SeverityHigh,
		vfinding.SeverityMedium:   store.SeverityMedium,
		vfinding.SeverityLow:      store.SeverityLow,
		vfinding.SeverityInfo:     store.SeverityInfo,
	}
	for in, out := range want {
		if got := severity(in); got != out {
			t.Errorf("severity(%v) = %q, want %q", in, got, out)
		}
	}
	if got := severity(vfinding.Severity(99)); got != store.SeverityInfo {
		t.Errorf("unknown severity = %q, want info", got)
	}
}

// TestRequestIsTranslatedFaithfully checks the adapter does not quietly widen
// an assessment beyond what was asked for.
func TestRequestIsTranslatedFaithfully(t *testing.T) {
	f := &fakeAssessor{result: vfinding.NewResult("vantage", "1.0.0")}
	a := newAdapter(t, f)

	_, err := a.Assess(context.Background(), Request{
		AssetID: "a", Domain: "example.com",
		Checks: []string{"spf", "dmarc"}, NoNetwork: true,
		Hosts: []string{"mail.example.com"},
	})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if len(f.seen.Targets) != 1 || f.seen.Targets[0] != "example.com" {
		t.Fatalf("targets = %v, want [example.com]", f.seen.Targets)
	}
	if !f.seen.Selection.NoNetwork {
		t.Error("NoNetwork was not propagated")
	}
	if f.seen.Enumerate {
		t.Error("enumeration must stay opt-in: it queries a third party")
	}
	if len(f.seen.Selection.Only) != 2 {
		t.Fatalf("check selection = %v, want 2 entries", f.seen.Selection.Only)
	}
}

// TestInvalidProfileIsRejected ensures a misspelled profile fails loudly
// rather than silently assessing the default.
func TestInvalidProfileIsRejected(t *testing.T) {
	a := newAdapter(t, &fakeAssessor{result: vfinding.NewResult("vantage", "1.0.0")})
	got, err := a.Assess(context.Background(), Request{
		AssetID: "a", Domain: "example.com", Profile: "not-a-profile",
	})
	if err == nil {
		t.Fatal("expected an unknown profile to be rejected")
	}
	if got.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeFailed)
	}
}

// TestProgressReachesTheEventBus verifies the per-check state survives onto
// the bus, so a UI can show real coverage rather than a bare percentage.
func TestProgressReachesTheEventBus(t *testing.T) {
	f := &fakeAssessor{
		result: vfinding.NewResult("vantage", "1.0.0"),
		emit: []vaudit.Progress{
			{Phase: vaudit.PhaseTargetStarted, Target: "example.com"},
			{Phase: vaudit.PhaseCheckCompleted, Target: "example.com", Check: "spf",
				State: vfinding.StateCheckFailed, ChecksDone: 1, ChecksTotal: 2},
			{Phase: vaudit.PhaseTargetCompleted, Target: "example.com"},
		},
	}
	bus := &recordingBus{}
	a := newAdapter(t, f, WithEventBus(bus))

	if _, err := a.Assess(context.Background(), Request{AssetID: "a", Domain: "example.com"}); err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if len(bus.payloads) != 1 {
		t.Fatalf("published %d events, want 1 (check completions only)", len(bus.payloads))
	}
	p := bus.payloads[0]
	if p.State != store.CoverageCheckFailed {
		t.Errorf("progress state = %q, want %q", p.State, store.CoverageCheckFailed)
	}
	if p.Check != "spf" || p.AssetID != "a" {
		t.Errorf("progress lost its identity: %+v", p)
	}
}

// TestDomainIsRequired guards against an empty assessment being reported as a
// clean one.
func TestDomainIsRequired(t *testing.T) {
	a := newAdapter(t, &fakeAssessor{})
	got, err := a.Assess(context.Background(), Request{AssetID: "a"})
	if err == nil {
		t.Fatal("expected an empty domain to be rejected")
	}
	if got.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeFailed)
	}
}

func TestNewRequiresAssessor(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected a nil assessor to be rejected")
	}
}

// --- helpers ---

// recordingBus captures published progress payloads.
type recordingBus struct {
	mu       sync.Mutex
	payloads []ProgressPayload
}

func (b *recordingBus) Subscribe(event.EventType, event.Handler) {}

func (b *recordingBus) Publish(_ context.Context, e event.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p, ok := e.Payload.(ProgressPayload); ok {
		b.payloads = append(b.payloads, p)
	}
}

var _ event.Bus = (*recordingBus)(nil)

type stubRegistry struct {
	version string
	known   map[string]string
}

func (s stubRegistry) Version() string { return s.version }

func (s stubRegistry) Lookup(id string) (store.SignalRegistryEntry, bool) {
	v, ok := s.known[id]
	if !ok {
		return store.SignalRegistryEntry{}, false
	}
	return store.SignalRegistryEntry{SignalID: id, RegistryVersion: v}, true
}
