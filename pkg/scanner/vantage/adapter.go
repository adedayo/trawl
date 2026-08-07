// Package vantage adapts the vantage assessment library to Trawl's domain
// types.
//
// This is deliberately the only package in Trawl that imports vantage. Every
// other package speaks in terms of store.SignalObservation and
// store.AssessmentCoverage, so an upgrade that reshapes vantage's types is
// contained here and surfaces as a compile error in one place rather than
// spreading through the codebase.
package vantage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	vantage "github.com/adedayo/vantage/pkg"
	vaudit "github.com/adedayo/vantage/pkg/audit"
	vfinding "github.com/adedayo/vantage/pkg/finding"

	"github.com/adedayo/trawl/pkg/event"
	"github.com/adedayo/trawl/pkg/store"
)

// Outcome classifies how an assessment ended, as distinct from what it found.
//
// A consumer needs this to decide whether an absent signal is meaningful. An
// assessment that was refused or cancelled says nothing about the asset, and
// treating its silence as a clean bill of health is precisely the error the
// four-state model exists to prevent.
type Outcome string

const (
	// OutcomeCompleted means every requested check reached a conclusion.
	OutcomeCompleted Outcome = "completed"
	// OutcomePartial means the assessment ran but at least one check could not
	// reach a conclusion. The results that did complete remain valid.
	OutcomePartial Outcome = "partial"
	// OutcomeFailed means the assessment could not be performed at all.
	OutcomeFailed Outcome = "failed"
	// OutcomeRefused means the caller's own policy declined the assessment,
	// typically a scope guard. Nothing was attempted.
	OutcomeRefused Outcome = "refused"
	// OutcomeCancelled means the caller withdrew before completion.
	OutcomeCancelled Outcome = "cancelled"
)

// Registry resolves a vantage finding identifier to what it bears on. It is
// satisfied by the Phase 4 signal registry loader; until that lands, a nil
// Registry means every observation is recorded as unmapped rather than being
// dropped.
type Registry interface {
	Lookup(signalID string) (store.SignalRegistryEntry, bool)
	Version() string
}

// Request is one assessment of one asset.
type Request struct {
	// AssetID is the Trawl asset the results attach to.
	AssetID string
	// Domain is the name to assess.
	Domain string
	// Checks, when non-empty, replaces the profile's membership entirely.
	Checks []string
	// Profile selects the breadth of assessment. Empty means standard.
	Profile string
	// NoNetwork restricts the assessment to DNS.
	NoNetwork bool
	// Hosts are additional names within the domain to assess.
	Hosts []string
	// ExpectJurisdictions are the countries the operator declares their
	// infrastructure should be in.
	ExpectJurisdictions []string
}

// Result is an assessment translated into Trawl's types.
type Result struct {
	// Outcome says how the assessment ended.
	Outcome Outcome
	// Observations are the findings, in Trawl's form.
	Observations []store.SignalObservation
	// Coverage records, per check, whether it reached a conclusion. Every
	// requested check appears here, including those that did not run, so that
	// a coverage figure can always accompany an aggregate.
	Coverage []store.AssessmentCoverage
	// LibraryVersion is the vantage build that produced the result.
	LibraryVersion string
	// Err carries the reason when Outcome is not completed or partial.
	Err error
}

// Adapter translates between vantage and Trawl.
type Adapter struct {
	assessor vaudit.Assessor
	registry Registry
	bus      event.Bus
	now      func() time.Time
	version  string
}

// Option configures an Adapter.
type Option func(*Adapter)

// WithRegistry supplies the signal registry used to map identifiers.
func WithRegistry(r Registry) Option { return func(a *Adapter) { a.registry = r } }

// WithEventBus publishes progress onto the bus as the assessment advances.
func WithEventBus(b event.Bus) Option { return func(a *Adapter) { a.bus = b } }

// WithClock overrides the time source, for deterministic tests.
func WithClock(now func() time.Time) Option { return func(a *Adapter) { a.now = now } }

// WithLibraryVersion overrides the version stamped on observations. It exists
// for tests and for builds whose dependency metadata has been stripped; an
// ordinary build resolves the version from its own build info.
func WithLibraryVersion(v string) Option { return func(a *Adapter) { a.version = v } }

// New builds an Adapter over an assessor.
func New(assessor vaudit.Assessor, opts ...Option) (*Adapter, error) {
	if assessor == nil {
		return nil, errors.New("vantage: an assessor is required")
	}
	a := &Adapter{assessor: assessor, now: time.Now}
	for _, opt := range opts {
		opt(a)
	}
	if a.version == "" {
		a.version = libraryVersion()
	}
	return a, nil
}

// Assess runs one assessment and translates the result.
//
// It returns a Result even on failure. A caller must be able to record that an
// assessment was attempted and did not conclude — recording nothing would be
// indistinguishable from never having tried, which is the distinction the
// coverage model exists to preserve.
func (a *Adapter) Assess(ctx context.Context, req Request) (Result, error) {
	if req.Domain == "" {
		return Result{Outcome: OutcomeFailed}, errors.New("vantage: a domain is required")
	}

	selection := vaudit.Selection{
		Only:      req.Checks,
		NoNetwork: req.NoNetwork,
	}
	if req.Profile != "" {
		p, err := vaudit.ParseProfile(req.Profile)
		if err != nil {
			return Result{Outcome: OutcomeFailed, Err: err}, err
		}
		selection.Profile = p
	}

	vres, err := a.assessor.Assess(ctx, vaudit.Request{
		Targets:             []string{req.Domain},
		Selection:           selection,
		Hosts:               req.Hosts,
		ExpectJurisdictions: req.ExpectJurisdictions,
		Observer:            a.observer(ctx, req),
	})

	// A nil result with an error means the assessment never started: the
	// selection was invalid, or the context was already done.
	if vres == nil {
		out := Result{Outcome: classifyOutcome(err), Err: err}
		return out, err
	}

	res := a.translate(req, vres)
	res.Err = err
	if err != nil {
		res.Outcome = classifyOutcome(err)
	}
	return res, err
}

// Catalogue reports what the underlying library can assess, so that a caller
// can verify its signal registry covers every identifier before relying on it.
func (a *Adapter) Catalogue(ctx context.Context) (vaudit.Capabilities, error) {
	return a.assessor.Catalogue(ctx)
}

// translate converts a vantage result into Trawl's types.
func (a *Adapter) translate(req Request, vres *vfinding.Result) Result {
	now := a.now()

	// The library version is resolved from this build's dependencies, not
	// taken from the result. vres.Tool.Version reports whatever the embedder
	// stamped, and Trawl stamps nothing, so it arrives as the literal string
	// "unknown" — a placeholder that would be recorded on every observation as
	// though it were provenance.
	libVersion := a.version
	if libVersion == "" {
		// Nothing to be had from the build either. The result's own stamp is
		// then the only claim available, and it is preserved rather than
		// discarded so that a caller who did configure one still gets it.
		libVersion = vres.Tool.Version
	}

	res := Result{
		Outcome:        OutcomeCompleted,
		LibraryVersion: libVersion,
	}

	// Coverage first, so that the state each check settled on is available
	// when attributing findings to it.
	states := make(map[string]store.CoverageState, len(vres.Checks))
	for _, c := range vres.Checks {
		state := coverageState(c.State)
		states[c.Check] = state
		if state == store.CoverageCheckFailed || state == store.CoverageNotChecked {
			res.Outcome = OutcomePartial
		}
		res.Coverage = append(res.Coverage, store.AssessmentCoverage{
			ID:             uuid.NewString(),
			AssetID:        req.AssetID,
			CheckID:        c.Check,
			State:          state,
			Reason:         reasonFor(c.Check, vres.Errors),
			LibraryVersion: libVersion,
			AssessedAt:     now,
		})
	}

	for _, f := range vres.Findings {
		// A finding is evidence that the check reached a conclusion, so its
		// observation state is OK — the signal was positively observed. The
		// check's own state stays on the coverage record, where it belongs.
		state := store.CoverageOK
		if s, ok := states[f.Check]; ok && !s.Assessed() {
			// A finding arriving from a check that did not conclude is a
			// contradiction. Recording it at the check's state rather than
			// discarding it keeps the anomaly visible.
			state = s
		}

		obs := store.SignalObservation{
			ID:             uuid.NewString(),
			AssetID:        req.AssetID,
			SignalID:       f.ID,
			CheckID:        f.Check,
			State:          state,
			Severity:       severity(f.Severity),
			Evidence:       evidence(f),
			LibraryVersion: libVersion,
			ObservedAt:     now,
			FirstSeen:      now,
			LastSeen:       now,
		}

		// An identifier the registry does not know is retained and marked,
		// never dropped. A library upgrade that adds a finding must be
		// visible as an unmapped signal rather than vanishing silently.
		if a.registry != nil {
			if entry, ok := a.registry.Lookup(f.ID); ok {
				obs.Mapped = true
				obs.RegistryVersion = entry.RegistryVersion
			} else {
				obs.RegistryVersion = a.registry.Version()
			}
		}

		res.Observations = append(res.Observations, obs)
	}

	return res
}

// observer bridges vantage progress onto Trawl's event bus.
func (a *Adapter) observer(ctx context.Context, req Request) func(vaudit.Progress) {
	if a.bus == nil {
		return nil
	}
	return func(p vaudit.Progress) {
		if p.Phase != vaudit.PhaseCheckCompleted {
			return
		}
		a.bus.Publish(ctx, event.Event{
			Type:      event.EventScanProgress,
			Timestamp: a.now(),
			Payload: ProgressPayload{
				AssetID:     req.AssetID,
				Domain:      p.Target,
				Check:       p.Check,
				State:       coverageState(p.State),
				ChecksDone:  p.ChecksDone,
				ChecksTotal: p.ChecksTotal,
			},
		})
	}
}

// ProgressPayload is the event bus payload for assessment progress.
type ProgressPayload struct {
	AssetID     string              `json:"assetId"`
	Domain      string              `json:"domain"`
	Check       string              `json:"check"`
	State       store.CoverageState `json:"state"`
	ChecksDone  int                 `json:"checksDone"`
	ChecksTotal int                 `json:"checksTotal"`
}

// coverageState maps vantage's four states onto Trawl's.
//
// The mapping is total and exhaustive by design. An unrecognised state maps to
// check_failed rather than to ok, so that a vantage upgrade introducing a
// fifth state degrades into "we could not tell" rather than silently
// asserting a control is present.
func coverageState(s vfinding.State) store.CoverageState {
	switch s {
	case vfinding.StateOK:
		return store.CoverageOK
	case vfinding.StateNotFound:
		return store.CoverageNotFound
	case vfinding.StateNotChecked:
		return store.CoverageNotChecked
	case vfinding.StateCheckFailed:
		return store.CoverageCheckFailed
	default:
		return store.CoverageCheckFailed
	}
}

// severity maps vantage severities onto Trawl's.
func severity(s vfinding.Severity) store.FindingSeverity {
	switch s {
	case vfinding.SeverityCritical:
		return store.SeverityCritical
	case vfinding.SeverityHigh:
		return store.SeverityHigh
	case vfinding.SeverityMedium:
		return store.SeverityMedium
	case vfinding.SeverityLow:
		return store.SeverityLow
	default:
		return store.SeverityInfo
	}
}

// evidence renders a finding's evidence into a single human-readable string,
// falling back to the description so that an observation is never stored
// without justification.
func evidence(f vfinding.Finding) string {
	if len(f.Evidence) == 0 {
		return f.Description
	}
	out := ""
	for i, e := range f.Evidence {
		if i > 0 {
			out += "; "
		}
		out += fmt.Sprintf("%s=%s", e.Name, e.Value)
	}
	return out
}

// reasonFor finds the recorded error for a check, so that a non-conclusive
// coverage record names why. A coverage gap without a reason cannot be acted
// on, and is indistinguishable from a bug in the adapter.
func reasonFor(check string, errs []vfinding.CheckError) string {
	for _, e := range errs {
		if e.Check == check {
			return fmt.Sprintf("%s: %s", e.Code, e.Message)
		}
	}
	return ""
}

// classifyOutcome maps an assessment error onto an outcome.
func classifyOutcome(err error) Outcome {
	switch {
	case err == nil:
		return OutcomeCompleted
	case errors.Is(err, context.Canceled):
		return OutcomeCancelled
	case errors.Is(err, vantage.ErrOutOfScope):
		return OutcomeRefused
	default:
		return OutcomeFailed
	}
}
