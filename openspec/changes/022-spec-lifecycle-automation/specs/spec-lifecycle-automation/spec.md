# Capability: spec-lifecycle-automation

## Purpose

Make the claims the specification tree asserts about itself checkable. A
change's location asserts whether it is in flight or settled; a ledger's boxes
assert what is outstanding; STATUS.md asserts totals. Each of those has been
wrong, and none of them was checked.

## ADDED Requirements

### Requirement: The status document is generated from the ledgers
The system SHALL derive the per-change counts in the status document from the
change ledgers, and SHALL fail the build when the committed document disagrees
with them.

#### Scenario: A ledger changes and the document does not
- **GIVEN** a change whose ledger has gained an open item
- **WHEN** the check runs against the committed status document
- **THEN** it fails, naming the command that corrects it

#### Scenario: Prose is not regenerated
- **GIVEN** a status row carrying a human-written note beside its counts
- **WHEN** the counts are rewritten
- **THEN** the note is unchanged

#### Scenario: Rows without counts are untouched
- **GIVEN** a status row naming a change but carrying no numeric cell
- **WHEN** the document is reconciled
- **THEN** the row is unchanged

### Requirement: A change cannot exist in two places
The system SHALL report a change identifier appearing under both the active and
the archived directories.

#### Scenario: An archive that was recreated
- **GIVEN** a change moved to the archive and subsequently recreated at its
  original path
- **WHEN** the check runs
- **THEN** it fails, naming the change

### Requirement: An empty ledger is a failure
The system SHALL report a change whose ledger is empty, and SHALL report an
active change whose ledger contains no checklist item.

#### Scenario: A truncated ledger
- **GIVEN** a change whose tasks file has been emptied
- **WHEN** the check runs
- **THEN** it fails, rather than reporting the change as having nothing
  outstanding

#### Scenario: A superseded change explained in prose
- **GIVEN** an archived change whose ledger was replaced with an explanation of
  why it must not be implemented
- **WHEN** the check runs
- **THEN** it passes, because an archived ledger is allowed to be prose

### Requirement: Location and state must agree
The system SHALL report an archived change with open items, and an active
change with no open items.

#### Scenario: Archived with work outstanding
- **GIVEN** an archived change whose ledger has open items
- **WHEN** the check runs
- **THEN** it fails, because an archived change reads as settled truth

#### Scenario: Closed but still active
- **GIVEN** an active change whose ledger has no open items
- **WHEN** the check runs
- **THEN** it fails, unless the change is recorded as not archivable with a
  stated reason

#### Scenario: An exemption states its reason
- **GIVEN** a change exempted from archiving
- **WHEN** the exemption is read
- **THEN** it carries the reason the change is not archivable

### Requirement: Every change is indexed
The system SHALL report a change folder absent from the status document, and a
status row naming no change folder.

#### Scenario: An unindexed change
- **GIVEN** a change folder not named in the status document
- **WHEN** the check runs
- **THEN** it fails, because an unindexed change is invisible and an invisible
  gap is indistinguishable from a decision not to build something

### Requirement: Structural problems are reported, not corrected
The system SHALL NOT automatically resolve a structural problem, archive a
change, or tick a checklist item.

#### Scenario: A duplicated change is not deleted
- **GIVEN** a change present in two places
- **WHEN** the check runs
- **THEN** it reports the problem and changes nothing, because which copy is
  correct is known only to its author

#### Scenario: A closed ledger is not archived
- **GIVEN** an active change whose ledger has closed
- **WHEN** the check runs
- **THEN** it reports it and does not move it, because archiving is a judgement
  against the code rather than against the ledger

### Requirement: Requirements in force are readable without the changelog
The system SHALL maintain a tree of accepted requirements distinct from
proposed ones, such that a requirement's status follows from where it is
recorded.

#### Scenario: Reading what is guaranteed today
- **GIVEN** a reader wanting to know what the system guarantees
- **WHEN** they read the accepted requirements tree
- **THEN** they need not determine which changes shipped in order to interpret
  what they find

#### Scenario: Promotion preserves provenance
- **GIVEN** a requirement promoted from a change into the accepted tree
- **WHEN** it is read
- **THEN** it names the change that accepted it

#### Scenario: A proposed requirement is not promoted
- **GIVEN** a requirement belonging to a change still in flight
- **WHEN** the accepted tree is read
- **THEN** the requirement is absent from it
