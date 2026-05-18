package app

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/executor"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func startRepositoryWorkers(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	for _, repository := range cfg.App().MonitoredRepositories {
		repoName := strings.TrimSpace(repository.Repository)
		if repoName == "" || repository.Workers < 1 {
			continue
		}
		for workerIndex := 0; workerIndex < repository.Workers; workerIndex++ {
			workerIndex := workerIndex
			repository := repository
			go runRepositoryWorker(ctx, cfg, orch, logger, repository, workerIndex)
		}
	}
	return nil
}

func runRepositoryWorker(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger, repository config.MonitoredRepository, workerIndex int) {
	repoDir, err := cloneRepositoryWorkspace(ctx, cfg, repository.Repository, workerIndex)
	if err != nil {
		if logger != nil {
			logger.Printf("repository worker clone failed repository=%s worker=%d error=%v", repository.Repository, workerIndex, err)
		}
		return
	}

	runner := skill.NewRunner(repoDir, cfg.Root(), "", cfg.App().CopilotAllowTools)
	testRunner := executor.NewTestRunner()
	pusher, creator := newPRPublisher(cfg.App().Provider)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if err := runRepositoryWorkerCycle(ctx, cfg, orch, runner, testRunner, pusher, creator, repoDir, repository, workerIndex, logger); err != nil && ctx.Err() == nil {
			if logger != nil {
				logger.Printf("repository worker error repository=%s worker=%d error=%v", repository.Repository, workerIndex, err)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runRepositoryWorkerCycle(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, testRunner *executor.TestRunner, pusher BranchPusher, creator PRCreator, repoDir string, repository config.MonitoredRepository, workerIndex int, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if strings.TrimSpace(job.Repository) != strings.TrimSpace(repository.Repository) {
			continue
		}
		if !jobAssignedToWorker(job, repository.Repository, workerIndex, repository.Workers) {
			continue
		}

		switch job.State {
		case domain.StateDetected:
			if job.Type != domain.JobTypeIssue {
				continue
			}
			if err := processDesignJob(ctx, cfg, orch, runner, job, repoDir, logger); err != nil {
				return err
			}
		case domain.StateImplementationRunning:
			if job.Type != domain.JobTypeIssue {
				continue
			}
			if err := processImplementationJob(ctx, cfg, orch, runner, testRunner, job, repoDir, logger); err != nil {
				return err
			}
		case domain.StateCollectingContext:
			if job.Type != domain.JobTypePRReview {
				continue
			}
			if err := processReviewJob(ctx, cfg, orch, runner, job, repoDir, logger); err != nil {
				return err
			}
		case domain.StatePRCreating:
			if job.Type != domain.JobTypeIssue {
				continue
			}
			if err := processPRJob(ctx, cfg, orch, pusher, creator, job, repoDir, logger); err != nil {
				return err
			}
		}
	}

	return nil
}

func processDesignJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, job domain.Job, repoDir string, logger *log.Logger) error {
	jobDetail, events, err := orch.JobDetail(ctx, job.ID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateDesignRunning, "design_started", map[string]any{
		"provider": execution.Provider,
		"model":    execution.Model,
	}); err != nil {
		if logger != nil {
			logger.Printf("design state transition failed for %s: %v", job.ID, err)
		}
		return err
	}

	contextData, err := buildDesignContext(cfg, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	skillName, err := resolveDesignSkillName(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	if _, err := runner.RunDesign(ctx, skillName, contextData, execution); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateDesignReady, "design_ready", map[string]any{
		"artifactDir": contextData.ArtifactDir,
		"skill":       skillName,
	}); err != nil {
		return err
	}
	return orch.UpdateJobState(ctx, job.ID, domain.StateWaitingDesignApproval, "waiting_design_approval", map[string]any{
		"artifactDir": contextData.ArtifactDir,
		"skill":       skillName,
	})
}

func processImplementationJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, testRunner *executor.TestRunner, job domain.Job, repoDir string, logger *log.Logger) error {
	jobDetail, events, err := orch.JobDetail(ctx, job.ID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	runSpec, err := resolveImplementationRunSpec(cfg, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	contextData, err := buildImplementationContext(cfg, jobDetail, events, runSpec)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	if _, err := runner.RunImplementation(ctx, runSpec.SkillName, contextData, execution); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateTestRunning, "test_started", map[string]any{
		"artifactDir": contextData.ArtifactDir,
	}); err != nil {
		if logger != nil {
			logger.Printf("test_started state transition failed for %s: %v", job.ID, err)
		}
		return err
	}

	report, err := runTestsForJob(ctx, cfg, testRunner, job, contextData.ArtifactDir, repoDir)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{"error": err.Error()})
	}
	if !report.Success {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{
			"reportPath": filepath.Join(contextData.ArtifactDir, "test-report.json"),
		})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateImplementationReady, "implementation_ready", map[string]any{
		"artifactDir": contextData.ArtifactDir,
	}); err != nil {
		if logger != nil {
			logger.Printf("implementation_ready state transition failed for %s: %v", job.ID, err)
		}
		return err
	}
	return orch.UpdateJobState(ctx, job.ID, domain.StateWaitingFinalApproval, "waiting_final_approval", map[string]any{
		"artifactDir": contextData.ArtifactDir,
	})
}

func processReviewJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, job domain.Job, repoDir string, logger *log.Logger) error {
	jobDetail, events, err := orch.JobDetail(ctx, job.ID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateReviewRunning, "review_started", map[string]any{
		"provider": execution.Provider,
		"model":    execution.Model,
	}); err != nil {
		if logger != nil {
			logger.Printf("review state transition failed for %s: %v", job.ID, err)
		}
		return err
	}

	contextData, err := buildReviewContext(cfg, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	skillName, err := resolveReviewSkillName(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	if _, err := runner.RunReview(ctx, skillName, contextData, execution); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateReviewReady, "review_ready", map[string]any{
		"artifactDir": contextData.ArtifactDir,
		"skill":       skillName,
	}); err != nil {
		return err
	}
	return orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "review_completed", map[string]any{
		"artifactDir": contextData.ArtifactDir,
		"skill":       skillName,
	})
}

func processPRJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, pusher BranchPusher, creator PRCreator, job domain.Job, repoDir string, logger *log.Logger) error {
	req, err := buildPRCreateRequest(cfg, job, repoDir)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}
	req.WorkDir = repoDir

	if err := pusher.Push(ctx, req); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_push_failed", map[string]any{"error": err.Error()})
	}

	url, err := creator.Create(ctx, req)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}

	if err := writePRCreateArtifact(req.ArtifactDir, url, req); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "pr_created", map[string]any{
		"url":   url,
		"title": req.Title,
		"head":  req.BranchName,
	}); err != nil {
		if logger != nil {
			logger.Printf("pr_created state transition failed for %s: %v", job.ID, err)
		}
		return err
	}
	return nil
}

func cloneRepositoryWorkspace(ctx context.Context, cfg *config.Service, repository string, workerIndex int) (string, error) {
	workerDir := artifacts.RepositoryWorkerDir(cfg.Root(), cfg.App().ArtifactsDir, repository, workerIndex)
	if err := os.RemoveAll(workerDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(workerDir), 0o755); err != nil {
		return "", err
	}

	source := repositoryCloneSource(repository)
	cmd := exec.CommandContext(ctx, "git", "clone", "--quiet", source, workerDir)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	return workerDir, nil
}

func repositoryCloneSource(repository string) string {
	trimmed := strings.TrimSpace(repository)
	if trimmed == "" {
		return trimmed
	}
	if strings.Contains(trimmed, "://") || strings.HasPrefix(trimmed, "git@") || filepath.IsAbs(trimmed) {
		return trimmed
	}
	if _, err := os.Stat(trimmed); err == nil {
		return trimmed
	}
	return "https://github.com/" + trimmed + ".git"
}

func jobAssignedToWorker(job domain.Job, repository string, workerIndex int, workerCount int) bool {
	if workerCount <= 1 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(repository)))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(job.ID))
	return int(h.Sum32()%uint32(workerCount)) == workerIndex
}
