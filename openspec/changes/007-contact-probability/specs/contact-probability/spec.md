# Capability: contact-probability

## Purpose

Supply the term the exploit-probability framework names as external to itself: the probability that an attacker reaches an asset at all. Compute it as a decomposed, evidence-traced contact rate over a stated window, so that P(exploit | contact) can become an unconditional exploitation probability.

## ADDED Requirements

### Requirement: Contact modelled as a rate over a stated window
The system SHALL compute contact as a rate per unit time and derive the probability of contact over an explicitly stated observation window, and SHALL NOT emit a contact probability without the window it applies to.

#### Scenario: Window accompanies the probability
- **GIVEN** a reachable service with a computed contact rate
- **WHEN** a contact probability is produced
- **THEN** it is accompanied by the window in days and a stated rationale for that window

### Requirement: Reachability gates the estimate
A confirmed responding service SHALL be required for a non-zero external contact rate. A resolvable name without a confirmed responding service SHALL NOT produce a non-zero contact rate.

#### Scenario: Name resolves but nothing answers
- **GIVEN** an asset whose hostname resolves but where no service was confirmed responding
- **WHEN** its contact rate is computed
- **THEN** the contact rate is zero

#### Scenario: Reachability undetermined
- **GIVEN** an asset whose reachability could not be determined
- **WHEN** its contact rate is computed
- **THEN** no contact estimate is produced and the asset is reported as uncovered, rather than being assigned a zero or default rate

### Requirement: Named, traceable factors
Each contact estimate SHALL decompose into named factors — reachability, ambient attention, discoverability, access barrier and time at risk — each recording the observation or source that produced it, so any single factor can be challenged, removed and the estimate recomputed.

#### Scenario: Factor removal recomputes
- **GIVEN** a contact estimate with a discoverability factor derived from a certificate transparency observation
- **WHEN** that factor is removed
- **THEN** the estimate recomputes from the remaining factors and no other factor's value changes

### Requirement: Time at risk is measured and censoring is declared
Time at risk SHALL be derived from the system's own observation history, and SHALL be reported as a lower bound whenever the exposure began before observation started.

#### Scenario: Exposure predates observation
- **GIVEN** an asset observed as exposed on the first ever assessment of that asset
- **WHEN** time at risk is reported
- **THEN** it is marked as left-censored and presented as a minimum duration

#### Scenario: Detection latency is disclosed
- **GIVEN** an assessment cadence of one week
- **WHEN** time at risk is reported for a newly observed exposure
- **THEN** the reported figure discloses the detection latency inherent in the cadence

### Requirement: Contact probability of an open exposure increases with elapsed time
For an exposure that remains unremediated, the computed contact probability SHALL increase as wall-clock time elapses, without requiring any new finding.

#### Scenario: Unremediated exposure ages
- **GIVEN** an exposure still present and unremediated
- **WHEN** its contact probability is recomputed at a later date with no other change
- **THEN** the probability is strictly greater than the previously computed value

### Requirement: Access barriers attenuate but never eliminate contact
Controls that restrict who may successfully interact with a service SHALL reduce the contact rate without reducing it to zero.

#### Scenario: Authenticated endpoint
- **GIVEN** a reachable service requiring authentication
- **WHEN** its contact rate is computed
- **THEN** the rate is reduced relative to an equivalent unauthenticated service but remains greater than zero

### Requirement: Instance count and remediation coverage are computed
The system SHALL compute the number of instances of a weakness from inventory, aggregate contact probability across them, and compute a residual estimate over instances not confirmed remediated.

#### Scenario: Partial remediation
- **GIVEN** a weakness present on fifteen instances, of which three are confirmed remediated
- **WHEN** the aggregate contact probability is computed
- **THEN** it is computed over the twelve unremediated instances and reported as a residual alongside the remediation coverage of 3/15

### Requirement: Single-stage signal consumption
Every signal SHALL declare exactly one exploitation stage that consumes it, and the system SHALL NOT apply the same signal at more than one stage.

#### Scenario: Vulnerability attention signal
- **GIVEN** a finding whose specific vulnerability is on a known-exploited catalogue
- **WHEN** contact and attempt estimates are computed
- **THEN** that catalogue membership adjusts the attempt stage only and does not adjust the contact rate

#### Scenario: Duplicate consumption is a build failure
- **GIVEN** configuration in which one signal declares two consuming stages
- **WHEN** the required consistency check runs
- **THEN** the build fails

### Requirement: Contact prior and contact posterior are separate quantities
The system SHALL store the exposure-derived contact estimate as a prior, distinct from any telemetry-derived contact posterior, and SHALL prevent any assessment-derived code path from writing a contact posterior.

#### Scenario: Scanner observation cannot become contact evidence
- **GIVEN** a scan confirming a service is reachable
- **WHEN** contact quantities are updated
- **THEN** only the contact prior is affected, and the contact posterior is unchanged

#### Scenario: Enforcement is tested
- **GIVEN** the required separation check
- **WHEN** it runs against the assessment code paths
- **THEN** it fails if any assessment-derived path is capable of writing a contact posterior

### Requirement: Ambient contact only, targeted contact declared absent
The contact model SHALL price opportunistic, untargeted contact only, and any presentation of a contact estimate SHALL state that it excludes deliberately targeted adversaries.

#### Scenario: Presentation states the scope
- **GIVEN** a contact estimate shown to an operator
- **WHEN** it is displayed
- **THEN** it states that the figure prices ambient, opportunistic contact and excludes targeted adversary selection

### Requirement: Intervals, labels and coverage accompany every estimate
Every contact estimate SHALL carry an uncertainty interval, a calibration label, and the assessment coverage of the observations behind it, and SHALL NOT be presented as a bare point estimate.

#### Scenario: Display leads with the interval
- **GIVEN** a contact estimate with a wide interval
- **WHEN** it is displayed
- **THEN** the interval is presented with the estimate and the calibration label is shown
