# Capability: feed-catalogue

## Purpose

Keep what the external vulnerability catalogues said, and when each was last
consulted, so that an operator can tell how current the enrichment behind a
finding is and can refresh it again. Extends the storage and freshness
behaviour of Change 019; it does not change how a snapshot is parsed or how
priority is computed.

## ADDED Requirements

### Requirement: An ingested CISA catalogue is persisted with its entries
The system SHALL store the entries of each ingested CISA Known Exploited
Vulnerabilities snapshot together with the snapshot, in one transaction, and
SHALL retain only the latest snapshot's entries while keeping every snapshot's
metadata.

#### Scenario: A catalogue is ingested
- **WHEN** a KEV snapshot is applied
- **THEN** its entries are stored with it, and the feed status reports that the
  entries are held locally

#### Scenario: A newer catalogue arrives
- **GIVEN** a stored KEV snapshot with entries
- **WHEN** a snapshot with different content is applied
- **THEN** the earlier snapshot's entries are removed, its metadata is kept, and
  the new snapshot's entries are stored

#### Scenario: A snapshot predates stored entries
- **GIVEN** a KEV snapshot stored without entries by an earlier build
- **WHEN** identical content is ingested again
- **THEN** its entries are stored against the existing snapshot

### Requirement: The last successful consultation is recorded and shown
The system SHALL record, for each feed, the time the source was last
successfully consulted whether or not its content had changed, and SHALL show it
separately from the time the content in force arrived.

#### Scenario: A refresh finds the catalogue unchanged
- **GIVEN** a stored snapshot
- **WHEN** a refresh retrieves identical content
- **THEN** no second snapshot is stored, the snapshot's own time is unchanged,
  and the last-refreshed time moves forward

#### Scenario: A refresh fails
- **GIVEN** a stored snapshot and a recorded last-refreshed time
- **WHEN** a refresh fails
- **THEN** the failure and its reason are reported, the last-refreshed time is
  unchanged, and the snapshot in force is retained

### Requirement: Staleness is judged on the last consultation
The system SHALL mark a feed stale when its last successful consultation, or
its snapshot's time where none was recorded, is older than the staleness
threshold.

#### Scenario: Old content, recently confirmed
- **GIVEN** a snapshot that arrived five days ago and was reconfirmed an hour ago
- **WHEN** the feed status is read
- **THEN** it is not stale

#### Scenario: A database without a recorded check
- **GIVEN** a snapshot older than the threshold and no recorded consultation
- **WHEN** the feed status is read
- **THEN** it is stale

### Requirement: Refreshing again reaches findings recorded since
The system SHALL, when a refresh retrieves content already stored, evaluate any
CVE finding not yet evaluated against that snapshot, and SHALL NOT record a run
when nothing was evaluated.

#### Scenario: A finding is recorded after the catalogue was applied
- **GIVEN** a stored catalogue listing a CVE and a finding for it recorded
  afterwards
- **WHEN** the feed is refreshed and the content is unchanged
- **THEN** the finding is evaluated against the snapshot, its priority is
  recomputed, and the run is recorded

#### Scenario: Nothing new to evaluate
- **WHEN** an unchanged catalogue is refreshed and every finding was already
  evaluated against it
- **THEN** the consultation is recorded and no run is added to the history

#### Scenario: The source reports no change
- **GIVEN** a stored catalogue with entries
- **WHEN** the source answers that its content is unchanged
- **THEN** the consultation is recorded and findings not yet evaluated are
  evaluated from the stored entries

### Requirement: A single feed can be refreshed
The system SHALL allow one or more named feeds to be refreshed without fetching
the others, SHALL refuse an unknown feed name, and SHALL serialise refreshes.

#### Scenario: Refreshing CISA KEV alone
- **WHEN** only `cisa-kev` is refreshed
- **THEN** EPSS and NVD are not fetched and their status is unchanged

#### Scenario: A mistyped feed
- **WHEN** a feed name the engine does not ingest is requested
- **THEN** the request is refused as invalid and nothing is fetched

### Requirement: EPSS is looked up only for the CVEs on stored findings
The system SHALL, when FIRST's per-CVE API is configured, request EPSS scores
only for the distinct CVEs on stored findings, SHALL record which CVEs the
snapshot asked about, and SHALL NOT record a CVE that was not asked about as
having no score.

#### Scenario: Refreshing EPSS
- **GIVEN** stored findings referencing two CVEs
- **WHEN** EPSS is refreshed
- **THEN** only those two CVEs are requested and the snapshot holds the scores
  returned

#### Scenario: A CVE has no score yet
- **WHEN** the API returns nothing for an asked-about CVE
- **THEN** the finding's EPSS state is *not found*

#### Scenario: A finding is recorded after the request
- **GIVEN** a snapshot built before a finding's CVE was recorded
- **WHEN** the snapshot is applied
- **THEN** that finding's EPSS state remains *not checked*

#### Scenario: A batch fails
- **WHEN** any batch of the request fails
- **THEN** no snapshot is stored and the failure is recorded against the feed

#### Scenario: Only the daily file is configured
- **GIVEN** a mirror of the daily EPSS file and no per-CVE API
- **WHEN** EPSS is refreshed
- **THEN** the whole file is ingested as before
