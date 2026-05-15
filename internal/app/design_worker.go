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

func startDesignWorker(ctx context.Context, root string, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	runner := skill.NewRunner(root, cfg.App().Provider)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			if err := runPendingDesigns(ctx, cfg, orch, runner, logger); err != nil && ctx.Err() == nil {
				logger.Printf("design worker error: %v", err)
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

func runPendingDesigns(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if job.Type != domain.JobTypeIssue || job.State != domain.StateDetected {
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateDesignRunning, "design_started", map[string]any{
			"provider": cfg.App().Provider,
		}); err != nil {
			logger.Printf("design state transition failed for %s: %v", job.ID, err)
			continue
		}

		jobDetail, events, err := orch.JobDetail(ctx, job.ID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		contextData, err := buildDesignContext(cfg, jobDetail, events)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		skillName, err := resolveDesignSkillName(cfg, jobDetail.WatchRuleID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		if _, err := runner.RunDesign(ctx, skillName, contextData); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateDesignReady, "design_ready", map[string]any{
			"artifactDir": contextData.ArtifactDir,
			"skill":       skillName,
		}); err != nil {
			return err
		}
		if err := orch.UpdateJobState(ctx, job.ID, domain.StateWaitingDesignApproval, "waiting_design_approval", map[string]any{
			"artifactDir": contextData.ArtifactDir,
			"skill":       skillName,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolveDesignSkillName(cfg *config.Service, watchRuleID string) (string, error) {
	rule, ok := cfg.WatchRuleByID(watchRuleID)
	if !ok {
		return "", os.ErrNotExist
	}

	skillSet := strings.TrimSpace(rule.SkillSet)
	if skillSet == "" || skillSet == "default" {
		return "design", nil
	}
	return filepath.ToSlash(filepath.Join(skillSet, "design")), nil
}

func buildDesignContext(cfg *config.Service, job domain.Job, events []domain.Event) (skill.DesignContext, error) {
	ctxData := skill.DesignContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: filepath.Join(cfg.App().WorkspaceDir, cfg.App().ArtifactsDir, "designs", job.ID),
	}

	for _, event := range events {
		if event.EventType != string(domain.DomainEventIssueMatched) {
			continue
		}

		var payload struct {
			Body      string   `json:"body"`
			Author    string   `json:"author"`
			Labels    []string `json:"labels"`
			Assignees []string `json:"assignees"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return skill.DesignContext{}, err
		}
		ctxData.Body = payload.Body
		ctxData.Author = payload.Author
		ctxData.Labels = payload.Labels
		ctxData.Assignees = payload.Assignees
		break
	}

	return ctxData, nil
}
