package vantage

import (
	"testing"
	"time"

	vfinding "github.com/adedayo/vantage/pkg/finding"
	vobs "github.com/adedayo/vantage/pkg/observation"

	"github.com/adedayo/trawl/pkg/store"
)

func emailCheck(name string, state vfinding.State, email *vobs.Email) vfinding.CheckResult {
	c := vfinding.CheckResult{Check: name, Target: "example.com", State: state}
	if email != nil {
		c.Observation = &vfinding.Observation{Email: email}
	}
	return c
}

func at() time.Time { return time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC) }

// TestAControlNobodyCheckedIsNotAControlThatPassed is the rule the whole
// posture is built to protect.
//
// The assembly starts every control at not_checked and promotes only those a
// check reported. The opposite direction — start from a zero value meaning
// "fine", record problems as they arrive — turns every excluded, skipped or
// crashed check into a passing control, silently, and an operator reads an
// assessment that never happened as a clean bill of health.
func TestAControlNobodyCheckedIsNotAControlThatPassed(t *testing.T) {
	// Only DMARC ran. Six controls were never assessed.
	checks := []vfinding.CheckResult{
		emailCheck("dmarc", vfinding.StateOK, &vobs.Email{
			DMARC: &vobs.DMARC{Presence: vobs.PresencePublished, Policy: "reject", Percent: 100},
		}),
	}

	p := emailPosture("example.com", checks, nil, nil, at())
	if p == nil {
		t.Fatal("a check that ran must produce a posture")
	}

	for name, control := range map[string]store.EmailControl{
		"spf": p.SPF, "dkim": p.DKIM, "mtasts": p.MTASTS,
		"tlsrpt": p.TLSRPT, "bimi": p.BIMI, "caa": p.CAA,
	} {
		if control.State != store.CoverageNotChecked {
			t.Fatalf("%s = %q, want %q: a control nobody assessed must never read as one that passed",
				name, control.State, store.CoverageNotChecked)
		}
		if control.State.Passing() {
			t.Fatalf("%s reads as passing without having been checked", name)
		}
	}
}

// TestNoEmailCheckMeansNoPosture. Writing seven not_checked controls would
// imply an assessment took place and would overwrite a better-evidenced
// posture from an earlier run with a record of having looked at nothing.
func TestNoEmailCheckMeansNoPosture(t *testing.T) {
	checks := []vfinding.CheckResult{emailCheck("dnssec", vfinding.StateOK, nil)}

	if p := emailPosture("example.com", checks, nil, nil, at()); p != nil {
		t.Fatal("a run with no email check must leave the previous posture standing rather than replace it with an empty one")
	}
}

// TestOurOutageIsNotTheirFinding.
//
// A check that failed may still attach a partial observation. Reading
// "absent" from it would convert our own resolver failure into a finding about
// the domain — an operator dispatched to fix configuration that was never
// examined, and a real absence indistinguishable from an imagined one.
func TestOurOutageIsNotTheirFinding(t *testing.T) {
	checks := []vfinding.CheckResult{
		emailCheck("dmarc", vfinding.StateOK, &vobs.Email{
			DMARC: &vobs.DMARC{Presence: vobs.PresenceAbsent},
		}),
	}
	states := map[string]store.CoverageState{"dmarc": store.CoverageCheckFailed}
	reasons := map[string]string{"dmarc": "resolver timed out"}

	p := emailPosture("example.com", checks, states, reasons, at())

	if p.DMARC.State != store.CoverageCheckFailed {
		t.Fatalf("state = %q, want %q: a failed lookup must not be recorded as a missing record",
			p.DMARC.State, store.CoverageCheckFailed)
	}
	if p.DMARC.Reason != "resolver timed out" {
		t.Fatalf("reason = %q: a gap is only actionable when it says why", p.DMARC.Reason)
	}
	if p.Priority != "" {
		t.Fatalf("priority = %q: an outage must not manufacture a finding", p.Priority)
	}
}

// TestProbedDKIMBecomesCheckFailedNotAbsent.
//
// vantage reports the search as inconclusive; Trawl's vocabulary for "we
// looked and could not tell" is check_failed. Recording not_found would assert
// the domain has no DKIM, which the capability's requirements forbid outright
// and which would send a CISO to commission work already done.
func TestProbedDKIMBecomesCheckFailedNotAbsent(t *testing.T) {
	checks := []vfinding.CheckResult{
		emailCheck("dkim", vfinding.StateNotChecked, &vobs.Email{
			DKIM: &vobs.DKIM{
				SelectorsExamined: []string{"default", "google", "selector1"},
				Probed:            true,
			},
		}),
	}
	states := map[string]store.CoverageState{"dkim": store.CoverageNotFound}

	p := emailPosture("example.com", checks, states, nil, at())

	if p.DKIM.State != store.CoverageCheckFailed {
		t.Fatalf("state = %q, want %q: guessing selectors and finding none establishes nothing",
			p.DKIM.State, store.CoverageCheckFailed)
	}
	if p.DKIM.Conclusive {
		t.Fatal("an inconclusive search must not be marked conclusive")
	}
	if len(p.DKIMSelectorsExamined) != 3 {
		t.Fatalf("the selectors tried must be recorded so a reader can judge the search; got %v", p.DKIMSelectorsExamined)
	}
	if p.Priority != "" {
		t.Fatalf("priority = %q: an inconclusive search is not a deficiency", p.Priority)
	}
}

// TestNamedSelectorsProduceAConclusiveAbsence is the other half: an operator
// who supplies their selectors gets an answer rather than a hedge.
func TestNamedSelectorsProduceAConclusiveAbsence(t *testing.T) {
	checks := []vfinding.CheckResult{
		emailCheck("dkim", vfinding.StateNotFound, &vobs.Email{
			DKIM: &vobs.DKIM{SelectorsExamined: []string{"acme2026"}},
		}),
	}
	states := map[string]store.CoverageState{"dkim": store.CoverageNotFound}

	p := emailPosture("example.com", checks, states, nil, at())

	if p.DKIM.State != store.CoverageNotFound || !p.DKIM.Conclusive {
		t.Fatalf("state = %q conclusive = %v: selectors the operator named make an absence real",
			p.DKIM.State, p.DKIM.Conclusive)
	}
	if p.Priority != store.SeverityMedium {
		t.Fatalf("priority = %q, want %q", p.Priority, store.SeverityMedium)
	}
}

// TestTheDMARCTagsReachThePosture. Severity is a function of these, so each
// must arrive as data rather than be re-parsed from a record.
func TestTheDMARCTagsReachThePosture(t *testing.T) {
	checks := []vfinding.CheckResult{
		emailCheck("dmarc", vfinding.StateOK, &vobs.Email{
			DMARC: &vobs.DMARC{
				Presence: vobs.PresencePublished, Policy: "reject", SubdomainPolicy: "none",
				Percent: 40, AlignmentSPF: "s", AlignmentDKIM: "r", AggregateReporting: true,
			},
		}),
	}

	p := emailPosture("example.com", checks, nil, nil, at())

	if p.DMARCPolicy != "reject" || p.DMARCPercent != 40 || p.DMARCSubdomainPolicy != "none" {
		t.Fatalf("tags did not survive: p=%q pct=%d sp=%q", p.DMARCPolicy, p.DMARCPercent, p.DMARCSubdomainPolicy)
	}
	if p.Priority != store.SeverityMedium {
		t.Fatalf("priority = %q, want %q: partial enforcement is not enforcement", p.Priority, store.SeverityMedium)
	}
}

// TestTheRFCDefaultAppliesWhenNoPercentageIsPublished. Receivers apply pct=100
// when the tag is absent, so comparing on a zero would rate a fully enforced
// domain as partially enforced.
func TestTheRFCDefaultAppliesWhenNoPercentageIsPublished(t *testing.T) {
	p := emailPosture("example.com", []vfinding.CheckResult{
		emailCheck("dmarc", vfinding.StateOK, &vobs.Email{
			DMARC: &vobs.DMARC{
				Presence: vobs.PresencePublished, Policy: "reject",
				Percent: 100, AggregateReporting: true,
			},
		}),
	}, nil, nil, at())

	if p.Priority != "" {
		t.Fatalf("priority = %q: a fully enforced policy is not a finding", p.Priority)
	}
}

// TestTheAdjacentRecordDetailSurvives. A published MTA-STS policy in testing
// mode enforces nothing; recording only its presence would credit the domain
// with a control it is not operating.
func TestTheAdjacentRecordDetailSurvives(t *testing.T) {
	p := emailPosture("example.com", []vfinding.CheckResult{
		emailCheck("mtasts", vfinding.StateOK, &vobs.Email{
			Adjacent: &vobs.AdjacentRecord{
				Kind: "mtasts", Presence: vobs.PresencePublished, Detail: "testing",
			},
		}),
	}, nil, nil, at())

	if p.MTASTS.State != store.CoverageOK || p.MTASTS.Detail != "testing" {
		t.Fatalf("mtasts = %q/%q: a policy in testing mode is published and enforcing nothing",
			p.MTASTS.State, p.MTASTS.Detail)
	}
}
