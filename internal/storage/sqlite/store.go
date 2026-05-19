package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type Store struct {
	db *sql.DB
}

type JobListFilter string

const (
	JobListActiveOnly  JobListFilter = "active"
	JobListDeletedOnly JobListFilter = "deleted"
	JobListAll         JobListFilter = "all"
)

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) EnsureSeedData(ctx context.Context) error {
	return nil
}

func (s *Store) UpsertJob(ctx context.Context, job domain.Job) error {
	var deletedAt any
	if job.DeletedAt != nil {
		deletedAt = job.DeletedAt.UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, type, repository, github_number, state, title, branch_name, watch_rule_id, deleted_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			repository = excluded.repository,
			github_number = excluded.github_number,
			state = excluded.state,
			title = excluded.title,
			branch_name = excluded.branch_name,
			watch_rule_id = excluded.watch_rule_id,
			deleted_at = excluded.deleted_at,
			updated_at = excluded.updated_at
	`, job.ID, string(job.Type), job.Repository, job.GitHubNumber, string(job.State), job.Title, job.BranchName, job.WatchRuleID, deletedAt, job.CreatedAt.UTC(), job.UpdatedAt.UTC())
	return err
}

func (s *Store) ListJobs(ctx context.Context) ([]domain.Job, error) {
	return s.ListJobsByFilter(ctx, JobListActiveOnly)
}

func (s *Store) ListJobsByFilter(ctx context.Context, filter JobListFilter) ([]domain.Job, error) {
	whereClause := "WHERE deleted_at IS NULL"
	switch filter {
	case JobListDeletedOnly:
		whereClause = "WHERE deleted_at IS NOT NULL"
	case JobListAll:
		whereClause = ""
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, repository, github_number, state, title, branch_name, watch_rule_id, deleted_at, created_at, updated_at
		FROM jobs
		`+whereClause+`
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) GetJob(ctx context.Context, jobID string) (domain.Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, repository, github_number, state, title, branch_name, watch_rule_id, deleted_at, created_at, updated_at
		FROM jobs WHERE id = ?
	`, jobID)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Job{}, fmt.Errorf("job %q not found", jobID)
	}
	if err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (s *Store) FindJobBySource(ctx context.Context, repository string, githubNumber int, jobType domain.JobType) (domain.Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, repository, github_number, state, title, branch_name, watch_rule_id, deleted_at, created_at, updated_at
		FROM jobs
		WHERE repository = ? AND github_number = ? AND type = ?
		ORDER BY created_at ASC
		LIMIT 1
	`, repository, githubNumber, string(jobType))
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Job{}, fmt.Errorf("%w: %s#%d (%s)", domain.ErrJobNotFound, repository, githubNumber, jobType)
	}
	if err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (s *Store) AppendEvent(ctx context.Context, event domain.Event) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO job_events (job_id, event_type, state_from, state_to, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, event.JobID, event.EventType, event.StateFrom, event.StateTo, event.Payload, event.CreatedAt.UTC())
	return err
}

func (s *Store) ListEvents(ctx context.Context, jobID string) ([]domain.Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job_id, event_type, state_from, state_to, payload_json, created_at
		FROM job_events WHERE job_id = ?
		ORDER BY created_at ASC, id ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(&event.ID, &event.JobID, &event.EventType, &event.StateFrom, &event.StateTo, &event.Payload, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) GetEvent(ctx context.Context, eventID int64) (domain.Event, error) {
	var event domain.Event
	err := s.db.QueryRowContext(ctx, `
		SELECT id, job_id, event_type, state_from, state_to, payload_json, created_at
		FROM job_events
		WHERE id = ?
	`, eventID).Scan(&event.ID, &event.JobID, &event.EventType, &event.StateFrom, &event.StateTo, &event.Payload, &event.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Event{}, fmt.Errorf("event %d not found", eventID)
	}
	if err != nil {
		return domain.Event{}, err
	}
	return event, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			repository TEXT NOT NULL,
			github_number INTEGER NOT NULL,
			state TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			branch_name TEXT NOT NULL DEFAULT '',
			watch_rule_id TEXT NOT NULL DEFAULT '',
			deleted_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`ALTER TABLE jobs ADD COLUMN deleted_at TIMESTAMP NULL`,
		`CREATE TABLE IF NOT EXISTS job_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			state_from TEXT NOT NULL DEFAULT '',
			state_to TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			created_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_job_events_job_id_created_at ON job_events(job_id, created_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_repository_number_type ON jobs(repository, github_number, type)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			if strings.Contains(stmt, "ALTER TABLE jobs ADD COLUMN deleted_at") && strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

type jobScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner jobScanner) (domain.Job, error) {
	var job domain.Job
	var deletedAt sql.NullTime
	var typ, state string
	if err := scanner.Scan(&job.ID, &typ, &job.Repository, &job.GitHubNumber, &state, &job.Title, &job.BranchName, &job.WatchRuleID, &deletedAt, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return domain.Job{}, err
	}
	job.Type = domain.JobType(typ)
	job.State = domain.JobState(state)
	if deletedAt.Valid {
		value := deletedAt.Time.UTC()
		job.DeletedAt = &value
	}
	return job, nil
}
