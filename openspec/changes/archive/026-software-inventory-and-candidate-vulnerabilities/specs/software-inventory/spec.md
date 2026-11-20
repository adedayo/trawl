# Capability: software-inventory

## Purpose

Record, per asset, the software the engine has seen, in a form that can be
queried and compared, and say plainly where it could not be identified. It does
not govern what vulnerabilities such software may have (see
`candidate-vulnerabilities`).

## ADDED Requirements

### Requirement: Observed software is recorded as structured data
The system SHALL record, for each asset, each product observed from scan data,
with its version where one was observed, the source of the observation, the raw
string it was parsed from, and its first and last seen times.

#### Scenario: A technology with a version is observed
- **GIVEN** a scan result reporting `Nginx:1.18.0` for an asset
- **WHEN** the result is ingested
- **THEN** the asset has a product `nginx`, version `1.18.0`, the raw string
  and its first-seen time

#### Scenario: A technology without a version is observed
- **GIVEN** a scan result reporting `WordPress` with no version
- **WHEN** the result is ingested
- **THEN** the product is recorded with its version unknown, and is not
  recorded with any guessed version

#### Scenario: The same software is observed again
- **GIVEN** a recorded product and version on an asset
- **WHEN** a later scan reports it again
- **THEN** one record exists, with its last-seen time updated

### Requirement: A change of version is a change, not a new asset fact
The system SHALL, when a later scan reports a different version of a product on
an asset, record the new version as current and retain the earlier one as
superseded with the time it was last seen.

#### Scenario: Software is upgraded
- **GIVEN** an asset recorded with `nginx` `1.18.0`
- **WHEN** a later scan reports `nginx` `1.24.0`
- **THEN** `1.24.0` is current, `1.18.0` is superseded, and both remain readable

### Requirement: Product identity comes from a reviewed mapping
The system SHALL map observed names to a vendor and product only through a
versioned configuration, SHALL record a name it cannot map as unmapped, and
SHALL NOT infer a mapping from free text at run time.

#### Scenario: A mapped name
- **WHEN** an observed name is in the mapping
- **THEN** the record carries the mapped vendor and product and the mapping version

#### Scenario: An unmapped name
- **WHEN** an observed name is not in the mapping
- **THEN** the record is kept with its raw string and marked unmapped, and it is
  never looked up

#### Scenario: The mapping is incomplete
- **WHEN** the build is tested
- **THEN** every entry names a vendor and product, and no two names collide on
  conflicting products

### Requirement: Absence of software evidence is not an answer
The system SHALL report, for each asset, whether software was identified, and
SHALL NOT present an asset with no identified software as having no software
vulnerabilities.

#### Scenario: An asset with no technology data
- **GIVEN** an asset whose scan result carried no technology data
- **WHEN** its software coverage is read
- **THEN** it is reported as not assessed for software, and no vulnerability
  count is shown for it

#### Scenario: Software seen but not mapped
- **GIVEN** an asset whose only observed software is unmapped
- **WHEN** its software coverage is read
- **THEN** it reports software seen but not identifiable, distinct from no software seen
