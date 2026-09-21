package store

import (
	"encoding/json"
	"strconv"
	"time"
)

// EmailControl is the assessed state of one email-authentication control on
// one domain.
//
// It replaces a boolean, and the reason is the difference between a gap and an
// outage. `SPFValid bool` cannot say whether a domain publishes no SPF record
// or whether the resolver never answered, and those demand opposite responses:
// the first is work for the mail team, the second is work for us. Collapsed
// into one value, the second reads as the first, and a CISO is told their
// estate is misconfigured when in truth it is unassessed — or, worse, the
// collapse runs the other way and an outage reads as a clean bill of health.
type EmailControl struct {
	// State is the four-state outcome, using the same vocabulary as the rest
	// of the assessment path rather than a private one.
	State CoverageState `json:"state"`
	// Detail is the salient value where the control has one — the DMARC
	// policy, the MTA-STS enforcement mode. Empty where it has none.
	Detail string `json:"detail,omitempty"`
	// Reason explains a state that is not an assessment: which endpoint was
	// refused, which check was excluded by policy, what the resolver did.
	//
	// It is carried because "we could not tell" is only actionable when it
	// says why. Without it a reader has a gap they cannot close.
	Reason string `json:"reason,omitempty"`
	// Conclusive reports whether the state may be read as a fact about the
	// domain rather than about the limits of the search.
	//
	// It exists for DKIM. Selectors cannot be enumerated from DNS, so probing
	// the common list and finding nothing establishes nothing — the domain may
	// sign every message with a selector nobody can guess. A reader told "DKIM
	// absent" would commission work that is very likely already done.
	Conclusive bool `json:"conclusive"`
}

// EmailPosture is a domain's email-authentication posture.
//
// The parsed DMARC tags are carried alongside the control states because
// severity is a deterministic function of them, and the function must be able
// to run without re-parsing a record.
type EmailPosture struct {
	Domain string `json:"domain"`

	// The three authentication controls.
	SPF   EmailControl `json:"spf"`
	DKIM  EmailControl `json:"dkim"`
	DMARC EmailControl `json:"dmarc"`

	// The adjacent records. Their absence is worth surfacing and is never as
	// consequential as a missing DMARC policy.
	MTASTS EmailControl `json:"mtaSts"`
	TLSRPT EmailControl `json:"tlsRpt"`
	BIMI   EmailControl `json:"bimi"`
	CAA    EmailControl `json:"caa"`

	// DMARC tags, as data. Severity is derived from these.
	DMARCPolicy          string `json:"dmarcPolicy"`
	DMARCSubdomainPolicy string `json:"dmarcSubdomainPolicy,omitempty"`
	DMARCPercent         int    `json:"dmarcPercent"`
	DMARCAlignmentSPF    string `json:"dmarcAlignmentSpf,omitempty"`
	DMARCAlignmentDKIM   string `json:"dmarcAlignmentDkim,omitempty"`
	DMARCReporting       bool   `json:"dmarcReporting"`

	// SPFAllMechanism is the qualifier on the terminating all-mechanism.
	// "+" authorises the entire internet to send as the domain.
	SPFAllMechanism string `json:"spfAllMechanism,omitempty"`
	// SPFLookups is the number of DNS-querying mechanisms the record expands
	// to. Above ten, receivers may stop evaluating it altogether, so a domain
	// can publish a careful policy that is enforced nowhere.
	SPFLookups int `json:"spfLookups"`

	// DKIMSelectorsExamined and DKIMSelectorsFound make the search legible: a
	// reader seeing one found out of thirteen tried knows something quite
	// different from one told only that a key exists.
	DKIMSelectorsExamined []string `json:"dkimSelectorsExamined,omitempty"`
	DKIMSelectorsFound    []string `json:"dkimSelectorsFound,omitempty"`

	// Priority is the deterministic severity of the domain's worst
	// authentication shortfall. Never set by the AI-triage layer.
	Priority FindingSeverity `json:"priority"`

	LastChecked time.Time `json:"lastChecked"`
}

// DMARCSeverity is the deterministic priority of a domain's DMARC posture.
//
// It is a pure function of the policy tags and the coverage state — no
// network, no clock, no narrative layer. The engine-wide rule is that severity
// is computed and AI writes prose; this is that rule for this capability.
//
// The ordering encodes what an attacker can do, not how untidy the
// configuration looks:
//
//   - No record: anyone may spoof the domain and no receiver has instructions.
//   - Monitoring only: spoofing is observed and delivered.
//   - Partial enforcement: spoofing is blocked some of the time, which is a
//     probabilistic control rather than a closed route.
//   - Weak subdomain policy: the apex is protected and every subdomain is not,
//     which is the route an attacker will take precisely because the apex
//     looks defended.
func (p EmailPosture) DMARCSeverity() FindingSeverity {
	// An unassessed control has no severity. Returning a rating would state a
	// conclusion the evidence does not support, and — since severities are
	// maximised into aggregates — would let a resolver outage manufacture a
	// finding. Absence of evidence is reported as absence of evidence.
	if !p.DMARC.State.Assessed() {
		return ""
	}

	if p.DMARC.State == CoverageNotFound || p.DMARCPolicy == "" {
		return SeverityHigh
	}

	switch p.DMARCPolicy {
	case "none":
		// Reporting without enforcement. Spoofed mail is observed and
		// delivered; the domain owner may not even see the reports if no rua
		// destination is published.
		if !p.DMARCReporting {
			return SeverityHigh
		}
		return SeverityMedium
	case "quarantine", "reject":
		if p.DMARCPercent < 100 {
			// Partial enforcement is not enforcement. A reader told "reject"
			// concludes the route is closed; at pct=40 it is open three times
			// in five.
			return SeverityMedium
		}
		if p.DMARCSubdomainPolicy == "none" {
			// The apex is enforced and subdomains are not. An attacker reads
			// the same DNS we do and will send as a subdomain.
			return SeverityMedium
		}
		if p.DMARCPolicy == "quarantine" {
			// Quarantine delivers to junk rather than refusing. Better than
			// nothing, and not equivalent to rejection.
			return SeverityLow
		}
		return ""
	default:
		// A policy tag we do not recognise is not a policy receivers will
		// honour either, so the domain is unprotected while appearing
		// configured — which is worse than an obvious absence, because nobody
		// is looking.
		return SeverityHigh
	}
}

// SPFSeverity is the deterministic priority of a domain's SPF posture.
func (p EmailPosture) SPFSeverity() FindingSeverity {
	if !p.SPF.State.Assessed() {
		return ""
	}

	// "+all" is worse than publishing nothing. Publishing nothing leaves a
	// receiver's check inconclusive; "+all" makes it pass, so the domain
	// actively vouches for every host on the internet.
	if p.SPFAllMechanism == "+" {
		return SeverityHigh
	}
	if p.SPF.State == CoverageNotFound {
		return SeverityHigh
	}
	if p.SPFLookups > 10 {
		// Receivers are entitled to return permerror and stop, so the policy
		// is not applied at all.
		return SeverityMedium
	}
	if p.SPFAllMechanism == "?" {
		return SeverityMedium
	}
	return ""
}

// WorstSeverity is the domain's deterministic priority: the highest severity
// across the authentication controls.
//
// The adjacent records are deliberately excluded. The capability's
// requirements place them below any open SPF/DKIM/DMARC finding, and folding
// them in would let a missing BIMI logo raise a domain's priority above a
// domain that cannot be spoofed at all.
func (p EmailPosture) WorstSeverity() FindingSeverity {
	worst := FindingSeverity("")
	for _, s := range []FindingSeverity{p.DMARCSeverity(), p.SPFSeverity(), p.DKIMSeverity()} {
		if s.Rank() > worst.Rank() {
			worst = s
		}
	}
	return worst
}

// DKIMSeverity is the deterministic priority of a domain's DKIM posture.
//
// An inconclusive search has no severity, however many selectors were tried.
// The capability's requirements forbid recording "domain has no DKIM" when
// selectors were guessed, and assigning a severity would be that claim in
// another form — a rating is a statement that something is wrong.
func (p EmailPosture) DKIMSeverity() FindingSeverity {
	if !p.DKIM.State.Assessed() || !p.DKIM.Conclusive {
		return ""
	}
	if p.DKIM.State == CoverageNotFound {
		return SeverityMedium
	}
	return ""
}

// Assessed reports how many of the seven controls reached a conclusion, and
// how many were examined at all.
//
// Every aggregate over this posture must be able to state its own coverage.
// "Three deficiencies" means one thing out of seven assessed controls and
// quite another out of two.
func (p EmailPosture) Assessed() (assessed, total int) {
	for _, c := range p.controls() {
		total++
		if c.State.Assessed() {
			assessed++
		}
	}
	return assessed, total
}

func (p EmailPosture) controls() []EmailControl {
	return []EmailControl{p.SPF, p.DKIM, p.DMARC, p.MTASTS, p.TLSRPT, p.BIMI, p.CAA}
}

// PredatesAssessment reports a posture recorded before the four-state
// assessment existed, which therefore carries no control states at all.
//
// Change 006 Phase 9 widened this record from a row of booleans to seven
// four-state controls, and deliberately did not promote the old booleans:
// reconstructing four states from two would have preserved exactly the
// collapse the widening removed. The consequence is that every installation
// upgraded across that boundary holds postures that read as wholly unassessed
// until their domain is next scanned.
//
// That is true, and it is the right answer, but an interface showing seven
// unassessed controls is indistinguishable from one showing a domain nobody
// has got to yet — and both are indistinguishable, to a hurried reader, from a
// clean result. This predicate is what lets a view say which it is: the row
// was checked, at a time we can name, by a build whose answers we cannot
// carry forward.
//
// The signature is a recorded check that left every control without any state
// at all — not even check_failed. A posture written by the current engine
// always records a state for each control, because recording that a check
// could not be completed is itself one of the four. An unset state can
// therefore only mean the row was written before there were states to set.
//
// Note that "no control was assessed" is the wrong test, and was the first one
// written here: a domain whose every check failed — a resolver outage, say —
// assesses nothing, and telling that operator their data predates an upgrade
// would send them to rescan a domain that had just been scanned.
func (p EmailPosture) PredatesAssessment() bool {
	if p.LastChecked.IsZero() {
		return false
	}
	for _, c := range p.controls() {
		if c.State != "" {
			return false
		}
	}
	return true
}

// MarshalJSON serialises the posture together with its coverage.
//
// Assessed is a method, so it does not serialise, and a view showing "3 of 7
// assessed" would otherwise have to define "assessed" a second time in
// TypeScript. Two definitions of a coverage figure eventually disagree, and
// then one half of the product reports coverage the other half denies. The
// figure is therefore computed here, once, and travels with the record over
// both transports.
//
// It is added at the serialisation boundary rather than as a struct field
// because a stored count can go stale against the states it counts. Derived
// on the way out, it cannot.
func (p EmailPosture) MarshalJSON() ([]byte, error) {
	// The alias sheds this method, so marshalling the alias does not recurse.
	type posture EmailPosture
	assessed, total := p.Assessed()
	return json.Marshal(struct {
		posture
		AssessedControls   int  `json:"assessedControls"`
		TotalControls      int  `json:"totalControls"`
		PredatesAssessment bool `json:"predatesAssessment,omitempty"`
	}{
		posture:            posture(p),
		AssessedControls:   assessed,
		TotalControls:      total,
		PredatesAssessment: p.PredatesAssessment(),
	})
}

// DMARCPolicyAttribute is the posture attribute under which DMARC policy
// changes are tracked.
//
// The capability's requirements call for policy drift to be caught rather than
// recorded once: a domain whose policy weakens from p=reject to p=none has
// lost a control, and the finding history must show the transition rather than
// silently overwriting the prior state.
const DMARCPolicyAttribute = "dmarc_policy"

// DMARCFingerprint is the comparable form of a domain's DMARC policy.
//
// It carries the tags that decide what receivers do, and nothing else. A
// fingerprint over the whole record would change when a reporting address was
// edited, and a drift list that fires on cosmetic edits is one an operator
// stops reading — at which point the genuine weakening is missed too.
//
// An empty string means the policy was not assessed. The caller must not
// record it, since an unassessed run would otherwise overwrite the baseline
// and manufacture a transition on the next successful one.
func (p EmailPosture) DMARCFingerprint() string {
	if !p.DMARC.State.Assessed() {
		return ""
	}
	if p.DMARC.State == CoverageNotFound {
		return "absent"
	}

	fingerprint := "p=" + p.DMARCPolicy + ";pct=" + strconv.Itoa(p.DMARCPercent)
	if p.DMARCSubdomainPolicy != "" {
		fingerprint += ";sp=" + p.DMARCSubdomainPolicy
	}
	return fingerprint
}
