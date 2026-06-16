package sqlite

import (
	"context"
	"errors"
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

func TestPurgeJobRejectsActiveJobWithoutDeletingIt(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	job := domain.Job{
		ID:           "job-active-purge",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 7,
		State:        domain.StateDetected,
		Title:        "active",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := store.PurgeJob(context.Background(), job.ID); err == nil {
		t.Fatalf("expected PurgeJob() to reject active job")
	} else if !errors.Is(err, ErrJobNotDeleted) {
		t.Fatalf("expected ErrJobNotDeleted, got %v", err)
	}

	saved, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() after failed purge error = %v", err)
	}
	if saved.DeletedAt != nil {
		t.Fatalf("expected active job to remain active, got %+v", saved)
	}
}

func TestFindJobBySourceReturnsSavedJob(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	job := domain.Job{
		ID:           "job-source",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 99,
		State:        domain.StateDetected,
		Title:        "source",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	got, err := store.FindJobBySource(context.Background(), "owner/repo", 99, domain.JobTypeIssue)
	if err != nil {
		t.Fatalf("FindJobBySource() error = %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("FindJobBySource() = %q, want %q", got.ID, job.ID)
	}
}

func TestFindJobBySourceReturnsNotFoundError(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.FindJobBySource(context.Background(), "owner/repo", 1, domain.JobTypeIssue); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestGetEventReturnsInsertedEvent(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	job := domain.Job{
		ID:           "job-event",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 1,
		State:        domain.StateDetected,
		Title:        "event",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	event := domain.Event{
		JobID:     job.ID,
		EventType: "job_created",
		StateTo:   string(domain.StateDetected),
		Payload:   `{"body":"hello"}`,
		CreatedAt: nowUTC(),
	}
	if err := store.AppendEvent(context.Background(), event); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	events, err := store.ListEvents(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	got, err := store.GetEvent(context.Background(), events[0].ID)
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
	}
	if got.ID != events[0].ID || got.EventType != event.EventType {
		t.Fatalf("unexpected event: %#v", got)
	}
}

func TestGetEventReturnsNotFoundError(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.GetEvent(context.Background(), 999); err == nil || err.Error() != "event 999 not found" {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
