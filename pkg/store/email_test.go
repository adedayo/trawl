package store

import "testing"

// Deterministic severity is the requirement this file exists to hold. Priority
// is a function of the DMARC tags alone — no clock, no network, no narrative
// layer — so the same posture always prices the same, and two runs that
// disagree mean the domain changed rather than that we did.
func TestDMARCSeverityIsAFunctionOfTheTags(t *testing.T) {
	assessed := EmailControl{State: CoverageOK}

	cases := []struct {
		name    string
		posture EmailPosture
		want    FindingSeverity
		because string
	}{
		{
			name:    "no record at all",
			posture: EmailPosture{DMARC: EmailControl{State: CoverageNotFound}},
			want:    SeverityHigh,
			because: "no receiver has instructions, so anyone may spoof the domain and nothing is even observed",
		},
		{
			name: "monitoring with reporting",
			posture: EmailPosture{
				DMARC: assessed, DMARCPolicy: "none", DMARCPercent: 100, DMARCReporting: true,
			},
			want:    SeverityMedium,
			because: "spoofed mail is observed and delivered",
		},
		{
			name: "monitoring without reporting",
			posture: EmailPosture{
				DMARC: assessed, DMARCPolicy: "none", DMARCPercent: 100,
			},
			want:    SeverityHigh,
			because: "p=none with no rua destination observes nothing either; it is absence wearing the shape of a policy",
		},
		{
			name: "partial enforcement",
			posture: EmailPosture{
				DMARC: assessed, DMARCPolicy: "reject", DMARCPercent: 40, DMARCReporting: true,
			},
			want:    SeverityMedium,
			because: "a reject policy at pct=40 leaves the route open three times in five",
		},
		{
			name: "enforced apex, unprotected subdomains",
			posture: EmailPosture{
				DMARC: assessed, DMARCPolicy: "reject", DMARCPercent: 100,
				DMARCSubdomainPolicy: "none", DMARCReporting: true,
			},
			want:    SeverityMedium,
			because: "an attacker reads the same DNS we do and will send as a subdomain",
		},
		{
			name: "quarantine at full percentage",
			posture: EmailPosture{
				DMARC: assessed, DMARCPolicy: "quarantine", DMARCPercent: 100, DMARCReporting: true,
			},
			want:    SeverityLow,
			because: "delivery to junk is not refusal, and should not price as though it were",
		},
		{
			name: "fully enforced",
			posture: EmailPosture{
				DMARC: assessed, DMARCPolicy: "reject", DMARCPercent: 100, DMARCReporting: true,
			},
			want:    "",
			because: "an enforced policy is not a finding; rating it would send an operator to fix what is already right",
		},
		{
			name:    "unrecognised policy value",
			posture: EmailPosture{DMARC: assessed, DMARCPolicy: "block", DMARCPercent: 100},
			want:    SeverityHigh,
			because: "receivers will not honour it either, so the domain is unprotected while appearing configured",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.posture.DMARCSeverity(); got != tc.want {
				t.Fatalf("severity = %q, want %q: %s", got, tc.want, tc.because)
			}
		})
	}
}

// An unassessed control has no severity. Rating one would manufacture a
// finding out of a resolver outage — and because severities are maximised into
// aggregates, a single failed lookup would raise a whole domain's priority.
func TestAnUnassessedControlHasNoSeverity(t *testing.T) {
	for _, state := range []CoverageState{CoverageNotChecked, CoverageCheckFailed} {
		p := EmailPosture{DMARC: EmailControl{State: state}, DMARCPolicy: ""}
		if got := p.DMARCSeverity(); got != "" {
			t.Fatalf("state %q produced severity %q; absence of evidence must not be priced as evidence of absence", state, got)
		}
	}
}

// "+all" is worse than publishing nothing: publishing nothing leaves a
// receiver's check inconclusive, whereas "+all" makes it pass, so the domain
// actively vouches for every host on the internet.
func TestPermissiveSPFRatesAboveSilence(t *testing.T) {
	permissive := EmailPosture{
		SPF: EmailControl{State: CoverageOK}, SPFAllMechanism: "+",
	}
	if got := permissive.SPFSeverity(); got != SeverityHigh {
		t.Fatalf("+all severity = %q, want %q", got, SeverityHigh)
	}

	strict := EmailPosture{SPF: EmailControl{State: CoverageOK}, SPFAllMechanism: "-"}
	if got := strict.SPFSeverity(); got != "" {
		t.Fatalf("a strict policy is not a finding; got %q", got)
	}
}

// A record receivers will refuse to evaluate is not an enforced record.
func TestSPFLookupLimitIsAFinding(t *testing.T) {
	p := EmailPosture{SPF: EmailControl{State: CoverageOK}, SPFAllMechanism: "-", SPFLookups: 14}
	if got := p.SPFSeverity(); got != SeverityMedium {
		t.Fatalf("severity = %q, want %q: above ten lookups receivers may return permerror and apply no policy at all", got, SeverityMedium)
	}
}

// An inconclusive DKIM search has no severity, however many selectors were
// tried. A rating is a claim that something is wrong, and the capability's
// requirements forbid that claim when the selectors were guessed.
func TestInconclusiveDKIMIsNotRated(t *testing.T) {
	probed := EmailPosture{DKIM: EmailControl{State: CoverageNotFound, Conclusive: false}}
	if got := probed.DKIMSeverity(); got != "" {
		t.Fatalf("severity = %q: probing common selectors and finding none establishes nothing about the domain", got)
	}

	named := EmailPosture{DKIM: EmailControl{State: CoverageNotFound, Conclusive: true}}
	if got := named.DKIMSeverity(); got != SeverityMedium {
		t.Fatalf("severity = %q, want %q: selectors the operator named make the absence an answer", got, SeverityMedium)
	}
}

// The adjacent records are tiered below the authentication controls, as the
// capability's requirements state. Folding them into the domain's priority
// would let a missing BIMI logo outrank a domain that can be freely spoofed.
func TestAdjacentRecordsDoNotRaiseThePriority(t *testing.T) {
	p := EmailPosture{
		SPF:   EmailControl{State: CoverageOK},
		DKIM:  EmailControl{State: CoverageOK, Conclusive: true},
		DMARC: EmailControl{State: CoverageOK},

		DMARCPolicy: "reject", DMARCPercent: 100, DMARCReporting: true,
		SPFAllMechanism: "-",

		BIMI:   EmailControl{State: CoverageNotFound},
		MTASTS: EmailControl{State: CoverageNotFound},
		TLSRPT: EmailControl{State: CoverageNotFound},
		CAA:    EmailControl{State: CoverageNotFound},
	}

	if got := p.WorstSeverity(); got != "" {
		t.Fatalf("priority = %q: a domain that cannot be spoofed must not be rated for a missing logo", got)
	}
}

// Every aggregate must be able to state its own coverage. "No deficiencies"
// out of seven assessed controls means something; out of two it means almost
// nothing, and the two are indistinguishable without this.
func TestCoverageIsCountable(t *testing.T) {
	p := EmailPosture{
		SPF:    EmailControl{State: CoverageOK},
		DKIM:   EmailControl{State: CoverageCheckFailed},
		DMARC:  EmailControl{State: CoverageNotFound},
		MTASTS: EmailControl{State: CoverageNotChecked},
		TLSRPT: EmailControl{State: CoverageNotChecked},
		BIMI:   EmailControl{State: CoverageNotChecked},
		CAA:    EmailControl{State: CoverageNotChecked},
	}

	assessed, total := p.Assessed()
	if assessed != 2 || total != 7 {
		t.Fatalf("assessed %d of %d, want 2 of 7: a failed check is not an assessment", assessed, total)
	}
}
