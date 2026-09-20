package service

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	vpkg "github.com/adedayo/vantage/pkg"
	vaudit "github.com/adedayo/vantage/pkg/audit"

	"github.com/adedayo/trawl/pkg/event"
	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/store"
)

// AssessmentService runs vantage assessments, persists what they observed, and
// assembles the read model the desktop UI renders.
//
// It sits between the adapter — which is the only place vantage types are
// known — and the Wails bindings, which speak only in the view types declared
// here. That keeps presentation concerns out of the adapter and vantage types
// out of the frontend, so neither can drift into the other.
type AssessmentService struct {
	store    store.Store
	bus      event.Bus
	registry *vadapter.SignalRegistry
}

// outcomeUnknown is reported for an asset that carries assessment rows but no
// run record — data written before runs were kept, or by a path that does not
// record one. It is deliberately not one of the adapter's outcomes: the
// adapter's vocabulary describes runs that happened, and this describes the
// absence of any account of one.
const outcomeUnknown = "unknown"

// NormaliseDomain reduces a domain to the form used for comparison and
// display: lower-cased, trimmed, and without a trailing dot.
func NormaliseDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}

// ensureAsset returns the identifier of the asset an assessment attaches to,
// registering the domain in the inventory if it is not already there.
//
// The asset store upserts on value and returns the canonical identifier, so a
// domain already discovered by subdomain enumeration keeps the identity it was
// given there. Minting a separate identifier for assessment would split one
// asset into two — the inventory's view and the posture's view — and nothing
// downstream could join them again.
//
// It is also a referential requirement: observations and coverage carry a
// foreign key onto assets, so an assessment of an unregistered domain would be
// rejected by the store rather than merely being untidy.
func (svc *AssessmentService) ensureAsset(ctx context.Context, domain string) (string, error) {
	asset := store.Asset{
		ID:              domain,
		Type:            store.AssetTypeDomain,
		Value:           domain,
		Status:          store.AssetStatusActive,
		DiscoverySource: "assessment",
		Confidence:      1,
	}
	if err := svc.store.SaveAsset(ctx, &asset); err != nil {
		return "", fmt.Errorf("assessment: registering %q as an asset: %w", domain, err)
	}
	return asset.ID, nil
}

// domainOfAsset maps asset identifiers to their values, for display.
func (svc *AssessmentService) domainOfAsset(ctx context.Context) map[string]string {
	out := map[string]string{}
	assets, err := svc.store.GetAssets(ctx, "")
	if err != nil {
		return out
	}
	for _, a := range assets {
		out[a.ID] = a.Value
	}
	return out
}

// ResolveAssetID finds the asset identifier for a domain without creating one.
//
// It returns an empty string when the domain has never been seen, so that a
// read path can report "nothing assessed" rather than silently registering an
// asset as a side effect of somebody looking at a page.
func (svc *AssessmentService) ResolveAssetID(ctx context.Context, domain string) string {
	want := NormaliseDomain(domain)
	assets, err := svc.store.GetAssets(ctx, "")
	if err != nil {
		return ""
	}
	for _, a := range assets {
		if NormaliseDomain(a.Value) == want {
			return a.ID
		}
	}
	return ""
}

// NewAssessmentService builds the service and loads the signal registry.
//
// The registry is supplied as bytes rather than a path so that a packaged
// desktop binary carries it in the executable. A registry that must be found
// on disk is a registry that is missing in production, and observations mapped
// against a missing registry would all be recorded as unmapped.
func NewAssessmentService(s store.Store, bus event.Bus, registryJSON []byte) (*AssessmentService, error) {
	reg, err := vadapter.ReadSignalRegistry(bytes.NewReader(registryJSON))
	if err != nil {
		return nil, fmt.Errorf("assessment: loading the signal registry: %w", err)
	}
	return &AssessmentService{store: s, bus: bus, registry: reg}, nil
}

// SyncRegistry persists the loaded registry so that readers of the store see
// the same mapping the adapter assessed against.
func (svc *AssessmentService) SyncRegistry(ctx context.Context) error {
	return svc.store.ReplaceSignalRegistry(ctx, svc.registry.Entries())
}

// --- View model ---------------------------------------------------------
//
// Every timestamp is carried as an RFC 3339 string. Wails cannot generate a
// binding for time.Time, and a silently zeroed date in the UI is worse than a
// string the frontend must parse.

// SignalView is one observation, joined to what the registry says it means.
type SignalView struct {
	SignalID string `json:"signalId"`
	CheckID  string `json:"checkId"`

	// Condition is the registry's description of what was observed. It is
	// empty for an unmapped signal, where Evidence is the only account.
	Condition     string `json:"condition"`
	WeaknessClass string `json:"weaknessClass"`
	Scenario      string `json:"scenario"`
	Stage         string `json:"stage"`
	Control       string `json:"control"`
	Direction     string `json:"direction"`

	State    store.CoverageState   `json:"state"`
	Severity store.FindingSeverity `json:"severity"`
	Evidence string                `json:"evidence"`
	Mapped   bool                  `json:"mapped"`

	// Description, Remediation and References come from vantage's finding
	// catalogue rather than from the stored observation. They are static
	// library text, identical for every occurrence of an identifier, so
	// copying them into each stored row would duplicate prose that a library
	// upgrade should be free to correct. Reading them at view-build time
	// means the explanation always matches the library actually installed.
	//
	// Without these, the UI can only show the title and the raw evidence
	// keys, which states what was observed but never why it matters — and a
	// finding an operator cannot interpret is a finding they cannot act on.
	Description string   `json:"description,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
	References  []string `json:"references,omitempty"`

	RegistryVersion string `json:"registryVersion"`
	LibraryVersion  string `json:"libraryVersion"`
	ObservedAt      string `json:"observedAt"`
	FirstSeen       string `json:"firstSeen"`
}

// CheckView is the coverage record for one check, so the UI can say why a
// control is unknown rather than merely that it is.
type CheckView struct {
	CheckID string              `json:"checkId"`
	State   store.CoverageState `json:"state"`
	Reason  string              `json:"reason,omitempty"`
}

// ControlView is one defensive mechanism — SPF, DMARC, DNSSEC — with its
// derived posture, the coverage behind that posture, and the advisories raised
// against it.
type ControlView struct {
	Control string               `json:"control"`
	Posture store.ControlPosture `json:"posture"`

	// Coverage counts the checks feeding this control by state. It accompanies
	// the posture always: "compliant" and "we never looked" must never be
	// distinguishable only by reading the signal list.
	Coverage store.CoverageSummary `json:"coverage"`

	Checks  []CheckView  `json:"checks"`
	Signals []SignalView `json:"signals"`
}

// ScenarioView is one attack scenario with how much assessment supports it.
type ScenarioView struct {
	Scenario    string                `json:"scenario"`
	Coverage    store.CoverageSummary `json:"coverage"`
	Aggravating int                   `json:"aggravating"`
	// Significant is the subset of Aggravating at medium severity or above.
	// Both are carried so the UI can lead with the weightier number without
	// the lighter one ceasing to exist.
	Significant int  `json:"significant"`
	Mitigating  int  `json:"mitigating"`
	Supported   bool `json:"supported"`
}

// DomainAssessment is everything the UI needs about one domain.
type DomainAssessment struct {
	AssetID string `json:"assetId"`
	Domain  string `json:"domain"`

	// Outcome is how the last assessment ended, distinct from what it found.
	Outcome string `json:"outcome"`
	// Error names why, when the outcome is not completed or partial.
	Error string `json:"error,omitempty"`

	Coverage  store.CoverageSummary `json:"coverage"`
	Fraction  float64               `json:"coverageFraction"`
	Controls  []ControlView         `json:"controls"`
	Scenarios []ScenarioView        `json:"scenarios"`
	// Unmapped are observations the registry does not know. They are surfaced
	// rather than hidden so that a library upgrade adding a finding is visible
	// to the operator instead of silently absent.
	Unmapped []SignalView `json:"unmapped"`

	// Attribution is where each address the domain resolves to is hosted.
	// It accompanies the posture because an operator asking "is any of this
	// somewhere it should not be" is asking a question the control model
	// cannot answer.
	Attribution []store.AssetAttribution `json:"attribution"`

	// AttributionProvenance says where the range data came from and when. It
	// travels with the attribution, never separately: an attribution shown
	// without its basis invites the reader to treat it as current when it may
	// have been fetched from a cache days ago.
	AttributionProvenance []store.AttributionProvenance `json:"attributionProvenance"`

	RegistryVersion string `json:"registryVersion"`
	LibraryVersion  string `json:"libraryVersion"`
	AssessedAt      string `json:"assessedAt,omitempty"`
}

// --- Assessment ---------------------------------------------------------

// Assess runs an assessment of one domain, bounded by the authorised scope,
// persists the results and returns the assembled view.
//
// The scope is a required argument. An assessment with no declared scope is
// refused rather than defaulted to "everything": the transport guard is
// Trawl's strongest guarantee, and a caller who forgot to state the
// authorisation must not silently receive an unguarded run.
func (svc *AssessmentService) Assess(
	ctx context.Context,
	domain string,
	authorisedDomains []string,
	consentedEndpoints []string,
) (DomainAssessment, error) {
	domain = NormaliseDomain(domain)
	if domain == "" {
		return DomainAssessment{}, fmt.Errorf("assessment: a domain is required")
	}

	started := time.Now().UTC()

	// The asset is registered before anything is queried — before the scope
	// guard, even — so that a refusal has somewhere to attach. Registering a
	// domain in the local inventory contacts nothing, so doing it ahead of the
	// guard cannot reach outside the authorised scope.
	assetID, err := svc.ensureAsset(ctx, domain)
	if err != nil {
		return DomainAssessment{}, err
	}

	scope := vadapter.NewScope(authorisedDomains, consentedEndpoints)
	if !scope.PermitsTarget(domain) {
		// Recorded, not merely returned. A refusal that leaves no trace is
		// indistinguishable from an assessment that was never requested.
		reason := fmt.Sprintf("%q is outside the authorised scope", domain)
		if err := svc.recordRun(ctx, store.AssessmentRun{
			AssetID:    assetID,
			Outcome:    string(vadapter.OutcomeRefused),
			Error:      reason,
			StartedAt:  started,
			FinishedAt: time.Now().UTC(),
		}); err != nil {
			return DomainAssessment{}, err
		}
		return DomainAssessment{
			AssetID: assetID,
			Domain:  domain,
			Outcome: string(vadapter.OutcomeRefused),
			Error:   reason,
		}, nil
	}

	resolver := vpkg.NewClient(vpkg.Config{})
	assessor, _, _, err := vadapter.NewScopedAssessor(resolver, scope)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: building the assessor: %w", err)
	}

	adapter, err := vadapter.New(assessor,
		vadapter.WithRegistry(svc.registry),
		vadapter.WithEventBus(svc.bus),
	)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: building the adapter: %w", err)
	}

	res, assessErr := adapter.Assess(ctx, vadapter.Request{
		AssetID: assetID,
		Domain:  domain,
		Profile: string(vaudit.ProfileStandard),
		Scope:   scope,
	})

	run := store.AssessmentRun{
		AssetID:        assetID,
		Outcome:        string(res.Outcome),
		Profile:        string(vaudit.ProfileStandard),
		LibraryVersion: res.LibraryVersion,
		StartedAt:      started,
		FinishedAt:     time.Now().UTC(),
	}
	if assessErr != nil {
		run.Error = assessErr.Error()
	}

	// Persistence happens whatever the outcome. Coverage records for checks
	// that did not conclude are the point of the four-state model, and
	// discarding them on error would erase exactly the information a reader
	// needs to know how much of the assessment actually happened.
	if err := svc.persist(ctx, res); err != nil {
		return DomainAssessment{}, err
	}
	if err := svc.recordRun(ctx, run); err != nil {
		return DomainAssessment{}, err
	}

	view, err := svc.View(ctx, assetID)
	if err != nil {
		return DomainAssessment{}, err
	}
	// The outcome and error now come back from the stored run, so they are not
	// reapplied here: overriding them would hide any disagreement between what
	// was written and what the read path makes of it.
	if res.LibraryVersion != "" {
		view.LibraryVersion = res.LibraryVersion
	}
	return view, nil
}

// recordRun persists how an assessment ended.
func (svc *AssessmentService) recordRun(ctx context.Context, run store.AssessmentRun) error {
	if err := svc.store.RecordAssessmentRun(ctx, &run); err != nil {
		return fmt.Errorf("assessment: recording the run for %s: %w", run.AssetID, err)
	}
	return nil
}

// persist writes observations and coverage to the store.
func (svc *AssessmentService) persist(ctx context.Context, res vadapter.Result) error {
	// Discovered names first. They are facts about the estate that stand
	// independently of whether the rest of this result can be written, and
	// recording them first means a later failure loses the assessment rather
	// than the asset. Nothing is removed here: the inventory upserts, so a
	// name a run did not disclose keeps its place, because a log falling
	// silent is not evidence that a host was withdrawn.
	for i := range res.Discovered {
		if err := svc.store.SaveAsset(ctx, &res.Discovered[i]); err != nil {
			return fmt.Errorf("assessment: recording %s discovered by certificate transparency: %w", res.Discovered[i].Value, err)
		}
	}

	for i := range res.Coverage {
		if err := svc.store.RecordAssessmentCoverage(ctx, &res.Coverage[i]); err != nil {
			return fmt.Errorf("assessment: recording coverage for %s: %w", res.Coverage[i].CheckID, err)
		}
	}
	for i := range res.Observations {
		if err := svc.store.SaveSignalObservation(ctx, &res.Observations[i]); err != nil {
			return fmt.Errorf("assessment: saving observation %s: %w", res.Observations[i].SignalID, err)
		}
	}

	// Only written when the network check actually produced an observation.
	// Replacing with an empty set is how "we looked and attributed nothing"
	// is recorded, so calling it unconditionally would erase a previous run's
	// attribution every time the check was excluded by policy or never ran —
	// silently, and in the direction that looks clean.
	if res.AttributionAttempted {
		assetID := ""
		if len(res.Coverage) > 0 {
			assetID = res.Coverage[0].AssetID
		}
		if len(res.Attribution) > 0 {
			assetID = res.Attribution[0].AssetID
		}
		if assetID != "" {
			// Before the replacement, because the comparison needs the basis
			// the previous attribution rested on and the replacement discards
			// it.
			if err := svc.recordAttributionChange(ctx, assetID, res); err != nil {
				return err
			}
			if err := svc.store.ReplaceAssetAttribution(ctx, assetID, res.Attribution, res.AttributionProvenance); err != nil {
				return fmt.Errorf("assessment: saving attribution: %w", err)
			}
		}
	}

	// The email-authentication posture, and the policy drift it may represent.
	if res.EmailPosture != nil {
		assetID := ""
		if len(res.Coverage) > 0 {
			assetID = res.Coverage[0].AssetID
		}
		if err := svc.recordEmailPosture(ctx, assetID, res.EmailPosture); err != nil {
			return err
		}
	}
	return nil
}

// recordEmailPosture writes the posture and tracks DMARC policy as a posture
// attribute, so that a weakening is a raised transition with history rather
// than a silent overwrite.
//
// The drift is recorded only when the policy was actually assessed. An
// unassessed run that wrote its fingerprint would move the baseline to
// "unknown", and the next successful run would then report a change from
// nothing to the policy that had been there all along — a regression invented
// by our own outage, arriving in the operator's list alongside the real ones
// and indistinguishable from them.
func (svc *AssessmentService) recordEmailPosture(ctx context.Context, assetID string, posture *store.EmailPosture) error {
	// Recomputed here rather than trusted from the caller. Priority is a
	// deterministic function of the observed tags, and deriving it at the
	// boundary means no code path — present or future, adapter or importer —
	// can persist a domain under a severity its evidence does not support.
	posture.Priority = posture.WorstSeverity()

	if err := svc.store.SaveEmailPosture(ctx, posture); err != nil {
		return fmt.Errorf("assessment: saving the email posture for %s: %w", posture.Domain, err)
	}

	fingerprint := posture.DMARCFingerprint()
	if fingerprint == "" || assetID == "" {
		return nil
	}

	if _, err := svc.store.RecordPostureObservation(ctx, assetID, store.DMARCPolicyAttribute, fingerprint); err != nil {
		return fmt.Errorf("assessment: recording the DMARC policy change for %s: %w", posture.Domain, err)
	}
	return nil
}

// recordAttributionChange tracks hosting as a posture attribute, suppressing
// changes that the provider data explains.//
// A change in what an address attributes to has two possible causes, and only
// one of them is a change in the estate. Providers republish their ranges
// constantly; a prefix moving between two publications makes an address
// attribute differently without anything having moved. Raising that as a
// regression would fill the list with noise, and a list of mostly noise is
// one an operator stops reading — at which point the genuine move is missed
// as well.
//
// So the comparison is conditioned on the basis. When both runs attributed
// against the same provider data, a difference is the estate's and is raised.
// When the basis itself changed, the difference is recorded as the new
// baseline and nothing is raised: not because it does not matter, but because
// this evidence cannot distinguish it from a republication.
//
// The suppressed case still writes the snapshot. Skipping it would leave the
// baseline at the pre-refresh value, so the next run would raise the same
// change anyway — suppression that defers rather than suppresses.
func (svc *AssessmentService) recordAttributionChange(ctx context.Context, assetID string, res vadapter.Result) error {
	previous, err := svc.store.GetAttributionProvenance(ctx, assetID)
	if err != nil {
		return fmt.Errorf("assessment: reading the previous attribution basis: %w", err)
	}

	fingerprint := store.HostingFingerprint(res.Attribution)

	if vadapter.SameAttributionBasis(previous, res.AttributionProvenance) {
		if _, err := svc.store.RecordPostureObservation(ctx, assetID, store.AttributionAttribute, fingerprint); err != nil {
			return fmt.Errorf("assessment: recording the hosting change: %w", err)
		}
		return nil
	}

	if err := svc.store.RecordPostureBaseline(ctx, assetID, store.AttributionAttribute, fingerprint); err != nil {
		return fmt.Errorf("assessment: recording the hosting baseline: %w", err)
	}
	return nil
}

// --- Read model ---------------------------------------------------------

// View assembles the stored assessment for one asset without running anything.
func (svc *AssessmentService) View(ctx context.Context, assetID string) (DomainAssessment, error) {
	return svc.viewWithDomain(ctx, assetID, "")
}

// viewWithDomain assembles a view, using the supplied domain for display when
// the caller already knows it and avoiding a lookup per asset in a batch.
func (svc *AssessmentService) viewWithDomain(ctx context.Context, assetID, domain string) (DomainAssessment, error) {
	registry, err := svc.effectiveRegistry(ctx)
	if err != nil {
		return DomainAssessment{}, err
	}

	coverage, err := svc.store.GetAssessmentCoverage(ctx, assetID)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: reading coverage: %w", err)
	}
	observations, err := svc.store.GetSignalObservations(ctx, assetID)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: reading observations: %w", err)
	}
	runs, err := svc.store.GetAssessmentRuns(ctx, assetID)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: reading the assessment run: %w", err)
	}
	var run store.AssessmentRun
	if len(runs) > 0 {
		run = runs[0]
	}

	attribution, err := svc.store.GetAssetAttribution(ctx, assetID)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: reading attribution: %w", err)
	}
	provenance, err := svc.store.GetAttributionProvenance(ctx, assetID)
	if err != nil {
		return DomainAssessment{}, fmt.Errorf("assessment: reading attribution provenance: %w", err)
	}

	if domain == "" {
		domain = svc.domainOfAsset(ctx)[assetID]
	}
	return svc.assemble(assetID, domain, registry, coverage, observations, run,
		attributionBundle{rows: attribution, provenance: provenance}), nil
}

// attributionBundle keeps an asset's attribution together with the basis it
// rests on.
//
// They are passed as one value rather than two arguments so that a caller
// cannot assemble a view with attribution and forget the provenance.
// Attribution shown without its basis reads as current when it may have come
// from a cache days old, and that is a reassuring error rather than a visible
// one.
type attributionBundle struct {
	rows       []store.AssetAttribution
	provenance []store.AttributionProvenance
}

// effectiveRegistry returns the registry the read model should join against.
//
// It prefers the stored copy, so that a view reflects the mapping observations
// were actually recorded under. An empty store means it has not been seeded
// yet; falling back to the embedded registry lets a first run render meaning
// rather than a wall of unmapped identifiers.
func (svc *AssessmentService) effectiveRegistry(ctx context.Context) ([]store.SignalRegistryEntry, error) {
	registry, err := svc.store.GetSignalRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("assessment: reading the signal registry: %w", err)
	}
	if len(registry) == 0 {
		return svc.registry.Entries(), nil
	}
	return registry, nil
}

// Views assembles the stored assessment for every domain asset that has one.
//
// Everything is read in a fixed number of queries and grouped in memory,
// rather than assembling each asset in turn. A portfolio view is the common
// case here, and a per-asset read would issue four queries per domain to
// retrieve slices of data the first four queries already returned in full.
func (svc *AssessmentService) Views(ctx context.Context) ([]DomainAssessment, error) {
	registry, err := svc.effectiveRegistry(ctx)
	if err != nil {
		return nil, err
	}
	observations, err := svc.store.GetSignalObservations(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("assessment: reading observations: %w", err)
	}
	coverage, err := svc.store.GetAssessmentCoverage(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("assessment: reading coverage: %w", err)
	}
	runs, err := svc.store.GetAssessmentRuns(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("assessment: reading assessment runs: %w", err)
	}
	attribution, err := svc.store.GetAssetAttribution(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("assessment: reading attribution: %w", err)
	}
	provenance, err := svc.store.GetAttributionProvenance(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("assessment: reading attribution provenance: %w", err)
	}

	observationsOf := map[string][]store.SignalObservation{}
	for _, o := range observations {
		observationsOf[o.AssetID] = append(observationsOf[o.AssetID], o)
	}
	coverageOf := map[string][]store.AssessmentCoverage{}
	for _, c := range coverage {
		coverageOf[c.AssetID] = append(coverageOf[c.AssetID], c)
	}
	runOf := make(map[string]store.AssessmentRun, len(runs))
	for _, r := range runs {
		runOf[r.AssetID] = r
	}
	attributionOf := map[string]attributionBundle{}
	for _, a := range attribution {
		b := attributionOf[a.AssetID]
		b.rows = append(b.rows, a)
		attributionOf[a.AssetID] = b
	}
	for _, p := range provenance {
		b := attributionOf[p.AssetID]
		b.provenance = append(b.provenance, p)
		attributionOf[p.AssetID] = b
	}

	// An asset with coverage but no observations is a clean assessment, and it
	// must appear. So must one with only a run record: that is how a refusal
	// or an outright failure presents, and those are the cases an operator
	// most needs to see. Collecting identifiers from all three sides is what
	// stops any of them vanishing from the list.
	seen := map[string]bool{}
	for id := range observationsOf {
		seen[id] = true
	}
	for id := range coverageOf {
		seen[id] = true
	}
	for id := range runOf {
		seen[id] = true
	}
	// Attribution counts too. It is evidence that the estate was examined, so
	// an asset carrying only attribution must still appear — dropping it would
	// hide a domain we have hosting data for behind the fact that no check
	// happened to record coverage against it.
	for id := range attributionOf {
		seen[id] = true
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	domains := svc.domainOfAsset(ctx)

	out := make([]DomainAssessment, 0, len(ids))
	for _, id := range ids {
		out = append(out, svc.assemble(
			id, domains[id], registry, coverageOf[id], observationsOf[id], runOf[id], attributionOf[id],
		))
	}
	return out, nil
}

// assemble joins registry, coverage, observations and the run record into the
// view model. It takes them all as arguments rather than reading the store, so
// that the join can be exercised directly by a test.
func (svc *AssessmentService) assemble(
	assetID string,
	domain string,
	registry []store.SignalRegistryEntry,
	coverage []store.AssessmentCoverage,
	observations []store.SignalObservation,
	run store.AssessmentRun,
	attribution attributionBundle,
) DomainAssessment {
	entryOf := make(map[string]store.SignalRegistryEntry, len(registry))
	for _, e := range registry {
		entryOf[e.SignalID] = e
	}

	postures := store.DeriveControlPostures(registry, coverage, observations)
	scenarios := store.ComputeScenarioCoverage(registry, coverage, observations)

	// Which checks and dedup groups belong to which control. The registry is
	// the authority: deriving membership from the observations instead would
	// make a control disappear on the run where it raised nothing, which is
	// precisely the run where its health matters most.
	checksOfControl := map[string]map[string]bool{}
	groupsOfControl := map[string]map[string]bool{}
	for _, e := range registry {
		if e.Control == "" || e.DedupGroup == "" {
			continue
		}
		if checksOfControl[e.Control] == nil {
			checksOfControl[e.Control] = map[string]bool{}
			groupsOfControl[e.Control] = map[string]bool{}
		}
		groupsOfControl[e.Control][e.DedupGroup] = true
		if check, _, ok := strings.Cut(e.DedupGroup, ":"); ok && check != "" {
			checksOfControl[e.Control][check] = true
		}
	}

	stateOfCheck := make(map[string]store.CoverageState, len(coverage))
	reasonOfCheck := make(map[string]string, len(coverage))
	for _, c := range coverage {
		stateOfCheck[c.CheckID] = c.State
		reasonOfCheck[c.CheckID] = c.Reason
	}

	signalsOfControl := map[string][]SignalView{}
	// Initialised rather than left nil. A nil slice marshals to JSON null,
	// and the view model promises an array: a consumer reading .length off
	// null fails at render, which presents as an empty panel rather than as
	// an error anyone can trace back to here.
	unmapped := []SignalView{}
	var latest time.Time
	registryVersion, libraryVersion := "", ""

	for _, o := range observations {
		view := SignalView{
			SignalID:        o.SignalID,
			CheckID:         o.CheckID,
			State:           o.State,
			Severity:        o.Severity,
			Evidence:        o.Evidence,
			Mapped:          o.Mapped,
			RegistryVersion: o.RegistryVersion,
			LibraryVersion:  o.LibraryVersion,
			ObservedAt:      formatTime(o.ObservedAt),
			FirstSeen:       formatTime(o.FirstSeen),
		}
		if e, ok := entryOf[o.SignalID]; ok {
			view.Condition = e.Condition
			view.WeaknessClass = e.WeaknessClass
			view.Scenario = e.Scenario
			view.Stage = e.Stage
			view.Control = e.Control
			view.Direction = e.Direction
		}
		if c, ok := catalogueEntry(o.SignalID); ok {
			view.Description = c.Description
			view.Remediation = c.Remediation
			view.References = c.References
			// An unmapped signal has no registry Condition, but the catalogue
			// still names it. Falling back here means the UI shows a title
			// rather than a bare identifier.
			if view.Condition == "" {
				view.Condition = c.Title
			}
		}

		if o.ObservedAt.After(latest) {
			latest = o.ObservedAt
		}
		if registryVersion == "" {
			registryVersion = o.RegistryVersion
		}
		if libraryVersion == "" {
			libraryVersion = o.LibraryVersion
		}

		if view.Control == "" {
			unmapped = append(unmapped, view)
			continue
		}
		signalsOfControl[view.Control] = append(signalsOfControl[view.Control], view)
	}

	controls := make([]ControlView, 0, len(checksOfControl))
	for control, checks := range checksOfControl {
		cv := ControlView{
			Control: control,
			Posture: store.PostureUnknown,
			Checks:  []CheckView{},
			Signals: []SignalView{},
		}

		checkIDs := make([]string, 0, len(checks))
		for check := range checks {
			checkIDs = append(checkIDs, check)
		}
		sort.Strings(checkIDs)

		for _, check := range checkIDs {
			state, ok := stateOfCheck[check]
			if !ok {
				// Absent from the coverage table means the check was never
				// attempted. That is not_checked, not a fifth state, and never
				// an implicit pass.
				state = store.CoverageNotChecked
			}
			cv.Checks = append(cv.Checks, CheckView{
				CheckID: check,
				State:   state,
				Reason:  reasonOfCheck[check],
			})
			cv.Coverage.Total++
			switch state {
			case store.CoverageOK:
				cv.Coverage.OK++
				cv.Coverage.AssessedOnly++
			case store.CoverageNotFound:
				cv.Coverage.NotFound++
				cv.Coverage.AssessedOnly++
			case store.CoverageCheckFailed:
				cv.Coverage.CheckFailed++
			default:
				cv.Coverage.NotChecked++
			}
		}

		// A control is deficient if any of its groups is, compliant only if
		// every one of them is, and unknown otherwise. Requiring unanimity for
		// compliance is what stops a partially assessed control reading as
		// healthy on the strength of the checks that happened to run.
		deficient, compliant, total := 0, 0, 0
		for group := range groupsOfControl[control] {
			total++
			switch postures[group] {
			case store.PostureDeficient:
				deficient++
			case store.PostureCompliant:
				compliant++
			}
		}
		switch {
		case deficient > 0:
			cv.Posture = store.PostureDeficient
		case total > 0 && compliant == total:
			cv.Posture = store.PostureCompliant
		}

		cv.Signals = signalsOfControl[control]
		if cv.Signals == nil {
			// A control with nothing raised against it is the common case, and
			// the frontend must be able to count it as zero rather than trip
			// over a null.
			cv.Signals = []SignalView{}
		}
		sort.Slice(cv.Signals, func(i, j int) bool {
			if cv.Signals[i].Severity != cv.Signals[j].Severity {
				return severityRank(cv.Signals[i].Severity) < severityRank(cv.Signals[j].Severity)
			}
			return cv.Signals[i].SignalID < cv.Signals[j].SignalID
		})

		controls = append(controls, cv)
	}

	// Deficient controls first, then unknown, then compliant: the operator's
	// attention belongs on what is broken and on what was never established,
	// in that order.
	sort.Slice(controls, func(i, j int) bool {
		if postureRank(controls[i].Posture) != postureRank(controls[j].Posture) {
			return postureRank(controls[i].Posture) < postureRank(controls[j].Posture)
		}
		return controls[i].Control < controls[j].Control
	})

	scenarioViews := make([]ScenarioView, 0, len(scenarios))
	for _, sc := range scenarios {
		scenarioViews = append(scenarioViews, ScenarioView{
			Scenario:    sc.Scenario,
			Coverage:    sc.Coverage,
			Aggravating: sc.Aggravating,
			Significant: sc.Significant,
			Mitigating:  sc.Mitigating,
			Supported:   sc.Supported(),
		})
	}
	// Ordered by weight first, so a scenario with one real weakness sorts
	// above one with three minor notes. Ties fall back to the raw count, then
	// to the name, so the order is total and does not shuffle between reads.
	sort.Slice(scenarioViews, func(i, j int) bool {
		if scenarioViews[i].Significant != scenarioViews[j].Significant {
			return scenarioViews[i].Significant > scenarioViews[j].Significant
		}
		if scenarioViews[i].Aggravating != scenarioViews[j].Aggravating {
			return scenarioViews[i].Aggravating > scenarioViews[j].Aggravating
		}
		return scenarioViews[i].Scenario < scenarioViews[j].Scenario
	})

	overall := store.CoverageSummary{}
	for _, c := range coverage {
		overall.Total++
		switch c.State {
		case store.CoverageOK:
			overall.OK++
			overall.AssessedOnly++
		case store.CoverageNotFound:
			overall.NotFound++
			overall.AssessedOnly++
		case store.CoverageCheckFailed:
			overall.CheckFailed++
		default:
			overall.NotChecked++
		}
		if c.AssessedAt.After(latest) {
			latest = c.AssessedAt
		}
		if libraryVersion == "" {
			libraryVersion = c.LibraryVersion
		}
	}

	if registryVersion == "" {
		registryVersion = svc.registry.Version()
	}
	if domain == "" {
		// Better the identifier than an empty heading: a card with no name
		// cannot be acted on, and the identifier is at least traceable.
		domain = assetID
	}

	// The outcome is whatever was recorded, never inferred from the presence
	// of rows. An asset carrying coverage but no run predates this record or
	// was written by a path that did not keep one; reporting that as unknown
	// is honest, where reporting it as completed is not.
	outcome := run.Outcome
	if outcome == "" {
		outcome = outcomeUnknown
	}
	if libraryVersion == "" {
		libraryVersion = run.LibraryVersion
	}
	if run.FinishedAt.After(latest) {
		// A refused or wholly failed run writes neither coverage nor
		// observations, so its own timestamp is the only evidence that
		// anything was attempted at all.
		latest = run.FinishedAt
	}

	// Never nil, so the transport encodes an empty array rather than null. A
	// consumer reading null has to decide for itself whether it means "none"
	// or "not supplied", and the two are not the same claim.
	rows := attribution.rows
	if rows == nil {
		rows = []store.AssetAttribution{}
	}
	provenance := attribution.provenance
	if provenance == nil {
		provenance = []store.AttributionProvenance{}
	}

	return DomainAssessment{
		AssetID:               assetID,
		Domain:                domain,
		Outcome:               outcome,
		Error:                 run.Error,
		Coverage:              overall,
		Fraction:              overall.Fraction(),
		Controls:              controls,
		Scenarios:             scenarioViews,
		Unmapped:              unmapped,
		Attribution:           rows,
		AttributionProvenance: provenance,
		RegistryVersion:       registryVersion,
		LibraryVersion:        libraryVersion,
		AssessedAt:            formatTime(latest),
	}
}

// severityRank orders severities most serious first.
func severityRank(s store.FindingSeverity) int {
	switch s {
	case store.SeverityCritical:
		return 0
	case store.SeverityHigh:
		return 1
	case store.SeverityMedium:
		return 2
	case store.SeverityLow:
		return 3
	default:
		return 4
	}
}

// postureRank orders postures by how much they demand attention.
func postureRank(p store.ControlPosture) int {
	switch p {
	case store.PostureDeficient:
		return 0
	case store.PostureUnknown:
		return 1
	default:
		return 2
	}
}

// formatTime renders a timestamp for the frontend, returning an empty string
// for the zero value rather than the year 1.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
