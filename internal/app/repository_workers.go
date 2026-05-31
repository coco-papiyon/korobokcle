package app

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
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
			logger.Printf("repository clone failed repository=%s worker=%d error=%v", repository.Repository, workerIndex, err)
		}
		return
	}

	workerLogger, cleanup, err := newRepositoryWorkerLogger(cfg, logger, repoDir, time.Now())
	if err != nil {
		if logger != nil {
			logger.Printf("repository worker logger init failed repository=%s worker=%d error=%v", repository.Repository, workerIndex, err)
		}
		return
	}
	defer cleanup()

	workerLogger.Printf("worker started repository=%s worker=%d", repository.Repository, workerIndex)
	workerLogger.Printf("repository clone completed repository=%s worker=%d worker_dir=%s", repository.Repository, workerIndex, repoDir)

	runner := skill.NewRunner(repoDir, cfg.Root(), "", cfg.App().CopilotAllowTools).WithLogger(workerLogger)
	testRunner := executor.NewTestRunner()
	pusher, creator := newPRPublisher(cfg.App().Provider)
	commentSubmitter := newPRCommentSubmitter(cfg.App().Provider)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		workerLogger.Printf("worker polling started repository=%s worker=%d", repository.Repository, workerIndex)
		if err := runRepositoryWorkerCycle(ctx, cfg, orch, runner, testRunner, pusher, creator, commentSubmitter, repoDir, repository, workerIndex, workerLogger); err != nil && ctx.Err() == nil {
			workerLogger.Printf("repository worker cycle failed repository=%s worker=%d error=%v", repository.Repository, workerIndex, err)
		} else {
			workerLogger.Printf("worker polling finished repository=%s worker=%d", repository.Repository, workerIndex)
		}

		select {
		case <-ctx.Done():
			workerLogger.Printf("worker stopped repository=%s worker=%d reason=context_done", repository.Repository, workerIndex)
			return
		case <-ticker.C:
		}
	}
}

func runRepositoryWorkerCycle(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, testRunner *executor.TestRunner, pusher BranchPusher, creator PRCreator, commentSubmitter PRCommentSubmitter, repoDir string, repository config.MonitoredRepository, workerIndex int, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	selectedJobs := jobsForRepositoryWorker(jobs, repository.Repository, workerIndex, repository.Workers)
	for _, job := range selectedJobs {
		if workerReservedByJob(job) && !workerProcessesJobState(job) {
			if logger != nil {
				logger.Printf("worker reserved repository=%s worker=%d job_id=%s state=%s", repository.Repository, workerIndex, job.ID, job.State)
			}
			return nil
		}
		if logger != nil {
			logger.Printf("job accepted repository=%s worker=%d job_id=%s state=%s type=%s", repository.Repository, workerIndex, job.ID, job.State, job.Type)
		}

		switch job.State {
		case domain.StateDetected:
			if job.Type != domain.JobTypeIssue {
				continue
			}
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d job_id=%s phase=design", repository.Repository, workerIndex, job.ID)
			}
			if err := processDesignJob(ctx, cfg, orch, runner, job, repoDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d job_id=%s phase=design error=%v", repository.Repository, workerIndex, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d job_id=%s phase=design", repository.Repository, workerIndex, job.ID)
			}
		case domain.StateImplementationRunning:
			if job.Type != domain.JobTypeIssue && job.Type != domain.JobTypePRFeedback {
				continue
			}
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d job_id=%s phase=implementation", repository.Repository, workerIndex, job.ID)
			}
			if err := processImplementationJob(ctx, cfg, orch, runner, testRunner, job, repoDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d job_id=%s phase=implementation error=%v", repository.Repository, workerIndex, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d job_id=%s phase=implementation", repository.Repository, workerIndex, job.ID)
			}
		case domain.StateCollectingContext:
			if job.Type != domain.JobTypePRReview {
				continue
			}
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d job_id=%s phase=review", repository.Repository, workerIndex, job.ID)
			}
			if err := processReviewJob(ctx, cfg, orch, runner, job, repoDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d job_id=%s phase=review error=%v", repository.Repository, workerIndex, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d job_id=%s phase=review", repository.Repository, workerIndex, job.ID)
			}
		case domain.StatePRCreating:
			if job.Type != domain.JobTypeIssue && job.Type != domain.JobTypePRFeedback {
				continue
			}
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d job_id=%s phase=pr", repository.Repository, workerIndex, job.ID)
			}
			if err := processPRJob(ctx, cfg, orch, pusher, creator, commentSubmitter, job, repoDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d job_id=%s phase=pr error=%v", repository.Repository, workerIndex, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d job_id=%s phase=pr", repository.Repository, workerIndex, job.ID)
			}
		}
	}

	return nil
}

func jobsForRepositoryWorker(jobs []domain.Job, repository string, workerIndex int, workerCount int) []domain.Job {
	var selected []domain.Job
	for _, job := range jobs {
		if !repositoryMatches(job.Repository, repository) {
			continue
		}
		if !jobAssignedToWorker(job, repository, workerIndex, workerCount) {
			continue
		}
		selected = append(selected, job)
	}
	for _, job := range selected {
		if workerReservedByJob(job) {
			return []domain.Job{job}
		}
	}
	return selected
}

func workerReservedByJob(job domain.Job) bool {
	if job.Type != domain.JobTypeIssue && job.Type != domain.JobTypePRFeedback {
		return false
	}
	switch job.State {
	case domain.StateImplementationRunning, domain.StateTestRunning, domain.StateImplementationReady, domain.StateWaitingFinalApproval, domain.StatePRCreating:
		return true
	default:
		return false
	}
}

func workerProcessesJobState(job domain.Job) bool {
	switch job.State {
	case domain.StateDetected, domain.StateImplementationRunning, domain.StateCollectingContext, domain.StatePRCreating:
		return true
	default:
		return false
	}
}

func processDesignJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, job domain.Job, repoDir string, logger *log.Logger) error {
	if logger != nil {
		logger.Printf("design job loading context job_id=%s repo_dir=%s", job.ID, repoDir)
	}
	if err := syncRepositoryWorkspace(ctx, cfg, job, repoDir, logger); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}
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

	contextData, err := buildRepositoryDesignContext(cfg, repoDir, jobDetail, events)
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
	if logger != nil {
		logger.Printf("design job ai output saved job_id=%s artifact_dir=%s skill=%s", job.ID, contextData.ArtifactDir, skillName)
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
	if logger != nil {
		logger.Printf("implementation job loading context job_id=%s repo_dir=%s", job.ID, repoDir)
	}
	if err := syncRepositoryWorkspace(ctx, cfg, job, repoDir, logger); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}
	jobDetail, events, err := orch.JobDetail(ctx, job.ID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	runSpec, err := resolveRepositoryImplementationRunSpec(cfg, repoDir, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	contextData, err := buildRepositoryImplementationContext(cfg, repoDir, jobDetail, events, runSpec)
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
	if logger != nil {
		logger.Printf("implementation job ai output saved job_id=%s artifact_dir=%s skill=%s", job.ID, contextData.ArtifactDir, runSpec.SkillName)
	}

	shouldRunTests, err := jobHasRunnableTestProfile(cfg, job)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{"error": err.Error()})
	}
	if shouldRunTests {
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
		if logger != nil {
			logger.Printf("implementation job tests finished job_id=%s success=%t artifact_dir=%s", job.ID, report.Success, contextData.ArtifactDir)
		}
		if !report.Success {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{
				"reportPath": filepath.Join(contextData.ArtifactDir, "test-report.json"),
			})
		}
	} else if logger != nil {
		logger.Printf("implementation job tests skipped job_id=%s artifact_dir=%s reason=empty_test_profile", job.ID, contextData.ArtifactDir)
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
	if logger != nil {
		logger.Printf("review job loading context job_id=%s repo_dir=%s", job.ID, repoDir)
	}
	if err := syncRepositoryWorkspace(ctx, cfg, job, repoDir, logger); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}
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

	contextData, err := buildRepositoryReviewContext(cfg, repoDir, jobDetail, events)
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
	if logger != nil {
		logger.Printf("review job ai output saved job_id=%s artifact_dir=%s skill=%s", job.ID, contextData.ArtifactDir, skillName)
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateReviewReady, "review_ready", map[string]any{
		"artifactDir": contextData.ArtifactDir,
		"skill":       skillName,
	}); err != nil {
		return err
	}
	return orch.UpdateJobState(ctx, job.ID, domain.StateReviewReady, "review_completed", map[string]any{
		"artifactDir": contextData.ArtifactDir,
		"skill":       skillName,
	})
}

func processPRJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, pusher BranchPusher, creator PRCreator, commentSubmitter PRCommentSubmitter, job domain.Job, repoDir string, logger *log.Logger) error {
	if logger != nil {
		logger.Printf("pr job preparing request job_id=%s repo_dir=%s", job.ID, repoDir)
	}
	buildReq := buildRepositoryPRCreateRequest
	if job.Type == domain.JobTypePRFeedback {
		buildReq = buildRepositoryPRFeedbackPushRequest
	}
	req, err := buildReq(ctx, cfg, job, repoDir)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}
	req.WorkDir = repoDir

	if logger != nil {
		logger.Printf("pr job pushing branch job_id=%s branch=%s artifact_dir=%s", job.ID, req.BranchName, req.ArtifactDir)
	}
	if err := pusher.Push(ctx, req); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_push_failed", map[string]any{"error": err.Error()})
	}

	if job.Type == domain.JobTypePRFeedback {
		url := fmt.Sprintf("https://github.com/%s/pull/%d", job.Repository, job.GitHubNumber)
		if logger != nil {
			logger.Printf("pr feedback job submitting review comment job_id=%s pull_number=%d", job.ID, job.GitHubNumber)
		}
		if err := commentSubmitter.Submit(ctx, PRCommentSubmitRequest{
			Repository:  job.Repository,
			PullNumber:  job.GitHubNumber,
			Body:        req.Body,
			ArtifactDir: req.ArtifactDir,
		}); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_comment_failed", map[string]any{"error": err.Error()})
		}
		if err := writePRCreateArtifact(req.ArtifactDir, url, req); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
		}
		if logger != nil {
			logger.Printf("pr feedback job completed job_id=%s url=%s", job.ID, url)
		}
		return orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "pr_updated", map[string]any{
			"url":   url,
			"title": req.Title,
			"head":  req.BranchName,
		})
	}

	if logger != nil {
		logger.Printf("pr job creating pull request job_id=%s branch=%s", job.ID, req.BranchName)
	}
	url, err := creator.Create(ctx, req)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}

	if err := writePRCreateArtifact(req.ArtifactDir, url, req); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}
	if logger != nil {
		logger.Printf("pr job completed job_id=%s url=%s", job.ID, url)
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
	workerDir := repositoryWorkerSourceDir(cfg, repository, workerIndex)
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		return "", err
	}
	source := repositoryCloneSource(repository)
	if _, err := os.Stat(filepath.Join(workerDir, ".git")); err == nil {
		if err := ensureRepositoryWorkspaceMetadata(workerDir, cfg.App().WorkspaceDir); err != nil {
			return "", err
		}
		if err := ensureRepositoryWorkerRemote(ctx, workerDir, source); err != nil {
			return "", err
		}
		if err := fetchRepositoryWorkerSource(ctx, workerDir); err != nil {
			return "", err
		}
		if err := setRepositoryWorkerRemoteHead(ctx, workerDir); err != nil {
			return "", err
		}
		if err := checkoutRepositoryWorkerBranch(ctx, workerDir); err != nil {
			return "", err
		}
		return workerDir, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err := initializeRepositoryWorkerGitDir(ctx, workerDir); err != nil {
		return "", err
	}
	if err := ensureRepositoryWorkerRemote(ctx, workerDir, source); err != nil {
		return "", err
	}
	if err := fetchRepositoryWorkerSource(ctx, workerDir); err != nil {
		return "", err
	}
	if err := setRepositoryWorkerRemoteHead(ctx, workerDir); err != nil {
		return "", err
	}
	if err := ensureRepositoryWorkspaceMetadata(workerDir, cfg.App().WorkspaceDir); err != nil {
		return "", err
	}
	if err := checkoutRepositoryWorkerBranch(ctx, workerDir); err != nil {
		return "", err
	}
	return workerDir, nil
}

func syncRepositoryWorkspace(ctx context.Context, cfg *config.Service, job domain.Job, repoDir string, logger *log.Logger) error {
	if job.Type == domain.JobTypePRFeedback {
		branchName := strings.TrimSpace(job.BranchName)
		if branchName == "" {
			return fmt.Errorf("resolve pull request branch: branch is empty")
		}
		if logger != nil {
			logger.Printf("syncing repository workspace job_id=%s repo_dir=%s pull_request_branch=%s", job.ID, repoDir, branchName)
		}
		commands := [][]string{
			{"git", "fetch", "--prune", "origin"},
			{"git", "checkout", "-f", "-B", branchName, "origin/" + branchName},
			{"git", "reset", "--hard", "origin/" + branchName},
			{"git", "clean", "-fd"},
		}
		for _, command := range commands {
			if _, err := runGitCommand(ctx, repoDir, command...); err != nil {
				return err
			}
		}
		return nil
	}

	configuredBranch := resolveMonitoredRepositoryBranch(cfg, job.Repository)
	baseBranch, err := resolveRepositoryBaseBranch(ctx, repoDir, configuredBranch)
	if err != nil {
		return err
	}

	if logger != nil {
		logger.Printf("syncing repository workspace job_id=%s repo_dir=%s base_branch=%s", job.ID, repoDir, baseBranch)
	}

	commands := [][]string{
		{"git", "fetch", "--prune", "origin"},
		{"git", "checkout", "-f", "-B", baseBranch, "origin/" + baseBranch},
		{"git", "reset", "--hard", "origin/" + baseBranch},
		{"git", "clean", "-fd"},
	}
	for _, command := range commands {
		if _, err := runGitCommand(ctx, repoDir, command...); err != nil {
			return err
		}
	}
	return nil
}

func resolveRepositoryBaseBranch(ctx context.Context, repoDir string, configuredBranch string) (string, error) {
	if branch := strings.TrimSpace(configuredBranch); branch != "" {
		return branch, nil
	}

	output, err := runGitCommand(ctx, repoDir, "git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		const prefix = "origin/"
		trimmed := strings.TrimSpace(output)
		if strings.HasPrefix(trimmed, prefix) && len(trimmed) > len(prefix) {
			return trimmed[len(prefix):], nil
		}
	}

	output, err = runGitCommand(ctx, repoDir, "git", "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("resolve base branch: %w", err)
	}
	branch := strings.TrimSpace(output)
	if branch == "" {
		return "", fmt.Errorf("resolve base branch: branch is empty")
	}
	return branch, nil
}

func resolveMonitoredRepositoryBranch(cfg *config.Service, repository string) string {
	for _, monitored := range cfg.App().MonitoredRepositories {
		if !repositoryMatches(repository, monitored.Repository) {
			continue
		}
		return strings.TrimSpace(monitored.Branch)
	}
	return ""
}

func runGitCommand(ctx context.Context, repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = repoDir
	raw, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(raw))
	if err != nil {
		return output, fmt.Errorf("%s failed: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func newRepositoryWorkerLogger(cfg *config.Service, fallback *log.Logger, workerDir string, startedAt time.Time) (*log.Logger, func(), error) {
	_ = fallback
	logPath := artifacts.RepositoryWorkerLogPathFromWorkerDir(workerDir, cfg.App().WorkspaceDir, startedAt)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, func() {}, err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, func() {}, err
	}
	logger := log.New(file, "", log.LstdFlags)
	return logger, func() { _ = file.Close() }, nil
}

func repositoryWorkerSourceDir(cfg *config.Service, repository string, workerIndex int) string {
	for _, monitored := range cfg.App().MonitoredRepositories {
		if !repositoryMatches(repository, monitored.Repository) {
			continue
		}
		if workerDir := repositoryWorkerDirectoryForIndex(cfg.Root(), monitored, workerIndex); workerDir != "" {
			return filepath.Join(workerDir, fmt.Sprintf("worker-%d", workerIndex))
		}
	}
	return artifacts.RepositoryWorkerSourceDir(cfg.Root(), cfg.App().ArtifactsDir, repository, workerIndex)
}

func repositoryWorkerDirectoryForIndex(root string, monitored config.MonitoredRepository, workerIndex int) string {
	if workerIndex >= 0 && workerIndex < len(monitored.WorkerDirs) {
		if trimmed := strings.TrimSpace(monitored.WorkerDirs[workerIndex]); trimmed != "" {
			if filepath.IsAbs(trimmed) {
				return filepath.Clean(trimmed)
			}
			return filepath.Join(root, trimmed)
		}
	}
	trimmed := strings.TrimSpace(monitored.WorkerDir)
	if trimmed != "" {
		if filepath.IsAbs(trimmed) {
			return filepath.Clean(trimmed)
		}
		return filepath.Join(root, trimmed)
	}
	return ""
}

func ensureRepositoryWorkspaceMetadata(workerDir string, workspaceDir string) error {
	workspacePath := artifacts.RepositoryWorkerWorkspaceDir(workerDir, workspaceDir)
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(workspacePath, "logs"), 0o755); err != nil {
		return err
	}
	return ensureGitExcludeEntry(workerDir, workspaceDir)
}

func initializeRepositoryWorkerGitDir(ctx context.Context, workerDir string) error {
	if _, err := runGitCommand(ctx, workerDir, "git", "init"); err != nil {
		return err
	}
	return nil
}

func ensureRepositoryWorkerRemote(ctx context.Context, workerDir string, source string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("repository source is empty")
	}

	if _, err := runGitCommand(ctx, workerDir, "git", "remote", "get-url", "origin"); err == nil {
		if _, err := runGitCommand(ctx, workerDir, "git", "remote", "set-url", "origin", source); err != nil {
			return err
		}
		return nil
	}

	if _, err := runGitCommand(ctx, workerDir, "git", "remote", "add", "origin", source); err != nil {
		return err
	}
	return nil
}

func fetchRepositoryWorkerSource(ctx context.Context, workerDir string) error {
	if _, err := runGitCommand(ctx, workerDir, "git", "fetch", "--prune", "--tags", "origin"); err != nil {
		return err
	}
	return nil
}

func setRepositoryWorkerRemoteHead(ctx context.Context, workerDir string) error {
	_, _ = runGitCommand(ctx, workerDir, "git", "remote", "set-head", "origin", "-a")
	return nil
}

func checkoutRepositoryWorkerBranch(ctx context.Context, workerDir string) error {
	branch, err := resolveRepositoryBaseBranch(ctx, workerDir, "")
	if err != nil {
		return err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}

	commands := [][]string{
		{"git", "checkout", "-f", "-B", branch, "origin/" + branch},
		{"git", "reset", "--hard", "origin/" + branch},
		{"git", "clean", "-fd"},
	}
	for _, command := range commands {
		if _, err := runGitCommand(ctx, workerDir, command...); err != nil {
			return err
		}
	}
	return nil
}

func ensureGitExcludeEntry(workerDir string, workspaceDir string) error {
	trimmed := strings.TrimSpace(workspaceDir)
	if trimmed == "" {
		trimmed = ".workspace"
	}
	if filepath.IsAbs(trimmed) {
		return nil
	}

	excludePath := filepath.Join(workerDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}

	entry := strings.TrimSpace(filepath.ToSlash(trimmed))
	if !strings.HasSuffix(entry, "/") {
		entry += "/"
	}

	raw, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(raw), entry) {
		return nil
	}

	f, err := os.OpenFile(excludePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(entry + "\n"); err != nil {
		return err
	}
	return nil
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
	_, _ = h.Write([]byte(canonicalRepositoryID(repository)))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(job.ID))
	return int(h.Sum32()%uint32(workerCount)) == workerIndex
}

func repositoryMatches(jobRepository string, configuredRepository string) bool {
	return canonicalRepositoryID(jobRepository) == canonicalRepositoryID(configuredRepository)
}

func canonicalRepositoryID(repository string) string {
	trimmed := strings.TrimSpace(repository)
	if trimmed == "" {
		return ""
	}

	candidate := strings.TrimSuffix(trimmed, ".git")
	if strings.HasPrefix(candidate, "git@") {
		if idx := strings.LastIndex(candidate, ":"); idx >= 0 && idx+1 < len(candidate) {
			candidate = candidate[idx+1:]
		}
	}
	if strings.Contains(candidate, "://") {
		if parsed, err := url.Parse(candidate); err == nil {
			candidate = strings.Trim(parsed.Path, "/")
		}
	}

	candidate = strings.Trim(path.Clean(strings.ReplaceAll(candidate, "\\", "/")), "/")
	parts := strings.Split(candidate, "/")
	if len(parts) >= 2 {
		candidate = strings.Join(parts[len(parts)-2:], "/")
	}
	return strings.ToLower(candidate)
}
