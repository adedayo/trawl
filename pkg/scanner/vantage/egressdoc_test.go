package vantage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vaudit "github.com/adedayo/vantage/pkg/audit"
)

// TestEgressDocumentationIsUpToDate is the drift guard on the generated
// operator documentation.
//
// The document tells an operator what this tool will contact. If it can fall
// behind the code, it is not documentation but a claim — and a stale claim
// about network behaviour is worse than none, because someone has read it and
// stopped worrying. Regenerating and diffing here means a vantage upgrade that
// adds a check fails this test rather than silently invalidating the file.
func TestEgressDocumentationIsUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "egress.md")

	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	generated, err := EgressDocumentation(realCapabilities(t), DefaultEgressPolicy())
	if err != nil {
		t.Fatalf("EgressDocumentation: %v", err)
	}

	if string(committed) != generated {
		t.Error("docs/egress.md no longer matches the checks' declared egress profiles.\n" +
			"Run `go run ./cmd/egressdoc` and commit the result.")
	}
}

// TestEgressDocumentationNamesWithheldChecksAndTheirReasons asserts the
// document reports what the deployment declines, not only what it does.
//
// A document listing only the checks that run reads as a complete account of
// the tool's behaviour while omitting the half an operator most needs: the
// assessment they are not getting, and what they would have to agree to in
// order to get it.
func TestEgressDocumentationNamesWithheldChecksAndTheirReasons(t *testing.T) {
	caps := capsOf(
		check("spf", vaudit.EgressProfile{Resolver: true}),
		check("axfr", vaudit.EgressProfile{TargetNameservers: true, Intrusive: true}),
		check("ct", vaudit.EgressProfile{
			Resolver:   true,
			ThirdParty: []vaudit.ThirdPartyService{vaudit.ServiceCertSpotter},
		}),
	)

	doc, err := EgressDocumentation(caps, DefaultEgressPolicy())
	if err != nil {
		t.Fatalf("EgressDocumentation: %v", err)
	}

	for _, want := range []string{
		"Checks withheld",
		"axfr",
		"ct",
		string(ClassIntrusive),
		string(vaudit.ServiceCertSpotter),
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("documentation does not mention %q", want)
		}
	}

	// The transport allowlist section must be honest about permitting nothing
	// beyond the operator's own domains, rather than omitting the section and
	// leaving the reader to infer it.
	if !strings.Contains(doc, "Only names within your authorised domains are contacted") {
		t.Error("documentation does not state that no third-party host is reachable")
	}
}

// TestEgressDocumentationRefusesAnInvalidPolicy asserts documentation is never
// produced for a policy that could not be enforced, since it would describe a
// deployment that cannot exist.
func TestEgressDocumentationRefusesAnInvalidPolicy(t *testing.T) {
	_, err := EgressDocumentation(realCapabilities(t), EgressPolicy{
		AllowedClasses: []EgressClass{"smoke-signal"},
	})
	if err == nil {
		t.Fatal("documentation was generated for an unenforceable policy")
	}
}
