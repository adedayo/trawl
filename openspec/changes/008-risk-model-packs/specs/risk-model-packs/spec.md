# Capability: risk-model-packs

## Purpose

Hold every parameter that can move a risk number as versioned, signed, source-cited configuration data — including the measured-state precision parameters that let continuous assessment narrow uncertainty — so that revising the model is a data change with a visible diff, and no number in the system lacks a locatable provenance.

## ADDED Requirements

### Requirement: No probability-affecting constant in engine code
Every parameter that affects a computed probability, uncertainty, rate or monetary value SHALL be supplied by a pack, and SHALL NOT be embedded as a literal in engine code.

#### Scenario: Required check on literals
- **GIVEN** engine code containing a numeric literal used as a probability adjustment
- **WHEN** the required check runs
- **THEN** the build fails

### Requirement: Provenance on every parameter
Every parameter SHALL carry a source citation, a retrieval date, a verification status and a calibration label, and SHALL NOT be expressible as a bare value.

#### Scenario: Parameter inspection
- **GIVEN** any parameter used in any computation
- **WHEN** an operator inspects it
- **THEN** it reports its source, retrieval date, verification status and calibration label

#### Scenario: Unverified by default
- **GIVEN** a newly shipped pack
- **WHEN** its parameters are inspected
- **THEN** parameters transcribed from published sources but not independently checked carry a needs-verification status

### Requirement: Pack version recorded on every estimate
Every stored estimate SHALL record the pack version that produced it, and estimates produced under different pack versions SHALL NOT be presented as a continuous trend without declaring the change.

#### Scenario: Trend across a pack change
- **GIVEN** a trend series spanning a pack update
- **WHEN** it is displayed
- **THEN** the point at which the pack changed is marked and the series either re-baselines or declares the discontinuity

### Requirement: Measured-state signals carry precision parameters
Each measured-state signal SHALL carry, in addition to its mean adjustment, a variance share, a dedup group, a decay constant and a precision cap.

#### Scenario: Signal definition completeness
- **GIVEN** a measured-state signal in a pack
- **WHEN** the pack is validated
- **THEN** validation fails unless the signal defines a variance share, dedup group, decay constant and cap

### Requirement: Variance share requires a stated heterogeneity argument
A non-zero variance share SHALL be accompanied by an explicit argument that the source population was heterogeneous in that signal, and SHALL be zero where the source already conditioned on it.

#### Scenario: Missing argument
- **GIVEN** a signal claiming a non-zero variance share with no heterogeneity argument
- **WHEN** the pack is validated
- **THEN** validation fails

#### Scenario: Source already conditioned
- **GIVEN** a class prior drawn from a study that conditioned on the signal in question
- **WHEN** that signal's variance share is defined
- **THEN** it is zero, and the reason is recorded

### Requirement: Variance shares are claimed once per dedup group
Signals sharing a dedup group SHALL contribute at most one variance share, taken as the maximum member share rather than the sum.

#### Scenario: Correlated signals
- **GIVEN** two observed signals belonging to the same dedup group, with variance shares of 0.4 and 0.3
- **WHEN** precision gain is computed
- **THEN** a single share of 0.4 is applied

### Requirement: Precision gain is capped
Total precision gain from measured-state evidence at any stage SHALL NOT exceed the pack's cap, applied after deduplication.

#### Scenario: Many favourable signals
- **GIVEN** a stage with several observed measured-state signals whose combined shares would exceed the cap
- **WHEN** precision gain is computed
- **THEN** the gain is limited to the cap and the limitation is recorded

### Requirement: Repeated observation does not accumulate evidence
Repeated observation of an unchanged configuration SHALL NOT increase effective sample size or precision gain beyond that of a single current observation.

#### Scenario: Continuous monitoring of a static configuration
- **GIVEN** an unchanged configuration observed five hundred times
- **WHEN** precision gain is computed
- **THEN** it equals the gain from a single current observation of that configuration

### Requirement: Measured-state evidence decays with age
Precision gain SHALL decay toward zero as the age of the observation increases, at a rate given per signal class by the pack.

#### Scenario: Stale observation
- **GIVEN** a measured-state observation substantially older than its decay constant
- **WHEN** precision gain is computed
- **THEN** the gain has decayed toward zero and the estimate's interval approaches the class-prior width

#### Scenario: Unassessed control
- **GIVEN** a control that was never assessed
- **WHEN** precision gain is computed
- **THEN** no gain is applied and the class prior stands unmodified

### Requirement: Scenario taxonomy mapping is explicit data
The mapping from published incident patterns to the deployment's scenario list SHALL be recorded as pack data, with shares summing to one, and every expert-estimated residual carrying a rationale and a prior-strength exception marker.

#### Scenario: Shares validation
- **GIVEN** a pack whose scenario shares do not sum to one
- **WHEN** the pack is validated
- **THEN** validation fails

#### Scenario: Uncovered scenario class
- **GIVEN** a scenario for which the source publishes no share
- **WHEN** it is defined in a pack
- **THEN** it carries an expert-estimated share taken from the residual, a rationale, and a prior-strength exception marker

### Requirement: Judgment parameters carry sweep points and ownership
The prior-strength parameters and the tail-relief weight SHALL each carry a value, a rationale, its published sweep points, and the role that owns the decision.

#### Scenario: Tail weight ownership
- **GIVEN** a pack defining the tail-relief weight
- **WHEN** it is inspected
- **THEN** it records that the decision is owned by the board, along with its sweep points

### Requirement: Packs are signed and refuse to load when unverifiable
A pack whose signature is absent or does not verify SHALL NOT be loaded.

#### Scenario: Tampered pack
- **GIVEN** a pack file modified after signing
- **WHEN** it is loaded
- **THEN** loading is refused and no computation proceeds using it

### Requirement: Overrides are a separate, attributed layer
Operator overrides SHALL be stored separately from shipped packs, SHALL never modify the shipped pack, and SHALL be attributed in every parameter resolution.

#### Scenario: Overridden parameter
- **GIVEN** a parameter overridden by an operator
- **WHEN** it is resolved for a computation
- **THEN** it reports its value as locally overridden, names the override layer, and the shipped pack file is unmodified

### Requirement: Pack updates produce a diff and never silently recompute
Applying a new pack SHALL produce a report of changed parameters and affected estimates, and SHALL NOT retroactively alter previously stored estimates.

#### Scenario: Pack upgrade
- **GIVEN** a stored set of estimates computed under an earlier pack
- **WHEN** a new pack is applied
- **THEN** a diff report is produced and the earlier estimates are retained unchanged with their original pack version
