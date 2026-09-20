package vantage

import (
	"strconv"
	"strings"
	"time"

	vfinding "github.com/adedayo/vantage/pkg/finding"
	vobs "github.com/adedayo/vantage/pkg/observation"

	"github.com/adedayo/trawl/pkg/store"
)

// emailChecks are the vantage checks bearing on the email-authentication
// capability, mapped to the control each one assesses.
//
// The capability's requirements name seven controls. Enumerating them here
// rather than inferring from whatever observations happen to arrive is what
// lets an unrequested check be recorded as not_checked: a control missing from
// the result is missing for a reason, and the reason has to be stated rather
// than left as a gap in a struct.
var emailChecks = map[string]string{
	"spf":    "spf",
	"dkim":   "dkim",
	"dmarc":  "dmarc",
	"mtasts": "mtasts",
	"tlsrpt": "tlsrpt",
	"bimi":   "bimi",
	"caa":    "caa",
}

// emailPosture assembles a domain's email-authentication posture from the
// structured observations of the checks that ran.
//
// Every control starts as not_checked and is promoted only by a check that
// actually reported. That direction is deliberate: the alternative — start
// from a zero value meaning "fine" and record problems as they arrive — turns
// every excluded, skipped or crashed check into a passing control, and does it
// silently. An assessment that was never performed must never be the same
// value as one that found nothing wrong.
func emailPosture(domain string, checks []vfinding.CheckResult, states map[string]store.CoverageState, reasons map[string]string, at time.Time) *store.EmailPosture {
	posture := &store.EmailPosture{
		Domain:      normalise(domain),
		LastChecked: at,
		// The RFC default, so that a comparison against a domain publishing no
		// pct= tag is made on the same basis receivers use.
		DMARCPercent: 100,
	}

	unchecked := store.EmailControl{State: store.CoverageNotChecked}
	posture.SPF, posture.DKIM, posture.DMARC = unchecked, unchecked, unchecked
	posture.MTASTS, posture.TLSRPT, posture.BIMI, posture.CAA = unchecked, unchecked, unchecked, unchecked

	seen := false
	for _, c := range checks {
		control, ours := emailChecks[c.Check]
		if !ours {
			continue
		}
		seen = true

		state := states[c.Check]
		if state == "" {
			state = coverageState(c.State)
		}
		reason := reasons[c.Check]

		var email *vobs.Email
		if c.Observation != nil {
			email = c.Observation.Email
		}

		switch control {
		case "spf":
			posture.SPF = spfControl(state, reason, email, posture)
		case "dkim":
			posture.DKIM = dkimControl(state, reason, email, posture)
		case "dmarc":
			posture.DMARC = dmarcControl(state, reason, email, posture)
		case "mtasts":
			posture.MTASTS = adjacentControl(state, reason, email)
		case "tlsrpt":
			posture.TLSRPT = adjacentControl(state, reason, email)
		case "bimi":
			posture.BIMI = adjacentControl(state, reason, email)
		case "caa":
			posture.CAA = adjacentControl(state, reason, email)
		}
	}

	if !seen {
		// No email check ran at all. Returning a posture of seven not_checked
		// controls would be truthful but would also write a row implying an
		// assessment took place; returning nothing leaves the previous
		// posture standing, which is the older but still better-evidenced
		// account.
		return nil
	}

	posture.Priority = posture.WorstSeverity()
	return posture
}

// spfControl translates the SPF observation, recording the facts severity is
// computed from on the posture itself.
func spfControl(state store.CoverageState, reason string, email *vobs.Email, into *store.EmailPosture) store.EmailControl {
	control := store.EmailControl{State: state, Reason: reason, Conclusive: state.Assessed()}
	if email == nil || email.SPF == nil {
		return control
	}

	spf := email.SPF
	into.SPFAllMechanism = spf.AllMechanism
	into.SPFLookups = spf.Lookups

	control.State = presenceState(state, spf.Presence)
	if spf.Record != "" {
		control.Detail = spf.Record
	}
	control.Conclusive = control.State.Assessed()
	return control
}

// dkimControl translates the DKIM observation.
//
// Conclusive carries the distinction the capability's requirements turn on:
// probing common selectors and finding nothing establishes nothing, because a
// domain may sign with a selector nobody can guess. The judgement is the
// library's — it knows whether it probed — so it is read, not re-derived.
func dkimControl(state store.CoverageState, reason string, email *vobs.Email, into *store.EmailPosture) store.EmailControl {
	control := store.EmailControl{State: state, Reason: reason, Conclusive: state.Assessed()}
	if email == nil || email.DKIM == nil {
		return control
	}

	dkim := email.DKIM
	into.DKIMSelectorsExamined = dkim.SelectorsExamined
	into.DKIMSelectorsFound = dkim.SelectorsFound

	control.Conclusive = state.Assessed() && dkim.Conclusive()
	if !dkim.Conclusive() && state.Assessed() {
		// The check ran and could not settle the question. That is
		// check_failed in Trawl's vocabulary — not not_found, which would
		// assert the control is absent, and not ok, which would assert it is
		// present. The reason names the limit so a reader can lift it by
		// supplying their own selectors.
		control.State = store.CoverageCheckFailed
		control.Reason = joinReasons(reason, strings.TrimSpace(
			"none of the "+plural(len(dkim.SelectorsExamined), "common selector")+
				" probed resolved; DKIM selectors cannot be enumerated from DNS, so this "+
				"establishes nothing about the domain. Configure the selectors it signs with "+
				"to settle it"))
	}

	if len(dkim.SelectorsFound) > 0 {
		control.Detail = strings.Join(dkim.SelectorsFound, ", ")
	}
	return control
}

// dmarcControl translates the DMARC observation, carrying every tag severity
// is a function of onto the posture.
func dmarcControl(state store.CoverageState, reason string, email *vobs.Email, into *store.EmailPosture) store.EmailControl {
	control := store.EmailControl{State: state, Reason: reason, Conclusive: state.Assessed()}
	if email == nil || email.DMARC == nil {
		return control
	}

	dmarc := email.DMARC
	into.DMARCPolicy = dmarc.Policy
	into.DMARCSubdomainPolicy = dmarc.SubdomainPolicy
	into.DMARCPercent = dmarc.Percent
	into.DMARCAlignmentSPF = dmarc.AlignmentSPF
	into.DMARCAlignmentDKIM = dmarc.AlignmentDKIM
	into.DMARCReporting = dmarc.AggregateReporting

	control.State = presenceState(state, dmarc.Presence)
	control.Detail = dmarc.Policy
	control.Conclusive = control.State.Assessed()
	return control
}

func adjacentControl(state store.CoverageState, reason string, email *vobs.Email) store.EmailControl {
	control := store.EmailControl{State: state, Reason: reason, Conclusive: state.Assessed()}
	if email == nil || email.Adjacent == nil {
		return control
	}
	control.State = presenceState(state, email.Adjacent.Presence)
	control.Detail = email.Adjacent.Detail
	control.Conclusive = control.State.Assessed()
	return control
}

// presenceState reconciles the check's state with the observation's presence.
//
// The check state wins whenever it says the assessment did not happen. A check
// that failed may still have attached a partial observation, and reading
// "absent" from it would convert our own outage into a finding about the
// domain — the failure that this entire four-state path exists to prevent.
func presenceState(state store.CoverageState, presence vobs.Presence) store.CoverageState {
	if !state.Assessed() {
		return state
	}
	switch presence {
	case vobs.PresencePublished:
		return store.CoverageOK
	case vobs.PresenceAbsent:
		return store.CoverageNotFound
	default:
		return store.CoverageCheckFailed
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
