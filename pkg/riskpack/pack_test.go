package riskpack

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func validPack(t *testing.T) []byte {
	t.Helper()
	pack := map[string]any{
		"packVersion":   "2026.09",
		"schemaVersion": 1,
		"sections": map[string]any{
			"weaknessClassPriors": map[string]any{
				"secret.applicability": map[string]any{
					"value": 0.85, "ess": 8, "source": "study", "retrievedAt": "2026-09-01",
					"verificationStatus": "needs-verification", "label": "sourced",
				},
			},
			"measuredStateSignals": map[string]any{
				"DMARC-001": map[string]any{
					"stage": "success", "deltaLogit": map[string]any{
						"value": 1.5, "source": "study", "retrievedAt": "2026-09-01",
						"verificationStatus": "needs-verification", "label": "sourced",
					}, "varianceShare": map[string]any{
						"value": 0.4, "source": "study", "retrievedAt": "2026-09-01",
						"verificationStatus": "needs-verification", "label": "sourced",
					}, "heterogeneityArgument": "mixed population", "dedupGroup": "dmarc", "tauDays": 180, "capG": 2,
				},
			},
		},
	}
	contents, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestLoadRefusesTamperedPack(t *testing.T) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	packPath := filepath.Join(dir, "pack.json")
	sigPath := packPath + ".sig"
	contents := validPack(t)
	if err := os.WriteFile(packPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sigPath, ed25519.Sign(private, contents), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(packPath, sigPath, public)
	if err != nil || loaded.Digest == "" || loaded.PackVersion != "2026.09" {
		t.Fatalf("valid pack did not load: %v", err)
	}
	if err := os.WriteFile(packPath, append(contents, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(packPath, sigPath, public); err == nil {
		t.Fatal("tampered pack loaded")
	}
}

func TestFixtureSectionsExposeTypedParameters(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating test fixture")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "base.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pack Pack
	if err := json.Unmarshal(contents, &pack); err != nil {
		t.Fatal(err)
	}
	if err := pack.Validate(); err != nil {
		t.Fatal(err)
	}
	priors, err := pack.Parameters("weaknessClassPriors")
	if err != nil || len(priors) != 3 {
		t.Fatalf("priors unavailable: %d, %v", len(priors), err)
	}
	signals, err := pack.MeasuredSignals()
	if err != nil || len(signals) != 1 || signals["SURF-DMARC-001"].Stage != "success" {
		t.Fatalf("signals unavailable: %+v, %v", signals, err)
	}
}

func TestValidateRequiresMeasuredSignalProvenance(t *testing.T) {
	contents := validPack(t)
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	sections := document["sections"].(map[string]any)
	signals := sections["measuredStateSignals"].(map[string]any)
	signal := signals["DMARC-001"].(map[string]any)
	signal["heterogeneityArgument"] = ""
	contents, _ = json.Marshal(document)
	var pack Pack
	if err := json.Unmarshal(contents, &pack); err != nil {
		t.Fatal(err)
	}
	if err := pack.Validate(); err == nil {
		t.Fatal("incomplete measured signal passed validation")
	}
}

func TestResolveAttributesParameterLayer(t *testing.T) {
	base := Parameter{Value: json.RawMessage(`0.4`), Source: "study", RetrievedAt: "2026-09-01", VerificationStatus: NeedsVerification, Label: Sourced}
	override := Parameter{Value: json.RawMessage(`0.6`), Source: "operator", RetrievedAt: "2026-09-21", VerificationStatus: Verified, Label: Sourced}
	resolved := Resolve(map[string]Parameter{"signal": base}, map[string]Parameter{"signal": override})
	if resolved["signal"].Layer != "override" || resolved["signal"].VerificationStatus != LocallyOverridden {
		t.Fatalf("override attribution missing: %+v", resolved["signal"])
	}
	baseResolved := Resolve(map[string]Parameter{"signal": base}, nil)
	if baseResolved["signal"].Layer != "base" {
		t.Fatalf("base attribution missing: %+v", baseResolved["signal"])
	}
}

func TestPrecisionGainDeduplicatesCapsAndDecays(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	result, err := PrecisionGain([]Observation{
		{SignalID: "a", DedupGroup: "same", VarianceShare: 0.4, TauDays: 180, Assessed: true, ObservedAt: now},
		{SignalID: "b", DedupGroup: "same", VarianceShare: 0.3, TauDays: 180, Assessed: true, ObservedAt: now},
		{SignalID: "c", DedupGroup: "other", VarianceShare: 0.9, TauDays: 180, Assessed: true, ObservedAt: now},
	}, now, 2)
	if err != nil || result.Gain != 2 || !result.CapApplied || len(result.Groups) != 2 {
		t.Fatalf("unexpected precision result: %+v, %v", result, err)
	}
	stale, err := PrecisionGain([]Observation{
		{DedupGroup: "same", VarianceShare: 0.4, TauDays: 1, Assessed: true, ObservedAt: now.Add(-time.Hour * 24 * 30)},
	}, now, 2)
	if err != nil || stale.Gain >= 1.01 {
		t.Fatalf("stale evidence retained too much gain: %+v", stale)
	}
}

func TestRepeatedObservationDoesNotAccumulateESS(t *testing.T) {
	now := time.Now()
	one, err := PrecisionGain([]Observation{{DedupGroup: "x", VarianceShare: 0.4, TauDays: 100, Assessed: true, ObservedAt: now}}, now, 2)
	if err != nil {
		t.Fatal(err)
	}
	many, err := PrecisionGain([]Observation{
		{DedupGroup: "x", VarianceShare: 0.4, TauDays: 100, Assessed: true, ObservedAt: now},
		{DedupGroup: "x", VarianceShare: 0.4, TauDays: 100, Assessed: true, ObservedAt: now.Add(-time.Hour)},
	}, now, 2)
	if err != nil || one.Gain != many.Gain {
		t.Fatalf("repeated observations accumulated gain: one=%+v many=%+v", one, many)
	}
	ess, err := ConditionedESS(8, one.Gain)
	if err != nil || ess <= 8 {
		t.Fatalf("conditioned ESS did not increase: %v, %v", ess, err)
	}
}
