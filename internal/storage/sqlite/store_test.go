package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	job := domain.Job{
		ID:           "job-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 1,
		State:        domain.StateDetected,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	jobs, err := store.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
}

func TestListJobsByFilterRespectsDeletedAt(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	activeJob := domain.Job{
		ID:           "job-active",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 1,
		State:        domain.StateDetected,
		Title:        "active",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	deletedAt := nowUTC()
	deletedJob := domain.Job{
		ID:           "job-deleted",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 2,
		State:        domain.StateCompleted,
		Title:        "deleted",
		DeletedAt:    &deletedAt,
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), activeJob); err != nil {
		t.Fatalf("UpsertJob(active) error = %v", err)
	}
	if err := store.UpsertJob(context.Background(), deletedJob); err != nil {
		t.Fatalf("UpsertJob(deleted) error = %v", err)
	}

	active, err := store.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(active) != 1 || active[0].ID != activeJob.ID {
		t.Fatalf("expected only active job, got %+v", active)
	}

	deleted, err := store.ListJobsByFilter(context.Background(), JobListDeletedOnly)
	if err != nil {
		t.Fatalf("ListJobsByFilter(deleted) error = %v", err)
	}
	if len(deleted) != 1 || deleted[0].ID != deletedJob.ID || deleted[0].DeletedAt == nil {
		t.Fatalf("expected only deleted job, got %+v", deleted)
	}

	all, err := store.ListJobsByFilter(context.Background(), JobListAll)
	if err != nil {
		t.Fatalf("ListJobsByFilter(all) error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(all))
	}
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
