package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/adedayo/vantage/pkg/netattr"
)

// The cache is only useful if vantage will accept it. Asserting the interface
// here means a change to netattr's contract fails at compile time rather than
// silently falling back to per-assessment downloads.
var _ netattr.RangeStore = (*RangeCache)(nil)

func newRangeCache(t *testing.T) *RangeCache {
	t.Helper()
	s, err := NewSQLiteStore(t.TempDir() + "/trawl.db")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return NewRangeCache(s.db)
}

func TestRangeCacheRoundTripsContentAndAge(t *testing.T) {
	c := newRangeCache(t)
	ctx := context.Background()

	// Truncated because SQLite stores a formatted string, and the age reported
	// to an operator only needs to be honest, not nanosecond-exact.
	when := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Millisecond)
	body := []byte(`{"prefixes":["203.0.113.0/24"]}`)

	if err := c.Put(ctx, "https://example.test/ranges.json", body, when); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, at, ok := c.Get(ctx, "https://example.test/ranges.json")
	if !ok {
		t.Fatal("expected a cache hit")
	}
	if string(got) != string(body) {
		t.Errorf("content is %q, wanted it returned verbatim", got)
	}
	if !at.Equal(when) {
		t.Errorf("fetched_at is %v, want %v", at, when)
	}
}

// Stale entries must still be returned. Withholding week-old provider ranges
// would silently retract attributions that were correct yesterday; the caller
// decides whether the age is acceptable, having been told what it is.
func TestRangeCacheReturnsStaleEntriesWithTheirAge(t *testing.T) {
	c := newRangeCache(t)
	ctx := context.Background()

	old := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(time.Millisecond)
	if err := c.Put(ctx, "https://example.test/old.json", []byte("x"), old); err != nil {
		t.Fatalf("Put: %v", err)
	}

	_, at, ok := c.Get(ctx, "https://example.test/old.json")
	if !ok {
		t.Fatal("a stale entry must still be offered, with its age disclosed")
	}
	if time.Since(at) < 29*24*time.Hour {
		t.Errorf("reported age of %v understates the true staleness", time.Since(at))
	}
}

func TestRangeCacheMissIsNotAnError(t *testing.T) {
	c := newRangeCache(t)
	if _, _, ok := c.Get(context.Background(), "https://example.test/absent.json"); ok {
		t.Error("expected a miss for an unseen URL")
	}
}

func TestRangeCacheRefreshReplacesContent(t *testing.T) {
	c := newRangeCache(t)
	ctx := context.Background()
	url := "https://example.test/ranges.json"

	first := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Millisecond)
	if err := c.Put(ctx, url, []byte("old"), first); err != nil {
		t.Fatalf("Put: %v", err)
	}
	second := time.Now().UTC().Truncate(time.Millisecond)
	if err := c.Put(ctx, url, []byte("new"), second); err != nil {
		t.Fatalf("Put (refresh): %v", err)
	}

	got, at, ok := c.Get(ctx, url)
	if !ok {
		t.Fatal("expected a hit after refresh")
	}
	if string(got) != "new" {
		t.Errorf("content is %q, want the refreshed body", got)
	}
	if !at.Equal(second) {
		t.Errorf("fetched_at is %v, want the refresh time %v", at, second)
	}
}

// An abandoned endpoint should not keep its data alive forever, but a fresh one
// must survive the purge.
func TestPurgeRemovesOnlyExpiredEntries(t *testing.T) {
	c := newRangeCache(t)
	ctx := context.Background()

	if err := c.Put(ctx, "stale", []byte("s"), time.Now().UTC().Add(-90*24*time.Hour)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := c.Put(ctx, "fresh", []byte("f"), time.Now().UTC()); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := c.PurgeRangeCacheOlderThan(ctx, 30*24*time.Hour); err != nil {
		t.Fatalf("PurgeRangeCacheOlderThan: %v", err)
	}

	if _, _, ok := c.Get(ctx, "stale"); ok {
		t.Error("expected the expired entry to be purged")
	}
	if _, _, ok := c.Get(ctx, "fresh"); !ok {
		t.Error("the purge removed an entry that was still within its window")
	}
}

// A nil cache must behave as "no cache", not panic. Callers that run without a
// database should degrade to fetching, which is correct if slower.
func TestNilRangeCacheDegradesToNoCache(t *testing.T) {
	var c *RangeCache
	if _, _, ok := c.Get(context.Background(), "anything"); ok {
		t.Error("a nil cache must never report a hit")
	}
	if err := c.Put(context.Background(), "anything", []byte("x"), time.Now()); err != nil {
		t.Errorf("a nil cache must accept writes silently, got %v", err)
	}
}
