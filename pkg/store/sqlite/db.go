package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore initializes SQLite database with WAL mode pragmas and auto-migrations.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
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

	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
