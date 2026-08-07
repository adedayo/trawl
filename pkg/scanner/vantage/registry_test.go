package vantage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	vaudit "github.com/adedayo/vantage/pkg/audit"
	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// registryPath is the shipped registry under test.
func registryPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "config", "signals", "vantage-1.json")
}

// TestRegistryCoversEveryCatalogueIdentifier is the required CI check for
// Phase 4.
//
// A vantage upgrade that adds a finding must fail here rather than surface at
// run time as an observation nobody can interpret. The adapter retains an
// unmapped signal rather than dropping it, so this is not a crash — but an
// unmapped signal contributes nothing to a scenario, which means an upgrade
// could quietly reduce the risk model's coverage while appearing to work.
func TestRegistryCoversEveryCatalogueIdentifier(t *testing.T) {
	reg, err := LoadSignalRegistry(registryPath(t))
	if err != nil {
		t.Fatalf("loading the registry: %v", err)
	}

	var ids []string
	for _, e := range vfinding.Catalogue() {
		ids = append(ids, e.ID)
	}
	if len(ids) == 0 {
		t.Fatal("the vantage catalogue is empty; the check would pass vacuously")
	}

	if missing := reg.MissingFrom(ids); len(missing) > 0 {
		t.Fatalf("%d identifier(s) the library can raise are unmapped:\n  %s\n\n"+
			"Run: go run ./cmd/signalgen", len(missing), strings.Join(missing, "\n  "))
	}
}

// TestRegistryCoversEveryCheckDeclaration approaches the same property from the
// other direction: through what the checks declare rather than what the
// catalogue contains. A finding declared by a check but absent from the
// catalogue would slip past the test above.
func TestRegistryCoversEveryCheckDeclaration(t *testing.T) {
	reg, err := LoadSignalRegistry(registryPath(t))
	if err != nil {
		t.Fatalf("loading the registry: %v", err)
	}

	a, err := vaudit.NewAssessor(&countingResolver{})
	if err != nil {
		t.Fatalf("building an assessor: %v", err)
	}
	caps, err := a.Catalogue(context.Background())
	if err != nil {
		t.Fatalf("reading the catalogue: %v", err)
	}

	var declared []string
	for _, c := range caps.Checks {
		declared = append(declared, c.Findings...)
	}
	if missing := reg.MissingFrom(declared); len(missing) > 0 {
		t.Fatalf("%d identifier(s) declared by checks are unmapped:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestRegistryEntriesAreWellFormed guards the fields the risk engine depends
// on. A blank scenario or direction would let a signal be stored, counted and
// then contribute nothing, which is harder to notice than an outright failure.
func TestRegistryEntriesAreWellFormed(t *testing.T) {
	reg, err := LoadSignalRegistry(registryPath(t))
	if err != nil {
		t.Fatalf("loading the registry: %v", err)
	}
	if reg.Version() == "" {
		t.Fatal("the registry must declare a version: it is stamped on every observation")
	}

	for _, e := range reg.Entries() {
		if e.WeaknessClass == "" {
			t.Errorf("%s: no weakness class", e.SignalID)
		}
		if e.Scenario == "" {
			t.Errorf("%s: no scenario", e.SignalID)
		}
		if e.Stage == "" {
			t.Errorf("%s: no kill-chain stage", e.SignalID)
		}
		if e.DedupGroup == "" {
			t.Errorf("%s: no dedup group; findings would be counted as independent evidence", e.SignalID)
		}
		if e.Control == "" {
			t.Errorf("%s: no control", e.SignalID)
		}
		switch e.Direction {
		case "aggravating", "mitigating":
		default:
			t.Errorf("%s: direction %q must be aggravating or mitigating", e.SignalID, e.Direction)
		}
		if e.RegistryVersion != reg.Version() {
			t.Errorf("%s: registry version not stamped on the entry", e.SignalID)
		}
	}
}

// TestUnversionedRegistryIsRefused pins the fail-closed parse. Defaulting the
// version would make every observation's provenance a fiction.
func TestUnversionedRegistryIsRefused(t *testing.T) {
	_, err := ReadSignalRegistry(strings.NewReader(`{"entries":[]}`))
	if err == nil {
		t.Fatal("expected an unversioned registry to be refused")
	}
}

// TestDuplicateIdentifierIsRefused guards against one mapping silently
// shadowing another.
func TestDuplicateIdentifierIsRefused(t *testing.T) {
	_, err := ReadSignalRegistry(strings.NewReader(`{
		"registryVersion": "v1",
		"entries": [
			{"signalId": "SURF-SPF-001"},
			{"signalId": "SURF-SPF-001"}
		]
	}`))
	if err == nil {
		t.Fatal("expected a duplicate identifier to be refused")
	}
}

// TestRegistryMapsRealFindings ties the registry to the adapter, confirming
// that a real catalogue identifier arrives mapped rather than merely retained.
func TestRegistryMapsRealFindings(t *testing.T) {
	reg, err := LoadSignalRegistry(registryPath(t))
	if err != nil {
		t.Fatalf("loading the registry: %v", err)
	}

	catalogue := vfinding.Catalogue()
	first := catalogue[0]

	res := vfinding.NewResult("vantage", "1.0.0")
	res.Checks = []vfinding.CheckResult{{Check: first.Check, Target: "example.com", State: vfinding.StateOK}}
	res.Findings = []vfinding.Finding{{
		ID: first.ID, Check: first.Check, Target: "example.com", Severity: first.Severity,
	}}

	a := newAdapter(t, &fakeAssessor{result: res}, WithRegistry(reg))
	got, err := a.Assess(context.Background(), Request{AssetID: "a", Domain: "example.com"})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if len(got.Observations) != 1 {
		t.Fatalf("observations = %d, want 1", len(got.Observations))
	}
	if !got.Observations[0].Mapped {
		t.Fatalf("%s arrived unmapped despite being in the registry", first.ID)
	}
	if got.Observations[0].RegistryVersion != reg.Version() {
		t.Error("the registry version was not recorded on the observation")
	}
}
