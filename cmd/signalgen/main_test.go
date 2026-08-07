package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// loadRegistry reads the committed registry rather than regenerating one, so
// that these tests assert what the application will actually ship with.
func loadRegistry(t *testing.T) registry {
	t.Helper()
	path := filepath.Join("..", "..", "config", "signals", "vantage-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the registry: %v", err)
	}
	var r registry
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parsing the registry: %v", err)
	}
	return r
}

// A finding tagged only for resilience describes something breaking, not
// something being read or forged. Filing it under an interception or spoofing
// scenario overstates that scenario — and because a control posture turns
// deficient on any assessed advisory regardless of severity, a low resilience
// note ends up contributing as much to the count as a genuine weakness.
//
// This is the check that stops the classification drifting back. It reads the
// catalogue's own per-finding tags, which are hand-written, and holds the
// generated registry to them.
func TestResilienceFindingsAreNotFiledAsAttacks(t *testing.T) {
	attackScenarios := []string{"email-interception", "email-spoofing", "domain-hijack"}

	// Findings where the attack scenario is correct despite the tag, with the
	// reason recorded. An exception list that must be edited deliberately is
	// honest; loosening the assertion until it passes is not, because it would
	// also stop catching the next mistake.
	justified := map[string]string{
		"SURF-NS-004": "a nameserver named only by the parent keeps receiving queries " +
			"after the operator believes it removed — an abandoned delegation is claimable, " +
			"which is a hijack and not merely an availability defect",
	}

	scenarioOf := map[string]string{}
	for _, e := range loadRegistry(t).Entries {
		scenarioOf[e.SignalID] = e.Scenario
	}

	for _, c := range vfinding.Catalogue() {
		// Only findings whose tags say resilience and nothing about
		// authentication. A finding carrying both is genuinely about both, and
		// the scenario is a legitimate judgement either way.
		if !slices.Contains(c.Tags, vfinding.TagResilience) {
			continue
		}
		if slices.Contains(c.Tags, vfinding.TagEmailAuth) {
			continue
		}
		if _, ok := justified[c.ID]; ok {
			continue
		}

		if s := scenarioOf[c.ID]; slices.Contains(attackScenarios, s) {
			t.Errorf("%s is tagged resilience only, but is classified under %q.\n"+
				"  %s\n"+
				"  Add a correction to bySignal in cmd/signalgen, or record a "+
				"justification in this test.",
				c.ID, s, c.Title)
		}
	}
}

// The registry must actually reflect bySignal. The generator preserves
// existing entries by design, so an override added without regenerating would
// silently do nothing at all.
func TestRegistryAgreesWithSignalOverrides(t *testing.T) {
	entryOf := map[string]entry{}
	for _, e := range loadRegistry(t).Entries {
		entryOf[e.SignalID] = e
	}

	for id, want := range bySignal {
		got, ok := entryOf[id]
		if !ok {
			t.Errorf("bySignal names %s, which is not in the registry", id)
			continue
		}
		if !matches(got, want) {
			t.Errorf("%s: registry has (%s, %s, %s, %s), bySignal wants (%s, %s, %s, %s). "+
				"Run: go run ./cmd/signalgen",
				id, got.WeaknessClass, got.Scenario, got.Stage, got.Control,
				want.weaknessClass, want.scenario, want.stage, want.control)
		}
	}
}

// An override naming an identifier the catalogue does not define is dead
// configuration: it looks like a reviewed decision and does nothing. That is
// worse than no override, because it invites the reader to believe the case
// has been handled.
func TestSignalOverridesNameRealFindings(t *testing.T) {
	known := map[string]bool{}
	for _, c := range vfinding.Catalogue() {
		known[c.ID] = true
	}
	for id := range bySignal {
		if !known[id] {
			t.Errorf("bySignal names %s, which the vantage catalogue does not define", id)
		}
	}
}
