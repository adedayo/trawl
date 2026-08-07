package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// RangeCache is a durable, process-shared cache of third-party data files —
// cloud provider address ranges today, and anything else with a freshness
// window later.
//
// It satisfies vantage's netattr.RangeStore. Backing it with the same database
// as everything else means a portfolio scan downloads several megabytes once
// rather than once per assessment, and that a restart does not throw the work
// away.
//
// Freshness is deliberately not enforced here. The store returns whatever it
// holds along with the age, and the caller decides whether that age is
// acceptable — because only the caller knows whether stale data beats no data.
// For attribution it does: last week's ranges still name the right operator,
// whereas attributing nothing would silently retract findings that were
// correct yesterday.
type RangeCache struct {
	db *sql.DB
}

// NewRangeCache builds a cache over an open database.
func NewRangeCache(db *sql.DB) *RangeCache { return &RangeCache{db: db} }

// Get returns cached content and when it was obtained, at any age.
func (c *RangeCache) Get(ctx context.Context, url string) ([]byte, time.Time, bool) {
	if c == nil || c.db == nil {
		return nil, time.Time{}, false
	}
	var (
		data    []byte
		fetched string
	)
	err := c.db.QueryRowContext(ctx,
		`SELECT content, fetched_at FROM range_cache WHERE url = ?`, url,
	).Scan(&data, &fetched)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			// A cache read failure is not worth propagating: the caller will
			// fetch instead, which is the correct behaviour anyway.
			return nil, time.Time{}, false
		}
		return nil, time.Time{}, false
	}

	at, err := time.Parse(time.RFC3339Nano, fetched)
	if err != nil {
		// An unparseable timestamp means the age cannot be disclosed, and
		// undisclosed staleness is worse than a refetch.
		return nil, time.Time{}, false
	}
	return data, at, true
}

// Put records content obtained at the given time.
func (c *RangeCache) Put(ctx context.Context, url string, data []byte, at time.Time) error {
	if c == nil || c.db == nil {
		return nil
	}
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO range_cache (url, content, fetched_at) VALUES (?, ?, ?)
		 ON CONFLICT(url) DO UPDATE SET content = excluded.content, fetched_at = excluded.fetched_at`,
		url, data, at.UTC().Format(time.RFC3339Nano))
	return err
}

// PurgeRangeCacheOlderThan removes entries beyond an age, so an abandoned
// endpoint does not keep stale data alive indefinitely.
func (c *RangeCache) PurgeRangeCacheOlderThan(ctx context.Context, maxAge time.Duration) error {
	if c == nil || c.db == nil {
		return nil
	}
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339Nano)
	_, err := c.db.ExecContext(ctx, `DELETE FROM range_cache WHERE fetched_at < ?`, cutoff)
	return err
}
