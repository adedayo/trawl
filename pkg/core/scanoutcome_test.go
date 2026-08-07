package core_test

import (
	"errors"
	"testing"

	"github.com/adedayo/trawl/pkg/core"
)

// A scan that failed halfway must be distinguishable from one that succeeded.
//
// Both previously emitted an identical completion event, with the error
// swallowed into a log line the desktop operator has no terminal to read. The
// UI could only report that scanning had stopped, which reads as success — and
// a partial assessment presented as a complete one invites the conclusion that
// there is no exposure when the truth is that nobody finished looking.
func TestScanOutcomeDistinguishesFailureFromSuccess(t *testing.T) {
	req := core.ScanRequest{Domain: "example.com"}

	clean := core.NewScanOutcome(req, nil)
	if clean.Status != core.ScanStatusCompleted {
		t.Errorf("Expected a clean scan to report %q, got %q",
			core.ScanStatusCompleted, clean.Status)
	}
	if clean.Error != "" {
		t.Errorf("A clean scan must carry no error, got %q", clean.Error)
	}

	failed := core.NewScanOutcome(req, errors.New("resolution failed"))
	if failed.Status != core.ScanStatusPartial {
		t.Errorf("Expected a failed scan to report %q, got %q",
			core.ScanStatusPartial, failed.Status)
	}
	if failed.Error != "resolution failed" {
		t.Errorf("The reason must survive to the caller, got %q", failed.Error)
	}
}

// The event must name what it is about. A completion carrying no target leaves
// a portfolio scan unable to say which domain came back incomplete, which
// defeats the point of reporting the outcome at all.
func TestScanOutcomeNamesItsTarget(t *testing.T) {
	domain := core.NewScanOutcome(core.ScanRequest{Domain: "example.com"}, nil)
	if domain.Domain != "example.com" {
		t.Errorf("Expected the domain to be carried, got %q", domain.Domain)
	}

	repo := core.NewScanOutcome(core.ScanRequest{RepoURL: "https://example.com/r"}, nil)
	if repo.RepoURL != "https://example.com/r" {
		t.Errorf("Expected the repository to be carried, got %q", repo.RepoURL)
	}
}

// Subscribers tell a completion from a per-check progress event by the phase,
// rather than by guessing from which other fields are populated.
func TestScanOutcomeIsMarkedAsACompletion(t *testing.T) {
	if got := core.NewScanOutcome(core.ScanRequest{Domain: "example.com"}, nil).Phase; got != "complete" {
		t.Errorf("Expected phase %q, got %q", "complete", got)
	}
}
