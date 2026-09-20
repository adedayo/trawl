# Capability: alerting-and-delivery

## Purpose

Carry an event that the engine has already determined to be worth knowing to a
human or a system outside Trawl, once, with its provenance and its coverage
intact. Governs delivery and suppression; it does not govern what counts as a
regression, which the detection capabilities decide.

## ADDED Requirements

### Requirement: Alertable events are delivered to configured destinations
The system SHALL deliver events matching a configured routing rule to that
rule's destination, and SHALL record the attempt and its outcome.

#### Scenario: A new critical finding is delivered
- **GIVEN** a configured webhook destination and a rule matching critical
  findings
- **WHEN** a new critical finding is recorded
- **THEN** a delivery is attempted to that destination and its outcome is
  recorded

#### Scenario: No matching rule
- **GIVEN** an event matching no configured rule
- **WHEN** it is published
- **THEN** no delivery is attempted, and the event is still recorded as having
  occurred

#### Scenario: Delivery does not block detection
- **GIVEN** a destination that is timing out
- **WHEN** a scan produces events
- **THEN** the scan completes and the events are stored, independent of
  delivery

### Requirement: Repeating conditions are not repeatedly alerted
The system SHALL derive a stable identity for each alertable event from what
the event asserts, and SHALL NOT deliver a second alert for an event whose
identity has already been delivered and whose underlying condition has not
changed.

#### Scenario: A rescan observes the same condition
- **GIVEN** a finding already alerted on
- **WHEN** a later scan observes the same finding unchanged
- **THEN** no further alert is delivered

#### Scenario: The condition changes
- **GIVEN** a finding already alerted on as high severity
- **WHEN** a later assessment raises it to critical
- **THEN** a new alert is delivered, identifying itself as a change to a known
  condition rather than as a new discovery

#### Scenario: A resolved condition recurs
- **GIVEN** a condition that was alerted, then resolved
- **WHEN** it is observed again
- **THEN** it is alerted again, because a recurrence is news

### Requirement: Delivery is at least once and recognisably idempotent
The system SHALL retry failed deliveries, and SHALL include in every delivery
an identifier that is stable across retries of the same event.

#### Scenario: A retry is recognisable
- **GIVEN** a delivery that failed and was retried
- **WHEN** the receiver compares the two payloads
- **THEN** they carry the same event identifier

#### Scenario: Retries are bounded
- **GIVEN** a destination failing persistently
- **WHEN** the retry budget is exhausted
- **THEN** the delivery is recorded as failed and abandoned, rather than
  retried indefinitely

#### Scenario: Ordering is not promised
- **GIVEN** multiple events delivered to one destination
- **WHEN** they arrive
- **THEN** each carries the time of the condition it describes, so a receiver
  need not rely on arrival order

### Requirement: Routing is deterministic
The system SHALL determine the destinations for an event by a pure function of
the event and the configured rules, and SHALL NOT consult any AI provider in
the delivery path.

#### Scenario: The same event routes the same way
- **GIVEN** a fixed set of rules
- **WHEN** the same event is evaluated repeatedly
- **THEN** the same destinations are selected every time

#### Scenario: No model decides whether a human is told
- **GIVEN** an AI provider configured
- **WHEN** routing is evaluated
- **THEN** the provider is not consulted

### Requirement: An alert carries the coverage of what it asserts
An alert that states or implies a conclusion about a set of assets SHALL carry
the assessment coverage of that set.

#### Scenario: A summary with gaps
- **GIVEN** a periodic summary over an estate in which some assets are
  `not_checked`
- **WHEN** the alert is composed
- **THEN** it states the number not checked, and does not assert that the
  estate is clear

#### Scenario: A single finding
- **GIVEN** an alert about one finding
- **WHEN** it is composed
- **THEN** it carries the finding's own evidence and the check that produced
  it, so the recipient can act without opening the dashboard

### Requirement: Silence is distinguishable from health
The system SHALL record the last successful delivery per destination and SHALL
surface a destination that is failing.

#### Scenario: A destination has gone bad
- **GIVEN** a destination whose recent deliveries have all failed
- **WHEN** the operator views destinations
- **THEN** it is shown as failing, with the time of its last success

#### Scenario: A quiet destination
- **GIVEN** a destination that has matched no events
- **WHEN** it is viewed
- **THEN** it is shown as configured and idle, distinct from failing

### Requirement: Outbound delivery cannot be aimed at arbitrary internal hosts
The system SHALL validate destination URLs before use, and SHALL refuse
destinations resolving to loopback, link-local or private address ranges unless
the operator has explicitly permitted them.

#### Scenario: A destination resolving internally is refused
- **GIVEN** a webhook URL resolving to a private address
- **WHEN** it is configured without an explicit permission
- **THEN** it is refused, with the reason stated

#### Scenario: Re-resolution at delivery time
- **GIVEN** a destination that validated at configuration time
- **WHEN** it resolves to a refused range at delivery time
- **THEN** the delivery is refused, so that a DNS change cannot turn an
  accepted destination into an internal one

### Requirement: Operational failures are distinguished from security events
The system SHALL classify alerts arising from Trawl's own failures separately
from alerts about the assessed estate.

#### Scenario: A feed fetch failure
- **GIVEN** a failed catalogue fetch
- **WHEN** it is alerted
- **THEN** it is classified as operational and may be routed separately, so
  that it does not appear among findings about the estate

### Requirement: Destination configuration is external to the engine
The system SHALL read destination URLs, credentials and routing rules from
external configuration.

#### Scenario: Nothing org-specific is committed
- **GIVEN** the repository
- **WHEN** it is searched for destination URLs or credentials
- **THEN** none are present, and the engine reads them from configuration
