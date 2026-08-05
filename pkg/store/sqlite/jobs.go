package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/adedayo/trawl/pkg/store"
	"github.com/google/uuid"
)

// EnqueueJob records a new pending job for a worker to claim.
func (s *SQLiteStore) EnqueueJob(ctx context.Context, jobType string, targets []string) (*store.Job, error) {
	if jobType == "" {
		return nil, fmt.Errorf("job type is required")
	}
	if targets == nil {
		targets = []string{}
	}

	encoded, err := json.Marshal(targets)
	if err != nil {
		return nil, fmt.Errorf("failed to encode job targets: %w", err)
	}

	job := &store.Job{
		ID:        uuid.NewString(),
		Type:      jobType,
		Status:    store.JobStatusPending,
		Targets:   targets,
		CreatedAt: time.Now().UTC(),
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO jobs (id, type, status, targets, created_at) VALUES (?, ?, ?, ?, ?)`,
		job.ID, job.Type, string(job.Status), string(encoded), job.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return job, nil
}

// PopJob atomically claims the oldest pending job of the given type.
//
// The claim is a single UPDATE ... RETURNING so that two workers polling
// concurrently cannot be handed the same job. An empty queue returns
// (nil, nil): it is the normal steady state, not a failure.
func (s *SQLiteStore) PopJob(ctx context.Context, jobType string) (*store.Job, error) {
	if jobType == "" {
		return nil, fmt.Errorf("job type is required")
	}

	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = ?, started_at = ?
		WHERE id = (
			SELECT id FROM jobs
			WHERE type = ? AND status = ?
			ORDER BY created_at
			LIMIT 1
		)
		RETURNING id, type, status, targets, created_at, started_at`,
		string(store.JobStatusRunning), now, jobType, string(store.JobStatusPending),
	)

	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to pop job: %w", err)
	}
	return job, nil
}

// CompleteJob marks a claimed job terminal. Only "completed" and "failed"
// are accepted, so a worker cannot quietly return a job to the queue.
func (s *SQLiteStore) CompleteJob(ctx context.Context, jobID string, status store.JobStatus, errMsg string) error {
	if status != store.JobStatusCompleted && status != store.JobStatusFailed {
		return fmt.Errorf("invalid terminal job status: %q", status)
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, completed_at = ?, error = ? WHERE id = ?`,
		string(status), time.Now().UTC(), errMsg, jobID,
	)
	if err != nil {
		return fmt.Errorf("failed to complete job %s: %w", jobID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm job completion for %s: %w", jobID, err)
	}
	if affected == 0 {
		return fmt.Errorf("job %s not found", jobID)
	}
	return nil
}

// GetJobs returns jobs filtered by status. An empty status returns all jobs.
func (s *SQLiteStore) GetJobs(ctx context.Context, status store.JobStatus) ([]store.Job, error) {
	query := `SELECT id, type, status, targets, created_at, started_at FROM jobs`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, string(status))
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	jobs := []store.Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

// rowScanner unifies *sql.Row and *sql.Rows so the column mapping lives once.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(rs rowScanner) (*store.Job, error) {
	var (
		job       store.Job
		status    string
		targets   string
		startedAt sql.NullTime
	)

	if err := rs.Scan(&job.ID, &job.Type, &status, &targets, &job.CreatedAt, &startedAt); err != nil {
		return nil, err
	}

	job.Status = store.JobStatus(status)
	if startedAt.Valid {
		t := startedAt.Time
		job.StartedAt = &t
	}
	if err := json.Unmarshal([]byte(targets), &job.Targets); err != nil {
		return nil, fmt.Errorf("failed to decode targets for job %s: %w", job.ID, err)
	}
	if job.Targets == nil {
		job.Targets = []string{}
	}

	return &job, nil
}
