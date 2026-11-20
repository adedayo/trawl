# Capability: finding-explanation

## Purpose

Explain every finding in plain English, to a reader who is not a security
specialist: why it matters, how it could be abused, and what to do. The
explanation is curated, versioned and deterministic. It is distinct from any
AI annotation, which is advisory and subordinate.

## ADDED Requirements

### Requirement: Every finding has an executive explanation
The system SHALL render, for every finding, the sections: what we found, why it
matters, how it could be abused, what good looks like, what to do, how a re-scan
confirms the fix, and references.

#### Scenario: A catalogue finding
- **GIVEN** a finding raised under a Vantage catalogue identifier
- **WHEN** its explanation is requested
- **THEN** all sections are present, and the first section includes this
  occurrence's own detail and evidence

#### Scenario: A finding with no narrative of its own
- **GIVEN** a finding whose identifier has no narrative entry
- **WHEN** its explanation is requested
- **THEN** the narrative for its weakness class, service class, secret type or
  CVE template is used, and the view states which level supplied it

### Requirement: Narrative coverage is enforced
The system SHALL fail its build when any catalogue identifier, registry weakness
class or service class has neither a narrative nor a fallback.

#### Scenario: A library upgrade adds an identifier
- **GIVEN** a Vantage release adding a catalogue identifier
- **WHEN** the build runs without a corresponding narrative or fallback
- **THEN** the build fails and names the identifier

### Requirement: The narrative asserts nothing the engine cannot support
The system SHALL NOT include in narrative text any monetary value or
probability of compromise, and SHALL NOT describe a control in a non-assessed
state as secure, working or fine.

#### Scenario: A narrative contains a figure
- **WHEN** a narrative entry states a currency amount or a probability
- **THEN** the lint test fails

#### Scenario: A control was not assessed
- **GIVEN** a control in `not_checked` or `check_failed`
- **WHEN** its explanation is rendered
- **THEN** it says the control could not be assessed, and does not imply it is
  in place

### Requirement: Exploitation status is stated in words with its date
The system SHALL express KEV and EPSS enrichment as plain language accompanied
by the date the feed was checked, and SHALL state when the enrichment was not
checked or failed.

#### Scenario: A listed CVE
- **GIVEN** a finding whose CVE appears in the exploited-vulnerability catalogue
- **WHEN** its explanation is rendered
- **THEN** it states that the weakness is known to be used in real attacks and
  the date that was checked

#### Scenario: Enrichment unavailable
- **GIVEN** a finding whose feed check failed
- **WHEN** its explanation is rendered
- **THEN** it says exploitation status could not be established, rather than
  implying it is not exploited

### Requirement: Explanation text is generated at view time
The system SHALL NOT persist narrative prose against a finding. It SHALL record
the narrative version a view was rendered from.

#### Scenario: A narrative is corrected
- **GIVEN** an existing finding
- **WHEN** a corrected narrative is released and the finding is viewed
- **THEN** the corrected wording is shown, labelled with the new version

### Requirement: AI annotation never replaces the explanation
The system SHALL present any AI annotation separately from, and subordinate to,
the explanation, and the explanation SHALL be complete when no AI provider is
configured.

#### Scenario: No provider configured
- **WHEN** a finding is viewed
- **THEN** the full explanation is shown and no degraded state is displayed
