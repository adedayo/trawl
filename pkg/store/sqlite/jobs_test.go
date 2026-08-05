package sqlite

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adedayo/trawl/pkg/store"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "trawl-test.db"))
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestJobQueue_Lifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	enqueued, err := s.EnqueueJob(ctx, "scan", []string{"example.com", "example.org"})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}
	if enqueued.Status != store.JobStatusPending {
		t.Fatalf("expected pending, got %q", enqueued.Status)
	}

	popped, err := s.PopJob(ctx, "scan")
	if err != nil {
		t.Fatalf("PopJob: %v", err)
	}
	if popped == nil {
		t.Fatal("expected a job, got nil")
	}
	if popped.ID != enqueued.ID {
		t.Fatalf("popped wrong job: %s != %s", popped.ID, enqueued.ID)
	}
	if popped.Status != store.JobStatusRunning {
		t.Fatalf("pop must transition to running, got %q", popped.Status)
	}
	if len(popped.Targets) != 2 || popped.Targets[0] != "example.com" {
		t.Fatalf("targets did not round-trip: %v", popped.Targets)
	}
	if popped.StartedAt == nil {
		t.Fatal("expected startedAt to be set on claim")
	}

	if err := s.CompleteJob(ctx, popped.ID, store.JobStatusCompleted, ""); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	done, err := s.GetJobs(ctx, store.JobStatusCompleted)
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(done) != 1 {
		t.Fatalf("expected 1 completed job, got %d", len(done))
	}
}

// An empty queue is the normal steady state. It must not be reported as an
// error, or every idle poll from a worker becomes a false alarm.
func TestJobQueue_EmptyQueueIsNotAnError(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	job, err := s.PopJob(ctx, "scan")
	if err != nil {
		t.Fatalf("expected no error on empty queue, got %v", err)
	}
	if job != nil {
		t.Fatalf("expected nil job on empty queue, got %+v", job)
	}
}

// Jobs must not be claimed across types: a scan worker polling must never be
// handed a secret-scan job, which would send the wrong tool at the target.
func TestJobQueue_PopIsScopedToType(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.EnqueueJob(ctx, "secret_scan", []string{"https://example.com/repo"}); err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	job, err := s.PopJob(ctx, "scan")
	if err != nil {
		t.Fatalf("PopJob: %v", err)
	}
	if job != nil {
		t.Fatalf("scan worker was handed a %q job", job.Type)
	}
}

// Two workers polling concurrently must never receive the same job, or the
// same target gets scanned twice and the results are double-counted.
func TestJobQueue_ConcurrentPopsAreExclusive(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const jobCount = 20
	for i := 0; i < jobCount; i++ {
		if _, err := s.EnqueueJob(ctx, "scan", []string{"example.com"}); err != nil {
			t.Fatalf("EnqueueJob: %v", err)
		}
	}

	const workers = 8
	var (
		mu   sync.Mutex
		seen = map[string]int{}
		wg   sync.WaitGroup
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				job, err := s.PopJob(ctx, "scan")
				if err != nil || job == nil {
					return
				}
				mu.Lock()
				seen[job.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(seen) != jobCount {
		t.Fatalf("expected %d distinct jobs claimed, got %d", jobCount, len(seen))
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("job %s was claimed %d times", id, count)
		}
	}
}

// A worker must not be able to quietly return a job to the queue by reporting
// a non-terminal status.
func TestJobQueue_CompleteRejectsNonTerminalStatus(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	job, err := s.EnqueueJob(ctx, "scan", []string{"example.com"})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	if err := s.CompleteJob(ctx, job.ID, store.JobStatusPending, ""); err == nil {
		t.Fatal("expected CompleteJob to reject a pending status")
	}
	if err := s.CompleteJob(ctx, "no-such-job", store.JobStatusCompleted, ""); err == nil {
		t.Fatal("expected CompleteJob to reject an unknown job id")
	}
}
