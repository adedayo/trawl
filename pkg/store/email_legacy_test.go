package store_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adedayo/trawl/pkg/store"
)

// A posture written before Change 006 Phase 9 widened the record carries no
// control states, because the old booleans were deliberately not promoted.
// Seven unassessed controls and an empty view are indistinguishable from a
// clean result to a hurried reader, so the record has to be able to say which
// it is.
func TestAPostureFromBeforeTheWideningSaysSo(t *testing.T) {
	legacy := store.EmailPosture{
		Domain:      "example.test",
		LastChecked: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if !legacy.PredatesAssessment() {
		t.Error("a checked posture with no assessed control should report that it predates the assessment")
	}
	if assessed, total := legacy.Assessed(); assessed != 0 || total != 7 {
		t.Errorf("assessed, total = %d, %d; want 0, 7", assessed, total)
	}
}

// A domain nobody has scanned yet is a different thing, and must not carry the
// same explanation. Its controls are unassessed because no check has run, not
// because an upgrade could not carry the answers forward.
func TestANeverCheckedPostureIsNotReportedAsLegacy(t *testing.T) {
	fresh := store.EmailPosture{Domain: "example.test"}

	if fresh.PredatesAssessment() {
		t.Error("a posture that was never checked should not be reported as predating the assessment")
	}
}

// A run that reached a conclusion about anything at all — including that a
// check failed — was performed by the current engine.
func TestAPostureWithAnyConclusionIsNotLegacy(t *testing.T) {
	assessed := store.EmailPosture{
		Domain:      "example.test",
		LastChecked: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		SPF:         store.EmailControl{State: store.CoverageCheckFailed},
	}

	if assessed.PredatesAssessment() {
		t.Error("a posture carrying a check_failed state was written by the current engine")
	}
}

// The flag has to reach both transports, so a view need not re-derive it and
// the two halves of the product cannot disagree about which domains are stale.
func TestThePredatesFlagIsSerialised(t *testing.T) {
	legacy := store.EmailPosture{
		Domain:      "example.test",
		LastChecked: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"predatesAssessment":true`) {
		t.Errorf("encoded posture does not carry the flag: %s", encoded)
	}

	current := store.EmailPosture{
		Domain:      "example.test",
		LastChecked: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DMARC:       store.EmailControl{State: store.CoverageOK},
	}
	encoded, err = json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "predatesAssessment") {
		t.Errorf("a current posture should omit the flag entirely: %s", encoded)
	}
}
