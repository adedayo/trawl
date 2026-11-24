package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adedayo/trawl/pkg/contact"
	"github.com/adedayo/trawl/pkg/narrative"
	"github.com/adedayo/trawl/pkg/store"
)

// FindingsService assembles the unified findings read model: one list across
// every source, one detail view per finding, and the change views drawn from the
// append-only history. It reads; it never writes, and it never computes a
// severity or priority, which are stored values.
type FindingsService struct {
	store     store.Store
	explainer narrative.Explainer
	clock     func() time.Time
}

// NewFindingsService builds the read model. explainer renders the plain-English
// explanation; nil falls back to the catalogue's own text so a deployment that
// has not wired a narrative set still shows something honest.
func NewFindingsService(s store.Store, explainer narrative.Explainer) *FindingsService {
	return &FindingsService{store: s, explainer: explainer, clock: func() time.Time { return time.Now().UTC() }}
}

// SetClock fixes the clock the read model measures age against. For tests.
func (s *FindingsService) SetClock(clock func() time.Time) { s.clock = clock }

func rfc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// findingRecord is one finding with the rows it was built from.
type findingRecord struct {
	summary   FindingSummary
	signal    *store.SignalObservation
	finding   *store.Finding
	secret    *store.SecretFinding
	candidate *store.CandidateFinding

	checkID     string
	checkState  store.CoverageState
	checkReason string
}

type loaded struct {
	records  []*findingRecord
	byKey    map[string]*findingRecord
	events   map[string][]store.FindingEvent
	assets   map[string]string
	registry map[string]store.SignalRegistryEntry
}

func statusOf(s store.FindingStatus) string {
	if s == "" {
		return string(store.FindingOpen)
	}
	return string(s)
}

func (s *FindingsService) load(ctx context.Context) (*loaded, error) {
	ld := &loaded{
		byKey:    map[string]*findingRecord{},
		events:   map[string][]store.FindingEvent{},
		assets:   map[string]string{},
		registry: map[string]store.SignalRegistryEntry{},
	}

	assets, err := s.store.GetAssets(ctx, store.AssetStatusActive)
	if err != nil {
		return nil, fmt.Errorf("findings: reading assets: %w", err)
	}
	for _, a := range assets {
		ld.assets[a.ID] = a.Value
	}

	reg, err := s.store.GetSignalRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("findings: reading the signal registry: %w", err)
	}
	for _, e := range reg {
		ld.registry[e.SignalID] = e
	}

	coverage, err := s.store.GetAssessmentCoverage(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("findings: reading coverage: %w", err)
	}
	cov := make(map[string]store.AssessmentCoverage, len(coverage))
	for _, c := range coverage {
		cov[c.AssetID+"\x00"+c.CheckID] = c
	}

	events, err := s.store.GetFindingEvents(ctx, store.EventQuery{})
	if err != nil {
		return nil, fmt.Errorf("findings: reading history: %w", err)
	}
	for _, e := range events {
		ld.events[e.Key] = append(ld.events[e.Key], e)
	}

	now := s.clock()
	add := func(r *findingRecord) {
		r.summary.AssetValue = ld.assets[r.summary.AssetID]
		end := now
		if r.summary.ResolvedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.summary.ResolvedAt); err == nil {
				end = t
			}
		}
		if first, err := time.Parse(time.RFC3339, r.summary.FirstSeen); err == nil && end.After(first) {
			r.summary.AgeDays = int(end.Sub(first).Hours() / 24)
		}
		r.summary.LastChange, r.summary.LastChangeType = r.summary.FirstSeen, string(store.EventFirstSeen)
		if evs := ld.events[r.summary.Key]; len(evs) > 0 {
			last := evs[len(evs)-1]
			r.summary.LastChange, r.summary.LastChangeType = rfc(last.At), string(last.Type)
		}
		ld.records = append(ld.records, r)
		ld.byKey[r.summary.Key] = r
	}

	signals, err := s.store.GetAllSignalObservations(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("findings: reading observations: %w", err)
	}
	for i := range signals {
		o := &signals[i]
		// An observation recorded at a state that is not an assessment is not a
		// finding: it is evidence of nothing.
		if !o.State.Assessed() {
			continue
		}
		title := o.SignalID
		category := o.CheckID
		if e, ok := catalogueEntry(o.SignalID); ok {
			title = e.Title
		} else if r, ok := ld.registry[o.SignalID]; ok && r.Condition != "" {
			title = r.Condition
		}
		r := &findingRecord{
			signal:  o,
			checkID: o.CheckID,
			summary: FindingSummary{
				Key: store.FindingKey(store.SourceSignal, o.ID), Source: SourceSignal,
				AssetID: o.AssetID, Title: title, SignalID: o.SignalID,
				Severity: o.Severity, Priority: string(o.Severity), Status: statusOf(o.Status),
				Category: category, FirstSeen: rfc(o.FirstSeen), LastSeen: rfc(o.LastSeen),
			},
		}
		if o.ResolvedAt != nil {
			r.summary.ResolvedAt = rfc(*o.ResolvedAt)
		}
		if c, ok := cov[o.AssetID+"\x00"+o.CheckID]; ok {
			r.checkState, r.checkReason = c.State, c.Reason
			// Open only because the check could not conclude on the latest run:
			// still counted, but not confirmed by it.
			if r.summary.Status != string(store.FindingResolved) && !c.State.Assessed() {
				r.summary.NotReassessed = true
				r.summary.NotReassessedReason = notReassessedReason(c)
			}
		}
		add(r)
	}

	findings, err := s.store.GetFindings(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("findings: reading findings: %w", err)
	}
	for i := range findings {
		f := &findings[i]
		prio := f.Priority
		if prio == "" {
			prio = string(f.Severity)
		}
		r := &findingRecord{
			finding: f,
			summary: FindingSummary{
				Key: store.FindingKey(store.SourceFinding, f.ID), Source: SourceFinding,
				AssetID: f.AssetID, Title: f.Title, Severity: f.Severity, Priority: prio,
				Status: statusOf(f.Status), Category: f.Category, CVE: f.CVE, KEV: f.KEVListed,
				FirstSeen: rfc(f.FirstSeen), LastSeen: rfc(f.LastSeen),
			},
		}
		if f.ResolvedAt != nil {
			r.summary.ResolvedAt = rfc(*f.ResolvedAt)
		}
		add(r)
	}

	secrets, err := s.store.GetSecretFindings(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("findings: reading secrets: %w", err)
	}
	for i := range secrets {
		sf := &secrets[i]
		sev := store.SecretSeverity(sf.Verified)
		r := &findingRecord{
			secret: sf,
			summary: FindingSummary{
				Key: store.FindingKey(store.SourceSecret, sf.ID), Source: SourceSecret,
				AssetID: sf.AssetID, SecretID: sf.ID, Title: fmt.Sprintf("Exposed %s in %s", sf.SecretType, sf.FilePath),
				Severity: sev, Priority: string(sev), Status: statusOf(sf.Status), Category: "secret",
				FirstSeen: rfc(sf.FirstSeen), LastSeen: rfc(sf.LastSeen),
			},
		}
		if sf.ResolvedAt != nil {
			r.summary.ResolvedAt = rfc(*sf.ResolvedAt)
		}
		add(r)
	}

	candidates, err := s.store.GetCandidateFindings(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("findings: reading candidates: %w", err)
	}
	for i := range candidates {
		cf := &candidates[i]
		prio := cf.Priority
		if prio == "" {
			prio = store.ComputeFindingPriority(cf.Severity, cf.EPSS, cf.KEVListed)
		}
		r := &findingRecord{
			candidate: cf,
			summary: FindingSummary{
				Key: store.FindingKey(store.SourceCandidate, cf.ID), Source: SourceCandidate,
				AssetID: cf.AssetID, Title: cf.Title, Severity: cf.Severity, Priority: prio,
				Status: statusOf(cf.Status), NotReassessed: cf.NotReassessed, NotReassessedReason: cf.NotReassessedReason,
				Category: "software-candidate", CVE: cf.CVE, KEV: cf.KEVListed,
				Unconfirmed: true, SeveritySource: cf.SeveritySource,
				FirstSeen: rfc(cf.FirstSeen), LastSeen: rfc(cf.LastSeen),
			},
		}
		if cf.ResolvedAt != nil {
			r.summary.ResolvedAt = rfc(*cf.ResolvedAt)
		}
		add(r)
	}
	return ld, nil
}

func notReassessedReason(c store.AssessmentCoverage) string {
	switch c.State {
	case store.CoverageCheckFailed:
		if c.Reason != "" {
			return "The latest check could not reach a conclusion: " + c.Reason
		}
		return "The latest check could not reach a conclusion."
	default:
		if c.Reason != "" {
			return "The latest run did not perform this check: " + c.Reason
		}
		return "The latest run did not perform this check."
	}
}

// Index lists findings across every source, most serious first, then most
// recently changed.
func (s *FindingsService) Index(ctx context.Context, f FindingFilter) ([]FindingSummary, error) {
	ld, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	var since time.Time
	if f.ChangedSince != "" {
		since, err = time.Parse(time.RFC3339, f.ChangedSince)
		if err != nil {
			return nil, fmt.Errorf("findings: changedSince %q is not an RFC 3339 time: %w", f.ChangedSince, err)
		}
	}

	out := make([]FindingSummary, 0, len(ld.records))
	for _, r := range ld.records {
		sum := r.summary
		switch f.Status {
		case "":
		case "active":
			if sum.Status == string(store.FindingResolved) {
				continue
			}
		default:
			if sum.Status != f.Status {
				continue
			}
		}
		if f.Severity != "" && string(sum.Severity) != f.Severity {
			continue
		}
		if f.Source != "" && sum.Source != f.Source {
			continue
		}
		if f.AssetID != "" && sum.AssetID != f.AssetID {
			continue
		}
		if !since.IsZero() {
			changed, err := time.Parse(time.RFC3339, sum.LastChange)
			if err != nil || changed.Before(since) {
				continue
			}
			// A finding whose only event is the baseline has not changed: the
			// baseline says it was known, not that anything moved.
			if sum.LastChangeType == string(store.EventBaseline) {
				continue
			}
		}
		out = append(out, sum)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source == SourceCandidate && out[j].Source == SourceCandidate && out[i].KEV != out[j].KEV {
			return out[i].KEV
		}
		if pi, pj := store.FindingSeverity(out[i].Priority).Rank(), store.FindingSeverity(out[j].Priority).Rank(); pi != pj {
			return pi > pj
		}
		if ri, rj := out[i].Severity.Rank(), out[j].Severity.Rank(); ri != rj {
			return ri > rj
		}
		if out[i].LastChange != out[j].LastChange {
			return out[i].LastChange > out[j].LastChange
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func eventView(e store.FindingEvent) FindingEventView {
	return FindingEventView{
		ID: e.ID, Type: string(e.Type), At: rfc(e.At), RunID: e.RunID, Cause: e.Cause,
		PrevSeverity: string(e.PrevSeverity), Severity: string(e.Severity),
		PrevPriority: e.PrevPriority, Priority: e.Priority,
		LibraryVersion: e.LibraryVersion, RegistryVersion: e.RegistryVersion,
		SnapshotID: e.SnapshotID, Note: e.Note,
	}
}

// loadOne loads only the single finding named by (source, rowID) and its own
// indexed event timeline, avoiding a full-estate scan across all tables and
// every finding_event when opening a finding's detail view.
func (s *FindingsService) loadOne(ctx context.Context, key, source, rowID string) (*findingRecord, []store.FindingEvent, map[string]store.SignalRegistryEntry, error) {
	var r *findingRecord
	registry := map[string]store.SignalRegistryEntry{}

	switch source {
	case store.SourceSignal:
		signals, err := s.store.GetAllSignalObservations(ctx, "")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("findings: reading observations: %w", err)
		}
		for i := range signals {
			o := &signals[i]
			if o.ID != rowID || !o.State.Assessed() {
				continue
			}
			title := o.SignalID
			category := o.CheckID
			if e, err := s.store.GetSignalRegistryEntry(ctx, o.SignalID); err == nil && e != nil {
				registry[o.SignalID] = *e
			}
			if e, ok := catalogueEntry(o.SignalID); ok {
				title = e.Title
			} else if reg, ok := registry[o.SignalID]; ok && reg.Condition != "" {
				title = reg.Condition
			}
			rec := &findingRecord{
				signal:  o,
				checkID: o.CheckID,
				summary: FindingSummary{
					Key: key, Source: SourceSignal,
					AssetID: o.AssetID, Title: title, SignalID: o.SignalID,
					Severity: o.Severity, Priority: string(o.Severity), Status: statusOf(o.Status),
					Category: category, FirstSeen: rfc(o.FirstSeen), LastSeen: rfc(o.LastSeen),
				},
			}
			if o.ResolvedAt != nil {
				rec.summary.ResolvedAt = rfc(*o.ResolvedAt)
			}
			if coverage, err := s.store.GetAssessmentCoverage(ctx, o.AssetID); err == nil {
				for _, c := range coverage {
					if c.CheckID == o.CheckID {
						rec.checkState, rec.checkReason = c.State, c.Reason
						if rec.summary.Status != string(store.FindingResolved) && !c.State.Assessed() {
							rec.summary.NotReassessed = true
							rec.summary.NotReassessedReason = notReassessedReason(c)
						}
						break
					}
				}
			}
			r = rec
			break
		}

	case store.SourceFinding:
		findings, err := s.store.GetFindings(ctx, "")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("findings: reading findings: %w", err)
		}
		for i := range findings {
			f := &findings[i]
			if f.ID != rowID {
				continue
			}
			prio := f.Priority
			if prio == "" {
				prio = string(f.Severity)
			}
			rec := &findingRecord{
				finding: f,
				summary: FindingSummary{
					Key: key, Source: SourceFinding,
					AssetID: f.AssetID, Title: f.Title, Severity: f.Severity, Priority: prio,
					Status: statusOf(f.Status), Category: f.Category, CVE: f.CVE, KEV: f.KEVListed,
					FirstSeen: rfc(f.FirstSeen), LastSeen: rfc(f.LastSeen),
				},
			}
			if f.ResolvedAt != nil {
				rec.summary.ResolvedAt = rfc(*f.ResolvedAt)
			}
			r = rec
			break
		}

	case store.SourceSecret:
		secrets, err := s.store.GetSecretFindings(ctx, "")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("findings: reading secrets: %w", err)
		}
		for i := range secrets {
			sf := &secrets[i]
			if sf.ID != rowID {
				continue
			}
			sev := store.SecretSeverity(sf.Verified)
			rec := &findingRecord{
				secret: sf,
				summary: FindingSummary{
					Key: key, Source: SourceSecret,
					AssetID: sf.AssetID, SecretID: sf.ID, Title: fmt.Sprintf("Exposed %s in %s", sf.SecretType, sf.FilePath),
					Severity: sev, Priority: string(sev), Status: statusOf(sf.Status), Category: "secret",
					FirstSeen: rfc(sf.FirstSeen), LastSeen: rfc(sf.LastSeen),
				},
			}
			if sf.ResolvedAt != nil {
				rec.summary.ResolvedAt = rfc(*sf.ResolvedAt)
			}
			r = rec
			break
		}

	case store.SourceCandidate:
		candidates, err := s.store.GetCandidateFindings(ctx, "")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("findings: reading candidates: %w", err)
		}
		for i := range candidates {
			cf := &candidates[i]
			if cf.ID != rowID {
				continue
			}
			prio := cf.Priority
			if prio == "" {
				prio = store.ComputeFindingPriority(cf.Severity, cf.EPSS, cf.KEVListed)
			}
			rec := &findingRecord{
				candidate: cf,
				summary: FindingSummary{
					Key: key, Source: SourceCandidate,
					AssetID: cf.AssetID, Title: cf.Title, Severity: cf.Severity, Priority: prio,
					Status: statusOf(cf.Status), NotReassessed: cf.NotReassessed, NotReassessedReason: cf.NotReassessedReason,
					Category: "software-candidate", CVE: cf.CVE, KEV: cf.KEVListed,
					Unconfirmed: true, SeveritySource: cf.SeveritySource,
					FirstSeen: rfc(cf.FirstSeen), LastSeen: rfc(cf.LastSeen),
				},
			}
			if cf.ResolvedAt != nil {
				rec.summary.ResolvedAt = rfc(*cf.ResolvedAt)
			}
			r = rec
			break
		}
	}

	if r == nil {
		return nil, nil, nil, ErrFindingNotFound
	}

	if r.summary.AssetID != "" {
		if asset, err := s.store.GetAssetByID(ctx, r.summary.AssetID); err == nil && asset != nil {
			r.summary.AssetValue = asset.Value
		}
	}

	events, err := s.store.GetFindingEvents(ctx, store.EventQuery{Key: key})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("findings: reading history: %w", err)
	}

	now := s.clock()
	end := now
	if r.summary.ResolvedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.summary.ResolvedAt); err == nil {
			end = t
		}
	}
	if first, err := time.Parse(time.RFC3339, r.summary.FirstSeen); err == nil && end.After(first) {
		r.summary.AgeDays = int(end.Sub(first).Hours() / 24)
	}
	r.summary.LastChange, r.summary.LastChangeType = r.summary.FirstSeen, string(store.EventFirstSeen)
	if len(events) > 0 {
		last := events[len(events)-1]
		r.summary.LastChange, r.summary.LastChangeType = rfc(last.At), string(last.Type)
	}
	return r, events, registry, nil
}

// Detail assembles the drill-down for one finding.
func (s *FindingsService) Detail(ctx context.Context, key string) (FindingDetail, error) {
	source, rowID, err := store.ParseFindingKey(key)
	if err != nil {
		return FindingDetail{}, ErrFindingNotFound
	}
	r, events, registry, err := s.loadOne(ctx, key, source, rowID)
	if err != nil {
		return FindingDetail{}, err
	}

	d := FindingDetail{
		FindingSummary: r.summary,
		CheckID:        r.checkID,
		CheckState:     r.checkState,
		CheckReason:    r.checkReason,
		Timeline:       []FindingEventView{},
		Regressions:    []store.Regression{},
	}
	for _, e := range events {
		d.Timeline = append(d.Timeline, eventView(e))
	}
	if n := len(d.Timeline); n > 0 {
		d.Provenance.RunID = d.Timeline[n-1].RunID
		d.Provenance.LibraryVersion = d.Timeline[n-1].LibraryVersion
		d.Provenance.RegistryVersion = d.Timeline[n-1].RegistryVersion
	}

	in := narrative.Input{
		Source: r.summary.Source, Title: r.summary.Title, Severity: r.summary.Severity,
		CheckState: r.checkState, CheckReason: r.checkReason,
	}

	switch {
	case r.signal != nil:
		o := r.signal
		d.Detail, d.Evidence = o.Detail, o.Evidence
		d.Provenance.LibraryVersion, d.Provenance.RegistryVersion = o.LibraryVersion, o.RegistryVersion
		in.SignalID, in.Check, in.Detail, in.Evidence = o.SignalID, o.CheckID, o.Detail, o.Evidence
		if e, ok := registry[o.SignalID]; ok {
			in.WeaknessClass, in.Scenario = e.WeaknessClass, e.Scenario
		}
		if c, ok := catalogueEntry(o.SignalID); ok {
			d.Description = c.Description
			in.Catalogue = &narrative.Catalogue{
				Title: c.Title, Description: c.Description, Remediation: c.Remediation,
				References: c.References, Tags: c.Tags,
			}
			if in.Check == "" {
				in.Check = c.Check
			}
		}
		d.Regressions = s.regressionsFor(ctx, r.summary.AssetID, signalRegressionAttribute(o.CheckID), "")

	case r.finding != nil:
		f := r.finding
		d.Description, d.Evidence = f.Description, f.Proof
		in.Detail, in.Evidence = f.Description, f.Proof
		in.CVE = f.CVE
		if strings.HasPrefix(f.ID, "service:") {
			in.ServiceClass, in.Check = s.serviceClassOf(ctx, f)
		} else {
			in.Check = f.Category
		}
		s.applyEnrichment(&d, &in, f)
		if f.CVE != "" {
			d.Regressions = s.regressionsFor(ctx, f.AssetID, "", f.CVE)
		}

	case r.secret != nil:
		sf := r.secret
		in.SecretType = sf.SecretType
		d.Evidence = fmt.Sprintf("%s line %d (rule %s, reference %s)", sf.FilePath, sf.StartLine, sf.RuleID, sf.RedactedRef)
		in.Evidence = d.Evidence
		d.Description = fmt.Sprintf("A %s credential was found in %s.", sf.SecretType, sf.RepoURL)
		if sf.Verified {
			in.Detail = "The credential was verified as live."
		}

	case r.candidate != nil:
		cf := r.candidate
		d.Description = cf.Description
		d.ObservedString = cf.RawString
		d.MatchedVendor = cf.Vendor
		d.MatchedProduct = cf.Product
		d.MatchedVersion = cf.Version
		d.LookupSnapshotID = cf.LookupSnapshotID
		d.LookupAt = rfc(cf.LookupAt)
		d.Evidence = fmt.Sprintf("Observed %q mapped to %s:%s version %s (NVD snapshot %s)", cf.RawString, cf.Vendor, cf.Product, cf.Version, cf.LookupSnapshotID)
		d.Detail = fmt.Sprintf("Unconfirmed candidate: observed %q matched %s %s against NVD (%s severity: %s).", cf.RawString, cf.Product, cf.Version, cf.SeveritySource, cf.Severity)
		in.CVE = cf.CVE
		in.Detail = d.Detail
		in.Evidence = d.Evidence
		s.applyCandidateEnrichment(ctx, &d, &in, cf)
	}

	d.Explanation = s.explain(in)
	d.NarrativeVersion = d.Explanation.Version
	return d, nil
}

// ErrFindingNotFound is returned for a key that names no stored finding,
// including one that is not a key at all.
var ErrFindingNotFound = fmt.Errorf("findings: finding not found")

func (s *FindingsService) explain(in narrative.Input) narrative.Explanation {
	if s.explainer != nil {
		return s.explainer.Explain(in)
	}
	e := narrative.Explanation{Level: narrative.LevelGeneric, LevelKey: "generic", Version: "none"}
	if in.Catalogue != nil {
		e.WhatWeFound = in.Catalogue.Description
		e.WhatToDo = []string{in.Catalogue.Remediation}
		e.References, e.Standards = in.Catalogue.References, in.Catalogue.Tags
	}
	return e
}

// signalRegressionAttribute names the posture attribute whose changes concern a
// check, where one exists.
func signalRegressionAttribute(check string) string {
	switch check {
	case "dmarc":
		return store.DMARCPolicyAttribute
	case "net", "hosting":
		return store.AttributionAttribute
	}
	return ""
}

func (s *FindingsService) regressionsFor(ctx context.Context, assetID, attribute, cve string) []store.Regression {
	out := []store.Regression{}
	if attribute == "" && cve == "" {
		return out
	}
	regs, err := s.store.GetRegressions(ctx)
	if err != nil {
		return out
	}
	for _, r := range regs {
		if r.AssetID != assetID {
			continue
		}
		if (attribute != "" && r.AttributeType == attribute) || (cve != "" && strings.Contains(r.AttributeType, cve)) {
			out = append(out, r)
		}
	}
	return out
}

// serviceClassOf recovers the service class of a service finding from the
// observation it was raised from. The finding identifier is
// service:<asset>:<host>:<port>:<layer>:<code>; the code is returned as the
// check so that an advisory such as missing-hsts is not described as a bare
// exposure.
func (s *FindingsService) serviceClassOf(ctx context.Context, f *store.Finding) (string, string) {
	parts := strings.Split(strings.TrimPrefix(f.ID, "service:"), ":")
	if len(parts) < 5 {
		return "", ""
	}
	code, layer := parts[len(parts)-1], parts[len(parts)-2]
	port, err := strconv.Atoi(parts[len(parts)-3])
	if err != nil {
		return "", code
	}
	check := code
	if code == "exposed" {
		check = ""
	}
	obs, err := s.store.GetServiceObservations(ctx, f.AssetID)
	if err != nil {
		return "", check
	}
	for _, o := range obs {
		if int(o.Port) == port && o.Layer == layer {
			return string(contact.ClassifyService(o.Service, o.Port, o.Protocol)), check
		}
	}
	return string(contact.ClassifyService("", uint16(port), "")), check
}

// applyEnrichment states what the feeds said about a CVE finding, and which
// snapshots said it.
func (s *FindingsService) applyEnrichment(d *FindingDetail, in *narrative.Input, f *store.Finding) {
	seen := map[string]bool{}
	for _, e := range f.Enrichments {
		if e.Snapshot != nil && !seen[e.Snapshot.ID] {
			seen[e.Snapshot.ID] = true
			d.Provenance.FeedSnapshots = append(d.Provenance.FeedSnapshots, *e.Snapshot)
		}
		checked := ""
		if e.CheckedAt != nil {
			checked = rfc(*e.CheckedAt)
		}
		switch e.Feed {
		case store.FeedKEV:
			in.KEVState = string(e.State)
			in.KEVCheckedAt = checked
			in.KEVListed = e.KEVListed != nil && *e.KEVListed
		case store.FeedEPSS:
			in.EPSSState = string(e.State)
			in.EPSSCheckedAt = checked
			// 0.3 is the floor at which priority is raised to high; the text
			// says "elevated" and never states the figure.
			in.EPSSHigh = e.EPSS != nil && *e.EPSS >= 0.3
		}
	}
	if f.CVE != "" {
		if in.KEVState == "" {
			in.KEVState = string(store.EnrichmentNotChecked)
		}
		if in.EPSSState == "" {
			in.EPSSState = string(store.EnrichmentNotChecked)
		}
	}
}

func (s *FindingsService) applyCandidateEnrichment(ctx context.Context, d *FindingDetail, in *narrative.Input, cf *store.CandidateFinding) {
	seen := map[string]bool{}
	if cf.LookupSnapshotID != "" {
		if snap, err := s.store.GetFeedSnapshot(ctx, cf.LookupSnapshotID); err == nil && snap != nil {
			seen[snap.ID] = true
			d.Provenance.FeedSnapshots = append(d.Provenance.FeedSnapshots, *snap)
		}
	}
	for _, e := range cf.Enrichments {
		if e.Snapshot != nil && !seen[e.Snapshot.ID] {
			seen[e.Snapshot.ID] = true
			d.Provenance.FeedSnapshots = append(d.Provenance.FeedSnapshots, *e.Snapshot)
		}
		checked := ""
		if e.CheckedAt != nil {
			checked = rfc(*e.CheckedAt)
		}
		switch e.Feed {
		case store.FeedKEV:
			in.KEVState = string(e.State)
			in.KEVCheckedAt = checked
			in.KEVListed = e.KEVListed != nil && *e.KEVListed
		case store.FeedEPSS:
			in.EPSSState = string(e.State)
			in.EPSSCheckedAt = checked
			in.EPSSHigh = e.EPSS != nil && *e.EPSS >= 0.3
		}
	}
	if cf.CVE != "" {
		if in.KEVState == "" {
			in.KEVState = string(store.EnrichmentNotChecked)
		}
		if in.EPSSState == "" {
			in.EPSSState = string(store.EnrichmentNotChecked)
		}
	}
}

// --- Runs and history ---------------------------------------------------

func (s *FindingsService) runView(r store.ScanRun, assets map[string]string) ScanRunView {
	return ScanRunView{
		ID: r.ID, Kind: r.Kind, AssetID: r.AssetID, AssetValue: assets[r.AssetID],
		Profile: r.Profile, Outcome: r.Outcome, LibraryVersion: r.LibraryVersion,
		RegistryVersion: r.RegistryVersion, StartedAt: rfc(r.StartedAt), FinishedAt: rfc(r.FinishedAt),
		Coverage: r.Coverage, CoverageFraction: r.Coverage.Fraction(),
	}
}

func (s *FindingsService) assetValues(ctx context.Context) map[string]string {
	out := map[string]string{}
	if assets, err := s.store.GetAssets(ctx, store.AssetStatusActive); err == nil {
		for _, a := range assets {
			out[a.ID] = a.Value
		}
	}
	return out
}

// Scans lists the run ledger, newest first.
func (s *FindingsService) Scans(ctx context.Context) ([]ScanRunView, error) {
	runs, err := s.store.GetScanRuns(ctx, 0)
	if err != nil {
		return nil, err
	}
	assets := s.assetValues(ctx)
	out := make([]ScanRunView, 0, len(runs))
	for _, r := range runs {
		out = append(out, s.runView(r, assets))
	}
	return out, nil
}

func (s *FindingsService) point(run store.ScanRun, sum *store.ScanSummary, assets map[string]string) HistoryPoint {
	p := HistoryPoint{Run: s.runView(run, assets), At: rfc(run.FinishedAt)}
	if sum != nil {
		p.At = rfc(sum.At)
		p.Critical, p.High, p.Medium, p.Low, p.Info = sum.Critical, sum.High, sum.Medium, sum.Low, sum.Info
		p.Open, p.Resolved, p.New, p.ResolvedInRun = sum.Open, sum.Resolved, sum.New, sum.ResolvedInRun
	}
	return p
}

// History returns one point per recorded run summary from since, oldest first.
// A run whose summary was pruned is absent: the chart draws only what was
// recorded and never interpolates.
func (s *FindingsService) History(ctx context.Context, since time.Time) ([]HistoryPoint, error) {
	sums, err := s.store.GetScanSummaries(ctx, since)
	if err != nil {
		return nil, err
	}
	runs, err := s.store.GetScanRuns(ctx, 0)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]store.ScanRun, len(runs))
	for _, r := range runs {
		byID[r.ID] = r
	}
	assets := s.assetValues(ctx)
	out := make([]HistoryPoint, 0, len(sums))
	for i := range sums {
		run, ok := byID[sums[i].RunID]
		if !ok {
			continue
		}
		out = append(out, s.point(run, &sums[i], assets))
	}
	return out, nil
}

// Changes reports what moved between two runs. An omitted `to` is the latest
// run, and an omitted `from` is the nearest earlier run of the same kind on the
// same asset (else the run immediately before). With no earlier run the window
// opens at the beginning of history.
func (s *FindingsService) Changes(ctx context.Context, fromID, toID string) (ChangeSet, error) {
	cs := ChangeSet{New: []FindingChange{}, Resolved: []FindingChange{}, Worsened: []FindingChange{}, Improved: []FindingChange{}}

	runs, err := s.store.GetScanRuns(ctx, 0) // newest first
	if err != nil {
		return cs, err
	}
	if len(runs) == 0 {
		cs.Note = "No scans have been recorded yet."
		return cs, nil
	}
	index := func(id string) int {
		for i, r := range runs {
			if r.ID == id {
				return i
			}
		}
		return -1
	}

	toIdx := 0
	if toID != "" {
		if toIdx = index(toID); toIdx < 0 {
			return cs, fmt.Errorf("findings: no such run %q", toID)
		}
	}
	to := runs[toIdx]
	// With no `from`, compare against the nearest earlier run of the same kind
	// on the same asset, so "since the last scan" means the last scan of that
	// thing rather than whatever unrelated run happened to come just before it
	// (a feed refresh would otherwise be compared with an assessment). Failing
	// that, the run immediately before.
	fromIdx := toIdx + 1
	for i := toIdx + 1; i < len(runs) && fromID == ""; i++ {
		if runs[i].Kind == to.Kind && runs[i].AssetID == to.AssetID {
			fromIdx = i
			break
		}
	}
	if fromID != "" {
		if fromIdx = index(fromID); fromIdx < 0 {
			return cs, fmt.Errorf("findings: no such run %q", fromID)
		}
	}
	var from *store.ScanRun
	if fromIdx < len(runs) {
		from = &runs[fromIdx]
	}

	sums, err := s.store.GetScanSummaries(ctx, time.Time{})
	if err != nil {
		return cs, err
	}
	sumOf := map[string]*store.ScanSummary{}
	for i := range sums {
		sumOf[sums[i].RunID] = &sums[i]
	}
	assets := s.assetValues(ctx)
	toPoint := s.point(to, sumOf[to.ID], assets)
	cs.To = &toPoint
	q := store.EventQuery{Until: to.FinishedAt}
	if from != nil {
		fromPoint := s.point(*from, sumOf[from.ID], assets)
		cs.From = &fromPoint
		q.After = from.FinishedAt
	}

	cs.Note = comparisonNote(from, &to)

	events, err := s.store.GetFindingEvents(ctx, q)
	if err != nil {
		return cs, err
	}
	ld, err := s.load(ctx)
	if err != nil {
		return cs, err
	}
	for _, e := range events {
		if e.Type == store.EventBaseline {
			continue
		}
		r, ok := ld.byKey[e.Key]
		if !ok {
			continue
		}
		change := FindingChange{Finding: r.summary, Event: eventView(e)}
		switch e.Type {
		case store.EventFirstSeen, store.EventReopened:
			cs.New = append(cs.New, change)
		case store.EventResolved:
			cs.Resolved = append(cs.Resolved, change)
		case store.EventChanged, store.EventPriorityChanged:
			prev, cur := e.PrevSeverity.Rank(), e.Severity.Rank()
			if e.PrevPriority != "" && e.Priority != "" {
				prev, cur = store.FindingSeverity(e.PrevPriority).Rank(), store.FindingSeverity(e.Priority).Rank()
			}
			switch {
			case cur > prev:
				cs.Worsened = append(cs.Worsened, change)
			case cur < prev:
				cs.Improved = append(cs.Improved, change)
			}
		}
	}
	return cs, nil
}

// comparisonNote states, in words, what limits how a comparison should be read:
// a drop in coverage, or runs that did not assess the same thing.
func comparisonNote(from, to *store.ScanRun) string {
	if from == nil {
		return "This is the earliest recorded run, so everything it recorded is shown as new."
	}
	var notes []string
	if from.Kind != to.Kind || from.AssetID != to.AssetID {
		notes = append(notes, "These runs did not assess the same thing, so their coverage is not directly comparable.")
	} else if from.Coverage.Total > 0 && to.Coverage.Total > 0 && to.Coverage.Fraction() < from.Coverage.Fraction() {
		notes = append(notes, fmt.Sprintf(
			"Coverage was lower on the later run (%d of %d checks reached a conclusion, against %d of %d). Fewer findings may reflect checks that did not conclude rather than fixes.",
			to.Coverage.AssessedOnly, to.Coverage.Total, from.Coverage.AssessedOnly, from.Coverage.Total))
	}
	return strings.Join(notes, " ")
}
