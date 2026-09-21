# Capability: component-library-adoption

## Purpose

Govern how interactive behaviour is constructed in the frontend, so that focus
management, keyboard interaction and assistive-technology semantics are
provided by a reviewed primitive rather than reinvented per surface. Concerns
construction, not appearance.

## ADDED Requirements

### Requirement: Interactive behaviour comes from a primitive
The frontend SHALL implement overlay, menu, dialog, listbox and combobox
behaviour using the adopted primitive layer, and SHALL NOT hand-implement focus
trapping, focus restoration or roving focus in a component.

#### Scenario: A new overlay surface
- **GIVEN** a new surface requiring an overlay
- **WHEN** it is built
- **THEN** it uses the primitive, rather than a hand-written element with
  manually managed focus

#### Scenario: A native element suffices
- **GIVEN** an interaction fully served by a native accessible element
- **WHEN** it is built
- **THEN** the native element is used, because a primitive wrapping a
  `<button>` adds a dependency and removes nothing

### Requirement: Migration preserves appearance and behaviour
A migration of an existing surface SHALL NOT alter its visual design or the
behaviour its tests assert.

#### Scenario: A surface is migrated
- **GIVEN** a surface converted to use a primitive
- **WHEN** its existing tests run
- **THEN** they pass unmodified, or each modification is recorded with its
  justification

#### Scenario: Restyling is separated from restructuring
- **GIVEN** a change that both migrates and restyles a surface
- **WHEN** it is reviewed
- **THEN** it is rejected, because every hunk is then two changes and neither
  can be assessed

### Requirement: A migrated surface asserts the behaviour it gained
Each migrated surface SHALL carry an assertion about the keyboard or focus
behaviour the primitive provides.

#### Scenario: Keyboard interaction is tested
- **GIVEN** a migrated surface with an overlay
- **WHEN** its tests run
- **THEN** at least one asserts focus or keyboard behaviour, so that the reason
  for the migration is demonstrated rather than assumed

#### Scenario: An untested migration
- **GIVEN** a surface migrated with no such assertion
- **WHEN** the change is reviewed
- **THEN** it is incomplete, because it has bought nothing that leaving the
  markup alone would not also have bought

### Requirement: Only used primitives are present
The repository SHALL contain only primitives that a component uses.

#### Scenario: An unused primitive
- **GIVEN** a generated primitive no component imports
- **WHEN** the repository is reviewed
- **THEN** it is removed, because unused generated code looks like capability
  and carries a licence

### Requirement: The documented stack matches the installed one
Project documentation SHALL describe the component and styling libraries
actually in use.

#### Scenario: Documentation asserting an intention
- **GIVEN** documentation stating a library is used for all components
- **WHEN** that library is not installed
- **THEN** the documentation is incorrect and is corrected, rather than read as
  a description of the codebase

### Requirement: The adoption decision is recorded, not defaulted
The project SHALL record whether the component library is adopted or declined,
with reasoning, rather than leaving the question open.

#### Scenario: An open question outlives the decision
- **GIVEN** an unresolved adoption item older than the surfaces it governs
- **WHEN** the ledger is read
- **THEN** it states a decision and its reasoning, because an item left open
  indefinitely is a decision taken by default and never written down
