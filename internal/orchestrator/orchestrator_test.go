package orchestrator

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestRerunDesignAllowedFromWaitingDesignApproval(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 1,
		State:        domain.StateWaitingDesignApproval,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.RerunDesign(context.Background(), job.ID, "retry"); err != nil {
		t.Fatalf("RerunDesign() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateDetected {
		t.Fatalf("expected detected, got %s", saved.State)
	}
}

func TestRerunDesignRejectedFromOtherStates(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-2",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 2,
		State:        domain.StateImplementationRunning,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	err := orch.RerunDesign(context.Background(), job.ID, "retry")
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestRerunImplementationAllowedFromWaitingFinalApproval(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-3",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 3,
		State:        domain.StateWaitingFinalApproval,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.RerunImplementation(context.Background(), job.ID, "retry"); err != nil {
		t.Fatalf("RerunImplementation() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateImplementationRunning {
		t.Fatalf("expected implementation_running, got %s", saved.State)
	}
}

func TestRerunImplementationRejectedFromOtherStates(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-4",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 4,
		State:        domain.StateDesignReady,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	err := orch.RerunImplementation(context.Background(), job.ID, "retry")
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestRerunDesignFromEventAllowedFromFailedJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-5",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 5,
		State:        domain.StateFailed,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := orch.store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "design_started",
		StateFrom: string(domain.StateDetected),
		StateTo:   string(domain.StateDesignRunning),
		Payload:   "{}",
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	events, err := orch.store.ListEvents(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}

	if err := orch.RerunDesignFromEvent(context.Background(), job.ID, &events[0].ID, "retry"); err != nil {
		t.Fatalf("RerunDesignFromEvent() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateDetected {
		t.Fatalf("expected detected, got %s", saved.State)
	}
}

func TestRerunImplementationUsesLatestEventWhenFailed(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-6",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 6,
		State:        domain.StateFailed,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := orch.store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "implementation_failed",
		StateFrom: string(domain.StateImplementationRunning),
		StateTo:   string(domain.StateFailed),
		Payload:   "{}",
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if err := orch.RerunImplementation(context.Background(), job.ID, "retry"); err != nil {
		t.Fatalf("RerunImplementation() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateImplementationRunning {
		t.Fatalf("expected implementation_running, got %s", saved.State)
	}
}

func TestRerunPRCreationFromEventAllowedFromFailedJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-7",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 7,
		State:        domain.StateFailed,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := orch.store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "pr_create_failed",
		StateFrom: string(domain.StatePRCreating),
		StateTo:   string(domain.StateFailed),
		Payload:   "{}",
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	events, err := orch.store.ListEvents(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}

	if err := orch.RerunPRCreationFromEvent(context.Background(), job.ID, &events[0].ID, "retry"); err != nil {
		t.Fatalf("RerunPRCreationFromEvent() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StatePRCreating {
		t.Fatalf("expected pr_creating, got %s", saved.State)
	}
}

func newTestOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store)
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
