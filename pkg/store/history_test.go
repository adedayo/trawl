package store

import (
	"strings"
	"testing"
)

func TestScanFindingIDIsStableAndDiscriminating(t *testing.T) {
	a := ScanFindingID("asset", "tpl", "https://h/x")
	if a != ScanFindingID("asset", "tpl", "https://h/x") {
		t.Error("the same finding must keep the same identity across scans")
	}
	for _, other := range []string{
		ScanFindingID("asset2", "tpl", "https://h/x"),
		ScanFindingID("asset", "tpl2", "https://h/x"),
		ScanFindingID("asset", "tpl", "https://h/y"),
	} {
		if other == a {
			t.Error("a different asset, template or location must be a different finding")
		}
	}
	// Separators matter: ("ab","c") and ("a","bc") are different findings.
	if ScanFindingID("ab", "c", "") == ScanFindingID("a", "bc", "") {
		t.Error("parts are not separated in the digest")
	}
	if !strings.HasPrefix(a, "scan:") {
		t.Errorf("id %q should say what it identifies", a)
	}
}

func TestSecretFindingIDIgnoresTheLineNumber(t *testing.T) {
	a := SecretFindingID("repo", "rule", "a.env", "sum")
	if a != SecretFindingID("repo", "rule", "a.env", "sum") {
		t.Error("unstable secret identity")
	}
	if a == SecretFindingID("repo", "rule", "b.env", "sum") || a == SecretFindingID("repo", "rule", "a.env", "other") {
		t.Error("a different file or secret must be a different finding")
	}
}

func TestSecretSeverityRatesVerifiedAboveUnverified(t *testing.T) {
	if SecretSeverity(true).Rank() <= SecretSeverity(false).Rank() {
		t.Error("a secret verified as live must rate above one merely present")
	}
}
