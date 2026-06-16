package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/notification"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestProcessMatchCreatesPRFeedbackJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{
		ID:   "rule-feedback",
		Name: "PR feedback",
	}
	event := domain.DomainEvent{
		Type:   domain.DomainEventPRReviewMatched,
		RuleID: rule.ID,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     42,
			Title:      "Refactor API",
			Target:     domain.TargetPullRequestReview,
			BranchName: "feature/pr-42",
			ReviewComments: []domain.ReviewComment{
				{ID: 1, Author: "reviewer", Body: "rename this"},
			},
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("ProcessMatch() error = %v", err)
	}

	jobs, err := orch.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Type != domain.JobTypePRFeedback {
		t.Fatalf("expected pr_feedback, got %s", jobs[0].Type)
	}
	if jobs[0].State != domain.StateImplementationRunning {
		t.Fatalf("expected implementation_running, got %s", jobs[0].State)
	}
	if jobs[0].BranchName != "feature/pr-42" {
		t.Fatalf("expected branch feature/pr-42, got %q", jobs[0].BranchName)
	}

	_, events, err := orch.JobDetail(context.Background(), jobs[0].ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if len(events) != 1 || events[0].EventType != string(domain.DomainEventPRReviewMatched) {
		t.Fatalf("unexpected events: %+v", events)
	}

	var payload struct {
		ReviewComments []domain.ReviewComment `json:"reviewComments"`
	}
	if err := json.Unmarshal([]byte(events[0].Payload), &payload); err != nil {
		t.Fatalf("Unmarshal(payload) error = %v", err)
	}
	if len(payload.ReviewComments) != 1 || payload.ReviewComments[0].Body != "rename this" {
		t.Fatalf("unexpected review comments payload: %+v", payload.ReviewComments)
	}
}

func TestProcessMatchCreatesPRReviewJobUsesHeadBranch(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-pr-review", Name: "PR review"}
	event := domain.DomainEvent{
		Type:   domain.DomainEventPRMatched,
		RuleID: rule.ID,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     99,
			Title:      "Add feature",
			Target:     domain.TargetPullRequest,
			BranchName: "feature/add-feature",
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("ProcessMatch() error = %v", err)
	}

	jobs, err := orch.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Type != domain.JobTypePRReview {
		t.Fatalf("expected pr_review, got %s", jobs[0].Type)
	}
	if jobs[0].BranchName != "feature/add-feature" {
		t.Fatalf("expected branch feature/add-feature, got %q", jobs[0].BranchName)
	}
}

func TestProcessMatchUpdatesExistingPRReviewJobBranch(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-pr-review-1",
		Type:         domain.JobTypePRReview,
		Repository:   "owner/repo",
		GitHubNumber: 99,
		State:        domain.StateCollectingContext,
		Title:        "Add feature",
		BranchName:   "korobokcle/pr-review-99",
		WatchRuleID:  "rule-pr-review",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-pr-review", Name: "PR review"}
	event := domain.DomainEvent{
		Type:   domain.DomainEventPRMatched,
		RuleID: rule.ID,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     99,
			Title:      "Add feature",
			Target:     domain.TargetPullRequest,
			BranchName: "issue_97",
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("ProcessMatch() error = %v", err)
	}

	saved, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.BranchName != "issue_97" {
		t.Fatalf("expected branch issue_97, got %q", saved.BranchName)
	}
	if len(events) != 0 {
		t.Fatalf("expected no new events for updated PR review job, got %+v", events)
	}
}

func TestProcessMatchRestartsIdlePRFeedbackJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-feedback-1",
		Type:         domain.JobTypePRFeedback,
		Repository:   "owner/repo",
		GitHubNumber: 42,
		State:        domain.StateWaitingFinalApproval,
		Title:        "old title",
		BranchName:   "old-branch",
		WatchRuleID:  "old-rule",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-feedback", Name: "PR feedback"}
	event := domain.DomainEvent{
		Type: domain.DomainEventPRReviewMatched,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     42,
			Title:      "new title",
			Target:     domain.TargetPullRequestReview,
			BranchName: "feature/pr-42",
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("ProcessMatch() error = %v", err)
	}

	saved, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateImplementationRunning {
		t.Fatalf("expected implementation_running, got %s", saved.State)
	}
	if saved.Title != "new title" || saved.BranchName != "feature/pr-42" || saved.WatchRuleID != "rule-feedback" {
		t.Fatalf("unexpected updated job: %+v", saved)
	}
	if len(events) != 1 || events[0].StateTo != string(domain.StateImplementationRunning) {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestProcessMatchSkipsDuplicatePRFeedbackEvent(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-feedback", Name: "PR feedback"}
	event := domain.DomainEvent{
		Type: domain.DomainEventPRReviewMatched,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     42,
			Title:      "Refactor API",
			Target:     domain.TargetPullRequestReview,
			BranchName: "feature/pr-42",
			ReviewComments: []domain.ReviewComment{
				{ID: 1001, Author: "reviewer", Body: "rename this"},
			},
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("first ProcessMatch() error = %v", err)
	}

	jobs, err := orch.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	jobID := jobs[0].ID

	_, eventsBefore, err := orch.JobDetail(context.Background(), jobID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if len(eventsBefore) != 1 {
		t.Fatalf("expected 1 event before duplicate, got %d", len(eventsBefore))
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("second ProcessMatch() error = %v", err)
	}

	savedAfter, eventsAfter, err := orch.JobDetail(context.Background(), jobID)
	if err != nil {
		t.Fatalf("JobDetail() after duplicate error = %v", err)
	}
	if savedAfter.State != domain.StateImplementationRunning {
		t.Fatalf("expected implementation_running after duplicate, got %s", savedAfter.State)
	}
	if len(eventsAfter) != 1 {
		t.Fatalf("expected duplicate event to be skipped, got %d events", len(eventsAfter))
	}
}

func TestProcessMatchDoesNotRestoreDeletedPRReviewJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	deletedAt := nowUTC()
	job := domain.Job{
		ID:           "job-pr-review-deleted",
		Type:         domain.JobTypePRReview,
		Repository:   "owner/repo",
		GitHubNumber: 55,
		State:        domain.StateCompleted,
		Title:        "existing review",
		DeletedAt:    &deletedAt,
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-pr-review", Name: "PR review"}
	event := domain.DomainEvent{
		Type: domain.DomainEventPRMatched,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     55,
			Title:      "existing review updated",
			Target:     domain.TargetPullRequest,
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("ProcessMatch() error = %v", err)
	}

	saved, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.DeletedAt == nil {
		t.Fatalf("expected deleted job to stay deleted, got %+v", saved)
	}
	if saved.State != domain.StateCompleted {
		t.Fatalf("expected completed state to remain, got %s", saved.State)
	}
	if len(events) != 0 {
		t.Fatalf("expected no new events for deleted job, got %+v", events)
	}
}

func TestProcessMatchDoesNotRestoreDeletedPRFeedbackJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	deletedAt := nowUTC()
	job := domain.Job{
		ID:           "job-pr-feedback-deleted",
		Type:         domain.JobTypePRFeedback,
		Repository:   "owner/repo",
		GitHubNumber: 56,
		State:        domain.StateCompleted,
		Title:        "existing feedback",
		DeletedAt:    &deletedAt,
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-feedback", Name: "PR feedback"}
	event := domain.DomainEvent{
		Type: domain.DomainEventPRReviewMatched,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     56,
			Title:      "existing feedback updated",
			Target:     domain.TargetPullRequestReview,
			BranchName: "feature/pr-56",
			ReviewComments: []domain.ReviewComment{
				{ID: 2001, Author: "reviewer", Body: "new feedback"},
			},
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("ProcessMatch() error = %v", err)
	}

	saved, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.DeletedAt == nil {
		t.Fatalf("expected deleted job to stay deleted, got %+v", saved)
	}
	if saved.State != domain.StateCompleted {
		t.Fatalf("expected completed state to remain, got %s", saved.State)
	}
	if len(events) != 0 {
		t.Fatalf("expected no new events for deleted job, got %+v", events)
	}
}

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

func TestRerunDesignUsesLatestEventWhenInterrupted(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-5-interrupted",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 55,
		State:        domain.StateInterrupted,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := orch.store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "design_interrupted",
		StateFrom: string(domain.StateDesignRunning),
		StateTo:   string(domain.StateInterrupted),
		Payload:   `{"reason":"startup_recovery"}`,
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
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

func TestRerunImplementationUsesLatestEventWhenInterrupted(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-6-interrupted",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 66,
		State:        domain.StateInterrupted,
		Title:        "test job",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := orch.store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "implementation_interrupted",
		StateFrom: string(domain.StateImplementationRunning),
		StateTo:   string(domain.StateInterrupted),
		Payload:   `{"reason":"startup_recovery"}`,
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

func TestApproveReviewAllowedFromReviewReady(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-review-1",
		Type:         domain.JobTypePRReview,
		Repository:   "owner/repo",
		GitHubNumber: 21,
		State:        domain.StateReviewReady,
		Title:        "test review",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.ApproveReview(context.Background(), job.ID); err != nil {
		t.Fatalf("ApproveReview() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateCompleted {
		t.Fatalf("expected completed, got %s", saved.State)
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

func TestDeleteAndRestoreJob(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	job := domain.Job{
		ID:           "job-delete-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 99,
		State:        domain.StateCompleted,
		Title:        "delete me",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	if err := orch.DeleteJob(context.Background(), job.ID); err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}

	activeJobs, err := orch.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(activeJobs) != 0 {
		t.Fatalf("expected deleted job to be hidden, got %+v", activeJobs)
	}

	deletedJobs, err := orch.ListJobsByFilter(context.Background(), JobListDeletedOnly)
	if err != nil {
		t.Fatalf("ListJobsByFilter(deleted) error = %v", err)
	}
	if len(deletedJobs) != 1 || deletedJobs[0].DeletedAt == nil {
		t.Fatalf("expected deleted job, got %+v", deletedJobs)
	}

	if err := orch.RestoreJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RestoreJob() error = %v", err)
	}

	restored, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if restored.DeletedAt != nil {
		t.Fatalf("expected restored job to clear DeletedAt, got %+v", restored)
	}
	if len(events) != 2 || events[0].EventType != "job_deleted" || events[1].EventType != "job_restored" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestPurgeDeletedJobRemovesJobAndEvents(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	deletedAt := nowUTC()
	job := domain.Job{
		ID:           "job-purge-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 100,
		State:        domain.StateCompleted,
		Title:        "purge me",
		DeletedAt:    &deletedAt,
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := orch.store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := orch.store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "job_deleted",
		StateFrom: string(domain.StateCompleted),
		StateTo:   string(domain.StateCompleted),
		Payload:   `{"deletedAt":"2026-05-19T00:00:00Z"}`,
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if err := orch.PurgeJob(context.Background(), job.ID); err != nil {
		t.Fatalf("PurgeJob() error = %v", err)
	}

	if _, err := orch.store.GetJob(context.Background(), job.ID); err == nil {
		t.Fatalf("expected GetJob() to fail after purge")
	}

	events, err := orch.store.ListEvents(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected events to be removed, got %+v", events)
	}
}

func TestProcessMatchAfterPurgeUsesFreshJobID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	orch := New(store, notification.NewNopNotifier())
	appConfig := config.DefaultFiles().App
	rule := config.WatchRule{ID: "rule-issue", Name: "Issue"}
	event := domain.DomainEvent{
		Type: domain.DomainEventIssueMatched,
		Item: domain.RepositoryItem{
			Repository: "owner/repo",
			Number:     101,
			Title:      "issue",
			Target:     domain.TargetIssue,
		},
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("first ProcessMatch() error = %v", err)
	}

	jobs, err := orch.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	oldJobID := jobs[0].ID

	if err := orch.DeleteJob(context.Background(), oldJobID); err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}
	if err := orch.PurgeJob(context.Background(), oldJobID); err != nil {
		t.Fatalf("PurgeJob() error = %v", err)
	}

	oldArtifactDir := artifacts.WorkerDir(root, appConfig.ArtifactsDir, oldJobID, artifacts.WorkerDesign)
	if err := os.MkdirAll(oldArtifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(oldArtifactDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldArtifactDir, "result.md"), []byte("stale artifact"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	if err := orch.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("second ProcessMatch() error = %v", err)
	}

	jobs, err = orch.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() after purge error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 recreated job, got %d", len(jobs))
	}
	if jobs[0].ID == oldJobID {
		t.Fatalf("expected fresh job ID after purge, reused %q", oldJobID)
	}

	job, events, err := orch.JobDetail(context.Background(), jobs[0].ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if job.ID != jobs[0].ID {
		t.Fatalf("unexpected job detail id: got %q want %q", job.ID, jobs[0].ID)
	}
	if len(events) == 0 {
		t.Fatalf("expected job events for recreated job")
	}
}

func TestRecoverInterruptedJobs(t *testing.T) {
	t.Parallel()

	orch := newTestOrchestrator(t)
	jobs := []domain.Job{
		{
			ID:           "job-design-running",
			Type:         domain.JobTypeIssue,
			Repository:   "owner/repo",
			GitHubNumber: 21,
			State:        domain.StateDesignRunning,
			Title:        "design",
			CreatedAt:    nowUTC(),
			UpdatedAt:    nowUTC(),
		},
		{
			ID:           "job-pr-creating",
			Type:         domain.JobTypeIssue,
			Repository:   "owner/repo",
			GitHubNumber: 22,
			State:        domain.StatePRCreating,
			Title:        "pr",
			CreatedAt:    nowUTC(),
			UpdatedAt:    nowUTC(),
		},
		{
			ID:           "job-waiting-final",
			Type:         domain.JobTypeIssue,
			Repository:   "owner/repo",
			GitHubNumber: 23,
			State:        domain.StateWaitingFinalApproval,
			Title:        "waiting",
			CreatedAt:    nowUTC(),
			UpdatedAt:    nowUTC(),
		},
	}
	for _, job := range jobs {
		if err := orch.store.UpsertJob(context.Background(), job); err != nil {
			t.Fatalf("UpsertJob() error = %v", err)
		}
	}

	recovered, err := orch.RecoverInterruptedJobs(context.Background())
	if err != nil {
		t.Fatalf("RecoverInterruptedJobs() error = %v", err)
	}
	if recovered != 2 {
		t.Fatalf("expected 2 recovered jobs, got %d", recovered)
	}

	designJob, designEvents, err := orch.JobDetail(context.Background(), "job-design-running")
	if err != nil {
		t.Fatalf("JobDetail(design) error = %v", err)
	}
	if designJob.State != domain.StateInterrupted {
		t.Fatalf("expected interrupted state, got %s", designJob.State)
	}
	if len(designEvents) == 0 || designEvents[len(designEvents)-1].EventType != "design_interrupted" {
		t.Fatalf("expected design_interrupted event, got %+v", designEvents)
	}

	prJob, prEvents, err := orch.JobDetail(context.Background(), "job-pr-creating")
	if err != nil {
		t.Fatalf("JobDetail(pr) error = %v", err)
	}
	if prJob.State != domain.StateInterrupted {
		t.Fatalf("expected interrupted state, got %s", prJob.State)
	}
	if len(prEvents) == 0 || prEvents[len(prEvents)-1].EventType != "pr_interrupted" {
		t.Fatalf("expected pr_interrupted event, got %+v", prEvents)
	}

	waitingJob, waitingEvents, err := orch.JobDetail(context.Background(), "job-waiting-final")
	if err != nil {
		t.Fatalf("JobDetail(waiting) error = %v", err)
	}
	if waitingJob.State != domain.StateWaitingFinalApproval {
		t.Fatalf("expected waiting_final_approval state, got %s", waitingJob.State)
	}
	if len(waitingEvents) != 0 {
		t.Fatalf("expected no recovery event for waiting job, got %+v", waitingEvents)
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
