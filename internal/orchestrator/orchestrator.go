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
	"github.com/coco-papiyon/korobokcle/internal/issuebody"
	"github.com/coco-papiyon/korobokcle/internal/naming"
	"github.com/coco-papiyon/korobokcle/internal/notification"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

type Orchestrator struct {
	store    *sqlite.Store
	notifier notification.Notifier
}

var ErrInvalidStateTransition = errors.New("invalid state transition")
var ErrJobNotDeleted = errors.New("job is not deleted")

type JobListFilter string

const (
	JobListActiveOnly  JobListFilter = "active"
	JobListDeletedOnly JobListFilter = "deleted"
	JobListAll         JobListFilter = "all"
)

func New(store *sqlite.Store, notifier notification.Notifier) *Orchestrator {
	if notifier == nil {
		notifier = notification.NewNopNotifier()
	}
	return &Orchestrator{store: store, notifier: notifier}
}

func (o *Orchestrator) ListJobs(ctx context.Context) ([]domain.Job, error) {
	return o.store.ListJobs(ctx)
}

func (o *Orchestrator) ListJobsByFilter(ctx context.Context, filter JobListFilter) ([]domain.Job, error) {
	switch filter {
	case JobListDeletedOnly:
		return o.store.ListJobsByFilter(ctx, sqlite.JobListDeletedOnly)
	case JobListAll:
		return o.store.ListJobsByFilter(ctx, sqlite.JobListAll)
	default:
		return o.store.ListJobsByFilter(ctx, sqlite.JobListActiveOnly)
	}
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

func (o *Orchestrator) RecordIssueBodyRefresh(ctx context.Context, jobID string, body string) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{"body": body})
	if err != nil {
		return err
	}
	storedEvent := domain.Event{
		JobID:     job.ID,
		EventType: issuebody.EventTypeRefreshed,
		StateFrom: string(job.State),
		StateTo:   string(job.State),
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
	}
	if err := o.store.AppendEvent(ctx, storedEvent); err != nil {
		return err
	}
	log.Printf("info job issue body refreshed job=%s state=%s", job.ID, job.State)
	return nil
}

func (o *Orchestrator) ProcessMatch(ctx context.Context, appConfig config.App, rule config.WatchRule, event domain.DomainEvent) error {
	jobType := domain.JobTypeIssue
	state := domain.StateDetected
	if event.Item.Target == domain.TargetPullRequest {
		jobType = domain.JobTypePRReview
		state = domain.StateCollectingContext
	} else if event.Item.Target == domain.TargetPullRequestReview {
		jobType = domain.JobTypePRFeedback
		state = domain.StateImplementationRunning
	}

	branchName := makeBranchName(appConfig, event.Item)
	if event.Item.Target == domain.TargetPullRequest {
		branchName = strings.TrimSpace(event.Item.BranchName)
		if branchName == "" {
			branchName = makePRReviewBranchName(event.Item.Number)
		}
	} else if event.Item.Target == domain.TargetPullRequestReview {
		branchName = strings.TrimSpace(event.Item.BranchName)
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
	if existing, err := o.store.FindJobBySource(ctx, job.Repository, job.GitHubNumber, job.Type); err == nil {
		if existing.DeletedAt != nil {
			return nil
		}
		job.ID = existing.ID
		job.CreatedAt = existing.CreatedAt
		job.UpdatedAt = time.Now().UTC()
		if jobType == domain.JobTypePRReview {
			if existing.BranchName != job.BranchName || existing.Title != job.Title || existing.WatchRuleID != job.WatchRuleID || existing.State != job.State {
				job.State = existing.State
				return o.store.UpsertJob(ctx, job)
			}
			return nil
		}
		if jobType != domain.JobTypePRFeedback {
			return nil
		}
		events, err := o.store.ListEvents(ctx, existing.ID)
		if err != nil {
			return err
		}
		if prFeedbackAlreadyIncorporated(events, event.Item.ReviewComments) {
			return nil
		}
		job.State = existing.State
		if jobType == domain.JobTypePRFeedback && !prFeedbackJobBusy(existing.State) {
			job.State = domain.StateImplementationRunning
		}
	} else if errors.Is(err, domain.ErrJobNotFound) {
		job.ID = job.ID + "-" + uuid.NewString()
	} else {
		return err
	}

	return o.upsertMatchedJob(ctx, job, rule, event)
}

func (o *Orchestrator) DeleteJob(ctx context.Context, jobID string) error {
	return o.updateJobDeletedAt(ctx, jobID, time.Now().UTC(), "job_deleted")
}

func (o *Orchestrator) RestoreJob(ctx context.Context, jobID string) error {
	return o.updateJobDeletedAt(ctx, jobID, time.Time{}, "job_restored")
}

func (o *Orchestrator) PurgeJob(ctx context.Context, jobID string) error {
	if err := o.store.PurgeJob(ctx, jobID); err != nil {
		if errors.Is(err, sqlite.ErrJobNotDeleted) {
			return ErrJobNotDeleted
		}
		return err
	}
	return nil
}

func (o *Orchestrator) upsertMatchedJob(ctx context.Context, job domain.Job, rule config.WatchRule, event domain.DomainEvent) error {
	if err := o.store.UpsertJob(ctx, job); err != nil {
		return err
	}
	log.Printf("info job started job=%s type=%s repository=%s number=%d state=%s watch_rule=%s", job.ID, job.Type, job.Repository, job.GitHubNumber, job.State, job.WatchRuleID)

	payload, err := json.Marshal(map[string]any{
		"ruleId":         rule.ID,
		"ruleName":       rule.Name,
		"repository":     event.Item.Repository,
		"number":         event.Item.Number,
		"url":            event.Item.URL,
		"target":         event.Item.Target,
		"title":          event.Item.Title,
		"body":           event.Item.Body,
		"author":         event.Item.Author,
		"labels":         event.Item.Labels,
		"assignees":      event.Item.Assignees,
		"branchName":     event.Item.BranchName,
		"baseBranch":     event.Item.BaseBranch,
		"reviewComments": event.Item.ReviewComments,
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

func (o *Orchestrator) updateJobDeletedAt(ctx context.Context, jobID string, deletedAt time.Time, eventType string) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	if eventType == "job_deleted" && job.DeletedAt != nil {
		return nil
	}
	if eventType == "job_restored" && job.DeletedAt == nil {
		return nil
	}

	if deletedAt.IsZero() {
		job.DeletedAt = nil
	} else {
		value := deletedAt.UTC()
		job.DeletedAt = &value
	}
	job.UpdatedAt = time.Now().UTC()
	if err := o.store.UpsertJob(ctx, job); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"deletedAt": formatOptionalTime(job.DeletedAt),
	})
	if err != nil {
		return err
	}
	return o.store.AppendEvent(ctx, domain.Event{
		JobID:     job.ID,
		EventType: eventType,
		StateFrom: string(job.State),
		StateTo:   string(job.State),
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
	})
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
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

func (o *Orchestrator) RecoverInterruptedJobs(ctx context.Context) (int, error) {
	jobs, err := o.store.ListJobs(ctx)
	if err != nil {
		return 0, err
	}

	recovered := 0
	for _, job := range jobs {
		eventType, ok := interruptedEventType(job.State)
		if !ok {
			continue
		}

		previous := job.State
		job.State = domain.StateInterrupted
		job.UpdatedAt = time.Now().UTC()
		if err := o.store.UpsertJob(ctx, job); err != nil {
			return recovered, err
		}

		payload, err := json.Marshal(map[string]any{
			"reason":        "startup_recovery",
			"previousState": previous,
		})
		if err != nil {
			return recovered, err
		}

		if err := o.store.AppendEvent(ctx, domain.Event{
			JobID:     job.ID,
			EventType: eventType,
			StateFrom: string(previous),
			StateTo:   string(domain.StateInterrupted),
			Payload:   string(payload),
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			return recovered, err
		}
		recovered++
	}

	return recovered, nil
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

func (o *Orchestrator) ApproveReview(ctx context.Context, jobID string) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.State != domain.StateReviewReady {
		return fmt.Errorf("%w: review approval is not available for this state", ErrInvalidStateTransition)
	}
	return o.UpdateJobState(ctx, jobID, domain.StateCompleted, "review_approved", nil)
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
	case domain.StateFailed, domain.StateInterrupted:
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
	case "design_interrupted":
		return rerunPhaseDesign
	case string(domain.DomainEventPRReviewMatched), "design_approved", "implementation_started", "implementation_ready", "waiting_final_approval", "final_rejected", "implementation_failed", "test_failed", "implementation_rerun_requested", "implementation_interrupted", "test_interrupted":
		return rerunPhaseImplementation
	case "final_approved", "pr_creating_started", "pr_create_failed", "pr_created", "pr_updated", "pr_rerun_requested", "pr_interrupted":
		return rerunPhasePR
	case "review_started", "review_ready", "review_failed", "review_rerun_requested", "review_interrupted":
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

func interruptedEventType(state domain.JobState) (string, bool) {
	switch state {
	case domain.StateDesignRunning:
		return "design_interrupted", true
	case domain.StateImplementationRunning:
		return "implementation_interrupted", true
	case domain.StateTestRunning:
		return "test_interrupted", true
	case domain.StateReviewRunning, domain.StateCollectingContext:
		return "review_interrupted", true
	case domain.StatePRCreating:
		return "pr_interrupted", true
	default:
		return "", false
	}
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
	log.Printf("info notification sent event=%s state=%s repository=%s number=%d", event.EventType, event.StateTo, job.Repository, job.GitHubNumber)
	log.Printf("DEBUG notification sent job=%s event=%s state=%s repository=%s number=%d title=%q message=%q payload=%s", job.ID, event.EventType, event.StateTo, job.Repository, job.GitHubNumber, notificationPayload.Title, notificationPayload.Message, event.Payload)
}

func notificationTitle(job domain.Job, event domain.Event) string {
	switch event.EventType {
	case string(domain.DomainEventIssueMatched):
		return fmt.Sprintf("Issue matched: %s#%d", job.Repository, job.GitHubNumber)
	case string(domain.DomainEventPRMatched):
		return fmt.Sprintf("PR matched: %s#%d", job.Repository, job.GitHubNumber)
	case string(domain.DomainEventPRReviewMatched):
		return fmt.Sprintf("PR feedback matched: %s#%d", job.Repository, job.GitHubNumber)
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
	case "review_approved":
		return fmt.Sprintf("Review approved: %s#%d", job.Repository, job.GitHubNumber)
	case "pr_created":
		return fmt.Sprintf("PR created: %s#%d", job.Repository, job.GitHubNumber)
	case "pr_updated":
		return fmt.Sprintf("PR updated: %s#%d", job.Repository, job.GitHubNumber)
	}
	if event.StateTo == string(domain.StateFailed) || strings.HasSuffix(event.EventType, "_failed") {
		return fmt.Sprintf("Job failed: %s#%d", job.Repository, job.GitHubNumber)
	}
	return fmt.Sprintf("%s: %s#%d", strings.ReplaceAll(event.EventType, "_", " "), job.Repository, job.GitHubNumber)
}

func prFeedbackJobBusy(state domain.JobState) bool {
	switch state {
	case domain.StateImplementationRunning, domain.StateTestRunning, domain.StatePRCreating:
		return true
	default:
		return false
	}
}

func prFeedbackAlreadyIncorporated(events []domain.Event, reviewComments []domain.ReviewComment) bool {
	if len(reviewComments) == 0 {
		return false
	}

	knownCommentIDs := make(map[int64]struct{}, len(reviewComments))
	for _, event := range events {
		if event.EventType != string(domain.DomainEventPRReviewMatched) {
			continue
		}

		var payload struct {
			ReviewComments []domain.ReviewComment `json:"reviewComments"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			continue
		}
		for _, comment := range payload.ReviewComments {
			if comment.ID == 0 {
				continue
			}
			knownCommentIDs[comment.ID] = struct{}{}
		}
	}

	for _, comment := range reviewComments {
		if comment.ID == 0 {
			return false
		}
		if _, ok := knownCommentIDs[comment.ID]; !ok {
			return false
		}
	}
	return true
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
