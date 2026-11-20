# Capability: candidate-vulnerabilities

## Purpose

Surface CVEs that may apply to software an asset is running, found from the
software's identity rather than from a scanner template, and keep them clearly
apart from findings a scan demonstrated.

## ADDED Requirements

### Requirement: Candidates are looked up from mapped product and version
The system SHALL ask NVD which CVEs apply to each mapped product and known
version, SHALL NOT look up a product whose version is unknown or whose name is
unmapped, and SHALL store each answer as a snapshot with its provenance.

#### Scenario: A product and version are looked up
- **GIVEN** an asset with a mapped product and known version
- **WHEN** candidates are refreshed
- **THEN** NVD is asked about that product and version, and the CVEs returned
  are stored with the snapshot that produced them

#### Scenario: The version is unknown
- **GIVEN** a product recorded with its version unknown
- **WHEN** candidates are refreshed
- **THEN** no lookup is made for it, and it is reported as seen with version unknown

#### Scenario: The lookup fails
- **WHEN** a lookup fails part-way
- **THEN** no partial answer is stored, the failure is recorded, and previously
  stored candidates are retained and marked as not re-checked

### Requirement: Lookups are cached and explicit
The system SHALL ask once per product and version until the fingerprint changes
or the operator refreshes, SHALL respect the NVD rate limit, and SHALL NOT look
up software at startup or on ingest.

#### Scenario: Two assets run the same software
- **GIVEN** two assets running the same product and version
- **WHEN** candidates are refreshed
- **THEN** NVD is asked once for that product and version

#### Scenario: Nothing has changed
- **GIVEN** a stored answer for a product and version
- **WHEN** candidates are refreshed without a change in software
- **THEN** no request is made unless the operator forces one

### Requirement: A candidate is never presented as a confirmed finding
The system SHALL record a matched CVE as a candidate finding with its own source
and key, SHALL label it unconfirmed everywhere it is shown, SHALL show the
observed string and the match beside it, and SHALL NOT state that the asset is
vulnerable.

#### Scenario: A candidate is listed
- **WHEN** a candidate appears in the findings list
- **THEN** it is labelled unconfirmed and its detail shows the raw string observed
  and the product and version matched

#### Scenario: Executive counts
- **WHEN** the executive view counts exposure
- **THEN** candidates are excluded from the confirmed figures and counted and
  labelled separately

#### Scenario: Narrative wording
- **WHEN** a candidate's explanation is rendered
- **THEN** it says the software appears to be affected and that nothing has shown
  the flaw is present, and carries no assertion of vulnerability, probability or
  monetary value

### Requirement: Candidates have history and resolve only when re-assessed
The system SHALL give a candidate the identity and event history defined by
Change 025, and SHALL resolve it only when a later scan concluded the asset's
software and it no longer matches.

#### Scenario: The software is upgraded past the affected range
- **GIVEN** an open candidate for `nginx` `1.18.0`
- **WHEN** a later scan reports `nginx` `1.24.0`, which the lookup does not match
- **THEN** the candidate is resolved and a resolved event is recorded

#### Scenario: The software is no longer observed
- **GIVEN** an open candidate
- **WHEN** a later scan of the asset reports no technology data
- **THEN** the candidate remains open and is marked not re-assessed

### Requirement: Enrichment follows the candidate set
The system SHALL include candidate CVEs in the set whose KEV status and EPSS
scores are fetched, using the referenced-only retrieval, and SHALL rank a
KEV-listed candidate above other candidates while keeping it labelled unconfirmed.

#### Scenario: A candidate is in KEV
- **GIVEN** a candidate whose CVE is KEV-listed
- **WHEN** feeds are applied
- **THEN** its priority is raised by the pure priority function, the event cites
  the snapshot, and it remains labelled unconfirmed

#### Scenario: A candidate recorded after a snapshot
- **GIVEN** a feed snapshot built before a candidate's CVE was known
- **WHEN** the snapshot is applied
- **THEN** the candidate's enrichment is not checked, not not found

### Requirement: Lookup egress is disclosed
The system SHALL document that product and version strings are sent to NVD, and
SHALL NOT send any other asset data in a lookup.

#### Scenario: A lookup request
- **WHEN** a lookup is made
- **THEN** the request carries a CPE or product and version and no asset name,
  address or other identifier
