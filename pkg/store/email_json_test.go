package store

import (
	"encoding/json"
	"testing"
)

// The coverage figure a view renders must be the engine's figure. It is
// carried on the wire for that reason, and these tests exist so that a change
// to Assessed cannot quietly stop travelling with the record.
func TestEmailPostureMarshalsItsCoverage(t *testing.T) {
	p := EmailPosture{
		Domain: "example.org",
		SPF:    EmailControl{State: CoverageOK},
		DKIM:   EmailControl{State: CoverageNotFound},
		DMARC:  EmailControl{State: CoverageCheckFailed, Reason: "resolver timed out"},
		MTASTS: EmailControl{State: CoverageNotChecked, Reason: "excluded by egress policy"},
		TLSRPT: EmailControl{State: CoverageNotChecked},
		BIMI:   EmailControl{State: CoverageNotChecked},
		CAA:    EmailControl{State: CoverageOK},
	}

	var got struct {
		AssessedControls int `json:"assessedControls"`
		TotalControls    int `json:"totalControls"`
	}
	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshalling the posture: %v", err)
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("reading the marshalled posture: %v", err)
	}

	assessed, total := p.Assessed()
	if got.AssessedControls != assessed || got.TotalControls != total {
		t.Fatalf("serialised coverage %d/%d disagrees with Assessed() %d/%d",
			got.AssessedControls, got.TotalControls, assessed, total)
	}
	if total != 7 {
		t.Fatalf("expected seven controls, got %d", total)
	}
}

// Every state must be exercised, because the definition the UI mirrors is
// "which states count as assessed" and a definition agreed on three of four
// states is not agreed at all.
func TestEmailPostureCoverageAcrossEveryState(t *testing.T) {
	states := []CoverageState{CoverageOK, CoverageNotFound, CoverageNotChecked, CoverageCheckFailed}
	for _, s := range states {
		p := EmailPosture{SPF: EmailControl{State: s}}
		payload, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("marshalling a %s posture: %v", s, err)
		}
		var got struct {
			AssessedControls int `json:"assessedControls"`
		}
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatalf("reading a %s posture: %v", s, err)
		}
		want := 0
		if s.Assessed() {
			want = 1
		}
		if got.AssessedControls != want {
			t.Fatalf("state %s: serialised %d assessed, Assessed() says %d", s, got.AssessedControls, want)
		}
	}
}

// The added fields are additive: nothing the transports already served may be
// dropped by the custom marshaller.
func TestEmailPostureMarshalKeepsItsFields(t *testing.T) {
	p := EmailPosture{
		Domain:                "example.org",
		DMARC:                 EmailControl{State: CoverageOK, Detail: "v=DMARC1; p=reject; pct=40"},
		DMARCPolicy:           "reject",
		DMARCPercent:          40,
		SPFAllMechanism:       "-",
		SPFLookups:            12,
		DKIMSelectorsExamined: []string{"selector1", "google"},
		Priority:              SeverityMedium,
	}

	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshalling the posture: %v", err)
	}
	var round EmailPosture
	if err := json.Unmarshal(payload, &round); err != nil {
		t.Fatalf("reading the marshalled posture: %v", err)
	}

	if round.Domain != p.Domain || round.DMARCPolicy != p.DMARCPolicy ||
		round.DMARCPercent != p.DMARCPercent || round.SPFLookups != p.SPFLookups ||
		round.SPFAllMechanism != p.SPFAllMechanism || round.Priority != p.Priority ||
		round.DMARC.Detail != p.DMARC.Detail || len(round.DKIMSelectorsExamined) != 2 {
		t.Fatalf("marshalling lost a field: %+v", round)
	}
}
