package orchestrator

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/notification"
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

func TestUpdateJobStateSendsNotification(t *testing.T) {
	t.Parallel()

	recorder := &recordingNotifier{}
	orch := newTestOrchestratorWithNotifier(t, recorder)
	job := domain.Job{
		ID:           "job-notify",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 8,
		State:        domain.StateDesignRunning,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.UpdateJobState(context.Background(), job.ID, domain.StateDesignReady, "design_ready", map[string]any{"skill": "design"}); err != nil {
		t.Fatalf("UpdateJobState() error = %v", err)
	}
	if len(recorder.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(recorder.notifications))
	}
	if recorder.notifications[0].Event != "design_ready" {
		t.Fatalf("expected design_ready notification, got %q", recorder.notifications[0].Event)
	}
}

func TestUpdateJobStateIgnoresNotificationFailure(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestratorWithNotifier(t, failingNotifier{})
	job := domain.Job{
		ID:           "job-notify-fail",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 9,
		State:        domain.StateDesignRunning,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.UpdateJobState(context.Background(), job.ID, domain.StateDesignReady, "design_ready", nil); err != nil {
		t.Fatalf("UpdateJobState() error = %v", err)
	}
}

func TestApproveFinalAllowedFromWaitingFinalApproval(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-final-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 10,
		State:        domain.StateWaitingFinalApproval,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.ApproveFinal(context.Background(), job.ID, "ship it"); err != nil {
		t.Fatalf("ApproveFinal() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StatePRCreating {
		t.Fatalf("expected pr_creating, got %s", saved.State)
	}
}

func TestApproveFinalAllowedAfterTestFailed(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-final-2",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 11,
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
		EventType: "test_failed",
		StateFrom: string(domain.StateTestRunning),
		StateTo:   string(domain.StateFailed),
		Payload:   "{}",
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if err := orch.ApproveFinal(context.Background(), job.ID, "ship with known test failure"); err != nil {
		t.Fatalf("ApproveFinal() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StatePRCreating {
		t.Fatalf("expected pr_creating, got %s", saved.State)
	}
}

func TestApproveFinalRejectedFromOtherFailedStates(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-final-3",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 12,
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

	err := orch.ApproveFinal(context.Background(), job.ID, "should fail")
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func newTestOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	return newTestOrchestratorWithNotifier(t, notification.NewNopNotifier())
}

func newTestOrchestratorWithNotifier(t *testing.T, notifier notification.Notifier) *Orchestrator {
	t.Helper()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store, notifier)
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

type recordingNotifier struct {
	notifications []notification.Notification
}

func (n *recordingNotifier) Notify(_ context.Context, event notification.Notification) error {
	n.notifications = append(n.notifications, event)
	return nil
}

type failingNotifier struct{}

func (failingNotifier) Notify(context.Context, notification.Notification) error {
	return errors.New("boom")
}
