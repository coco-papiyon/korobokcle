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
