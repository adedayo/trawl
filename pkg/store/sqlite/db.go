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

func (s *SQLiteStore) migrate(ctx context.Context) error {
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
		category TEXT NOT NULL,
		proof TEXT,
		ai_annotation TEXT,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
	);

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
		dane_valid INTEGER DEFAULT 0
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
		mapped INTEGER NOT NULL DEFAULT 0,
		registry_version TEXT NOT NULL,
		library_version TEXT NOT NULL,
		observed_at DATETIME NOT NULL,
		first_seen DATETIME NOT NULL,
		last_seen DATETIME NOT NULL,
		UNIQUE(asset_id, signal_id),
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

	_, err := s.db.ExecContext(ctx, schema)
	return err
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
	"posture_snapshots",
	"regressions",
	"signal_observations",
	"assessment_coverage",
	"assessment_runs",
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
