# Capability: asset-inventory

## Purpose

Maintain the canonical, deduplicated record of every known external asset (IP or domain) with lifecycle state and provenance, so every other capability has one source of truth for "what do we own."

## ADDED Requirements

### Requirement: Stable asset identity
The system SHALL store each asset with a stable identity, type (`ip` | `domain`), discovery source, `first_seen` timestamp, `last_seen` timestamp, and status (`pending` | `active` | `stale` | `decommissioned`).

#### Scenario: New asset recorded
- **GIVEN** a domain `vpn.example.com` has never been seen before
- **WHEN** a discovery or scan job reports it
- **THEN** a new asset record is created with `first_seen` and `last_seen` set to now, and status `pending`

### Requirement: Re-observation updates, never duplicates
The system SHALL treat re-observation of an existing asset as an update to `last_seen`, not a new record.

#### Scenario: Same IP seen again
- **GIVEN** an asset already exists with ip `203.0.113.10`
- **WHEN** a scan job reports seeing it again
- **THEN** `last_seen` updates on the existing record and no duplicate record is created

### Requirement: Automatic staleness
The system SHALL mark an asset stale if it has not been re-observed by any discovery or scan job within a configurable window (default 30 days).

#### Scenario: Asset not re-observed
- **GIVEN** an asset has not been seen in 31 days
- **WHEN** the nightly lifecycle job runs
- **THEN** its status transitions to `stale` and a low-priority notice is logged

### Requirement: Retained history, no deletion
The system SHALL retain decommissioned/stale assets in history rather than deleting them, to preserve audit trail.

#### Scenario: Historical query
- **GIVEN** an asset was decommissioned six months ago
- **WHEN** the operator queries asset history
- **THEN** the record and its full first/last-seen and status-transition history are still retrievable
