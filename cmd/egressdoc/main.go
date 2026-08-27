// Command egressdoc regenerates docs/egress.md from the egress profiles the
// assessment checks declare.
//
// It is a separate command rather than a `go generate` comment so that CI can
// run it and diff the result: the committed document is then verifiable
// against the linked library, and an upgrade that changes what Trawl contacts
// cannot land with documentation still describing the old behaviour.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	vaudit "github.com/adedayo/vantage/pkg/audit"

	tvantage "github.com/adedayo/trawl/pkg/scanner/vantage"
)

func main() {
	out := flag.String("out", filepath.Join("docs", "egress.md"), "file to write")
	check := flag.Bool("check", false, "verify the committed file is up to date instead of writing it")
	flag.Parse()

	doc, err := render()
	if err != nil {
		fail(err)
	}

	if *check {
		existing, err := os.ReadFile(*out)
		if err != nil {
			fail(fmt.Errorf("reading %s: %w", *out, err))
		}
		if string(existing) != doc {
			fail(fmt.Errorf("%s is out of date with the checks' declared egress profiles; "+
				"run `go run ./cmd/egressdoc` and commit the result", *out))
		}
		return
	}

	if err := os.WriteFile(*out, []byte(doc), 0o644); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
}

// render builds the document for the default policy, which is what a
// deployment gets when it says nothing and therefore what the documentation
// should describe. A deployment that has widened its policy can render its own
// by calling the library function with its configuration.
func render() (string, error) {
	var caps vaudit.Capabilities
	for _, d := range vaudit.Descriptions() {
		caps.Checks = append(caps.Checks, vaudit.CheckCapability{Description: d})
	}
	return tvantage.EgressDocumentation(caps, tvantage.DefaultEgressPolicy())
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "egressdoc:", err)
	os.Exit(1)
}
