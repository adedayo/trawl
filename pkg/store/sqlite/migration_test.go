package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/adedayo/trawl/pkg/store/sqlite"
)

// schemaAsShipped is the database as an earlier release created it: the
// current schema without any column that has since been added.
//
// It is frozen. Do not regenerate it from the live schema, and do not edit it
// to make a test pass — those are the two ways this guard stops working, and
// both leave it looking green.
//
// Freezing is the entire point. A test that derived the old shape from the
// current one could only ever confirm that the migration does what the
// migration does. Because this literal does not move, a column added to the
// CREATE TABLE block and forgotten in addedColumns shows up as a column the
// fresh database has and the migrated one does not — which is exactly the
// omission that would otherwise surface at run time, on an existing
// installation, as a failed write.
//
// Append to it only when a release ships, recording the shape that release
// created.
const schemaAsShipped = `
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

CREATE TABLE IF NOT EXISTS attribution_provenance (
	asset_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	url TEXT NOT NULL,
	fetched_at DATETIME NOT NULL,
	PRIMARY KEY(asset_id, provider, url),
	FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS range_cache (
	url TEXT PRIMARY KEY,
	content BLOB NOT NULL,
	fetched_at DATETIME NOT NULL
);
`

// The guard the existing tests did not provide. They assert the new columns
// arrive; they do not assert that nothing else differs, so a column added to
// the CREATE TABLE block and forgotten in addedColumns passed.
func TestAMigratedDatabaseHasTheSameShapeAsAFreshOne(t *testing.T) {
	migratedPath := seedOldDatabase(t)
	openAt(t, migratedPath).Close()

	freshPath := filepath.Join(t.TempDir(), "fresh.db")
	openAt(t, freshPath).Close()

	was := shapeOf(t, migratedPath)
	is := shapeOf(t, freshPath)

	for table, columns := range is {
		got, ok := was[table]
		if !ok {
			t.Errorf("table %q is missing from the migrated database", table)
			continue
		}
		if strings.Join(got, ",") != strings.Join(columns, ",") {
			t.Errorf("table %q:\n  migrated: %v\n  fresh:    %v\n"+
				"a column in the CREATE TABLE block has no entry in addedColumns, "+
				"so existing installations will not receive it", table, got, columns)
		}
	}
	for table := range was {
		if _, ok := is[table]; !ok {
			t.Errorf("table %q exists only in the migrated database", table)
		}
	}
}

func TestMigrationRecordsTheSchemaVersion(t *testing.T) {
	path := seedOldDatabase(t)

	s := openAt(t, path)
	s.Close()

	if got := userVersionAt(t, path); got != 5 {
		t.Errorf("user_version = %d, want 5", got)
	}
}

// A store written by a newer build and opened by an older one is the failure
// most likely to corrupt data quietly, because every individual query against
// the columns the old build knows about still succeeds.
func TestADatabaseFromTheFutureIsRefused(t *testing.T) {
	path := seedOldDatabase(t)
	setUserVersion(t, path, 99)

	_, err := sqlite.NewSQLiteStore(path)
	if err == nil {
		t.Fatal("opening a newer store succeeded; it should have been refused")
	}

	var newer *sqlite.ErrNewerSchema
	if !errors.As(err, &newer) {
		t.Fatalf("error = %v; want an ErrNewerSchema the caller can distinguish from an ordinary open failure", err)
	}
	if newer.Found != 99 || newer.Expected != 5 {
		t.Errorf("found = %d, expected = %d", newer.Found, newer.Expected)
	}

	// A store that will not open is the first thing a user sees, so the
	// message has to name the cause and the remedy rather than surface as a
	// failed query.
	msg := err.Error()
	for _, want := range []string{"newer version of Trawl", "Upgrade Trawl", "99", "5"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not mention %q", msg, want)
		}
	}
}

// Refusal must not be a side effect of having written to the store first.
func TestARefusedDatabaseIsNotWrittenTo(t *testing.T) {
	path := seedOldDatabase(t)
	setUserVersion(t, path, 99)

	if _, err := sqlite.NewSQLiteStore(path); err == nil {
		t.Fatal("expected refusal")
	}

	if got := userVersionAt(t, path); got != 99 {
		t.Errorf("user_version = %d; the refused open changed it", got)
	}
	if columnsOfTable(t, path, "signal_observations")["detail"] {
		t.Error("the refused open migrated the schema anyway")
	}
}

func TestOpeningIsIdempotent(t *testing.T) {
	path := seedOldDatabase(t)

	for i := range 3 {
		s, err := sqlite.NewSQLiteStore(path)
		if err != nil {
			t.Fatalf("open %d: %v", i+1, err)
		}
		s.Close()
	}

	if got := userVersionAt(t, path); got != 5 {
		t.Errorf("user_version = %d, want 5", got)
	}
}

// seedOldDatabase writes a database in the shape an earlier release created,
// with a row in it, and returns its path.
func seedOldDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(schemaAsShipped); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO assets (id, type, value, status, discovery_source, confidence, first_seen, last_seen)
		 VALUES ('a1','domain','example.test','active','seed',1.0,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatal(err)
	}
	return path
}

func openAt(t *testing.T, path string) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// shapeOf reads the column names of every table, sorted, so that comparison
// does not depend on the order columns happen to have been added in — which
// differs between a fresh database and a migrated one by construction.
func shapeOf(t *testing.T, path string) map[string][]string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	return readShape(t, db)
}

func readShape(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	ctx := context.Background()

	rows, err := db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	shape := map[string][]string{}
	for _, table := range tables {
		shape[table] = readColumns(t, db, table)
	}
	return shape
}

func readColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var (
			cid       int
			name, typ string
			notNull   int
			dflt      any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	return names
}

func columnsOfTable(t *testing.T, path, table string) map[string]bool {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	present := map[string]bool{}
	for _, name := range readColumns(t, db, table) {
		present[name] = true
	}
	return present
}

func userVersionAt(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func setUserVersion(t *testing.T, path string, version int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA user_version = " + strconv.Itoa(version)); err != nil {
		t.Fatal(err)
	}
}
