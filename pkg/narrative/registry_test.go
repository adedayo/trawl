package narrative

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

// ---- helpers ------------------------------------------------------------

func fullEntry(level, key, label string) Entry {
	tag := level + "/" + key
	return Entry{
		Level: level, Key: key, Label: label,
		WhyItMatters:       "why " + tag,
		HowItCouldBeAbused: "abused " + tag,
		WhatGoodLooksLike:  "good " + tag,
		WhatToDo:           []string{"step one " + tag, "step two " + tag},
		HowRescanConfirms:  "rescan " + tag,
	}
}

func doc(t *testing.T, version string, entries ...Entry) []byte {
	t.Helper()
	data, err := json.Marshal(contentFile{Version: version, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// testRegistry has one entry at every level so lookup order is observable.
func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := LoadFrom(doc(t, "test-1",
		fullEntry(LevelSignal, "SURF-X-001", ""),
		fullEntry(LevelWeaknessClass, "cls", ""),
		fullEntry(LevelCheck, "chk", ""),
		fullEntry(LevelServiceClass, "rdp", "remote desktop service"),
		fullEntry(LevelSecretType, "aws.access_key", "exact access key"),
		fullEntry(LevelSecretType, "aws.*", "cloud credential"),
		fullEntry(LevelCVE, KeyCVE, ""),
		fullEntry(LevelGeneric, KeyGeneric, ""),
	))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// ---- lookup order and fallback -------------------------------------------

func TestLookupOrderAndFallback(t *testing.T) {
	r := testRegistry(t)
	cases := []struct {
		name         string
		in           Input
		wantLevel    string
		wantLevelKey string
	}{
		{"signal beats everything", Input{SignalID: "SURF-X-001", WeaknessClass: "cls", Check: "chk", ServiceClass: "rdp", SecretType: "aws.access_key", CVE: "CVE-2024-0001"}, LevelSignal, "SURF-X-001"},
		{"signal id is case-insensitive", Input{SignalID: "surf-x-001"}, LevelSignal, "SURF-X-001"},
		{"unknown signal falls to check", Input{SignalID: "SURF-X-999", WeaknessClass: "cls", Check: "chk"}, LevelCheck, "chk"},
		{"unknown signal and check fall to class", Input{SignalID: "SURF-X-999", WeaknessClass: "cls", Check: "other"}, LevelWeaknessClass, "cls"},
		{"check beats class, which is the broader of the two", Input{WeaknessClass: "cls", Check: "chk"}, LevelCheck, "chk"},
		{"unknown class falls to check", Input{WeaknessClass: "nope", Check: "chk"}, LevelCheck, "chk"},
		{"check beats service class", Input{Check: "chk", ServiceClass: "rdp"}, LevelCheck, "chk"},
		{"service class", Input{ServiceClass: "rdp", SecretType: "aws.access_key"}, LevelServiceClass, "rdp"},
		{"unknown service class falls through", Input{ServiceClass: "web", SecretType: "aws.access_key"}, LevelSecretType, "aws.access_key"},
		{"secret exact beats family", Input{SecretType: "aws.access_key", CVE: "CVE-2024-0001"}, LevelSecretType, "aws.access_key"},
		{"secret family fallback", Input{SecretType: "aws.session_token"}, LevelSecretType, "aws.*"},
		{"secret type is case-insensitive", Input{SecretType: "AWS.Session_Token"}, LevelSecretType, "aws.*"},
		{"unknown secret family falls through", Input{SecretType: "zzz.thing", CVE: "CVE-2024-0001"}, LevelCVE, KeyCVE},
		{"cve template", Input{CVE: "CVE-2024-0001"}, LevelCVE, KeyCVE},
		{"blank cve is not a cve", Input{CVE: "   "}, LevelGeneric, KeyGeneric},
		{"nothing matches", Input{}, LevelGeneric, KeyGeneric},
		{"nothing known matches", Input{SignalID: "x", WeaknessClass: "y", Check: "z", ServiceClass: "w", SecretType: "v"}, LevelGeneric, KeyGeneric},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.Explain(tc.in)
			if got.Level != tc.wantLevel || got.LevelKey != tc.wantLevelKey {
				t.Fatalf("got %s/%s, want %s/%s", got.Level, got.LevelKey, tc.wantLevel, tc.wantLevelKey)
			}
			if got.Version != "test-1" {
				t.Fatalf("version = %q", got.Version)
			}
			if want := "why " + tc.wantLevel + "/" + tc.wantLevelKey; got.WhyItMatters != want {
				t.Fatalf("wording came from the wrong entry: %q, want %q", got.WhyItMatters, want)
			}
		})
	}
}

func TestResolveReportsWhetherAnEntryMatched(t *testing.T) {
	r := testRegistry(t)
	if _, _, ok := r.Resolve(Input{}); !ok {
		t.Fatal("the generic entry must always resolve")
	}
}

// ---- loading and validation ----------------------------------------------

func TestLoadFromRejectsMalformedContent(t *testing.T) {
	good := fullEntry(LevelGeneric, KeyGeneric, "")
	noGeneric := fullEntry(LevelWeaknessClass, "cls", "")
	blank := func(mutate func(*Entry)) Entry {
		e := fullEntry(LevelWeaknessClass, "cls", "")
		mutate(&e)
		return e
	}
	cases := []struct {
		name string
		data [][]byte
		want string
	}{
		{"no documents", nil, "no content documents"},
		{"not json", [][]byte{[]byte("{")}, "document 1"},
		{"unknown field", [][]byte{[]byte(`{"version":"1","entries":[],"extra":1}`)}, "unknown field"},
		{"missing version", [][]byte{doc(t, "", good)}, "version is required"},
		{"mismatched versions", [][]byte{doc(t, "1", good), doc(t, "2", noGeneric)}, "one version"},
		{"duplicate entry", [][]byte{doc(t, "1", good, good)}, "duplicate"},
		{"duplicate across documents", [][]byte{doc(t, "1", good), doc(t, "1", good)}, "duplicate"},
		{"unknown level", [][]byte{doc(t, "1", good, fullEntry("nonsense", "k", ""))}, "unknown level"},
		{"missing key", [][]byte{doc(t, "1", good, fullEntry(LevelCheck, " ", ""))}, "key is required"},
		{"cve with another key", [][]byte{doc(t, "1", good, fullEntry(LevelCVE, "other", ""))}, "single key"},
		{"generic with another key", [][]byte{doc(t, "1", fullEntry(LevelGeneric, "other", ""))}, "single key"},
		{"missing why", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.WhyItMatters = " " }))}, "whyItMatters is required"},
		{"missing abuse", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.HowItCouldBeAbused = "" }))}, "howItCouldBeAbused is required"},
		{"missing good", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.WhatGoodLooksLike = "" }))}, "whatGoodLooksLike is required"},
		{"missing rescan", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.HowRescanConfirms = "" }))}, "howRescanConfirms is required"},
		{"no steps", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.WhatToDo = nil }))}, "at least one step"},
		{"empty step", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.WhatToDo = []string{"ok", " "} }))}, "empty step"},
		{"unknown placeholder", [][]byte{doc(t, "1", good, blank(func(e *Entry) { e.WhyItMatters = "see {{nope}}" }))}, "unknown placeholder"},
		{"no generic fallback", [][]byte{doc(t, "1", noGeneric)}, "no generic entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadFrom(tc.data...)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestLoadFromMergesDocumentsOfOneVersion(t *testing.T) {
	r, err := LoadFrom(
		doc(t, "9", fullEntry(LevelGeneric, KeyGeneric, "")),
		doc(t, "9", fullEntry(LevelCheck, "chk", "")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if r.Version() != "9" {
		t.Fatalf("version = %q", r.Version())
	}
	if got := r.Keys(LevelCheck); !reflect.DeepEqual(got, []string{"chk"}) {
		t.Fatalf("keys = %v", got)
	}
}

func TestEmbeddedContentLoads(t *testing.T) {
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r.Version() == "" {
		t.Fatal("embedded content has no version")
	}
	for _, level := range []string{LevelSignal, LevelWeaknessClass, LevelCheck, LevelServiceClass, LevelSecretType, LevelCVE, LevelGeneric} {
		if len(r.Keys(level)) == 0 {
			t.Errorf("embedded content has no %s entries", level)
		}
	}
}

func TestEntryReturnsACopy(t *testing.T) {
	r := testRegistry(t)
	e, ok := r.Entry(LevelCheck, "chk")
	if !ok {
		t.Fatal("entry missing")
	}
	e.WhatToDo[0] = "tampered"
	again, _ := r.Entry(LevelCheck, "chk")
	if again.WhatToDo[0] == "tampered" {
		t.Fatal("Entry handed out the registry's own slice")
	}
}

// ---- rendering: what we found ---------------------------------------------

func TestWhatWeFoundUsesCatalogueAndOccurrence(t *testing.T) {
	r := testRegistry(t)
	in := Input{
		SignalID: "SURF-X-001",
		Detail:   "The record at example.org ends in plus all",
		Evidence: "v=spf1 +all",
		Catalogue: &Catalogue{
			Title:       "SPF record permits all senders",
			Description: "The record terminates in a rule that authorises every host.",
		},
	}
	got := r.Explain(in)
	want := "SPF record permits all senders.\n\nThe record terminates in a rule that authorises every host.\n\nIn this case: The record at example.org ends in plus all.\n\nEvidence recorded: v=spf1 +all"
	if got.WhatWeFound != want {
		t.Fatalf("WhatWeFound =\n%q\nwant\n%q", got.WhatWeFound, want)
	}
}

func TestOccurrenceDataAppearsOnlyInWhatWeFound(t *testing.T) {
	r := testRegistry(t)
	in := Input{
		SignalID: "SURF-X-001", Detail: "UNIQUE-DETAIL-9F3A", Evidence: "UNIQUE-EVIDENCE-7C1B",
		Catalogue: &Catalogue{Title: "T", Description: "D", Remediation: "R", References: []string{"https://example.org/r"}},
	}
	got := r.Explain(in)
	if !strings.Contains(got.WhatWeFound, "UNIQUE-DETAIL-9F3A") || !strings.Contains(got.WhatWeFound, "UNIQUE-EVIDENCE-7C1B") {
		t.Fatalf("occurrence data missing from WhatWeFound: %q", got.WhatWeFound)
	}
	got.WhatWeFound = ""
	raw, _ := json.Marshal(got)
	for _, marker := range []string{"UNIQUE-DETAIL-9F3A", "UNIQUE-EVIDENCE-7C1B"} {
		if strings.Contains(string(raw), marker) {
			t.Errorf("%s leaked outside WhatWeFound: %s", marker, raw)
		}
	}
}

func TestWhatWeFoundWithoutCatalogueUsesTitle(t *testing.T) {
	r := testRegistry(t)
	got := r.Explain(Input{ServiceClass: "rdp", Title: "Externally reachable rdp service", Detail: "host=h port=3389"})
	if !strings.HasPrefix(got.WhatWeFound, "Externally reachable rdp service.") || !strings.Contains(got.WhatWeFound, "In this case: host=h port=3389.") {
		t.Fatalf("WhatWeFound = %q", got.WhatWeFound)
	}
	if empty := r.Explain(Input{}).WhatWeFound; empty == "" {
		t.Fatal("WhatWeFound must never be empty")
	}
}

func TestDetailRepeatingTheDescriptionIsNotRepeated(t *testing.T) {
	r := testRegistry(t)
	got := r.Explain(Input{Title: "T", Detail: "Same words", Catalogue: &Catalogue{Title: "T", Description: "Same words"}})
	if strings.Count(got.WhatWeFound, "Same words") != 1 {
		t.Fatalf("WhatWeFound = %q", got.WhatWeFound)
	}
}

// ---- rendering: placeholders ----------------------------------------------

func TestPlaceholderSubstitution(t *testing.T) {
	entry := func(level, key, label, text string) Entry {
		e := fullEntry(level, key, label)
		e.WhyItMatters = text
		return e
	}
	all := "service={{service}} secret={{ secretType }} cve={{cve}} title={{title}} check={{check}}"
	r, err := LoadFrom(doc(t, "p",
		entry(LevelGeneric, KeyGeneric, "", all),
		entry(LevelServiceClass, "rdp", "remote desktop service", all),
		entry(LevelSecretType, "aws.*", "cloud credential", all),
		entry(LevelCVE, KeyCVE, "", all),
	))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"nothing to fill", Input{}, "service=service secret=secret cve=this vulnerability title=this finding check=this check"},
		{"service label", Input{ServiceClass: "rdp", Title: "Open rdp", Check: "net"}, "service=remote desktop service secret=secret cve=this vulnerability title=Open rdp check=net"},
		{"secret label", Input{SecretType: "aws.access_key"}, "service=service secret=cloud credential cve=this vulnerability title=this finding check=this check"},
		{"unlabelled secret keeps the raw type", Input{SecretType: "zzz.thing", Title: "x"}, "service=service secret=zzz.thing cve=this vulnerability title=x check=this check"},
		{"cve", Input{CVE: "CVE-2024-12345", Catalogue: &Catalogue{Title: "Catalogue title"}}, "service=service secret=secret cve=CVE-2024-12345 title=Catalogue title check=this check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.Explain(tc.in).WhyItMatters; got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestSubstitutedValuesAreNotExpandedAgain(t *testing.T) {
	e := fullEntry(LevelGeneric, KeyGeneric, "")
	e.WhyItMatters = "about {{title}} and {{cve}}"
	r, err := LoadFrom(doc(t, "p", e))
	if err != nil {
		t.Fatal(err)
	}
	got := r.Explain(Input{Title: "{{cve}}", CVE: "{{title}}"})
	if got.WhyItMatters != "about {{cve}} and {{title}}" {
		t.Fatalf("slots in input were expanded: %q", got.WhyItMatters)
	}
	// Detail is interpolated as text, never as a template.
	got = r.Explain(Input{Detail: "{{service}} {{cve}}"})
	if !strings.Contains(got.WhatWeFound, "{{service}} {{cve}}") {
		t.Fatalf("detail was altered: %q", got.WhatWeFound)
	}
}

// ---- rendering: what to do -------------------------------------------------

func TestRemediationIsAppendedUnlessDuplicate(t *testing.T) {
	r := testRegistry(t)
	cases := []struct {
		name        string
		remediation string
		wantLast    string
		wantSteps   int
	}{
		{"appended", "Restrict transfers to the secondary servers.", "Restrict transfers to the secondary servers.", 3},
		{"none", "", "step two check/chk", 2},
		{"duplicate of a step", "STEP ONE   check/chk!", "step two check/chk", 2},
		{"contains a step", "Do this: step two check/chk, then verify.", "step two check/chk", 2},
		{"contained in a step", "two check", "step two check/chk", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.Explain(Input{Check: "chk", Catalogue: &Catalogue{Remediation: tc.remediation}})
			if len(got.WhatToDo) != tc.wantSteps || got.WhatToDo[len(got.WhatToDo)-1] != tc.wantLast {
				t.Fatalf("steps = %q", got.WhatToDo)
			}
			if got.WhatToDo[0] != "step one check/chk" {
				t.Fatalf("narrative steps must come first, in order: %q", got.WhatToDo)
			}
		})
	}
}

func TestReferencesAndStandards(t *testing.T) {
	r := testRegistry(t)
	refs := []string{"https://example.org/a", "https://example.org/b"}
	got := r.Explain(Input{Check: "chk", Catalogue: &Catalogue{
		References: refs,
		Tags:       []string{"pki", "ncsc-mailcheck", "resilience", "cis-controls", "nist-800-81", "pci-dss"},
	}})
	if !reflect.DeepEqual(got.References, refs) {
		t.Fatalf("references = %v", got.References)
	}
	if want := []string{"ncsc-mailcheck", "cis-controls", "nist-800-81", "pci-dss"}; !reflect.DeepEqual(got.Standards, want) {
		t.Fatalf("standards = %v, want %v", got.Standards, want)
	}
	refs[0] = "tampered"
	if got.References[0] == "tampered" {
		t.Fatal("References shares the caller's slice")
	}
	if none := r.Explain(Input{Check: "chk"}); none.References == nil || len(none.References) != 0 {
		t.Fatalf("References without a catalogue should be an empty list, got %#v", none.References)
	}
}

// ---- rendering: exploitation status ----------------------------------------

func TestExploitationStatus(t *testing.T) {
	r := testRegistry(t)
	const (
		d1 = "2026-03-04T10:00:00Z"
		d2 = "2026-03-05T23:30:00-05:00" // 4:30 UTC on March 6
	)
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"not a cve finding", Input{Check: "chk"}, ""},
		{"kev listed", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true, KEVCheckedAt: d1},
			"This weakness is known to be used in real attacks (checked against the public exploited-vulnerabilities catalogue on March 4, 2026)."},
		{"kev ok not listed", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: false, KEVCheckedAt: d1},
			"This weakness was not listed in the public exploited-vulnerabilities catalogue on March 4, 2026. That does not mean it is not being used in attacks, only that it was not listed at that time."},
		{"kev not found", Input{CVE: "CVE-2024-1", KEVState: "not_found", KEVCheckedAt: d1},
			"No entry for this weakness was found in the public exploited-vulnerabilities catalogue on March 4, 2026. That does not rule out use in real attacks, only that it was not listed at that time."},
		{"kev not checked", Input{CVE: "CVE-2024-1", KEVState: "not_checked"}, exploitUnknown},
		{"kev check failed", Input{CVE: "CVE-2024-1", KEVState: "check_failed", KEVCheckedAt: d1}, exploitUnknown},
		{"cve with no enrichment state", Input{CVE: "CVE-2024-1"}, exploitUnknown},
		{"unrecognized kev state is not read as a verdict", Input{CVE: "CVE-2024-1", KEVState: "weird", KEVListed: true}, exploitUnknown},
		{"date is shown in UTC", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true, KEVCheckedAt: d2},
			"This weakness is known to be used in real attacks (checked against the public exploited-vulnerabilities catalogue on March 6, 2026)."},
		{"missing date is said, not invented", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true},
			"This weakness is known to be used in real attacks (checked against the public exploited-vulnerabilities catalogue; the date of that check was not recorded)."},
		{"unparsable date is said, not invented", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true, KEVCheckedAt: "yesterday"},
			"This weakness is known to be used in real attacks (checked against the public exploited-vulnerabilities catalogue; the date of that check was not recorded)."},
		{"epss elevated adds a sentence", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: false, KEVCheckedAt: d1, EPSSState: "ok", EPSSHigh: true, EPSSCheckedAt: d1},
			"This weakness was not listed in the public exploited-vulnerabilities catalogue on March 4, 2026. That does not mean it is not being used in attacks, only that it was not listed at that time. " +
				"Public attack-forecasting data (the Exploit Prediction Scoring System) also rates the likelihood of this weakness being used in attacks as elevated on March 4, 2026."},
		{"epss not high says nothing", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true, KEVCheckedAt: d1, EPSSState: "ok", EPSSHigh: false, EPSSCheckedAt: d1},
			"This weakness is known to be used in real attacks (checked against the public exploited-vulnerabilities catalogue on March 4, 2026)."},
		{"epss high but not checked says nothing", Input{CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true, KEVCheckedAt: d1, EPSSState: "not_checked", EPSSHigh: true},
			"This weakness is known to be used in real attacks (checked against the public exploited-vulnerabilities catalogue on March 4, 2026)."},
		{"epss high with kev failed", Input{CVE: "CVE-2024-1", KEVState: "check_failed", EPSSState: "ok", EPSSHigh: true, EPSSCheckedAt: d1},
			exploitUnknown + " Public attack-forecasting data (the Exploit Prediction Scoring System) also rates the likelihood of this weakness being used in attacks as elevated on March 4, 2026."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.Explain(tc.in).ExploitationStatus
			if got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
			for _, banned := range []string{"%", "percent", "probabilit", "score"} {
				if strings.Contains(strings.ToLower(got), banned) {
					t.Fatalf("exploitation wording must not carry figures: %q", got)
				}
			}
		})
	}
}

// ---- rendering: coverage ----------------------------------------------------

func TestCoverageNoteAndUnassessedWording(t *testing.T) {
	r := testRegistry(t)
	cat := &Catalogue{Title: "No DMARC record published", Description: "CATALOGUE-DESCRIPTION asserts a weakness.", Remediation: "Publish a record."}

	for _, state := range []store.CoverageState{store.CoverageNotChecked, store.CoverageCheckFailed} {
		t.Run(string(state), func(t *testing.T) {
			got := r.Explain(Input{Check: "chk", CheckState: state, CheckReason: "resolver timed out", Catalogue: cat, Detail: "d"})
			if !strings.HasPrefix(got.CoverageNote, "This control could not be assessed.") {
				t.Fatalf("CoverageNote = %q", got.CoverageNote)
			}
			if !strings.Contains(got.CoverageNote, "Reason recorded: resolver timed out.") {
				t.Fatalf("reason missing: %q", got.CoverageNote)
			}
			if !strings.Contains(got.WhatWeFound, "could not assess") || strings.Contains(got.WhatWeFound, "CATALOGUE-DESCRIPTION") {
				t.Fatalf("WhatWeFound must not assert the weakness for an unassessed control: %q", got.WhatWeFound)
			}
			if !strings.Contains(got.WhatGoodLooksLike, "could not be assessed") || !strings.Contains(got.WhatGoodLooksLike, "cannot say whether it is in place") {
				t.Fatalf("WhatGoodLooksLike implies the control is in place: %q", got.WhatGoodLooksLike)
			}
			if !strings.Contains(got.WhyItMatters, "could not be assessed") {
				t.Fatalf("WhyItMatters = %q", got.WhyItMatters)
			}
			if !strings.Contains(got.WhatToDo[0], "run it again") {
				t.Fatalf("first step should be to re-run the check: %q", got.WhatToDo)
			}
			if got.WhatToDo[len(got.WhatToDo)-1] != "Publish a record." {
				t.Fatalf("remediation missing: %q", got.WhatToDo)
			}
		})
	}

	t.Run("states differ in wording", func(t *testing.T) {
		a := r.Explain(Input{CheckState: store.CoverageNotChecked}).CoverageNote
		b := r.Explain(Input{CheckState: store.CoverageCheckFailed}).CoverageNote
		if a == b {
			t.Fatal("not_checked and check_failed are different statements and must read differently")
		}
	})

	t.Run("unrecognized state is treated as not assessed", func(t *testing.T) {
		got := r.Explain(Input{CheckState: "mystery"})
		if got.CoverageNote == "" || !strings.Contains(got.CoverageNote, "could not be assessed") {
			t.Fatalf("CoverageNote = %q", got.CoverageNote)
		}
	})

	for _, state := range []store.CoverageState{store.CoverageOK, store.CoverageNotFound, ""} {
		t.Run("assessed:"+string(state), func(t *testing.T) {
			got := r.Explain(Input{Check: "chk", CheckState: state, Catalogue: cat})
			if got.CoverageNote != "" {
				t.Fatalf("assessed control got a coverage note: %q", got.CoverageNote)
			}
			if !strings.Contains(got.WhatWeFound, "CATALOGUE-DESCRIPTION") {
				t.Fatalf("assessed finding lost its description: %q", got.WhatWeFound)
			}
			if strings.Contains(got.WhatGoodLooksLike, "could not be assessed") {
				t.Fatalf("assessed finding carries unassessed wording: %q", got.WhatGoodLooksLike)
			}
		})
	}
}

func TestExplainIsDeterministic(t *testing.T) {
	r := testRegistry(t)
	in := Input{
		SignalID: "SURF-X-001", CVE: "CVE-2024-1", KEVState: "ok", KEVListed: true, KEVCheckedAt: "2026-03-04T10:00:00Z",
		EPSSState: "ok", EPSSHigh: true, EPSSCheckedAt: "2026-03-04T10:00:00Z",
		CheckState: store.CoverageCheckFailed, CheckReason: "r", Detail: "d", Evidence: "e",
		Catalogue: &Catalogue{Title: "t", Description: "d", Remediation: "x", References: []string{"a"}, Tags: []string{"pci-dss"}},
	}
	first := r.Explain(in)
	for i := 0; i < 50; i++ {
		if got := r.Explain(in); !reflect.DeepEqual(first, got) {
			t.Fatalf("run %d differs:\n%#v\n%#v", i, first, got)
		}
	}
}

func TestExplainDoesNotMutateItsInput(t *testing.T) {
	r := testRegistry(t)
	cat := &Catalogue{References: []string{"a"}, Tags: []string{"pci-dss"}}
	in := Input{Check: "chk", Catalogue: cat}
	before, _ := json.Marshal(in)
	_ = r.Explain(in)
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("Explain mutated its input")
	}
}
