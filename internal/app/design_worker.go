package app

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/issuebody"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func startDesignWorker(ctx context.Context, repoRoot string, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	runner := skill.NewRunner(repoRoot, cfg.Root(), "", cfg.App().CopilotAllowTools)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			if err := runPendingDesigns(ctx, repoRoot, cfg, orch, runner, logger); err != nil && ctx.Err() == nil {
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

func runPendingDesigns(ctx context.Context, repoRoot string, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if job.Type != domain.JobTypeIssue || job.State != domain.StateDetected {
			continue
		}

		jobDetail, events, err := orch.JobDetail(ctx, job.ID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateDesignRunning, "design_started", map[string]any{
			"provider": execution.Provider,
			"model":    execution.Model,
		}); err != nil {
			logger.Printf("design state transition failed for %s: %v", job.ID, err)
			continue
		}

		contextData, err := buildDesignContext(cfg, repoRoot, jobDetail, events)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		skillName, err := resolveDesignSkillName(cfg, jobDetail.WatchRuleID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}

		if _, err := runner.RunDesign(ctx, skillName, contextData, execution); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
			continue
		}
		if err := copyAIResultToWorkDir(repoRoot, artifacts.WorkerDesign, jobDetail, contextData.ArtifactDir); err != nil {
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

func buildDesignContext(cfg *config.Service, workDir string, job domain.Job, events []domain.Event) (skill.DesignContext, error) {
	ctxData := skill.DesignContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerDesign),
	}

	for _, event := range events {
		switch event.EventType {
		case "design_rerun_requested":
			var payload struct {
				Comment string `json:"comment"`
			}
			if err := unmarshalEventPayload(event.Payload, &payload); err != nil {
				return skill.DesignContext{}, err
			}
			ctxData.RerunComment = strings.TrimSpace(payload.Comment)
		}
	}

	if existingDesign, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerDesign, job, ctxData.ArtifactDir, "result.md", "design.md"); err == nil {
		ctxData.ExistingDesign = string(existingDesign)
	} else if !errors.Is(err, os.ErrNotExist) {
		return skill.DesignContext{}, err
	}
	ctxData.ExistingImprovements = loadExistingImprovements(cfg, job.Repository)

	snapshot, err := issuebody.Resolve(events)
	if err != nil {
		return skill.DesignContext{}, err
	}
	ctxData.Body = snapshot.Body
	ctxData.Author = snapshot.Author
	ctxData.Labels = snapshot.Labels
	ctxData.Assignees = snapshot.Assignees

	return ctxData, nil
}
