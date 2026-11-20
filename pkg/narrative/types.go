// Package narrative renders the plain-English explanation of a finding: what
// was found, why it matters, how it could be abused, what good looks like and
// what to do about it.
//
// The text is curated, versioned and deterministic. It is distinct from any AI
// annotation (Change 021), which is advisory and subordinate; this is the
// baseline and is complete when no provider is configured. It is rendered at
// view time and never persisted against a finding, so a corrected narrative
// reaches existing findings without a migration.
//
// Rules the text must keep (enforced by tests in this package):
//   - no currency amount and no probability of compromise (Change 012 boundary)
//   - no organisation-specific names
//   - a control that was not assessed is never described as secure or fine
package narrative

import "github.com/adedayo/trawl/pkg/store"

// Levels at which an explanation can be supplied, most specific first.
const (
	LevelSignal        = "signal"
	LevelWeaknessClass = "weakness-class"
	LevelCheck         = "check"
	LevelServiceClass  = "service-class"
	LevelSecretType    = "secret-type"
	LevelCandidate     = "candidate"
	LevelCVE           = "cve"
	LevelGeneric       = "generic"
)

// Catalogue is the library's technical text for a finding identifier, passed in
// so that this package does not depend on the library that raises findings.
type Catalogue struct {
	Title       string
	Description string
	Remediation string
	References  []string
	Tags        []string
}

// Input is everything the renderer may use. Fields that do not apply to a
// source are left empty.
type Input struct {
	// Source is "signal", "finding" or "secret".
	Source string

	// Signal findings.
	SignalID      string
	WeaknessClass string
	Check         string // vantage check family, e.g. "spf", "dmarc"
	Scenario      string

	// Service-exposure findings.
	ServiceClass string // e.g. "rdp", "database"

	// Secret findings.
	SecretType string

	// CVE findings.
	CVE string

	Title    string
	Severity store.FindingSeverity

	// Detail is the part of the description specific to this occurrence, and
	// Evidence the measured value. Both are interpolated into the "what we
	// found" section and nowhere else.
	Detail   string
	Evidence string

	// CheckState and CheckReason describe the assessment behind the finding.
	// A state that is not assessed (not_checked, check_failed) must be stated
	// in the output, never implied away.
	CheckState  store.CoverageState
	CheckReason string

	// Exploitation enrichment, expressed in words by the renderer.
	// KEVState / EPSSState are store.EnrichmentState values ("ok", "not_found",
	// "not_checked", "check_failed", or "" when the finding has no CVE).
	KEVState      string
	KEVListed     bool
	KEVCheckedAt  string // RFC3339 or ""
	EPSSState     string
	EPSSHigh      bool // EPSS at or above the "elevated" floor used by priority
	EPSSCheckedAt string

	Catalogue *Catalogue
}

// Explanation is the rendered, sectioned explanation of one finding.
type Explanation struct {
	// Level and LevelKey say which narrative supplied the wording, so the view
	// can state it when it is not specific to this finding.
	Level    string `json:"level"`
	LevelKey string `json:"levelKey"`
	// Version identifies the narrative set the wording came from.
	Version string `json:"version"`

	WhatWeFound        string   `json:"whatWeFound"`
	WhyItMatters       string   `json:"whyItMatters"`
	HowItCouldBeAbused string   `json:"howItCouldBeAbused"`
	WhatGoodLooksLike  string   `json:"whatGoodLooksLike"`
	WhatToDo           []string `json:"whatToDo"`
	HowRescanConfirms  string   `json:"howRescanConfirms"`
	References         []string `json:"references"`
	// Standards are compliance tags carried by the catalogue entry.
	Standards []string `json:"standards,omitempty"`

	// ExploitationStatus is KEV/EPSS in plain words with the date checked, or a
	// statement that it could not be established. Empty for findings with no
	// CVE.
	ExploitationStatus string `json:"exploitationStatus,omitempty"`
	// CoverageNote is set when the assessment behind the finding did not
	// conclude; it says the control could not be assessed.
	CoverageNote string `json:"coverageNote,omitempty"`
}

// Explainer renders an explanation. The registry in this package implements it;
// callers depend on the interface.
type Explainer interface {
	Explain(Input) Explanation
}
