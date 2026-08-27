# Capability: dashboard

## MODIFIED Requirements

### Requirement: Live-updating views
The dashboard SHALL display live-updating asset and finding lists via the
application's real-time transport, with no manual refresh required. The
transport SHALL be Wails IPC in the desktop build and Server-Sent Events at
`GET /api/v1/events` in the server build; view code SHALL depend on the
transport abstraction rather than on either mechanism directly.

*(Modified from Change 001, which specified a Convex live query. Convex was
removed from the stack in Change 005 Phase 5. The requirement itself is
unchanged in substance — only the mechanism named.)*

#### Scenario: New finding appears live
- **GIVEN** a new finding is ingested
- **WHEN** the operator has the dashboard open
- **THEN** it appears in the finding list without a page reload

#### Scenario: The same view code serves both builds
- **GIVEN** a dashboard view that subscribes to live updates
- **WHEN** it is built for desktop and for server
- **THEN** it consumes the same transport interface in both, and no view
  imports a Wails or SSE symbol directly

#### Scenario: A dropped stream is visible, not silent
- **GIVEN** the live connection is lost
- **WHEN** the operator is looking at the dashboard
- **THEN** the interface states that it is no longer live and when it last
  updated, rather than continuing to display stale data as though current

### Requirement: Loading, empty, and error states are designed, not blank
Every view that depends on live or asynchronous data SHALL render an explicit
loading state (skeleton, not a blank screen), an explicit empty state with
guidance text when there is genuinely no data yet, and an explicit error state
distinct from either.

The empty state SHALL distinguish **nothing found** from **nothing looked at**.
These are different statements about the estate and SHALL NOT share a
rendering.

*(Modified from Change 001 to remove the Convex reference and to add the
second paragraph, which the four-state coverage model from Change 006 makes
both possible and necessary.)*

#### Scenario: Zero-findings first run shows guidance, not a blank table
- **GIVEN** a freshly-deployed instance with no scans run yet
- **WHEN** the operator opens the findings view
- **THEN** they see an empty state explaining that no scan has completed yet
  and when the next one is scheduled, not an empty table with no explanation

#### Scenario: Assessed-and-clean is distinguishable from never-assessed
- **GIVEN** one domain assessed with every check returning `ok`, and another
  with no assessment run recorded
- **WHEN** both are viewed
- **THEN** the first reads as assessed and clean, the second reads as not yet
  assessed, and neither can be mistaken for the other

## ADDED Requirements

### Requirement: Executive area over the same rows
The dashboard SHALL provide an executive area presenting estate-level posture
for a CISO audience. Every figure it displays SHALL be read from the same
stored rows the detail views read, and SHALL NOT be independently recomputed.

#### Scenario: Executive figure matches its detail view
- **GIVEN** a count or state shown in the executive area
- **WHEN** the operator drills into the corresponding detail view
- **THEN** the figures agree exactly, because both read one stored value

#### Scenario: Every executive figure is traversable
- **GIVEN** any figure in the executive area
- **WHEN** the operator selects it
- **THEN** the interface navigates to the filtered rows that produced it, in a
  bounded number of steps

### Requirement: Coverage travels with every posture claim
Every aggregate figure the dashboard presents SHALL be accompanied by the
assessment coverage of its inputs — checks concluded divided by checks
applicable — rendered inline rather than as a separate panel a reader may not
consult.

A view SHALL NOT render a `not_checked` or `check_failed` outcome in the
styling used for a passing one.

#### Scenario: Coverage is inline, not adjacent
- **GIVEN** an estate-level posture figure
- **WHEN** it is rendered
- **THEN** its coverage appears within the same visual element, such that the
  figure cannot be read or screenshotted without it

#### Scenario: An unassessed control is not styled as a healthy one
- **GIVEN** a domain whose DMARC check returned `check_failed`
- **WHEN** its email posture is displayed
- **THEN** it is visually distinct from a domain whose DMARC check returned
  `ok`, and is not counted among the compliant

### Requirement: The headline figure is refused below a coverage floor
The executive area SHALL present a single headline posture figure. When the
coverage of its inputs falls below a configured floor, the interface SHALL
refuse to render the figure and SHALL instead state what proportion of the
estate remains unassessed and what would raise it.

A caveat beside the number is not sufficient. Below the floor, the number is
not displayed at all.

#### Scenario: Low coverage suppresses the headline figure
- **GIVEN** an estate where assessment coverage is below the configured floor
- **WHEN** the executive area renders
- **THEN** no headline posture figure is shown, and the display names the
  unassessed proportion and the reason assessment did not conclude

#### Scenario: The floor is configuration, not a constant
- **GIVEN** a deployment that has set its own coverage floor
- **WHEN** the executive area renders
- **THEN** the configured floor governs, and the value in force is stated in
  the interface

### Requirement: Withheld assessment is reported, never omitted
Checks excluded by the deployment's egress policy SHALL be presented as
withheld, naming the excluding class or service and what permitting it would
add. They SHALL NOT be omitted from the interface.

#### Scenario: A policy exclusion is visible to the operator
- **GIVEN** a check excluded because the deployment has not consented to a
  third-party service it requires
- **WHEN** the asset's assessment detail is viewed
- **THEN** the check is listed as withheld, the service is named, and the
  operator can see that consenting would enable it

#### Scenario: Withheld checks do not inflate coverage
- **GIVEN** an asset with checks withheld by policy
- **WHEN** its coverage is computed and displayed
- **THEN** the withheld checks count as not concluded, so a restrictive policy
  lowers reported coverage rather than raising apparent health

### Requirement: Regression surface answers "what got worse"
The dashboard SHALL present posture regressions — a tracked attribute that
degraded between checks — as a first-class surface, showing the previous
value, the current value, when it was confirmed, and the affected asset.

Regressions SHALL be ordered by confirmation recency and SHALL be presented
separately from new findings, because "this became worse" and "we found this"
call for different responses.

#### Scenario: A weakened DMARC policy appears as a regression
- **GIVEN** a domain whose DMARC policy moved from `reject` to `none`
- **WHEN** the change is confirmed
- **THEN** it appears in the regression surface with both values and the
  confirmation time, and not merely as a new finding

#### Scenario: Attribution refresh does not present as estate change
- **GIVEN** a provider attribution that changed because provider data
  refreshed rather than because the estate moved
- **WHEN** the regression surface renders
- **THEN** the change is not presented as a posture regression

### Requirement: Priority order is presented as an ordering, not a queue length
The executive area SHALL present what to do first as an explicit ordering of
findings, showing for each the deterministic inputs to its position — exploited-in-the-wild status, exploitation likelihood, and asset exposure.

A count of open findings SHALL NOT be presented as the primary actionable
figure.

#### Scenario: The ordering exposes its inputs
- **GIVEN** the top-ranked finding in the executive area
- **WHEN** the operator inspects why it ranks there
- **THEN** the KEV status, EPSS score and exposure that determined its position
  are shown

#### Scenario: Ranking is reproducible across readers
- **GIVEN** the same underlying data
- **WHEN** two operators view the ordering
- **THEN** they see the same order, because it derives from stored
  deterministic values rather than from anything computed per session

### Requirement: AI narrative is visually subordinate and never load-bearing
Where AI-generated explanation is displayed, it SHALL be visually distinct
from measured values and labelled as advisory. The interface SHALL remain
complete and actionable when AI annotation is absent or disabled.

#### Scenario: AI text cannot be mistaken for measurement
- **GIVEN** a finding with an AI annotation
- **WHEN** its detail is displayed
- **THEN** the annotation is marked as AI-generated and rendered distinctly
  from the deterministic severity, KEV and EPSS fields

#### Scenario: The dashboard is complete with AI switched off
- **GIVEN** a deployment with no AI provider configured
- **WHEN** any dashboard view renders
- **THEN** no empty region, placeholder or error appears where annotation
  would be, and every view remains fully usable

### Requirement: No currency and no probability in this capability
The dashboard SHALL NOT display monetary figures, calibrated probabilities, or
composite risk scores derived from them. Those belong to the
`executive-workbench` capability and require the model packs, exploit
probability engine and loss model of Changes 008–011.

#### Scenario: Exposure is presented without being priced
- **GIVEN** the executive area rendering an estate with open findings
- **WHEN** it displays them
- **THEN** it reports observed exposure and ordering, and asserts no expected
  loss, no annualised frequency and no probability of compromise

### Requirement: Executive area is extended by the workbench, not replaced
The executive area SHALL be structured so that the `executive-workbench`
capability adds priced and calibrated figures alongside the observed ones,
without the observed figures being recomputed, relocated or removed.

#### Scenario: Observed figures survive the workbench
- **GIVEN** the executive-workbench capability is later implemented
- **WHEN** its board view is added
- **THEN** the coverage, regression and ordering surfaces defined here remain
  available and continue to read from the same stored rows

### Requirement: Accessibility floor applies to the executive area
The executive area SHALL meet the WCAG 2.1 AA floor already required of the
dashboard, and SHALL NOT convey coverage, severity or regression state through
colour alone.

#### Scenario: State survives loss of colour
- **GIVEN** any status indicator in the executive area
- **WHEN** it is viewed without colour discrimination
- **THEN** its meaning remains available through text, shape or icon

## REMOVED Requirements

### Requirement: Loading states tied to Convex live queries
**Reason**: Convex was removed from the deployment stack in Change 005 Phase 5.
The requirement is not dropped — it is restated above under *Loading, empty,
and error states are designed, not blank*, against the transport that actually
exists, and strengthened to distinguish nothing-found from nothing-looked-at.

**Migration**: None. No deployment ever shipped on the Convex path.
