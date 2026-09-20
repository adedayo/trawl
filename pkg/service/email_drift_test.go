package service

import (
	"context"
	"testing"

	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/store"
)

// persistPosture runs one assessment carrying the given DMARC posture.
func persistPosture(t *testing.T, svc *AssessmentService, ctx context.Context, dmarc store.EmailControl, policy string, percent int) {
	t.Helper()

	res := vadapter.Result{
		Coverage: []store.AssessmentCoverage{{
			AssetID: "asset-1", CheckID: "dmarc", State: dmarc.State,
		}},
		EmailPosture: &store.EmailPosture{
			Domain:       "example.com",
			DMARC:        dmarc,
			DMARCPolicy:  policy,
			DMARCPercent: percent,
		},
	}
	if err := svc.persist(ctx, res); err != nil {
		t.Fatalf("persist: %v", err)
	}
}

// TestAWeakenedDMARCPolicyIsRaisedAsATransition is the capability's drift
// requirement: a policy that was p=reject and is now p=none has lost a
// control, and the history must show the transition rather than silently
// overwriting the prior state.
func TestAWeakenedDMARCPolicyIsRaisedAsATransition(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	enforced := store.EmailControl{State: store.CoverageOK, Conclusive: true}
	persistPosture(t, svc, ctx, enforced, "reject", 100)
	persistPosture(t, svc, ctx, enforced, "none", 100)

	regressions, err := s.GetRegressions(ctx)
	if err != nil {
		t.Fatalf("GetRegressions: %v", err)
	}
	if len(regressions) != 1 {
		t.Fatalf("a policy weakening from reject to none is a lost control and must be raised; got %d regressions", len(regressions))
	}
	if regressions[0].AttributeType != store.DMARCPolicyAttribute {
		t.Fatalf("raised under the wrong attribute: %q", regressions[0].AttributeType)
	}
	if regressions[0].PreviousValue == regressions[0].CurrentValue {
		t.Fatal("the history must show the transition, not just the current state")
	}

	posture, err := s.GetEmailPostures(ctx)
	if err != nil {
		t.Fatalf("postures: %v", err)
	}
	if posture[0].DMARCPolicy != "none" {
		t.Fatalf("current policy = %q, want the weakened one", posture[0].DMARCPolicy)
	}
	if posture[0].Priority != store.SeverityHigh {
		t.Fatalf("priority = %q: p=none with no reporting address observes nothing and blocks nothing", posture[0].Priority)
	}
}

// TestAnUnassessedRunDoesNotMoveTheBaseline is the failure mode the drift
// tracking is most likely to introduce.
//
// If a failed run wrote its fingerprint, the baseline would move to "we could
// not tell", and the next successful run would report a change from nothing to
// the policy that had been there all along — a regression manufactured by our
// own outage, arriving in the operator's list beside the real ones and
// indistinguishable from them.
func TestAnUnassessedRunDoesNotMoveTheBaseline(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	enforced := store.EmailControl{State: store.CoverageOK, Conclusive: true}
	persistPosture(t, svc, ctx, enforced, "reject", 100)

	// The resolver failed. Nothing about the policy was established.
	persistPosture(t, svc, ctx, store.EmailControl{State: store.CoverageCheckFailed}, "", 100)

	// The policy is unchanged and assessable again.
	persistPosture(t, svc, ctx, enforced, "reject", 100)

	regressions, err := s.GetRegressions(ctx)
	if err != nil {
		t.Fatalf("GetRegressions: %v", err)
	}
	if len(regressions) != 0 {
		t.Fatalf("an unchanged policy must raise nothing across a failed run; got %d regressions manufactured by our own outage", len(regressions))
	}
}

// TestTheFingerprintIgnoresCosmeticEdits. A drift list that fires when a
// reporting address is edited is one an operator stops reading — and the
// genuine weakening is then missed along with the noise.
func TestTheFingerprintIgnoresCosmeticEdits(t *testing.T) {
	assessed := store.EmailControl{State: store.CoverageOK, Conclusive: true}

	first := store.EmailPosture{
		DMARC: assessed, DMARCPolicy: "reject", DMARCPercent: 100,
		DMARCReporting: true, DMARCAlignmentSPF: "s",
	}
	second := store.EmailPosture{
		DMARC: assessed, DMARCPolicy: "reject", DMARCPercent: 100,
		DMARCReporting: false, DMARCAlignmentSPF: "r",
	}

	if first.DMARCFingerprint() != second.DMARCFingerprint() {
		t.Fatalf("fingerprints differ (%q vs %q): only what receivers act on should count as drift",
			first.DMARCFingerprint(), second.DMARCFingerprint())
	}

	weakened := store.EmailPosture{DMARC: assessed, DMARCPolicy: "none", DMARCPercent: 100}
	if weakened.DMARCFingerprint() == first.DMARCFingerprint() {
		t.Fatal("a weakened policy must not fingerprint the same as an enforced one")
	}
}
