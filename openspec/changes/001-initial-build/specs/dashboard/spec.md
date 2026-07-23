# Capability: dashboard

## Purpose

Give the operator a real-time, single-pane view of current external asset posture and open findings, plus the human-review control point for scope expansion.

## ADDED Requirements

### Requirement: Live-updating views
The dashboard SHALL display live-updating asset and finding lists via real-time subscription, with no manual refresh required.

#### Scenario: New finding appears live
- **GIVEN** a new finding is ingested
- **WHEN** the operator has the dashboard open
- **THEN** it appears in the finding list without a page reload

### Requirement: Pending-asset review queue
The dashboard SHALL provide a pending-assets review queue where the operator can approve or reject medium/low-confidence discovered assets.

#### Scenario: Approval promotes to scan-eligible
- **GIVEN** a pending asset is approved in the review queue
- **WHEN** approval is submitted
- **THEN** its status becomes `active` and it becomes eligible for the next scan cycle

### Requirement: Finding detail surface
The dashboard SHALL surface, per finding: priority, KEV/EPSS status, AI summary, first/last seen, and current status.

#### Scenario: Finding detail view
- **GIVEN** the operator opens a specific finding
- **WHEN** the detail view renders
- **THEN** it shows priority, KEV flag, EPSS score, the AI-generated summary, first/last-seen timestamps, and current status in one view

### Requirement: Authentication required, extensible to multi-user
The dashboard SHALL require authentication; in single-operator deployments this may be a single credential gate, but the design SHALL NOT preclude adding multi-user RBAC later.

#### Scenario: Unauthenticated access blocked
- **GIVEN** an unauthenticated request reaches the dashboard
- **WHEN** it attempts to load asset or finding data
- **THEN** access is denied until authentication succeeds

### Requirement: Operational toggles surfaced and editable
The dashboard SHALL surface operator-facing behavioral config flags (e.g., `secretVerificationEnabled` from the `repository-secrets` capability) as editable UI controls that write directly to config, taking effect on the next scheduled run with no redeploy — not buried in a file the operator has to edit by hand.

#### Scenario: Toggle secret verification from the UI
- **GIVEN** the operator wants to stop `repository-secrets` from making outbound verification calls to credential providers
- **WHEN** they switch the "secret verification" toggle off in the dashboard
- **THEN** `config.secretVerificationEnabled` updates immediately and the next scheduled repository scan honors the new value

### Requirement: Modern Angular implementation, no legacy patterns
The Angular app SHALL be built with standalone components, signals for local and derived state (`signal()`/`computed()`), and the built-in control-flow syntax (`@if`/`@for`/`@switch`) for all templates. It SHALL NOT use `NgModule`-based bootstrapping, the structural directives `*ngIf`/`*ngFor`/`*ngSwitch`, or decorator-only reactive patterns where a signal is the more direct fit.

#### Scenario: Template review finds no legacy structural directives
- **GIVEN** any Angular template in the app
- **WHEN** it is inspected
- **THEN** conditional and iterative rendering use `@if`/`@for`/`@switch`, never `*ngIf`/`*ngFor`/`*ngSwitch`

### Requirement: Tailwind-based design system, no ad hoc styling
The app SHALL style exclusively through Tailwind utility classes plus a single shared design-token source (color, spacing, typography scale) — no component ships bespoke inline styles or a one-off CSS file duplicating a token that already exists elsewhere.

#### Scenario: New component reuses existing tokens
- **GIVEN** a new dashboard view is added
- **WHEN** its styling is reviewed
- **THEN** it draws colors, spacing, and typography from the shared Tailwind config rather than introducing new ad hoc values

### Requirement: Loading, empty, and error states are designed, not blank
Every view that depends on a Convex live query SHALL render an explicit loading state (skeleton, not a blank screen) while data resolves, an explicit empty state with guidance text when there is genuinely no data yet (e.g., "no findings yet — your first scheduled scan runs at..."), and an explicit error state distinct from either, rather than a silently blank or broken-looking screen in any of the three cases.

#### Scenario: Zero-findings first run shows guidance, not a blank table
- **GIVEN** a freshly-deployed instance with no scans run yet
- **WHEN** the operator opens the findings view
- **THEN** they see an empty state explaining that no scan has completed yet and when the next one is scheduled, not an empty table with no explanation

### Requirement: Accessibility floor
The dashboard SHALL meet WCAG 2.1 AA as a minimum across all views, verified by an automated accessibility check in CI (see `ci-cd-pipeline` capability), not left to manual spot-checking alone.

#### Scenario: Automated accessibility check gates merges
- **GIVEN** a pull request changes an Angular component
- **WHEN** CI runs
- **THEN** an automated accessibility scan runs against the affected views and the merge is blocked on any newly-introduced critical violation
