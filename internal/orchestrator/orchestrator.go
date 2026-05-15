package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

type Orchestrator struct {
	store *sqlite.Store
}

var ErrInvalidStateTransition = errors.New("invalid state transition")

func New(store *sqlite.Store) *Orchestrator {
	return &Orchestrator{store: store}
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

func (o *Orchestrator) ProcessMatch(ctx context.Context, rule config.WatchRule, event domain.DomainEvent) error {
	jobType := domain.JobTypeIssue
	state := domain.StateDetected
	if event.Item.Target == domain.TargetPullRequest {
		jobType = domain.JobTypePRReview
		state = domain.StateCollectingContext
	}

	job := domain.Job{
		ID:           makeJobID(event.Item.Repository, event.Item.Target, event.Item.Number),
		Type:         jobType,
		Repository:   event.Item.Repository,
		GitHubNumber: event.Item.Number,
		State:        state,
		Title:        event.Item.Title,
		BranchName:   makeBranchName(event.Item.Target, event.Item.Number),
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

	if err := o.store.AppendEvent(ctx, domain.Event{
		JobID:     job.ID,
		EventType: string(event.Type),
		StateTo:   string(job.State),
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
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

	return o.store.AppendEvent(ctx, domain.Event{
		JobID:     job.ID,
		EventType: eventType,
		StateFrom: string(previous),
		StateTo:   string(nextState),
		Payload:   rawPayload,
		CreatedAt: time.Now().UTC(),
	})
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

func (o *Orchestrator) RerunDesign(ctx context.Context, jobID string, comment string) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	switch job.State {
	case domain.StateWaitingDesignApproval, domain.StateDesignRejected:
	default:
		return fmt.Errorf("%w: design rerun is allowed only from waiting_design_approval or design_rejected", ErrInvalidStateTransition)
	}

	return o.UpdateJobState(ctx, jobID, domain.StateDetected, "design_rerun_requested", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) ApproveFinal(ctx context.Context, jobID string, comment string) error {
	return o.UpdateJobState(ctx, jobID, domain.StatePRCreating, "final_approved", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) RejectFinal(ctx context.Context, jobID string, comment string) error {
	return o.UpdateJobState(ctx, jobID, domain.StateFinalRejected, "final_rejected", map[string]any{
		"comment": comment,
	})
}

func (o *Orchestrator) RerunImplementation(ctx context.Context, jobID string, comment string) error {
	job, err := o.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	switch job.State {
	case domain.StateWaitingFinalApproval, domain.StateFinalRejected:
	default:
		return fmt.Errorf("%w: implementation rerun is allowed only from waiting_final_approval or final_rejected", ErrInvalidStateTransition)
	}

	return o.UpdateJobState(ctx, jobID, domain.StateImplementationRunning, "implementation_rerun_requested", map[string]any{
		"comment": comment,
	})
}

func makeJobID(repository string, target domain.MonitoredTarget, number int) string {
	replacer := strings.NewReplacer("/", "-", "_", "-")
	return fmt.Sprintf("%s-%s-%d", target, replacer.Replace(repository), number)
}

func makeBranchName(target domain.MonitoredTarget, number int) string {
	if target == domain.TargetPullRequest {
		return fmt.Sprintf("korobokcle/pr-review-%d", number)
	}
	return fmt.Sprintf("korobokcle/issue-%d", number)
}
