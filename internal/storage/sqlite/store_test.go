package sqlite

import (
	"context"
	"os"
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

func TestPurgeJobRemovesJobAndEventsButKeepsArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	deletedAt := nowUTC()
	job := domain.Job{
		ID:           "job-purge",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 3,
		State:        domain.StateCompleted,
		Title:        "purge",
		DeletedAt:    &deletedAt,
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "job_deleted",
		StateFrom: string(domain.StateCompleted),
		StateTo:   string(domain.StateCompleted),
		Payload:   `{"deletedAt":"2026-05-19T00:00:00Z"}`,
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	artifactDir := filepath.Join(root, "artifacts", job.ID, "design")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(artifactDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "result.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.txt) error = %v", err)
	}

	if err := store.PurgeJob(context.Background(), job.ID); err != nil {
		t.Fatalf("PurgeJob() error = %v", err)
	}

	if _, err := store.GetJob(context.Background(), job.ID); err == nil {
		t.Fatalf("expected GetJob() to fail after purge")
	}

	events, err := store.ListEvents(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected events to be removed, got %+v", events)
	}

	if _, err := os.Stat(filepath.Join(artifactDir, "result.txt")); err != nil {
		t.Fatalf("expected artifact to remain, stat error = %v", err)
	}
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
