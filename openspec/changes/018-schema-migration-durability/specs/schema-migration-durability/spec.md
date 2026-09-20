# Capability: schema-migration-durability

## Purpose

Ensure a store created by any released build can be opened by any later one
with its shape brought forward correctly, and that a shape which cannot be
brought forward is reported rather than operated on. Structural only: this
capability governs how the schema changes, not what any table means.

## ADDED Requirements

### Requirement: An existing database is brought to the current shape on open
The system SHALL bring an existing database to the current schema shape when it
is opened, and SHALL NOT require the database to have been created by the
current build.

#### Scenario: A database from an earlier release is opened
- **GIVEN** a database created by an earlier release, lacking columns the
  current release writes
- **WHEN** it is opened
- **THEN** the missing columns are added and the first write succeeds

#### Scenario: Migration is idempotent
- **GIVEN** a database already at the current shape
- **WHEN** it is opened repeatedly
- **THEN** no schema statement is applied and no error is raised

#### Scenario: Existing data survives
- **GIVEN** a database holding rows written by an earlier release
- **WHEN** it is brought to the current shape
- **THEN** no row is dropped and no existing column is altered

### Requirement: A migrated database is indistinguishable from a fresh one
The system SHALL ensure that a database brought forward from an earlier release
has the same schema as one created by the current release.

#### Scenario: Shapes converge
- **GIVEN** a database created by an earlier release and brought forward, and a
  database created fresh by the current release
- **WHEN** their schemas are compared
- **THEN** they are identical

#### Scenario: A forgotten migration fails the build
- **GIVEN** a column added to the table-creation schema but not to the set of
  after-the-fact additions
- **WHEN** the tests run
- **THEN** they fail, rather than the omission surfacing on a user's existing
  database

### Requirement: The database records the shape it is in
The system SHALL record a schema version in the database, and SHALL set it as
part of the same operation that changes the shape.

#### Scenario: Version is recorded
- **GIVEN** a database brought to the current shape
- **WHEN** its recorded schema version is read
- **THEN** it equals the version the current build expects

#### Scenario: Version follows the shape
- **GIVEN** a migration that fails partway
- **WHEN** the database is inspected
- **THEN** the recorded version does not claim a shape the database does not
  have

### Requirement: A database from a later release is refused
The system SHALL refuse to open a database whose recorded schema version is
newer than the running build expects, and SHALL report both versions.

#### Scenario: Downgrade is refused
- **GIVEN** a database written by a newer release
- **WHEN** an older build opens it
- **THEN** it refuses, naming the version found and the version expected, and
  does not write

#### Scenario: Refusal is legible
- **GIVEN** a refused database
- **WHEN** the error is shown to the operator
- **THEN** it states that the store was written by a newer version, rather than
  surfacing as a failed query

### Requirement: The migration mechanism states what it cannot do
The migration mechanism SHALL document, in the code that implements it, the
classes of schema change it does not support.

#### Scenario: An unsupported change is recognised as one
- **GIVEN** a developer making a schema change that is not a column addition
- **WHEN** they read the migration code
- **THEN** they find the limitation stated before they rely on the mechanism

### Requirement: Data that could not be carried forward is disclosed
Where a migration deliberately declines to reconstruct data, the system SHALL
present the affected records as unassessed rather than as assessed with an
absent result.

#### Scenario: A pre-widening posture reads as unassessed
- **GIVEN** an email posture written before the four-state widening
- **WHEN** it is read
- **THEN** every control reports as never assessed, and none reports as passing

#### Scenario: The operator is told why the view is empty
- **GIVEN** an installation upgraded from a release predating the widening
- **WHEN** the operator opens the affected view before any rescan
- **THEN** it states that the data predates the current assessment and requires
  a rescan, rather than presenting an absence that resembles a clean result
