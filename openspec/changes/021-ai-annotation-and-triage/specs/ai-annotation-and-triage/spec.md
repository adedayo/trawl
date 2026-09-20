# Capability: ai-annotation-and-triage

## Purpose

Let a language model add explanation and grouping to findings the engine has
already established, without letting it alter, suppress or reorder them.
Governs the boundary between advisory text and computed fact. It does not
govern how findings are produced or scored.

## ADDED Requirements

### Requirement: The system is fully functional with no provider configured
The system SHALL operate with no AI provider configured, and SHALL NOT present
any surface as incomplete, degraded or pending in that state.

#### Scenario: No provider
- **GIVEN** no configured AI provider
- **WHEN** any view is rendered
- **THEN** it presents complete information, with no placeholder awaiting an
  annotation and no prompt to configure one in order to proceed

#### Scenario: A provider that fails
- **GIVEN** a configured provider that is unreachable
- **WHEN** findings are viewed
- **THEN** the findings are shown in full and the annotation is shown as
  unavailable, distinct from an annotation asserting there is nothing to say

### Requirement: Annotation cannot alter a computed field
The system SHALL store AI output separately from severity, priority, KEV
status, EPSS score and any probability or monetary figure, and SHALL make it
structurally impossible for a provider response to write one.

#### Scenario: A response asserting a severity
- **GIVEN** a provider response containing a severity or priority value
- **WHEN** it is processed
- **THEN** the value is discarded, the stored severity and priority are
  unchanged, and the discard is recorded

#### Scenario: The boundary is enforced by a test
- **GIVEN** a change introducing a write to a computed field from the
  annotation path
- **WHEN** the tests run
- **THEN** they fail

#### Scenario: Recomputation ignores annotation
- **GIVEN** an annotated finding
- **WHEN** its priority is recomputed
- **THEN** the result is identical to that for the same finding unannotated

### Requirement: Every annotation carries its provenance
The system SHALL record, with each annotation, the provider, model identifier,
prompt version and generation time.

#### Scenario: Provenance is retrievable
- **GIVEN** a stored annotation
- **WHEN** it is read
- **THEN** it names the provider, model, prompt version and time

#### Scenario: Annotations from a given model can be found
- **GIVEN** a model later found to be systematically wrong about a class of
  finding
- **WHEN** its annotations are queried
- **THEN** they can be identified and invalidated as a set

#### Scenario: No silent failover
- **GIVEN** a configured provider that fails
- **WHEN** annotation is attempted
- **THEN** it fails and is recorded as failed, and no other provider is used

### Requirement: Annotation is labelled and visually subordinate
The system SHALL label every rendered annotation as AI-generated and advisory,
and SHALL render it subordinate to the deterministic content it accompanies.

#### Scenario: An annotation is rendered
- **GIVEN** a finding with an annotation
- **WHEN** it is displayed
- **THEN** the annotation is labelled as AI-generated and advisory, and is
  visually distinguishable from the finding's evidence and computed fields

#### Scenario: The requirement is not vacuous
- **GIVEN** the rendering tests
- **WHEN** they run
- **THEN** at least one exercises a present annotation, so that the requirement
  is verified against something rather than passing by absence

### Requirement: Model input is treated as hostile
The system SHALL treat finding content as untrusted input to the model, and
SHALL grant the model no capability beyond returning text.

#### Scenario: Instructions embedded in a finding
- **GIVEN** a finding whose title or evidence contains text instructing the
  model to alter a severity or suppress the finding
- **WHEN** it is annotated
- **THEN** the finding's stored fields are unchanged and it is not suppressed

#### Scenario: The model holds no tools
- **GIVEN** an annotation request
- **WHEN** it is issued
- **THEN** it exposes no tool, no store access and no network capability to the
  model

#### Scenario: Output is validated before storage
- **GIVEN** a response not matching the expected shape
- **WHEN** it is processed
- **THEN** it is rejected and recorded as a failed annotation

### Requirement: Annotation may not suppress a finding
The system SHALL NOT allow AI output to remove a finding, mark it resolved, or
exclude it from any count or view.

#### Scenario: A model believes a finding is a false positive
- **GIVEN** an annotation stating a finding is likely a false positive
- **WHEN** findings are listed and counted
- **THEN** the finding appears and is counted, with the annotation shown as
  advisory

### Requirement: Triage groups and drafts but does not rank
The system SHALL NOT use AI output to determine the order in which findings are
presented or addressed.

#### Scenario: Grouping is offered
- **GIVEN** several findings sharing an underlying cause
- **WHEN** triage assistance runs
- **THEN** it may propose them as a group, and the ordering within and between
  groups remains the deterministic priority ordering

#### Scenario: Queue order is unaffected
- **GIVEN** a remediation queue
- **WHEN** annotations are added or removed
- **THEN** the order of the queue does not change

### Requirement: Sending estate data to a third party is explicitly opted into
The system SHALL NOT transmit finding content to an external provider without
explicit operator configuration naming that provider.

#### Scenario: No default provider
- **GIVEN** a fresh installation
- **WHEN** findings are produced
- **THEN** no finding content leaves the installation

#### Scenario: The operator is told what is sent
- **GIVEN** an operator configuring a cloud provider
- **WHEN** they do so
- **THEN** the categories of data that will be transmitted are stated at the
  point of configuration
