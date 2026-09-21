package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/adedayo/trawl/pkg/store"
)

// Schemes are the DSN prefixes this backend answers to.
//
// "file" is included because it is what SQLite's own tooling uses, and an
// operator who writes the DSN they are used to should not be told it is
// unrecognised.
const (
	SchemeSQLite = "sqlite"
	SchemeFile   = "file"
)

// Registering here rather than at a composition root is what lets an
// entrypoint select a backend by configuration: linking this package in is the
// act that makes "sqlite:" resolvable, and no caller has to name the
// constructor.
func init() {
	store.Register(SchemeSQLite, func(dsn string) (store.Store, error) {
		return NewSQLiteStore(dsn)
	})
	store.Register(SchemeFile, func(dsn string) (store.Store, error) {
		return NewSQLiteStore(dsn)
	})
}

type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore initializes SQLite database with WAL mode pragmas and auto-migrations.
//
// dbPath may be a bare filesystem path or a scheme-qualified DSN such as
// "sqlite:/var/lib/trawl/trawl.db". An empty value selects the per-user
// default location, which is what a desktop install wants and what a container
// overrides.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dbPath = trimScheme(dbPath)

	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		trawlDir := filepath.Join(homeDir, ".trawl")
		if err := os.MkdirAll(trawlDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create trawl data directory: %w", err)
		}
		dbPath = filepath.Join(trawlDir, "trawl.db")
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

// trimScheme removes a leading "sqlite:" or "file:" so that the same value
// works whether it arrived through the factory or was passed directly.
//
// A Windows drive letter is left alone: "C:\data\trawl.db" is a path, not a
// scheme, and stripping its prefix would silently relocate the database.
func trimScheme(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	for _, scheme := range []string{SchemeSQLite + ":", SchemeFile + ":"} {
		if len(dsn) > len(scheme) && strings.EqualFold(dsn[:len(scheme)], scheme) {
			// Both "sqlite:/path" and "sqlite:///path" are accepted; the
			// authority component is empty for a local file either way.
			return strings.TrimPrefix(dsn[len(scheme):], "//")
		}
	}
	return dsn
}

// schemaVersion is the shape this build expects, recorded in the database with
// PRAGMA user_version.
//
// Raise it when the schema changes. Version 1 is the first version to be
// recorded at all: databases written before this existed report 0, which is
// indistinguishable from a database created moments ago and is why the
// migration must remain safe to run against a store already at the current
// shape.
//
// What the marker buys is the refusal below. Without it, a store written by a
// newer build and opened by an older one is accepted silently — every
// individual query still succeeds, because the columns the old build knows
// about are all still there — and the damage is discovered later, if at all.
const schemaVersion = 5

// ErrNewerSchema reports a database written by a build newer than this one.
//
// This is a distinct type because the caller has to be able to tell it apart
// from an ordinary open failure: a store that will not open is the first thing
// a user sees, and "database is locked" and "this store was written by a newer
// version of Trawl" call for entirely different actions.
type ErrNewerSchema struct {
	Found    int
	Expected int
}

func (e *ErrNewerSchema) Error() string {
	return fmt.Sprintf(
		"this database was written by a newer version of Trawl (store format %d; this build understands %d). "+
			"Upgrade Trawl to open it. Continuing with this build would write rows the newer format cannot interpret",
		e.Found, e.Expected)
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	found, err := s.userVersion(ctx)
	if err != nil {
		return err
	}
	if found > schemaVersion {
		return &ErrNewerSchema{Found: found, Expected: schemaVersion}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS assets (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		value TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL,
		discovery_source TEXT NOT NULL,
		confidence REAL NOT NULL,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		metadata TEXT
	);

	CREATE TABLE IF NOT EXISTS findings (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		severity TEXT NOT NULL,
		priority TEXT NOT NULL,
		cve TEXT,
		epss REAL,
		kev_listed INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'open',
		category TEXT NOT NULL,
		proof TEXT,
		ai_annotation TEXT,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS feed_snapshots (
		id TEXT PRIMARY KEY,
		feed TEXT NOT NULL,
		source_url TEXT NOT NULL,
		retrieved_at DATETIME NOT NULL,
		content_digest TEXT NOT NULL,
		record_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS finding_enrichments (
		finding_id TEXT NOT NULL,
		feed TEXT NOT NULL,
		cve TEXT NOT NULL,
		state TEXT NOT NULL,
		snapshot_id TEXT,
		epss REAL,
		kev_listed INTEGER,
		checked_at DATETIME,
		PRIMARY KEY(finding_id, feed),
		FOREIGN KEY(finding_id) REFERENCES findings(id) ON DELETE CASCADE,
		FOREIGN KEY(snapshot_id) REFERENCES feed_snapshots(id)
	);

	CREATE INDEX IF NOT EXISTS idx_finding_enrichments_cve ON finding_enrichments(cve);

	INSERT OR IGNORE INTO finding_enrichments (finding_id, feed, cve, state)
	SELECT id, 'cisa-kev', COALESCE(cve, ''), 'not_checked' FROM findings;
	INSERT OR IGNORE INTO finding_enrichments (finding_id, feed, cve, state)
	SELECT id, 'epss', COALESCE(cve, ''), 'not_checked' FROM findings;
	INSERT OR IGNORE INTO finding_enrichments (finding_id, feed, cve, state)
	SELECT id, 'nvd', COALESCE(cve, ''), 'not_checked' FROM findings;

	CREATE TABLE IF NOT EXISTS secret_findings (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		repo_url TEXT NOT NULL,
		rule_id TEXT NOT NULL,
		secret_type TEXT NOT NULL,
		redacted_ref TEXT NOT NULL,
		file_path TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		verified INTEGER DEFAULT 0,
		is_reused INTEGER DEFAULT 0,
		first_seen DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS posture_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		asset_id TEXT NOT NULL,
		attribute_type TEXT NOT NULL,
		value TEXT NOT NULL,
		observed_at DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS asset_exposure_history (
		asset_id TEXT NOT NULL,
		service TEXT NOT NULL,
		first_observed DATETIME NOT NULL,
		last_observed DATETIME NOT NULL,
		still_exposed INTEGER NOT NULL,
		left_censored INTEGER NOT NULL,
		observed_duration_seconds INTEGER NOT NULL,
		inferred_duration_seconds INTEGER NOT NULL,
		blind_duration_seconds INTEGER NOT NULL,
		expected_blind_seconds INTEGER NOT NULL,
		worst_blind_seconds INTEGER NOT NULL,
		PRIMARY KEY(asset_id, service),
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_exposure_history_service ON asset_exposure_history(service);

	CREATE TABLE IF NOT EXISTS regressions (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		attribute_type TEXT NOT NULL,
		previous_value TEXT NOT NULL,
		current_value TEXT NOT NULL,
		consecutive_fails INTEGER NOT NULL,
		confirmed_at DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS email_postures (
		domain TEXT PRIMARY KEY,
		spf_valid INTEGER NOT NULL,
		dkim_found INTEGER NOT NULL,
		dmarc_policy TEXT NOT NULL,
		priority TEXT NOT NULL,
		last_checked DATETIME NOT NULL,
		mta_sts_found INTEGER DEFAULT 0,
		mta_sts_mode TEXT,
		dnssec_valid INTEGER DEFAULT 0,
		dane_valid INTEGER DEFAULT 0,
		-- controls and details hold the four-state posture. The boolean
		-- columns above are retained so an older build can still open the
		-- database, and are written as a lossy mirror; these are the record.
		controls TEXT,
		details TEXT
	);

	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		status TEXT NOT NULL,
		targets TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		error TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_pop ON jobs(type, status, created_at);

	CREATE TABLE IF NOT EXISTS signal_registry (
		signal_id TEXT PRIMARY KEY,
		condition TEXT NOT NULL,
		weakness_class TEXT NOT NULL,
		scenario TEXT NOT NULL,
		stage TEXT NOT NULL,
		dedup_group TEXT NOT NULL,
		control TEXT NOT NULL,
		direction TEXT NOT NULL,
		registry_version TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS signal_observations (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		signal_id TEXT NOT NULL,
		check_id TEXT NOT NULL,
		state TEXT NOT NULL,
		severity TEXT NOT NULL,
		evidence TEXT,
		detail TEXT,
		mapped INTEGER NOT NULL DEFAULT 0,
		registry_version TEXT NOT NULL,
		library_version TEXT NOT NULL,
		observed_at DATETIME NOT NULL,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		UNIQUE(asset_id, signal_id),
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS findings (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		severity TEXT NOT NULL,
		priority TEXT NOT NULL,
		cve TEXT,
		epss REAL,
		kev_listed INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'open',
		category TEXT NOT NULL,
		proof TEXT,
		ai_annotation TEXT,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_signal_obs_asset ON signal_observations(asset_id, state);

	CREATE TABLE IF NOT EXISTS assessment_coverage (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		check_id TEXT NOT NULL,
		state TEXT NOT NULL,
		reason TEXT,
		library_version TEXT NOT NULL,
		assessed_at DATETIME NOT NULL,
		UNIQUE(asset_id, check_id),
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_coverage_asset ON assessment_coverage(asset_id, state);

	-- How the last assessment of each asset ended, as distinct from what it
	-- found. A refused or wholly failed run writes no coverage and no
	-- observations, so without this row it would be indistinguishable from a
	-- domain that was assessed and found clean.
	CREATE TABLE IF NOT EXISTS assessment_runs (
		asset_id TEXT PRIMARY KEY,
		outcome TEXT NOT NULL,
		error TEXT NOT NULL DEFAULT '',
		profile TEXT NOT NULL DEFAULT '',
		library_version TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL,
		finished_at DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	-- Where each address an asset resolves to is hosted, and on what basis.
	--
	-- One row per address rather than per asset: a name routinely resolves to
	-- several, and they need not agree. A name balanced across two
	-- jurisdictions is a fact to show, not a discrepancy to reduce to a
	-- winner.
	--
	-- provider is empty when no published range matched. That is the absence
	-- of a match and not a claim that the address is unhosted, which is why
	-- the adapter degrades the check to check_failed whenever the ranges
	-- failed to load.
	CREATE TABLE IF NOT EXISTS asset_attribution (
		asset_id TEXT NOT NULL,
		host TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT '',
		address TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		region TEXT NOT NULL DEFAULT '',
		jurisdiction TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT '',
		library_version TEXT NOT NULL DEFAULT '',
		observed_at DATETIME NOT NULL,
		PRIMARY KEY(asset_id, host, address),
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_attribution_provider ON asset_attribution(provider);

	CREATE TABLE IF NOT EXISTS service_observations (
		id TEXT PRIMARY KEY,
		asset_id TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		service TEXT NOT NULL,
		transport TEXT NOT NULL,
		protocol TEXT NOT NULL,
		layer TEXT NOT NULL,
		state TEXT NOT NULL,
		coverage TEXT NOT NULL,
		evidence TEXT,
		profile TEXT NOT NULL,
		observed_at DATETIME NOT NULL,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_service_observations_asset ON service_observations(asset_id, observed_at);

	-- Where each provider's ranges were obtained and when. Kept separate from
	-- the attributions because it answers a different question: not "where is
	-- this address" but "on what basis, and how old is it". Without it, two
	-- runs cannot distinguish a host that moved from range data that was
	-- merely refreshed.
	CREATE TABLE IF NOT EXISTS attribution_provenance (
		asset_id TEXT NOT NULL,
		provider TEXT NOT NULL,
		url TEXT NOT NULL,
		fetched_at DATETIME NOT NULL,
		PRIMARY KEY(asset_id, provider, url),
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

	-- Third-party reference data (cloud provider address ranges and the like),
	-- cached across assessments so a portfolio scan fetches once rather than
	-- once per target. fetched_at is stored so callers can disclose the age of
	-- attribution rather than presenting stale data as current.
	CREATE TABLE IF NOT EXISTS range_cache (
		url TEXT PRIMARY KEY,
		content BLOB NOT NULL,
		fetched_at DATETIME NOT NULL
	);

	`

	// The shape and the version it claims are set together, so a migration
	// that fails partway cannot leave the database asserting a shape it does
	// not have. SQLite's DDL is transactional, which is what makes this
	// possible; on an engine where it is not, the version would have to be
	// written before the change and repaired afterwards.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	if err := addColumns(ctx, tx); err != nil {
		return err
	}
	// PRAGMA user_version takes no bound parameter, so the value is
	// interpolated. It is a constant in this file, not input.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("recording schema version: %w", err)
	}

	return tx.Commit()
}

func (s *SQLiteStore) userVersion(ctx context.Context) (int, error) {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return version, nil
}

// addedColumns are columns introduced after their table first shipped.
//
// CREATE TABLE IF NOT EXISTS is a no-op against a database that already has
// the table, so a column added to the schema above reaches new installations
// only. Every existing database would keep the old shape and fail on the first
// write — at run time, on a user's machine, rather than here.
var addedColumns = map[string]map[string]string{
	// The email posture was a row of booleans. It is now four-state per
	// control, held as JSON because the shape is nested and is read whole.
	"email_postures": {
		"controls": "TEXT",
		"details":  "TEXT",
	},
	// Findings used to carry only the library's general explanation, which is
	// the same for every occurrence of an identifier. The sentence naming the
	// particular item at fault is specific to the observation, so it is kept
	// with it rather than recomputed from a catalogue that does not know it.
	"signal_observations": {
		"detail": "TEXT",
	},
	"findings": {
		"status": "TEXT NOT NULL DEFAULT 'open'",
	},
}

// addColumns brings an existing database up to the current shape.
//
// # What this mechanism does, and what it does not
//
// It adds columns. That is the whole of it. It cannot retype, rename or drop a
// column; it cannot backfill one; it cannot add or drop an index or change a
// constraint; and it cannot migrate data between tables. It has no ordering,
// because column additions commute and nothing else does.
//
// If your change is not a column addition, this will not carry it, and the
// failure will not be here — it will be on a user's existing database, at run
// time, long after the change looked finished. A convention that resembles a
// framework is worse than no framework, because it suppresses the question.
// Consider this the question.
//
// The recorded decision (Change 018) is that this, plus the version marker
// above, is the right amount of machinery for a single-file embedded store
// that ships as one binary. A full migration framework buys ordering and
// down-migrations; ordering is unnecessary while every change commutes, and
// down-migrations on a live store are a restore-from-backup operation that
// should not be made to look routine. Revisit when the first non-additive
// change arrives, and revisit it then rather than working around this.
//
// It is idempotent: SQLite has no ADD COLUMN IF NOT EXISTS, so the current
// columns are read first. An error from the ALTER cannot be distinguished from
// a real failure by its text without matching on a message SQLite is free to
// reword.
func addColumns(ctx context.Context, tx *sql.Tx) error {
	for table, columns := range addedColumns {
		existing, err := columnsOf(ctx, tx, table)
		if err != nil {
			return err
		}
		for name, decl := range columns {
			if existing[name] {
				continue
			}
			stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, decl)
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("adding column %s.%s: %w", table, name, err)
			}
		}
	}
	return nil
}

func columnsOf(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, fmt.Errorf("reading columns of %s: %w", table, err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name, typ  string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultVal, &pk); err != nil {
			return nil, fmt.Errorf("scanning columns of %s: %w", table, err)
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

// erasedTables are the tables holding what the engine discovered, in the order
// they are cleared.
//
// Children are cleared before their parent. The schema declares ON DELETE
// CASCADE, but SQLite enforces it only when foreign keys are enabled on the
// connection, and a wipe that quietly depends on a PRAGMA being set is a wipe
// that leaves orphans on the day it is not.
//
// Three tables are deliberately absent:
//
//	settings         what the operator configured, including authorised scope
//	signal_registry  reference data describing what signals mean
//	range_cache      third-party address ranges, not observations of the estate
//
// The button promises to erase discovered data and preserve configuration.
// Clearing the registry would leave every future observation unmapped, and
// clearing settings would revoke the authorisation the operator granted —
// neither is data this engine discovered, and neither is the operator's to
// lose by pressing a button labelled "start over".
var erasedTables = []string{
	"findings",
	"secret_findings",
	"asset_exposure_history",
	"posture_snapshots",
	"regressions",
	"signal_observations",
	"assessment_coverage",
	"assessment_runs",
	"asset_attribution",
	"attribution_provenance",
	"email_postures",
	"jobs",
	"assets",
}

// EraseDiscoveredData clears the estate in a single transaction.
func (s *SQLiteStore) EraseDiscoveredData(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin erase: %w", err)
	}
	// Rolled back unless the commit below succeeds, so a failure part way
	// leaves the estate as it was rather than half-erased.
	defer func() { _ = tx.Rollback() }()

	for _, table := range erasedTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("failed to erase %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit erase: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
