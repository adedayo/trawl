# Capability: email-posture-surface

## Purpose

Render a domain's email-authentication posture so that an operator can tell a
misconfiguration from an outage, a measured absence from an unmeasured one, and
a derived severity from the evidence that produced it. Presentation only: this
capability displays what `vantage-integration` observed and never re-derives,
upgrades or supplements it.

## ADDED Requirements

### Requirement: Every assessed control is rendered in its own state
The system SHALL render each of the seven email-authentication controls — SPF,
DKIM, DMARC, MTA-STS, TLS-RPT, BIMI and CAA — in one of four distinct states,
and SHALL NOT render any control as a binary pass or fail.

#### Scenario: An unchecked control is not drawn as passing
- **GIVEN** a posture in which MTA-STS has state `not_checked`
- **WHEN** the domain is rendered
- **THEN** MTA-STS is shown as unassessed, visually distinct from both a
  published policy and an absent one

#### Scenario: A failed check is distinguished from an absent record
- **GIVEN** two domains, one whose DMARC control is `not_found` and one whose
  DMARC control is `check_failed`
- **WHEN** both are rendered
- **THEN** they are visually distinguishable without opening either row

#### Scenario: No control is omitted
- **GIVEN** a posture in which four controls were never assessed
- **WHEN** the domain is rendered
- **THEN** all seven controls appear, so that a domain with three assessed
  controls does not resemble a domain with seven at a glance

### Requirement: An inconclusive state states why
The system SHALL display the recorded reason for any control whose state is
`not_checked` or `check_failed`.

#### Scenario: Refusal is attributed
- **GIVEN** a control that was not checked because its egress class is excluded
  by policy
- **WHEN** the control is rendered
- **THEN** the displayed reason names the policy that excluded it

#### Scenario: A missing reason is not fabricated
- **GIVEN** a control that is `check_failed` with no recorded reason
- **WHEN** the control is rendered
- **THEN** it is shown as unexplained, and no reason is inferred from the state

### Requirement: An inconclusive DKIM result is never reported as absence
The system SHALL NOT describe DKIM as absent, missing or not configured when
the DKIM control reports `conclusive: false`, and SHALL disclose the selectors
that were examined.

#### Scenario: Probed absence is reported as inconclusive
- **GIVEN** a DKIM control with state `not_found` and `conclusive: false`
- **WHEN** it is rendered
- **THEN** it reports that the selectors examined did not yield a key, and does
  not state that the domain lacks DKIM

#### Scenario: Named selectors give a conclusive answer
- **GIVEN** a DKIM control with state `not_found` and `conclusive: true`,
  following an assessment against operator-supplied selectors
- **WHEN** it is rendered
- **THEN** it reports the absence as a finding about the domain

#### Scenario: The search is legible
- **GIVEN** a DKIM control reporting one selector found of thirteen examined
- **WHEN** it is rendered
- **THEN** both figures are available to the reader

### Requirement: Coverage accompanies the posture
The system SHALL display, for each domain, how many of its controls were
actually assessed, and SHALL compute that figure from a single definition
shared with the engine.

#### Scenario: Coverage is shown alongside the posture
- **GIVEN** a domain with three assessed controls and four unassessed
- **WHEN** the domain is rendered
- **THEN** the assessed count is displayed with the posture, not behind an
  interaction

#### Scenario: One definition of assessed
- **GIVEN** the engine's definition of an assessed control
- **WHEN** the rendered coverage figure is computed
- **THEN** a test fails if the two definitions disagree for any state

### Requirement: Derived severity is displayed with its evidence
The system SHALL render the severity computed by the engine verbatim, SHALL NOT
derive or adjust a severity, and SHALL display the DMARC policy tags from which
the severity was derived.

#### Scenario: Severity is not recomputed in the client
- **GIVEN** a posture carrying a computed priority
- **WHEN** it is rendered
- **THEN** the displayed severity equals the stored one for every posture

#### Scenario: Partial enforcement reads as partial
- **GIVEN** a domain publishing `p=reject; pct=40`
- **WHEN** its DMARC control is rendered
- **THEN** the displayed policy discloses that it applies to part of the mail,
  and the domain is not presented as enforcing

#### Scenario: An unassessed control carries no severity
- **GIVEN** a posture whose DMARC control was never assessed and whose priority
  is therefore empty
- **WHEN** the domain is rendered
- **THEN** no severity is displayed, and the domain is not sorted or counted as
  though it held one

#### Scenario: Evidence supports the rating
- **GIVEN** a domain rated on its DMARC posture
- **WHEN** the rating is inspected
- **THEN** the policy, percentage, subdomain policy, alignment modes and
  reporting status that produced it are available without a further request

### Requirement: SPF evidence discloses what weakens it
The system SHALL display the SPF all-mechanism qualifier and the DNS lookup
count where SPF was assessed.

#### Scenario: A permissive record is legible
- **GIVEN** a domain whose SPF record terminates in `+all`
- **WHEN** it is rendered
- **THEN** the qualifier is disclosed, because it authorises the entire
  internet to send as the domain

#### Scenario: An unenforceable record is disclosed
- **GIVEN** an SPF record expanding to more than ten DNS-querying mechanisms
- **WHEN** it is rendered
- **THEN** the lookup count is shown, because receivers may stop evaluating the
  record and a careful policy may be enforced nowhere

### Requirement: A weakened DMARC policy is surfaced to the operator
The system SHALL surface a recorded change in a domain's DMARC policy.

#### Scenario: A weakening is visible
- **GIVEN** a domain whose DMARC policy moved from `reject` to `none` and was
  recorded as posture drift
- **WHEN** the domain is rendered
- **THEN** the change is surfaced to the operator

#### Scenario: A failed assessment raises no drift
- **GIVEN** a domain whose most recent assessment could not reach the resolver
- **WHEN** the domain is rendered
- **THEN** no policy change is reported, because no policy was observed

### Requirement: The posture view has one account of a domain
The system SHALL NOT present the assessment-derived control view and the
persisted posture record as independent accounts of the same control on the
same domain.

#### Scenario: No contradictory pair
- **GIVEN** a domain for which both an assessment result and a posture record
  exist
- **WHEN** the domain is rendered
- **THEN** each control appears once, and the reader is not asked to reconcile
  two states for it
