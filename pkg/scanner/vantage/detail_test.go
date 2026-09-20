package vantage

import (
	"strings"
	"testing"

	vfinding "github.com/adedayo/vantage/pkg/finding"
)

// The catalogue explains an identifier; the appended sentence explains this
// occurrence of it. Only the second belongs on the stored row — the first is
// library text that a vantage upgrade should be free to reword, and that is
// read from the installed library when the view is built.

func TestDetailKeepsOnlyTheOccurrenceSpecificSentence(t *testing.T) {
	entry, ok := vfinding.Lookup("SURF-SPF-009")
	if !ok {
		t.Fatal("SURF-SPF-009 is missing from the vantage catalogue")
	}

	f := vfinding.New("SURF-SPF-009", "example.com").
		WithDescription("Specifically, the term `include:dead.example` names " +
			"`dead.example`, which does not resolve or publishes no SPF record.")

	got := detail(f)

	if got == "" {
		t.Fatal("expected the occurrence-specific sentence to be extracted")
	}
	if want := "dead.example"; !strings.Contains(got, want) {
		t.Errorf("detail must name the offending term, got %q", got)
	}
	// The catalogue half must not be copied onto the row.
	if strings.Contains(got, entry.Description) {
		t.Errorf("detail duplicates the catalogue description: %q", got)
	}
}

// A finding that says nothing beyond its catalogue entry has no detail. Storing
// the catalogue text in that case would put library prose in the database and
// make the UI render the same paragraph twice.
func TestDetailIsEmptyWhenNothingWasAppended(t *testing.T) {
	f := vfinding.New("SURF-SPF-009", "example.com")

	if got := detail(f); got != "" {
		t.Errorf("expected no detail for an unembellished finding, got %q", got)
	}
}

// A severity adjustment is also specific to the occurrence: it explains why
// this instance was judged differently from the catalogue default, which the
// reader cannot infer from the catalogue.
func TestDetailCapturesSeverityAdjustments(t *testing.T) {
	f := vfinding.New("SURF-SPF-001", "example.com").
		WithSeverity(vfinding.SeverityLow,
			"Severity reduced because the domain publishes no MX records.")

	got := detail(f)

	if !strings.Contains(got, "Severity reduced") {
		t.Errorf("expected the severity rationale to be retained, got %q", got)
	}
}

// If a future vantage reworded a catalogue description, the prefix would stop
// matching. Losing the specifics is the one outcome that must not happen, so
// the whole description is kept instead.
func TestDetailFallsBackToTheWholeDescription(t *testing.T) {
	f := vfinding.New("SURF-SPF-009", "example.com")
	f.Description = "Some wording this build's catalogue does not start with."

	if got := detail(f); got != f.Description {
		t.Errorf("expected the full description as a fallback, got %q", got)
	}
}
