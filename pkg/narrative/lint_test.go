package narrative

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// The style and boundary rules the narrative text must keep (Change 025 spec,
// extending the Change 012 boundary):
//
//   - no currency amount and no probability of compromise
//   - no em dashes; US English
//   - nothing that reads as an assurance ("secure", "safe", "fine", "protected")
//     which could attach to a control that was not assessed
//   - no organisation-specific names, addresses or links; references belong in
//     the catalogue's reference list
//   - short sentences
//
// Rules that bound what the engine may assert (currency, percentages,
// probabilities, "1 in N", em dashes) are applied to every rendered output.
// Wording rules (assurance words, spelling, names, sentence length) apply to
// text this package authors, not to the library catalogue text it quotes.

type lintRule struct {
	name string
	re   *regexp.Regexp
}

var (
	boundaryRules = []lintRule{
		{"currency", regexp.MustCompile(`(?i)[$£€¥]|\b(dollars?|usd|gbp|eur|euros?|pounds? sterling|yen|cents?)\b`)},
		{"percentage", regexp.MustCompile(`(?i)%|\bper ?cent(age)?s?\b`)},
		{"probability", regexp.MustCompile(`(?i)\bprobabilit(y|ies)\b|\bodds\b`)},
		{"one-in-N", regexp.MustCompile(`(?i)\b(1|one) in (\d|a (hundred|thousand|million)|ten|hundred|thousand|million)|\b\d+ in \d+\b`)},
		{"em dash", regexp.MustCompile(`—`)},
	}
	wordingRules = []lintRule{
		{"assurance wording", regexp.MustCompile(`(?i)\b(secure|secured|safe|safely|fine|protected)\b`)},
		{"non-US spelling", regexp.MustCompile(`(?i)\b\w*(organis|authoris|recognis|minimis|prioritis|summaris|behaviour|colour|favour|centre|licence|defence|analyse|cancelled|modelling|labelled|neighbour|programme)\w*\b`)},
		{"email address", regexp.MustCompile(`\S+@\S+`)},
		{"link", regexp.MustCompile(`(?i)https?://|www\.`)},
		{"company name", regexp.MustCompile(`\b(Inc|Ltd|LLC|Corp|GmbH|plc|Limited)\b\.?`)},
	}
	// Product expansions that contain a banned word as part of a proper name.
	allowedPhrases = regexp.MustCompile(`Secure Shell|Hypertext Transfer Protocol Secure`)

	sentenceSplit = regexp.MustCompile(`[.!?]+\s+`)
)

const maxSentenceWords = 45

func lintBoundary(text string) []string {
	var out []string
	for _, rule := range boundaryRules {
		if m := rule.re.FindString(text); m != "" {
			out = append(out, rule.name+": "+m)
		}
	}
	return out
}

func lintWording(text string) []string {
	scrubbed := allowedPhrases.ReplaceAllString(text, "")
	var out []string
	for _, rule := range wordingRules {
		if m := rule.re.FindString(scrubbed); m != "" {
			out = append(out, rule.name+": "+m)
		}
	}
	for _, sentence := range sentenceSplit.Split(text, -1) {
		if n := len(strings.Fields(sentence)); n > maxSentenceWords {
			out = append(out, "sentence too long ("+strconv.Itoa(n)+" words): "+truncate(sentence, 60))
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// authoredSections returns the sections of an explanation whose wording this
// package writes. WhatWeFound carries the catalogue's own text and the last
// step may be the catalogue's remediation, so they are excluded here.
func authoredSections(x Explanation, hasCatalogue bool) map[string]string {
	steps := x.WhatToDo
	if hasCatalogue && len(steps) > 0 {
		steps = steps[:len(steps)-1]
	}
	return map[string]string{
		"whyItMatters":       x.WhyItMatters,
		"howItCouldBeAbused": x.HowItCouldBeAbused,
		"whatGoodLooksLike":  x.WhatGoodLooksLike,
		"whatToDo":           strings.Join(steps, "\n"),
		"howRescanConfirms":  x.HowRescanConfirms,
		"exploitationStatus": x.ExploitationStatus,
		"coverageNote":       x.CoverageNote,
	}
}

// ---- the rules themselves are tested, so they cannot silently stop working ----

func TestLintRulesCatchViolations(t *testing.T) {
	bad := []struct{ rule, text string }{
		{"currency", "It costs $5."}, {"currency", "worth 100 dollars"}, {"currency", "about 3 USD"}, {"currency", "£10"}, {"currency", "€10"},
		{"percentage", "a 50% rise"}, {"percentage", "ten percent of hosts"}, {"percentage", "a high percentage"},
		{"probability", "the probability of attack"}, {"probability", "long odds"},
		{"one-in-N", "a 1 in 20 chance"}, {"one-in-N", "one in a million"}, {"one-in-N", "3 in 10 sites"},
		{"em dash", "this — that"},
		{"assurance wording", "the control is secure"}, {"assurance wording", "that is fine"}, {"assurance wording", "a safe setup"}, {"assurance wording", "Protected by default"}, {"assurance wording", "it is Safely stored"},
		{"non-US spelling", "your organisation"}, {"non-US spelling", "this behaviour"}, {"non-US spelling", "it authorises"},
		{"email address", "write to someone@example.org"}, {"link", "see https://example.org"}, {"link", "see www.example.org"},
		{"company name", "Acme Inc."}, {"company name", "Example Ltd"},
		{"sentence too long", strings.Repeat("word ", maxSentenceWords+1) + "end."},
	}
	for _, tc := range bad {
		var found []string
		if tc.rule == "currency" || tc.rule == "percentage" || tc.rule == "probability" || tc.rule == "one-in-N" || tc.rule == "em dash" {
			found = lintBoundary(tc.text)
		} else {
			found = lintWording(tc.text)
		}
		hit := false
		for _, f := range found {
			if strings.HasPrefix(f, tc.rule) {
				hit = true
			}
		}
		if !hit {
			t.Errorf("rule %q did not flag %q (flagged: %v)", tc.rule, tc.text, found)
		}
	}
	good := []string{
		"Secure Shell (SSH) lets administrators log in.",
		"Hypertext Transfer Protocol Secure (HTTPS) is the encrypted form.",
		"The service answers from the internet.",
		"Good looks like two nameservers on separate networks.",
		"Ask your DNS administrator to remove the record.",
	}
	for _, text := range good {
		if f := append(lintBoundary(text), lintWording(text)...); len(f) > 0 {
			t.Errorf("rules wrongly flagged %q: %v", text, f)
		}
	}
}

// ---- the content ------------------------------------------------------------

func TestContentLint(t *testing.T) {
	r := loadEmbedded(t)
	for _, level := range []string{LevelSignal, LevelWeaknessClass, LevelCheck, LevelServiceClass, LevelSecretType, LevelCandidate, LevelCVE, LevelGeneric} {
		for _, key := range r.Keys(level) {
			entry, _ := r.Entry(level, key)
			texts := append(entry.texts(), entry.Label)
			for _, text := range texts {
				for _, problem := range append(lintBoundary(text), lintWording(text)...) {
					t.Errorf("%s %q: %s\n  in: %s", level, key, problem, truncate(text, 120))
				}
			}
		}
	}
}

// Content must read as target states and consequences, never as a statement
// that a particular control is in place.
func TestWhatGoodLooksLikeIsATargetNotAClaim(t *testing.T) {
	r := loadEmbedded(t)
	claim := regexp.MustCompile(`(?i)\b(your|the) (domain|organization|setup|configuration) (is|has) (in place|working|correct)\b|\bis in place\b|\bis working\b`)
	for _, level := range []string{LevelSignal, LevelWeaknessClass, LevelCheck, LevelServiceClass, LevelSecretType, LevelCandidate, LevelCVE, LevelGeneric} {
		for _, key := range r.Keys(level) {
			entry, _ := r.Entry(level, key)
			if !strings.HasPrefix(entry.WhatGoodLooksLike, "Good looks like") {
				t.Errorf("%s %q: whatGoodLooksLike must be phrased as a target (\"Good looks like ...\"): %s", level, key, truncate(entry.WhatGoodLooksLike, 80))
			}
			if m := claim.FindString(entry.WhatGoodLooksLike); m != "" {
				t.Errorf("%s %q: whatGoodLooksLike reads as a claim about current state: %q", level, key, m)
			}
		}
	}
}

func TestHowItCouldBeAbusedNamesAGoalAndAConsequence(t *testing.T) {
	r := loadEmbedded(t)
	for _, level := range []string{LevelSignal, LevelWeaknessClass, LevelCheck, LevelServiceClass, LevelSecretType, LevelCandidate, LevelCVE, LevelGeneric} {
		for _, key := range r.Keys(level) {
			entry, _ := r.Entry(level, key)
			text := strings.ToLower(entry.HowItCouldBeAbused)
			if !strings.Contains(text, "consequence") && !strings.Contains(text, "risk for the organization") {
				t.Errorf("%s %q: howItCouldBeAbused should state the consequence for the organization", level, key)
			}
		}
	}
}

func TestCandidateNarrativeDoesNotAssertVulnerability(t *testing.T) {
	r := loadEmbedded(t)
	out := r.Explain(Input{
		Source:   store.SourceCandidate,
		CVE:      "CVE-2021-23017",
		Title:    "CVE-2021-23017 (nginx 1.18.0 — unconfirmed)",
		Severity: store.SeverityHigh,
	})
	if out.Level != LevelCandidate || out.LevelKey != KeyCandidate {
		t.Fatalf("candidate resolved to %s/%s, want %s/%s", out.Level, out.LevelKey, LevelCandidate, KeyCandidate)
	}
	assertion := regexp.MustCompile(`(?i)\b(asset|system|host|server|target)\s+is\s+vulnerable\b|\bconfirmed\s+vulnerab|\bproved?\s+vulnerable\b`)
	all := append([]string{out.WhyItMatters, out.HowItCouldBeAbused, out.WhatGoodLooksLike, out.HowRescanConfirms}, out.WhatToDo...)
	joined := strings.ToLower(strings.Join(all, " "))
	for _, text := range all {
		if m := assertion.FindString(text); m != "" {
			t.Errorf("candidate narrative asserts vulnerability (%q) in: %s", m, text)
		}
		if probs := lintBoundary(text); len(probs) > 0 {
			t.Errorf("candidate narrative violates boundary rules: %v", probs)
		}
	}
	if !strings.Contains(joined, "backport") {
		t.Error("candidate narrative must include the distribution backport caveat")
	}
	if !strings.Contains(joined, "unconfirmed") {
		t.Error("candidate narrative must explicitly state that the candidate is unconfirmed")
	}
}

// ---- the rendered output -----------------------------------------------------

func TestRenderedOutputLintForEveryCatalogueIdentifier(t *testing.T) {
	r := loadEmbedded(t)
	classOf := map[string]string{}
	for _, e := range signalRegistry(t) {
		classOf[e.SignalID] = e.WeaknessClass
	}
	states := []store.CoverageState{"", store.CoverageOK, store.CoverageNotFound, store.CoverageNotChecked, store.CoverageCheckFailed}
	for _, entry := range vfinding.Catalogue() {
		for _, state := range states {
			in := catalogueInput(entry, classOf[entry.ID])
			in.CheckState = state
			in.CheckReason = "the resolver did not answer"
			out := r.Explain(in)
			label := entry.ID + "/" + string(state)

			// Boundary rules cover everything rendered.
			whole := append([]string{out.WhatWeFound, out.HowItCouldBeAbused, out.WhyItMatters, out.WhatGoodLooksLike, out.HowRescanConfirms,
				out.CoverageNote, out.ExploitationStatus}, out.WhatToDo...)
			for _, text := range whole {
				for _, problem := range lintBoundary(text) {
					t.Errorf("%s: %s\n  in: %s", label, problem, truncate(text, 120))
				}
			}

			// Wording rules cover the text this package writes: render again
			// without catalogue text or occurrence data.
			bare := in
			bare.Catalogue, bare.Detail, bare.Evidence = nil, "", ""
			for name, text := range authoredSections(r.Explain(bare), false) {
				for _, problem := range lintWording(text) {
					t.Errorf("%s %s: %s\n  in: %s", label, name, problem, truncate(text, 120))
				}
			}

			if state.Assessed() || state == "" {
				if out.CoverageNote != "" {
					t.Errorf("%s: assessed finding carries a coverage note", label)
				}
				continue
			}
			// Unassessed: must say so, and must not assert the weakness.
			if !strings.Contains(out.CoverageNote, "could not be assessed") {
				t.Errorf("%s: unassessed finding has no coverage note: %q", label, out.CoverageNote)
			}
			if !strings.Contains(out.WhatGoodLooksLike, "could not be assessed") {
				t.Errorf("%s: whatGoodLooksLike does not say the control could not be assessed", label)
			}
			if strings.Contains(out.WhatWeFound, entry.Description) {
				t.Errorf("%s: unassessed finding repeats the catalogue description as if it were found", label)
			}
			if !strings.Contains(out.WhatWeFound, "could not assess") {
				t.Errorf("%s: whatWeFound does not say the control could not be assessed", label)
			}
		}
	}
}

func TestExploitationWordingLintAcrossEveryState(t *testing.T) {
	r := loadEmbedded(t)
	kev := []string{"", "ok", "not_found", "not_checked", "check_failed"}
	for _, kevState := range kev {
		for _, listed := range []bool{false, true} {
			for _, epssState := range []string{"", "ok", "not_found", "not_checked", "check_failed"} {
				for _, high := range []bool{false, true} {
					in := Input{
						CVE: "CVE-2024-0001", KEVState: kevState, KEVListed: listed, KEVCheckedAt: "2026-03-04T10:00:00Z",
						EPSSState: epssState, EPSSHigh: high, EPSSCheckedAt: "2026-03-04T10:00:00Z",
					}
					out := r.Explain(in)
					text := out.ExploitationStatus
					if text == "" {
						t.Errorf("%+v: a CVE finding must always carry exploitation wording", in)
					}
					for _, p := range append(lintBoundary(text), lintWording(text)...) {
						t.Errorf("%+v: %s\n  in: %s", in, p, text)
					}
					// A date accompanies any statement that depends on a check.
					if (kevState == "ok" || kevState == "not_found") && !strings.Contains(text, "March 4, 2026") {
						t.Errorf("%+v: wording has no date: %s", in, text)
					}
					// Never implies safety when nothing was established.
					if (kevState == "" || kevState == "not_checked" || kevState == "check_failed") && !strings.Contains(text, "could not be established") {
						t.Errorf("%+v: unestablished status not stated: %s", in, text)
					}
				}
			}
		}
	}
}

func TestEveryEmbeddedEntryRendersCleanlyForItsOwnInput(t *testing.T) {
	r := loadEmbedded(t)
	inputs := map[string]Input{}
	for _, key := range r.Keys(LevelServiceClass) {
		inputs["service-class/"+key] = Input{ServiceClass: key, Title: "Externally reachable " + key + " service"}
	}
	for _, key := range r.Keys(LevelSecretType) {
		inputs["secret-type/"+key] = Input{SecretType: strings.Replace(key, ".*", ".example", 1), Title: "Secret found"}
	}
	inputs["cve"] = Input{CVE: "CVE-2024-0001", Title: "Example vulnerability"}
	inputs["generic"] = Input{Title: "Example"}
	for name, in := range inputs {
		out := r.Explain(in)
		raw := strings.Join(append([]string{out.WhatWeFound, out.WhyItMatters, out.HowItCouldBeAbused, out.WhatGoodLooksLike, out.HowRescanConfirms}, out.WhatToDo...), "\n")
		if strings.Contains(raw, "{{") || strings.Contains(raw, "}}") {
			t.Errorf("%s: unfilled placeholder in rendered output:\n%s", name, raw)
		}
		if out.WhatWeFound == "" || out.WhyItMatters == "" || out.HowItCouldBeAbused == "" || out.WhatGoodLooksLike == "" || len(out.WhatToDo) == 0 || out.HowRescanConfirms == "" {
			t.Errorf("%s: an explanation section is empty: %+v", name, out)
		}
		if out.Version == "" {
			t.Errorf("%s: no version", name)
		}
	}
}

// For a control that could not be assessed, "why it matters" and "how it could
// be abused" are still shown, under a sentence saying they are general. They
// must therefore describe the control and the weakness in general terms, and
// never state what was found about this occurrence; that belongs to the
// catalogue text in "what we found", which is withheld when nothing was
// assessed.
func TestSignalAndCheckWordingDoesNotAssertTheOccurrence(t *testing.T) {
	r := loadEmbedded(t)
	assertive := regexp.MustCompile(`(?i)\b(your domain (has|publishes)|this (record|name|domain|address|nameserver|zone|policy)|was found|were found|found none)\b`)
	for _, level := range []string{LevelSignal, LevelWeaknessClass, LevelCheck} {
		for _, key := range r.Keys(level) {
			entry, _ := r.Entry(level, key)
			for name, text := range map[string]string{"whyItMatters": entry.WhyItMatters, "howItCouldBeAbused": entry.HowItCouldBeAbused} {
				if m := assertive.FindString(text); m != "" {
					t.Errorf("%s %q %s states a fact about the occurrence (%q); keep it general", level, key, name, m)
				}
			}
		}
	}
}
