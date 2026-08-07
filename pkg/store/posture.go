package store

// ControlPosture is the derived standing of one control on one asset.
//
// It exists because vantage's catalogue is a set of advisories: every entry
// names a way a control falls short. A control that is perfectly configured
// therefore produces no signal at all, and its health is legible only as the
// conjunction of two facts — the check ran, and it raised nothing.
//
// Deriving this rather than storing it is deliberate. An inverse catalogue,
// carrying a positive identifier for every advisory, would double the surface
// to maintain and would drift the moment an advisory was added upstream
// without its mirror image. Silence plus coverage is the same information and
// cannot fall out of step with itself.
type ControlPosture string

const (
	// PostureCompliant means the control was assessed and no advisory applies.
	// This is the only state that may be read as the control working.
	PostureCompliant ControlPosture = "compliant"
	// PostureDeficient means the control was assessed and at least one
	// advisory applies.
	PostureDeficient ControlPosture = "deficient"
	// PostureUnknown means no conclusion was reached — the check did not run,
	// or ran and could not tell. It is never evidence of either health or
	// deficiency, and must not be counted as a control in place.
	PostureUnknown ControlPosture = "unknown"
)

// Mitigating reports whether the posture is evidence that lowers the
// probability of the scenarios its control bears on.
//
// Only an assessed, advisory-free control qualifies. An unknown posture is
// silence, and treating silence as mitigation is precisely how an unassessed
// estate comes to look well defended.
func (p ControlPosture) Mitigating() bool { return p == PostureCompliant }

// ScenarioCoverage states how much of the assessment bearing on one attack
// scenario actually happened, alongside what it found.
//
// Every aggregate over a scenario must be accompanied by one of these. A
// scenario probability computed from two checks out of nine is not wrong so
// much as unquantified, and presenting it without saying so invites a reader
// to act on a number that has no support.
type ScenarioCoverage struct {
	Scenario string `json:"scenario"`

	// Coverage counts the checks bearing on this scenario by state.
	Coverage CoverageSummary `json:"coverage"`

	// Aggravating counts distinct dedup groups carrying at least one advisory,
	// rather than raw advisories. Nine DMARC advisories on one domain are nine
	// views of one weakness; counting them individually would let a single
	// misconfiguration dominate a scenario by sheer repetition.
	Aggravating int `json:"aggravating"`

	// Significant counts the subset of those groups whose highest assessed
	// severity is medium or above.
	//
	// It accompanies Aggravating rather than replacing it. Reporting only the
	// filtered number would quietly discard low findings from the reader's
	// view, and reporting only the raw number makes a shared mail provider
	// weigh the same as an absent transport policy. Both are facts, and the
	// pair is what lets a reader tell a scenario with one real weakness from
	// one with three minor notes.
	Significant int `json:"significant"`

	// Mitigating counts dedup groups assessed clean.
	Mitigating int `json:"mitigating"`

	// Postures is the per-dedup-group breakdown behind the counts.
	Postures map[string]ControlPosture `json:"postures"`

	// Severities is the highest assessed severity per deficient group, so the
	// counts above can be audited rather than taken on trust.
	Severities map[string]FindingSeverity `json:"severities,omitempty"`
}

// Supported reports whether any assessment at all bears on this scenario. An
// unsupported scenario must not be presented as low risk.
func (s ScenarioCoverage) Supported() bool { return s.Coverage.AssessedOnly > 0 }

// DeriveControlPostures computes the posture of each dedup group for one asset.
//
// The three inputs are all required and none is sufficient alone: the registry
// says which control a signal bears on, the coverage says whether the check
// behind it ran, and the observations say what it found. Omitting coverage is
// the classic error — it turns "never assessed" into "nothing found", which
// reads as health.
func DeriveControlPostures(
	registry []SignalRegistryEntry,
	coverage []AssessmentCoverage,
	observations []SignalObservation,
) map[string]ControlPosture {
	// Which dedup group each signal belongs to, and which checks feed it.
	groupOfSignal := make(map[string]string, len(registry))
	checksOfGroup := map[string]map[string]bool{}
	for _, e := range registry {
		if e.DedupGroup == "" {
			continue
		}
		groupOfSignal[e.SignalID] = e.DedupGroup
	}

	// A dedup group is fed by whichever checks produced its signals. The
	// registry's group naming is check-scoped, so the check prefix is the
	// authority here rather than an inference from observations, which would
	// make a group vanish on the very assessment where nothing was found.
	for group := range setOfGroups(registry) {
		checksOfGroup[group] = map[string]bool{}
	}
	for _, e := range registry {
		if e.DedupGroup == "" {
			continue
		}
		if check := checkOfGroup(e.DedupGroup); check != "" {
			checksOfGroup[e.DedupGroup][check] = true
		}
	}

	stateOfCheck := make(map[string]CoverageState, len(coverage))
	for _, c := range coverage {
		stateOfCheck[c.CheckID] = c.State
	}

	// A group carries an advisory if any observation mapped into it was
	// actually assessed. An observation recorded at a non-assessed state is
	// not evidence of deficiency.
	deficient := map[string]bool{}
	for _, o := range observations {
		group, ok := groupOfSignal[o.SignalID]
		if !ok || !o.State.Assessed() {
			continue
		}
		deficient[group] = true
	}

	out := make(map[string]ControlPosture, len(checksOfGroup))
	for group, checks := range checksOfGroup {
		switch {
		case deficient[group]:
			out[group] = PostureDeficient
		case allAssessed(checks, stateOfCheck):
			// Every check feeding this group reached a conclusion and none
			// raised an advisory. This is the p=reject case: compliance is
			// visible only as assessed silence.
			out[group] = PostureCompliant
		default:
			out[group] = PostureUnknown
		}
	}
	return out
}

// DeriveGroupSeverities returns the highest severity actually assessed in each
// dedup group.
//
// Posture deliberately does not carry severity: a control with any advisory
// against it is deficient, and that is the honest statement. But an aggregate
// built from postures alone cannot tell a missing MTA-STS policy from a note
// that both exchangers share an operator, because both make exactly one group
// deficient. This supplies the missing dimension without weakening the posture
// model — the two are reported side by side rather than one folded into the
// other.
//
// A group with no assessed observation is absent from the result rather than
// present at zero, so that a caller cannot mistake "nothing found" for
// "nothing severe found".
func DeriveGroupSeverities(
	registry []SignalRegistryEntry,
	observations []SignalObservation,
) map[string]FindingSeverity {
	groupOfSignal := make(map[string]string, len(registry))
	for _, e := range registry {
		if e.DedupGroup != "" {
			groupOfSignal[e.SignalID] = e.DedupGroup
		}
	}

	out := map[string]FindingSeverity{}
	for _, o := range observations {
		group, ok := groupOfSignal[o.SignalID]
		// The same assessed-state test the posture derivation applies. An
		// observation recorded at a non-assessed state is not evidence of
		// anything, and must not raise a group's severity.
		if !ok || !o.State.Assessed() {
			continue
		}
		if o.Severity.Rank() > out[group].Rank() {
			out[group] = o.Severity
		}
	}
	return out
}

// allAssessed reports whether every check feeding a group reached a
// conclusion. One unassessed check is enough to make the group unknown: a
// control is not demonstrated compliant by the checks that happened to run.
func allAssessed(checks map[string]bool, states map[string]CoverageState) bool {
	if len(checks) == 0 {
		return false
	}
	for check := range checks {
		state, ok := states[check]
		if !ok || !state.Assessed() {
			return false
		}
	}
	return true
}

// setOfGroups collects the distinct dedup groups in a registry.
func setOfGroups(registry []SignalRegistryEntry) map[string]struct{} {
	out := map[string]struct{}{}
	for _, e := range registry {
		if e.DedupGroup != "" {
			out[e.DedupGroup] = struct{}{}
		}
	}
	return out
}

// checkOfGroup extracts the check name from a dedup group of the form
// "<check>:<weaknessClass>".
func checkOfGroup(group string) string {
	for i := 0; i < len(group); i++ {
		if group[i] == ':' {
			return group[:i]
		}
	}
	return group
}

// ComputeScenarioCoverage aggregates postures and coverage by scenario.
func ComputeScenarioCoverage(
	registry []SignalRegistryEntry,
	coverage []AssessmentCoverage,
	observations []SignalObservation,
) map[string]ScenarioCoverage {
	postures := DeriveControlPostures(registry, coverage, observations)
	severities := DeriveGroupSeverities(registry, observations)

	// Which scenarios each dedup group and check bear on.
	scenariosOfGroup := map[string]map[string]bool{}
	checksOfScenario := map[string]map[string]bool{}
	for _, e := range registry {
		if e.Scenario == "" || e.DedupGroup == "" {
			continue
		}
		if scenariosOfGroup[e.DedupGroup] == nil {
			scenariosOfGroup[e.DedupGroup] = map[string]bool{}
		}
		scenariosOfGroup[e.DedupGroup][e.Scenario] = true

		if checksOfScenario[e.Scenario] == nil {
			checksOfScenario[e.Scenario] = map[string]bool{}
		}
		if check := checkOfGroup(e.DedupGroup); check != "" {
			checksOfScenario[e.Scenario][check] = true
		}
	}

	stateOfCheck := make(map[string]CoverageState, len(coverage))
	for _, c := range coverage {
		stateOfCheck[c.CheckID] = c.State
	}

	out := map[string]ScenarioCoverage{}
	for scenario, checks := range checksOfScenario {
		sc := ScenarioCoverage{
			Scenario:   scenario,
			Postures:   map[string]ControlPosture{},
			Severities: map[string]FindingSeverity{},
		}

		for check := range checks {
			sc.Coverage.Total++
			switch stateOfCheck[check] {
			case CoverageOK:
				sc.Coverage.OK++
				sc.Coverage.AssessedOnly++
			case CoverageNotFound:
				sc.Coverage.NotFound++
				sc.Coverage.AssessedOnly++
			case CoverageCheckFailed:
				sc.Coverage.CheckFailed++
			default:
				// Absent from the coverage table means the check was never
				// attempted, which is not_checked rather than a fifth state.
				sc.Coverage.NotChecked++
			}
		}

		for group, scenarios := range scenariosOfGroup {
			if !scenarios[scenario] {
				continue
			}
			p := postures[group]
			sc.Postures[group] = p
			switch p {
			case PostureDeficient:
				sc.Aggravating++
				if s, ok := severities[group]; ok {
					sc.Severities[group] = s
					if s.Significant() {
						sc.Significant++
					}
				}
			case PostureCompliant:
				sc.Mitigating++
			}
		}
		out[scenario] = sc
	}
	return out
}
