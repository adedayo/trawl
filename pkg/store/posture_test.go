package store

import "testing"

// dmarcRegistry mirrors the shipped registry closely enough to reason about
// the case that prompted this: DMARC advisories only, no positive identifier.
func dmarcRegistry() []SignalRegistryEntry {
	ids := []string{"SURF-DMARC-001", "SURF-DMARC-002", "SURF-DMARC-005"}
	var out []SignalRegistryEntry
	for _, id := range ids {
		out = append(out, SignalRegistryEntry{
			SignalID: id, Scenario: "email-spoofing", Stage: "delivery",
			WeaknessClass: "email-authentication", DedupGroup: "dmarc:email-authentication",
			Control: "dmarc", Direction: "aggravating",
		})
	}
	out = append(out, SignalRegistryEntry{
		SignalID: "SURF-SPF-001", Scenario: "email-spoofing", Stage: "delivery",
		WeaknessClass: "email-authentication", DedupGroup: "spf:email-authentication",
		Control: "spf", Direction: "aggravating",
	})
	return out
}

// TestCompliantControlIsAssessedSilence is the case that motivated this code.
//
// A domain publishing p=reject raises no DMARC advisory, because the catalogue
// only describes shortfalls. Its compliance is therefore legible only as the
// conjunction of "the dmarc check ran" and "it raised nothing".
func TestCompliantControlIsAssessedSilence(t *testing.T) {
	postures := DeriveControlPostures(
		dmarcRegistry(),
		[]AssessmentCoverage{
			{CheckID: "dmarc", State: CoverageOK},
			{CheckID: "spf", State: CoverageOK},
		},
		nil, // p=reject and a good SPF record: no advisories at all
	)

	if got := postures["dmarc:email-authentication"]; got != PostureCompliant {
		t.Fatalf("p=reject posture = %q, want %q", got, PostureCompliant)
	}
	if !postures["dmarc:email-authentication"].Mitigating() {
		t.Error("an assessed, advisory-free control must count as mitigating")
	}
}

// TestSilenceWithoutCoverageIsNotCompliance is the error this design exists to
// prevent. Absence of advisories means nothing unless the check ran.
func TestSilenceWithoutCoverageIsNotCompliance(t *testing.T) {
	cases := []struct {
		name     string
		coverage []AssessmentCoverage
	}{
		{"check never ran", []AssessmentCoverage{{CheckID: "dmarc", State: CoverageNotChecked}}},
		{"check could not tell", []AssessmentCoverage{{CheckID: "dmarc", State: CoverageCheckFailed}}},
		{"check absent from the coverage table entirely", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			postures := DeriveControlPostures(dmarcRegistry(), c.coverage, nil)
			got := postures["dmarc:email-authentication"]
			if got != PostureUnknown {
				t.Fatalf("posture = %q, want %q", got, PostureUnknown)
			}
			if got.Mitigating() {
				t.Error("an unassessed control must never count as mitigating")
			}
		})
	}
}

// TestDeficientControlIsDetected covers the ordinary aggravating path.
func TestDeficientControlIsDetected(t *testing.T) {
	postures := DeriveControlPostures(
		dmarcRegistry(),
		[]AssessmentCoverage{{CheckID: "dmarc", State: CoverageOK}, {CheckID: "spf", State: CoverageOK}},
		[]SignalObservation{
			{SignalID: "SURF-DMARC-002", State: CoverageOK}, // p=none
		},
	)
	if got := postures["dmarc:email-authentication"]; got != PostureDeficient {
		t.Fatalf("p=none posture = %q, want %q", got, PostureDeficient)
	}
	if postures["dmarc:email-authentication"].Mitigating() {
		t.Error("a deficient control must not count as mitigating")
	}
	// The unrelated control is unaffected.
	if got := postures["spf:email-authentication"]; got != PostureCompliant {
		t.Errorf("spf posture = %q, want %q", got, PostureCompliant)
	}
}

// TestAdvisoriesAreDedupedWithinAControl guards against repetition inflating
// risk. Nine DMARC advisories on one domain are nine views of one weakness.
func TestAdvisoriesAreDedupedWithinAControl(t *testing.T) {
	sc := ComputeScenarioCoverage(
		dmarcRegistry(),
		[]AssessmentCoverage{{CheckID: "dmarc", State: CoverageOK}, {CheckID: "spf", State: CoverageOK}},
		[]SignalObservation{
			{SignalID: "SURF-DMARC-001", State: CoverageOK},
			{SignalID: "SURF-DMARC-002", State: CoverageOK},
			{SignalID: "SURF-DMARC-005", State: CoverageOK},
		},
	)
	got := sc["email-spoofing"]
	if got.Aggravating != 1 {
		t.Fatalf("aggravating groups = %d, want 1 (three advisories, one weakness)", got.Aggravating)
	}
	if got.Mitigating != 1 {
		t.Fatalf("mitigating groups = %d, want 1 (spf assessed clean)", got.Mitigating)
	}
}

// TestScenarioCoverageIsReportedAlongsideFindings ensures an aggregate can
// always disclose how much of the intended assessment happened.
func TestScenarioCoverageIsReportedAlongsideFindings(t *testing.T) {
	sc := ComputeScenarioCoverage(
		dmarcRegistry(),
		[]AssessmentCoverage{
			{CheckID: "dmarc", State: CoverageOK},
			{CheckID: "spf", State: CoverageCheckFailed},
		},
		nil,
	)
	got := sc["email-spoofing"]
	if got.Coverage.Total != 2 {
		t.Fatalf("total checks = %d, want 2", got.Coverage.Total)
	}
	if got.Coverage.AssessedOnly != 1 {
		t.Fatalf("assessed = %d, want 1", got.Coverage.AssessedOnly)
	}
	if got.Coverage.Fraction() != 0.5 {
		t.Fatalf("coverage fraction = %v, want 0.5", got.Coverage.Fraction())
	}
	// The failed check must not make its control look compliant.
	if got.Postures["spf:email-authentication"] != PostureUnknown {
		t.Errorf("a failed check left its control looking %q",
			got.Postures["spf:email-authentication"])
	}
	if got.Mitigating != 1 {
		t.Errorf("mitigating = %d, want 1 (dmarc only)", got.Mitigating)
	}
}

// TestUnsupportedScenarioIsNotLowRisk guards the presentation rule: a scenario
// nothing bears on must be reported as unsupported, not as safe.
func TestUnsupportedScenarioIsNotLowRisk(t *testing.T) {
	sc := ComputeScenarioCoverage(
		dmarcRegistry(),
		[]AssessmentCoverage{
			{CheckID: "dmarc", State: CoverageNotChecked},
			{CheckID: "spf", State: CoverageNotChecked},
		},
		nil,
	)
	got := sc["email-spoofing"]
	if got.Supported() {
		t.Fatal("a scenario with no assessed check must report as unsupported")
	}
	if got.Mitigating != 0 {
		t.Errorf("mitigating = %d, want 0: nothing was assessed", got.Mitigating)
	}
	if got.Coverage.Fraction() != 0 {
		t.Errorf("coverage fraction = %v, want 0", got.Coverage.Fraction())
	}
}

// TestObservationAtNonAssessedStateIsNotDeficiency covers the contradictory
// input the adapter deliberately retains: a finding recorded against a check
// that did not conclude is an anomaly, not evidence of a weak control.
func TestObservationAtNonAssessedStateIsNotDeficiency(t *testing.T) {
	postures := DeriveControlPostures(
		dmarcRegistry(),
		[]AssessmentCoverage{{CheckID: "dmarc", State: CoverageCheckFailed}},
		[]SignalObservation{{SignalID: "SURF-DMARC-002", State: CoverageCheckFailed}},
	)
	if got := postures["dmarc:email-authentication"]; got != PostureUnknown {
		t.Fatalf("posture = %q, want %q", got, PostureUnknown)
	}
}

// mailRegistry has two groups on one scenario: one that raises weighty
// advisories, one that raises only a resilience note.
func mailRegistry() []SignalRegistryEntry {
	return []SignalRegistryEntry{
		{
			SignalID: "SURF-MTASTS-001", Scenario: "email-interception", Stage: "delivery",
			WeaknessClass: "mail-transport", DedupGroup: "mtasts:mail-transport",
			Control: "mta-sts", Direction: "aggravating",
		},
		{
			SignalID: "SURF-MX-005", Scenario: "email-interception", Stage: "delivery",
			WeaknessClass: "mail-delivery", DedupGroup: "mx:mail-delivery",
			Control: "mx", Direction: "aggravating",
		},
	}
}

// The case that prompted this. A LOW note that both exchangers share an
// operator made its group deficient, and a deficient group counted exactly as
// much towards the scenario as a HIGH transport failure. The two were
// arithmetically indistinguishable, so the aggregate said "2" whether the
// estate had two real problems or one problem and one preference.
func TestScenarioSeparatesWeightyAdvisoriesFromMinorOnes(t *testing.T) {
	sc := ComputeScenarioCoverage(
		mailRegistry(),
		[]AssessmentCoverage{
			{CheckID: "mtasts", State: CoverageOK},
			{CheckID: "mx", State: CoverageOK},
		},
		[]SignalObservation{
			{SignalID: "SURF-MTASTS-001", State: CoverageOK, Severity: SeverityHigh},
			{SignalID: "SURF-MX-005", State: CoverageOK, Severity: SeverityLow},
		},
	)["email-interception"]

	// Both still count as aggravating: an advisory does apply to each.
	if sc.Aggravating != 2 {
		t.Errorf("Expected both groups aggravating, got %d", sc.Aggravating)
	}
	// But only one of them warrants attention on its own.
	if sc.Significant != 1 {
		t.Errorf("Expected 1 significant group, got %d", sc.Significant)
	}
	if sc.Severities["mx:mail-delivery"] != SeverityLow {
		t.Errorf("Expected the mx group to record its low severity, got %q",
			sc.Severities["mx:mail-delivery"])
	}
}

// A group takes the highest severity assessed against it, not the first or the
// last observed. Anything else would let ordering decide whether a critical
// finding was visible in the aggregate.
func TestGroupSeverityTakesTheHighestNotTheLatest(t *testing.T) {
	severities := DeriveGroupSeverities(
		dmarcRegistry(),
		[]SignalObservation{
			{SignalID: "SURF-DMARC-001", State: CoverageOK, Severity: SeverityHigh},
			{SignalID: "SURF-DMARC-002", State: CoverageOK, Severity: SeverityLow},
		},
	)

	if got := severities["dmarc:email-authentication"]; got != SeverityHigh {
		t.Errorf("Expected the highest severity to survive, got %q", got)
	}
}

// The same rule the posture derivation applies: an observation that did not
// conclude is not evidence of anything, and must not raise a group's severity.
func TestGroupSeverityIgnoresUnassessedObservations(t *testing.T) {
	severities := DeriveGroupSeverities(
		dmarcRegistry(),
		[]SignalObservation{
			{SignalID: "SURF-DMARC-001", State: CoverageNotChecked, Severity: SeverityCritical},
		},
	)

	if _, ok := severities["dmarc:email-authentication"]; ok {
		t.Error("A check that never ran must not contribute a severity")
	}
}

// Severity must never become a filter. A scenario carrying only low advisories
// still has a weakness recorded against it, and reporting zero would be a
// clean bill of health the assessment does not support.
func TestLowOnlyScenarioIsStillReportedAsAggravated(t *testing.T) {
	sc := ComputeScenarioCoverage(
		mailRegistry(),
		[]AssessmentCoverage{{CheckID: "mx", State: CoverageOK}},
		[]SignalObservation{{SignalID: "SURF-MX-005", State: CoverageOK, Severity: SeverityLow}},
	)["email-interception"]

	if sc.Aggravating != 1 {
		t.Errorf("A low advisory must still aggravate, got %d", sc.Aggravating)
	}
	if sc.Significant != 0 {
		t.Errorf("A low advisory must not be significant, got %d", sc.Significant)
	}
}

func TestSeverityRankOrdersAndTreatsUnknownAsLowest(t *testing.T) {
	if SeverityCritical.Rank() <= SeverityHigh.Rank() ||
		SeverityHigh.Rank() <= SeverityMedium.Rank() ||
		SeverityMedium.Rank() <= SeverityLow.Rank() ||
		SeverityLow.Rank() <= SeverityInfo.Rank() {
		t.Error("Severity ranks must be strictly ordered")
	}
	// An absent rating must not outrank one that was actually assigned.
	if FindingSeverity("").Rank() >= SeverityInfo.Rank() {
		t.Error("An unset severity must rank below info")
	}
	if SeverityLow.Significant() || !SeverityMedium.Significant() {
		t.Error("The significance threshold must sit at medium")
	}
}
