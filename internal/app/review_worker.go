package app

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func startReviewWorker(ctx context.Context, root string, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	runner := skill.NewRunner(root, cfg.App().Provider)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			if err := runPendingReviews(ctx, cfg, orch, runner, logger); err != nil && ctx.Err() == nil {
				logger.Printf("review worker error: %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

func runPendingReviews(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if job.Type != domain.JobTypePRReview || job.State != domain.StateCollectingContext {
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateReviewRunning, "review_started", map[string]any{
			"provider": cfg.App().Provider,
		}); err != nil {
			logger.Printf("review state transition failed for %s: %v", job.ID, err)
			continue
		}

		jobDetail, events, err := orch.JobDetail(ctx, job.ID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
			continue
		}

		contextData, err := buildReviewContext(cfg, jobDetail, events)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
			continue
		}

		skillName, err := resolveReviewSkillName(cfg, jobDetail.WatchRuleID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
			continue
		}

		if _, err := runner.RunReview(ctx, skillName, contextData); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateReviewReady, "review_ready", map[string]any{
			"artifactDir": contextData.ArtifactDir,
			"skill":       skillName,
		}); err != nil {
			return err
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "review_completed", map[string]any{
			"artifactDir": contextData.ArtifactDir,
			"skill":       skillName,
		}); err != nil {
			return err
		}
	}

	return nil
}

func resolveReviewSkillName(cfg *config.Service, watchRuleID string) (string, error) {
	rule, ok := cfg.WatchRuleByID(watchRuleID)
	if !ok {
		return "", os.ErrNotExist
	}

	skillSet := strings.TrimSpace(rule.SkillSet)
	if skillSet == "" || skillSet == "default" {
		return "review", nil
	}
	return filepath.ToSlash(filepath.Join(skillSet, "review")), nil
}

func buildReviewContext(cfg *config.Service, job domain.Job, events []domain.Event) (skill.ReviewContext, error) {
	ctxData := skill.ReviewContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		PullNumber:  job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: filepath.Join(cfg.App().WorkspaceDir, cfg.App().ArtifactsDir, "reviews", job.ID),
	}

	for _, event := range events {
		if event.EventType != string(domain.DomainEventPRMatched) {
			continue
		}

		var payload struct {
			Body      string   `json:"body"`
			Author    string   `json:"author"`
			Labels    []string `json:"labels"`
			Assignees []string `json:"assignees"`
			URL       string   `json:"url"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return skill.ReviewContext{}, err
		}
		ctxData.Body = payload.Body
		ctxData.Author = payload.Author
		ctxData.Labels = payload.Labels
		ctxData.Assignees = payload.Assignees
		ctxData.SourceURL = payload.URL
		ctxData.RepositoryHint = job.Repository
		break
	}

	return ctxData, nil
}
