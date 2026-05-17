package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/naming"
	"github.com/coco-papiyon/korobokcle/internal/notification"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

type Orchestrator struct {
	store    *sqlite.Store
	notifier notification.Notifier
}

var ErrInvalidStateTransition = errors.New("invalid state transition")

func New(store *sqlite.Store, notifier notification.Notifier) *Orchestrator {
	if notifier == nil {
		notifier = notification.NewNopNotifier()
	}
	return &Orchestrator{store: store, notifier: notifier}
}

func (o *Orchestrator) ListJobs(ctx context.Context) ([]domain.Job, error) {
	return o.store.ListJobs(ctx)
}

func (o *Orchestrator) JobDetail(ctx context.Context, jobID string) (domain.Job, []domain.Event, error) {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return domain.Job{}, nil, err
	}
	events, err := o.store.ListEvents(ctx, jobID)
	if err != nil {
		return domain.Job{}, nil, err
	}
	return job, events, nil
}

func (o *Orchestrator) ProcessMatch(ctx context.Context, appConfig config.App, rule config.WatchRule, event domain.DomainEvent) error {
	jobType := domain.JobTypeIssue
	state := domain.StateDetected
	if event.Item.Target == domain.TargetPullRequest {
		jobType = domain.JobTypePRReview
		state = domain.StateCollectingContext
	}

	branchName := makeBranchName(appConfig, event.Item)
	if event.Item.Target == domain.TargetPullRequest {
		branchName = makePRReviewBranchName(event.Item.Number)
	}

	job := domain.Job{
		ID:           makeJobID(event.Item.Repository, event.Item.Target, event.Item.Number),
		Type:         jobType,
		Repository:   event.Item.Repository,
		GitHubNumber: event.Item.Number,
		State:        state,
		Title:        event.Item.Title,
		BranchName:   branchName,
		WatchRuleID:  rule.ID,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if _, err := o.store.FindJobBySource(ctx, job.Repository, job.GitHubNumber, job.Type); err == nil {
		return nil
	} else if errors.Is(err, domain.ErrJobNotFound) {
		job.ID = job.ID + "-" + uuid.NewString()[:8]
	} else {
		return err
	}

	if err := o.store.UpsertJob(ctx, job); err != nil {
		return err
	}
	log.Printf("info job started job=%s type=%s repository=%s number=%d state=%s watch_rule=%s", job.ID, job.Type, job.Repository, job.GitHubNumber, job.State, job.WatchRuleID)

	payload, err := json.Marshal(map[string]any{
		"ruleId":     rule.ID,
		"ruleName":   rule.Name,
		"repository": event.Item.Repository,
		"number":     event.Item.Number,
		"url":        event.Item.URL,
		"target":     event.Item.Target,
		"title":      event.Item.Title,
		"body":       event.Item.Body,
		"author":     event.Item.Author,
		"labels":     event.Item.Labels,
		"assignees":  event.Item.Assignees,
	})
	if err != nil {
		return err
	}

	storedEvent := domain.Event{
		JobID:     job.ID,
		EventType: string(event.Type),
		StateTo:   string(job.State),
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
	}
	if err := o.store.AppendEvent(ctx, storedEvent); err != nil {
		return err
	}
	log.Printf("info job event started job=%s event=%s state_from=%s state_to=%s", job.ID, storedEvent.EventType, storedEvent.StateFrom, storedEvent.StateTo)
	o.notifyJobEvent(ctx, job, storedEvent)
	return nil
}

func (o *Orchestrator) UpdateJobState(ctx context.Context, jobID string, nextState domain.JobState, eventType string, payload map[string]any) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	previous := job.State
	job.State = nextState
	job.UpdatedAt = time.Now().UTC()
	if err := o.store.UpsertJob(ctx, job); err != nil {
		return err
	}

	rawPayload := "{}"
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		rawPayload = string(raw)
	}

	storedEvent := domain.Event{
		JobID:     job.ID,
		EventType: eventType,
		StateFrom: string(previous),
		StateTo:   string(nextState),
		Payload:   rawPayload,
		CreatedAt: time.Now().UTC(),
	}
	if err := o.store.AppendEvent(ctx, storedEvent); err != nil {
		return err
	}
	log.Printf("info job event started job=%s event=%s state_from=%s state_to=%s", job.ID, storedEvent.EventType, storedEvent.StateFrom, storedEvent.StateTo)
	o.notifyJobEvent(ctx, job, storedEvent)
	return nil
}

func (o *Orchestrator) ApproveDesign(ctx context.Context, jobID string, comment string) error {
	return o.UpdateJobState(ctx, jobID, domain.StateImplementationRunning, "design_approved", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) RejectDesign(ctx context.Context, jobID string, comment string) error {
	return o.UpdateJobState(ctx, jobID, domain.StateDesignRejected, "design_rejected", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) ApproveFinal(ctx context.Context, jobID string, comment string) error {
	if err := o.ensureFinalApprovalAllowed(ctx, jobID); err != nil {
		return err
	}
	return o.UpdateJobState(ctx, jobID, domain.StatePRCreating, "final_approved", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) RejectFinal(ctx context.Context, jobID string, comment string) error {
	if err := o.ensureFinalApprovalAllowed(ctx, jobID); err != nil {
		return err
	}
	return o.UpdateJobState(ctx, jobID, domain.StateFinalRejected, "final_rejected", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) RerunReview(ctx context.Context, jobID string, comment string) error {
	return o.RerunReviewFromEvent(ctx, jobID, nil, comment)
}

func (o *Orchestrator) RerunReviewFromEvent(ctx context.Context, jobID string, sourceEventID *int64, comment string) error {
	phase, err := o.resolveRerunPhase(ctx, jobID, sourceEventID)
	if err != nil {
		return err
	}
	if phase != rerunPhaseReview {
		return fmt.Errorf("%w: review rerun is not available for this event", ErrInvalidStateTransition)
	}

	return o.UpdateJobState(ctx, jobID, domain.StateCollectingContext, "review_rerun_requested", map[string]any{
		"comment": comment,
		"eventId": sourceEventID,
	})
}

func (o *Orchestrator) RerunDesign(ctx context.Context, jobID string, comment string) error {
	return o.RerunDesignFromEvent(ctx, jobID, nil, comment)
}

func (o *Orchestrator) RerunDesignFromEvent(ctx context.Context, jobID string, sourceEventID *int64, comment string) error {
	phase, err := o.resolveRerunPhase(ctx, jobID, sourceEventID)
	if err != nil {
		return err
	}
	if phase != rerunPhaseDesign {
		return fmt.Errorf("%w: design rerun is not available for this event", ErrInvalidStateTransition)
	}

	return o.UpdateJobState(ctx, jobID, domain.StateDetected, "design_rerun_requested", map[string]any{
		"comment": comment,
		"eventId": sourceEventID,
	})
}

func (o *Orchestrator) RerunImplementation(ctx context.Context, jobID string, comment string) error {
	return o.RerunImplementationFromEvent(ctx, jobID, nil, comment)
}

func (o *Orchestrator) RerunImplementationFromEvent(ctx context.Context, jobID string, sourceEventID *int64, comment string) error {
	phase, err := o.resolveRerunPhase(ctx, jobID, sourceEventID)
	if err != nil {
		return err
	}
	if phase != rerunPhaseImplementation {
		return fmt.Errorf("%w: implementation rerun is not available for this event", ErrInvalidStateTransition)
	}

	return o.UpdateJobState(ctx, jobID, domain.StateImplementationRunning, "implementation_rerun_requested", map[string]any{
		"comment": comment,
		"eventId": sourceEventID,
	})
}

func (o *Orchestrator) RerunPRCreation(ctx context.Context, jobID string, comment string) error {
	return o.RerunPRCreationFromEvent(ctx, jobID, nil, comment)
}

func (o *Orchestrator) RerunPRCreationFromEvent(ctx context.Context, jobID string, sourceEventID *int64, comment string) error {
	phase, err := o.resolveRerunPhase(ctx, jobID, sourceEventID)
	if err != nil {
		return err
	}
	if phase != rerunPhasePR {
		return fmt.Errorf("%w: pr rerun is not available for this event", ErrInvalidStateTransition)
	}

	return o.UpdateJobState(ctx, jobID, domain.StatePRCreating, "pr_rerun_requested", map[string]any{
		"comment": comment,
		"eventId": sourceEventID,
	})
}

type rerunPhase string

const (
	rerunPhaseDesign         rerunPhase = "design"
	rerunPhaseImplementation rerunPhase = "implementation"
	rerunPhasePR             rerunPhase = "pr"
	rerunPhaseReview         rerunPhase = "review"
)

func (o *Orchestrator) resolveRerunPhase(ctx context.Context, jobID string, sourceEventID *int64) (rerunPhase, error) {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	if sourceEventID != nil {
		event, err := o.store.GetEvent(ctx, *sourceEventID)
		if err != nil {
			return "", err
		}
		if event.JobID != job.ID {
			return "", fmt.Errorf("event %d does not belong to job %q", event.ID, jobID)
		}
		phase := rerunPhaseFromEvent(event)
		if phase == "" {
			return "", fmt.Errorf("%w: event %d is not rerunnable", ErrInvalidStateTransition, event.ID)
		}
		return phase, nil
	}

	phase := rerunPhaseFromJob(ctx, o, job)
	if phase == "" {
		return "", fmt.Errorf("%w: job state %q is not rerunnable", ErrInvalidStateTransition, job.State)
	}
	return phase, nil
}

func rerunPhaseFromJob(ctx context.Context, o *Orchestrator, job domain.Job) rerunPhase {
	switch job.State {
	case domain.StateWaitingDesignApproval, domain.StateDesignRejected, domain.StateDetected, domain.StateDesignRunning:
		return rerunPhaseDesign
	case domain.StateWaitingFinalApproval, domain.StateFinalRejected, domain.StateImplementationRunning, domain.StateTestRunning, domain.StateImplementationReady:
		return rerunPhaseImplementation
	case domain.StatePRCreating:
		return rerunPhasePR
	case domain.StateCollectingContext, domain.StateReviewRunning, domain.StateReviewReady:
		return rerunPhaseReview
	case domain.StateFailed:
		events, err := o.store.ListEvents(ctx, job.ID)
		if err != nil || len(events) == 0 {
			return ""
		}
		return rerunPhaseFromEvent(events[len(events)-1])
	default:
		return ""
	}
}

func rerunPhaseFromEvent(event domain.Event) rerunPhase {
	switch event.EventType {
	case string(domain.DomainEventIssueMatched), "design_started", "design_ready", "waiting_design_approval", "design_rejected", "design_failed", "design_rerun_requested":
		return rerunPhaseDesign
	case "design_approved", "implementation_started", "implementation_ready", "waiting_final_approval", "final_rejected", "implementation_failed", "test_failed", "implementation_rerun_requested":
		return rerunPhaseImplementation
	case "final_approved", "pr_creating_started", "pr_create_failed", "pr_created", "pr_rerun_requested":
		return rerunPhasePR
	case "review_started", "review_ready", "review_failed", "review_rerun_requested":
		return rerunPhaseReview
	}
	switch event.StateFrom {
	case string(domain.StateDesignRunning), string(domain.StateDetected):
		return rerunPhaseDesign
	case string(domain.StateImplementationRunning), string(domain.StateTestRunning), string(domain.StateWaitingFinalApproval), string(domain.StateImplementationReady):
		return rerunPhaseImplementation
	case string(domain.StatePRCreating):
		return rerunPhasePR
	case string(domain.StateCollectingContext), string(domain.StateReviewRunning):
		return rerunPhaseReview
	}
	return ""
}

func makeJobID(repository string, target domain.MonitoredTarget, number int) string {
	replacer := strings.NewReplacer("/", "-", "_", "-")
	return fmt.Sprintf("%s-%s-%d", target, replacer.Replace(repository), number)
}

func makeBranchName(appConfig config.App, item domain.RepositoryItem) string {
	return naming.RenderBranchName(appConfig.BranchTemplate, item)
}

func makePRReviewBranchName(number int) string {
	return fmt.Sprintf("korobokcle/pr-review-%d", number)
}

func (o *Orchestrator) ensureFinalApprovalAllowed(ctx context.Context, jobID string) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.State == domain.StateWaitingFinalApproval {
		return nil
	}
	if job.State != domain.StateFailed {
		return fmt.Errorf("%w: final approval is not available for job state %q", ErrInvalidStateTransition, job.State)
	}

	events, err := o.store.ListEvents(ctx, job.ID)
	if err != nil {
		return err
	}
	if len(events) == 0 || events[len(events)-1].EventType != "test_failed" {
		return fmt.Errorf("%w: final approval is only available after test_failed", ErrInvalidStateTransition)
	}
	return nil
}

func (o *Orchestrator) notifyJobEvent(ctx context.Context, job domain.Job, event domain.Event) {
	if o.notifier == nil {
		return
	}

	notificationPayload := notification.Notification{
		Title:      notificationTitle(job, event),
		Message:    notificationMessage(job, event),
		Event:      event.EventType,
		State:      event.StateTo,
		Repository: job.Repository,
		Number:     job.GitHubNumber,
		JobID:      job.ID,
	}
	if err := o.notifier.Notify(ctx, notificationPayload); err != nil {
		if errors.Is(err, notification.ErrNotificationSkipped) {
			return
		}
		log.Printf("notification failed job=%s event=%s: %v", job.ID, event.EventType, err)
		return
	}
	log.Printf("info notification sent job=%s event=%s state=%s repository=%s number=%d", job.ID, event.EventType, event.StateTo, job.Repository, job.GitHubNumber)
}

func notificationTitle(job domain.Job, event domain.Event) string {
	switch event.EventType {
	case string(domain.DomainEventIssueMatched):
		return fmt.Sprintf("Issue matched: %s#%d", job.Repository, job.GitHubNumber)
	case string(domain.DomainEventPRMatched):
		return fmt.Sprintf("PR matched: %s#%d", job.Repository, job.GitHubNumber)
	case "design_ready":
		return fmt.Sprintf("Design ready: %s#%d", job.Repository, job.GitHubNumber)
	case "waiting_design_approval":
		return fmt.Sprintf("Design approval required: %s#%d", job.Repository, job.GitHubNumber)
	case "implementation_ready":
		return fmt.Sprintf("Implementation ready: %s#%d", job.Repository, job.GitHubNumber)
	case "waiting_final_approval":
		return fmt.Sprintf("Final approval required: %s#%d", job.Repository, job.GitHubNumber)
	case "review_ready":
		return fmt.Sprintf("Review ready: %s#%d", job.Repository, job.GitHubNumber)
	case "review_completed":
		return fmt.Sprintf("Review completed: %s#%d", job.Repository, job.GitHubNumber)
	case "pr_created":
		return fmt.Sprintf("PR created: %s#%d", job.Repository, job.GitHubNumber)
	}
	if event.StateTo == string(domain.StateFailed) || strings.HasSuffix(event.EventType, "_failed") {
		return fmt.Sprintf("Job failed: %s#%d", job.Repository, job.GitHubNumber)
	}
	return fmt.Sprintf("%s: %s#%d", strings.ReplaceAll(event.EventType, "_", " "), job.Repository, job.GitHubNumber)
}

func notificationMessage(job domain.Job, event domain.Event) string {
	parts := []string{strings.TrimSpace(job.Title)}
	if detail := notificationDetail(event); detail != "" {
		parts = append(parts, detail)
	}
	parts = append(parts, fmt.Sprintf("job=%s", job.ID))
	return strings.Join(parts, " | ")
}

func notificationDetail(event domain.Event) string {
	if strings.TrimSpace(event.Payload) == "" || strings.TrimSpace(event.Payload) == "{}" {
		return fmt.Sprintf("event=%s", event.EventType)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return fmt.Sprintf("event=%s", event.EventType)
	}

	if value := strings.TrimSpace(stringValue(payload["error"])); value != "" {
		return value
	}
	if value := strings.TrimSpace(stringValue(payload["url"])); value != "" {
		return value
	}
	if value := strings.TrimSpace(stringValue(payload["skill"])); value != "" {
		return fmt.Sprintf("skill=%s", value)
	}
	return fmt.Sprintf("event=%s", event.EventType)
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
