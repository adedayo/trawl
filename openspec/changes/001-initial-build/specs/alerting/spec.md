# Capability: alerting

## Purpose

Notify the operator promptly and without excessive noise when new assets or high-priority findings appear — the early-warning delivery mechanism the whole system exists to feed.

## ADDED Requirements

### Requirement: Alert triggers
The system SHALL send a notification on: (a) any new asset promoted to `active` status, (b) any finding reaching `priority: critical` or `kev: true`.

#### Scenario: New active asset
- **GIVEN** an asset transitions from `pending` to `active`
- **WHEN** the transition is committed
- **THEN** a new-asset alert is sent

### Requirement: Distinguishable alert categories
New-asset alerts and new-finding alerts SHALL be routed as distinguishable message types (channel, prefix, or field) so the operator can triage by category at a glance.

#### Scenario: Category distinguishable in channel
- **GIVEN** both a new-asset alert and a new-finding alert fire in the same period
- **WHEN** the operator views their notification channel
- **THEN** each message is clearly labeled by category without opening it

### Requirement: Deduplicated notification
The system SHALL deduplicate alerts so the same unresolved finding does not re-notify on every scan cycle; re-notification SHALL only occur on state change (new, escalated, or resolved-then-reopened).

#### Scenario: Repeated cycle, no re-alert
- **GIVEN** a previously alerted critical finding remains open after the next scan cycle
- **WHEN** that cycle completes
- **THEN** no duplicate alert is sent

#### Scenario: Reopened finding re-alerts
- **GIVEN** a finding's status changes from `resolved` to `reopened`
- **WHEN** that transition is detected
- **THEN** a new alert fires

### Requirement: Pluggable delivery channel
Alert delivery SHALL be pluggable (Slack/Teams webhook initially, ticketing system as a future channel) via a single notification interface.

#### Scenario: Channel swap
- **GIVEN** the operator wants to switch from a Slack webhook to a ticketing-system integration
- **WHEN** they change the configured notification channel
- **THEN** no alerting logic elsewhere in the system needs to change, only the channel implementation behind the shared interface
