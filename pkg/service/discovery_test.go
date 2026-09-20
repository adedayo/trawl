package service

import (
	"testing"
	"time"

	vadapter "github.com/adedayo/trawl/pkg/scanner/vantage"
	"github.com/adedayo/trawl/pkg/store"
)

// TestADiscoveredNameReachesTheInventory is the end-to-end claim of the
// phase: a name nobody entered, disclosed by a certificate log, ends up as an
// asset an operator can see and a later run can assess.
//
// It drives persist directly rather than a live assessment, because the rule
// under test is what happens to a discovery once it exists, and routing it
// through the network would make the result depend on what the logs hold
// today.
func TestADiscoveredNameReachesTheInventory(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	err := svc.persist(ctx, vadapter.Result{
		Discovered: []store.Asset{{
			ID: "vpn.example.com", Type: store.AssetTypeSubdomain, Value: "vpn.example.com",
			Status: store.AssetStatusActive, DiscoverySource: "ct-log", Confidence: 1,
			FirstSeen: time.Now(), LastSeen: time.Now(),
		}},
	})
	if err != nil {
		t.Fatalf("persist: %v", err)
	}

	assets, err := s.GetAssets(ctx, "")
	if err != nil {
		t.Fatalf("assets: %v", err)
	}

	for _, a := range assets {
		if a.Value == "vpn.example.com" {
			if a.DiscoverySource != "ct-log" {
				t.Fatalf("the provenance must survive the write, or an operator cannot tell a discovery from something they entered; got %q", a.DiscoverySource)
			}
			return
		}
	}
	t.Fatalf("a discovered name must be in the inventory; saw %d assets and not that one", len(assets))
}

// TestDiscoveryAddsAndNeverRemoves.
//
// A log falling silent is not evidence that a host was withdrawn, and a name
// this run did not disclose must keep its place. Replacing the discovered set
// each run would empty the inventory the first time the log service was
// unreachable — silently, and in the direction that looks tidy.
func TestDiscoveryAddsAndNeverRemoves(t *testing.T) {
	svc, s, ctx := suppressionFixture(t)

	first := vadapter.Result{Discovered: []store.Asset{{
		ID: "vpn.example.com", Type: store.AssetTypeSubdomain, Value: "vpn.example.com",
		Status: store.AssetStatusActive, DiscoverySource: "ct-log", Confidence: 1,
	}}}
	if err := svc.persist(ctx, first); err != nil {
		t.Fatalf("first: %v", err)
	}

	// The second run discloses nothing at all.
	if err := svc.persist(ctx, vadapter.Result{}); err != nil {
		t.Fatalf("second: %v", err)
	}

	assets, err := s.GetAssets(ctx, "")
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	for _, a := range assets {
		if a.Value == "vpn.example.com" {
			return
		}
	}
	t.Fatal("a run that disclosed nothing must not withdraw what an earlier run found")
}
