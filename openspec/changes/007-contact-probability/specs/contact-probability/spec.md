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

### Requirement: Observed and inferred exposure intervals are accounted separately
Time at risk SHALL accrue over intervals bounded by successful assessments, and any interval during which the exposure was not successfully assessed SHALL be accounted as inferred rather than observed. Both figures SHALL be reported, and their sum SHALL NOT be presented as a single measured duration.

Repeated observation of the same weakness does not sharpen any susceptibility estimate. It sustains the exposure premise: each successful re-assessment converts an interval of assumed exposure into an observed one, and is an opportunity for the premise to be falsified. See RISK-ARC §5b.

#### Scenario: Continuous observation
- **GIVEN** an exposure assessed successfully every day since first detection
- **WHEN** time at risk is reported
- **THEN** the whole duration is accounted as observed and the inferred component is zero

#### Scenario: Monitoring lapses and resumes
- **GIVEN** an exposure first seen in March, not assessed between April and August, and seen again in September
- **WHEN** time at risk is reported
- **THEN** the April-to-August interval is accounted as inferred, is disclosed as such, and is not reported as observed exposure

#### Scenario: Two estates with equal total time at risk
- **GIVEN** one estate assessed hourly and another assessed quarterly, both reporting the same total time at risk
- **WHEN** either estimate is presented
- **THEN** the observed and inferred split accompanies it, so the difference in evidential standing is visible

### Requirement: Only a successful clean assessment closes an exposure window
An exposure window SHALL be closed only by an assessment that ran and returned a clean result. An assessment that did not run, or that failed, SHALL leave the window open and SHALL extend the inferred component.

#### Scenario: Finding disappears because the check failed
- **GIVEN** an exposure previously observed, and a subsequent assessment returning `check_failed`
- **WHEN** time at risk is recomputed
- **THEN** the window remains open and the interval is accounted as inferred

#### Scenario: Finding disappears because the check was not run
- **GIVEN** an exposure previously observed, and a subsequent assessment in which the relevant check was `not_checked`
- **WHEN** time at risk is recomputed
- **THEN** the window remains open, and remediation is not recorded

#### Scenario: Check runs and returns clean
- **GIVEN** an exposure previously observed, and a subsequent assessment in which the relevant check returned `ok`
- **WHEN** time at risk is recomputed
- **THEN** the window is closed at that assessment's timestamp

### Requirement: Blind time is separated from aware time
Time at risk SHALL be decomposed into the interval before the weakness was detected and the interval after, and both SHALL be reported. The pre-detection interval SHALL be estimated from the assessment cadence in force at the time and SHALL be labelled as estimated rather than observed.

Risk accrues over the whole of time at risk, but the organisation's stated position only tracked the post-detection interval. The pre-detection interval is therefore the measured size of a past understatement, not merely a gap in the record. See RISK-ARC §5c.

#### Scenario: Weakness introduced between assessments
- **GIVEN** an estate assessed on a 90-day cadence and a weakness first reported at an assessment
- **WHEN** time at risk is reported
- **THEN** the estimated blind interval is reported alongside the observed aware interval, with its cadence-derived basis stated, and the two are not presented as a single measured figure

#### Scenario: Continuous assessment
- **GIVEN** an estate assessed daily and a weakness first reported at an assessment
- **WHEN** time at risk is reported
- **THEN** the estimated blind interval is bounded by one day and is reported as such

### Requirement: Expected detection latency is derived from cadence and reported
The system SHALL derive an expected detection latency from the assessment cadence of each asset class, SHALL report it as a property of the monitoring programme rather than of any finding, and SHALL express the exposure attributable to it.

#### Scenario: Cadence change is evaluated
- **GIVEN** an asset class assessed quarterly, and the estate's observed rate of weakness introduction
- **WHEN** a change to daily assessment is evaluated
- **THEN** the system reports the resulting reduction in expected blind time and the corresponding reduction in expected contact probability

#### Scenario: Detection latency is not remediation latency
- **GIVEN** an estate with a short detection latency and a long unremediated backlog
- **WHEN** the monitoring programme is reported
- **THEN** detection latency and remediation latency are reported as separate figures and are not combined into a single responsiveness measure

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
