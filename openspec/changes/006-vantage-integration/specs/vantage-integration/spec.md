# Capability: vantage-integration

## Purpose

Adopt `vantage` as Trawl's external-vantage DNS, email-authentication and delegation assessor, and establish the **measured-state signal registry** and **assessment-coverage model** that the risk-quantification capabilities (007–012) consume. Observation only: this capability records what is externally observable and whether it was observed, never what it implies for probability.

## ADDED Requirements

### Requirement: Assessment runs in-process against a pinned module version
The system SHALL perform external assessment by embedding the assessment library in its own process, SHALL NOT depend on an external executable being installed, and SHALL pin the library by module version with no local replacement directive in committed dependency configuration.

#### Scenario: No external executable required
- **GIVEN** a deployment with no assessment binary present on the host
- **WHEN** an assessment runs
- **THEN** it completes normally, because assessment is compiled into the system's own binary

#### Scenario: Local replacement is refused
- **GIVEN** committed dependency configuration containing a local replacement directive for the assessment library
- **WHEN** the required check runs
- **THEN** the build fails

#### Scenario: Library version recorded
- **GIVEN** any persisted observation
- **WHEN** it is inspected
- **THEN** it records the assessment library version that produced it

### Requirement: Consumed library surface is guarded by contract tests
The system SHALL maintain tests exercising every part of the assessment library's interface that it consumes, so that an incompatible upstream change fails the build.

#### Scenario: Upstream signature change
- **GIVEN** an upgraded assessment library whose interface no longer satisfies a consumed assumption
- **WHEN** the build and contract tests run
- **THEN** they fail, rather than the incompatibility surfacing at runtime

### Requirement: Assessment library types are confined to the adapter
Types belonging to the assessment library SHALL be referenced only within the adapter package, and SHALL NOT appear in the interfaces of any other package.

#### Scenario: Blast radius contained
- **GIVEN** an upstream change to an assessment library type
- **WHEN** the affected code is identified
- **THEN** it is confined to the adapter package

### Requirement: Assessment outcomes are distinguished from assessment failures
The system SHALL distinguish a completed assessment, a partially completed assessment, a failed assessment, a refused out-of-scope assessment and a cancelled assessment, and SHALL NOT record findings from an assessment that did not complete.

#### Scenario: Partial completion retains what concluded
- **GIVEN** an assessment in which some checks could not conclude
- **WHEN** the result is persisted
- **THEN** concluded checks are stored with their outcomes and unconcluded checks are stored as `check_failed`

#### Scenario: Failed assessment yields no findings
- **GIVEN** an assessment that failed as a whole
- **WHEN** the result is persisted
- **THEN** no findings are recorded and every requested check is recorded as `check_failed`

#### Scenario: Cancellation retains partial results
- **GIVEN** an assessment cancelled before all checks ran
- **WHEN** the result is persisted
- **THEN** completed checks retain their outcomes and unreached checks are recorded as `not_checked`

### Requirement: Four-state assessment coverage preserved end to end
The system SHALL store and surface `ok`, `not_found`, `not_checked` and `check_failed` as distinct states for every check, and SHALL NOT collapse them into a binary pass/fail at any layer, including aggregation, display and export.

#### Scenario: Unassessed is not passing
- **GIVEN** a domain whose DNSSEC check was never attempted
- **WHEN** its posture is displayed or aggregated
- **THEN** the DNSSEC control is reported as `not_checked`, and is not counted as either present or absent

#### Scenario: Absence is a positive observation
- **GIVEN** a domain assessed for DMARC where no record is published
- **WHEN** its posture is recorded
- **THEN** the state is `not_found`, distinguishable from `not_checked`, and is available downstream as adverse measured-state evidence

### Requirement: Assessment coverage is reported with every derived figure
The system SHALL compute an assessment coverage figure — checks concluded divided by checks applicable — per asset and per scenario, and SHALL make it available alongside any aggregate derived from those checks.

#### Scenario: Coverage accompanies a posture grade
- **GIVEN** a domain where four of eleven applicable checks could not be concluded
- **WHEN** its posture summary is produced
- **THEN** the summary carries an assessment coverage of 7/11 and does not present the grade without it

### Requirement: Signal registry maps findings to weakness classes, stages and dedup groups
The system SHALL maintain a versioned registry mapping each vantage finding identifier to a weakness class, an affected scenario, the exploitation stage it informs, a dedup group, and the control it evidences. The registry SHALL be configuration data, not code.

#### Scenario: Mapped signal
- **GIVEN** a finding identifying an absent DMARC record
- **WHEN** it is persisted
- **THEN** it carries a weakness class, scenario, stage and dedup group resolved from the registry, and the registry version used

#### Scenario: Registry updates without an engine change
- **GIVEN** a revised registry file assigning an existing finding identifier to a different dedup group
- **WHEN** assessments are next run
- **THEN** the new grouping applies without a change to compiled engine code

### Requirement: Unmapped identifiers are retained, never discarded
When a finding identifier is absent from the registry, the system SHALL persist it marked as unmapped and SHALL surface it to operators.

#### Scenario: New upstream rule
- **GIVEN** an assessment returning a finding identifier not present in the registry
- **WHEN** the result is persisted
- **THEN** the finding is stored with an unmapped status and displayed, and it contributes to no risk computation until mapped

### Requirement: Registry completeness is a required check
The build SHALL fail when the assessment library's rule catalogue contains an identifier that the registry neither maps nor explicitly records as intentionally unmapped.

#### Scenario: Upstream adds a rule
- **GIVEN** an assessment library upgrade introducing a new finding identifier
- **WHEN** the required completeness check runs
- **THEN** it fails until the identifier is mapped or explicitly recorded as intentionally unmapped

### Requirement: Scope is enforced at the network transport
The system SHALL supply the network transport used by the assessment library, and that transport SHALL refuse any query for a target outside the authorized scope, so that an out-of-scope query cannot leave the process regardless of what the assessment logic requests.

#### Scenario: Out-of-scope domain
- **GIVEN** a domain absent from the authorized scope
- **WHEN** an assessment is requested for it
- **THEN** no network query is emitted and the request is refused with a scope violation record

#### Scenario: Assessment logic straying out of scope
- **GIVEN** an in-scope target whose assessment follows a reference to an out-of-scope name
- **WHEN** that reference is resolved
- **THEN** the transport refuses the query, the refusal is recorded, and no packet is emitted for the out-of-scope name

#### Scenario: Enforcement is tested by absence of queries
- **GIVEN** the required scope check
- **WHEN** an out-of-scope assessment is attempted against an instrumented transport
- **THEN** the check fails if the transport is invoked at all

### Requirement: Every check declares a structured egress profile
Each check in the assessment catalogue SHALL declare, as structured data, the classes of network egress it performs — ordinary resolution, direct contact with the target's nameservers, direct contact with the target over HTTPS, named third-party services, and whether it is intrusive.

#### Scenario: Profile available before invocation
- **GIVEN** the assessment catalogue
- **WHEN** it is queried
- **THEN** each check reports its egress classes and its named third-party endpoints, without any check having been run

#### Scenario: Undeclared egress fails the build
- **GIVEN** a catalogue entry without an egress profile
- **WHEN** the required egress conformance check runs
- **THEN** the build fails

#### Scenario: Unrecognised egress class fails the build
- **GIVEN** a catalogue entry declaring an egress class the deployment policy schema does not recognise
- **WHEN** the required egress conformance check runs
- **THEN** the build fails, rather than the class being silently permitted or silently ignored

### Requirement: Deployment policy is expressed over egress profiles, not check names
The set of checks requested SHALL be derived by filtering the catalogue through a deployment policy stated in terms of egress classes and consented third-party endpoints, and configuration SHALL NOT enumerate individual check names for this purpose.

#### Scenario: Policy excludes by class
- **GIVEN** a deployment policy that does not permit intrusive assessment
- **WHEN** the requested check set is derived
- **THEN** every check declaring an intrusive profile is excluded

#### Scenario: Derived transport permissions
- **GIVEN** a derived set of requested checks
- **WHEN** the assessment transport is constructed
- **THEN** its permitted egress is derived from the declared profiles of those checks, and no other egress is possible

### Requirement: Newly introduced egress fails closed
A check whose declared egress is not covered by the deployment's policy SHALL NOT be run, and SHALL be recorded as `not_checked` with the reason.

#### Scenario: Upgrade introduces an intrusive check
- **GIVEN** an assessment library upgrade adding a check declaring an intrusive profile, under a deployment that has not permitted intrusive assessment
- **WHEN** an assessment runs
- **THEN** the new check does not run and is recorded as `not_checked`, naming the policy that excluded it

#### Scenario: Upgrade introduces a new third-party dependency
- **GIVEN** an assessment library upgrade adding a check that contacts a third-party service absent from the consented service-endpoint allowlist
- **WHEN** an assessment runs
- **THEN** the new check does not run, is recorded as `not_checked`, and the unconsented endpoint is named to the operator

### Requirement: Third-party service endpoints are allowlisted separately from target scope
Requests to services that are neither the target's nor the operator's SHALL be governed by an explicit service-endpoint allowlist, distinct from the authorized target scope.

#### Scenario: Unlisted third-party endpoint
- **GIVEN** an assessment attempting to contact a third-party service not present in the service-endpoint allowlist
- **WHEN** the request is made
- **THEN** it is refused and recorded, and the dependent check is recorded as `check_failed`

### Requirement: Egress documentation is generated from declarations
Operator-facing documentation of what an assessment will touch SHALL be derived from the declared egress profiles rather than maintained separately.

#### Scenario: Blast radius before invocation
- **GIVEN** a configured deployment policy
- **WHEN** an operator asks what an assessment will contact
- **THEN** the answer is generated from the declared profiles of the checks the policy admits

### Requirement: Concurrent assessments are isolated
Assessment configuration and transport SHALL be supplied per assessment, and concurrent assessments under different authorized scopes SHALL NOT share or leak configuration or transport between one another.

#### Scenario: Simultaneous assessments under different scopes
- **GIVEN** two assessments running concurrently under different authorized scopes
- **WHEN** each performs its queries
- **THEN** neither is able to query a target permitted only by the other's scope

### Requirement: Intrusive and third-party checks are opt-in
Checks that query the target's own nameservers directly (zone transfer, open-resolver probing, wildcard probing, takeover assessment) and checks that query third-party services (Certificate Transparency enumeration) SHALL each be excluded by default and admitted only by explicit per-deployment policy.

#### Scenario: Zone transfer excluded by default
- **GIVEN** a deployment with default policy
- **WHEN** the requested check set is derived
- **THEN** no check declaring an intrusive profile is included

#### Scenario: Enumeration is separately consented
- **GIVEN** a deployment that permits intrusive assessment but has consented to no third-party service endpoints
- **WHEN** the requested check set is derived
- **THEN** Certificate Transparency enumeration is excluded

### Requirement: Discovered hostnames enter the normal discovery path
Hostnames obtained from Certificate Transparency enumeration SHALL be recorded as a discovery source and SHALL pass through the same deduplication and scope-authorization path as every other discovery source, with no privileged trust.

#### Scenario: Certificate-derived hostname out of scope
- **GIVEN** an enumerated hostname outside the authorized scope
- **WHEN** it is processed
- **THEN** it is not scanned, and it is recorded as an out-of-scope discovery observation

### Requirement: Attribution provenance distinguishes estate change from data refresh
Provider attribution SHALL be stored with the provenance of the underlying data, and a change in attribution attributable to refreshed provider data SHALL NOT be raised as a posture regression.

#### Scenario: Provider data refresh
- **GIVEN** an asset whose provider attribution changes between runs solely because the provider range data was refreshed
- **WHEN** posture regression is evaluated
- **THEN** no regression is raised, and the attribution change is recorded with its provenance

### Requirement: No probability or valuation in this capability
This capability SHALL record observations, coverage states and registry mappings only, and SHALL NOT compute probabilities, adjustments, uncertainty or monetary values.

#### Scenario: Observation carries no valuation
- **GIVEN** a persisted signal observation
- **WHEN** it is inspected
- **THEN** it contains the observed condition, state, evidence, timestamp and registry mapping, and contains no probability, log-odds adjustment or variance term
