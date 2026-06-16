package app

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/executor"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

type repositoryWorkerKind string

const (
	repositoryWorkerKindImplementation repositoryWorkerKind = "implementation"
	repositoryWorkerKindReview         repositoryWorkerKind = "review"
)

var repositoryWorkspaceLocks sync.Map

func startRepositoryWorkers(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	repositories := append([]config.MonitoredRepository(nil), cfg.App().MonitoredRepositories...)
	if len(repositories) == 0 {
		repositories = repositoryWorkersFromJobs(ctx, orch)
	}
	for _, repository := range repositories {
		repoName := strings.TrimSpace(repository.Repository)
		if repoName == "" {
			continue
		}
		implementationWorkers := repository.ImplementationWorkers
		if implementationWorkers < 1 {
			implementationWorkers = 1
		}
		reviewWorkers := repository.ReviewWorkers
		if reviewWorkers < 1 {
			reviewWorkers = 1
		}
		for workerIndex := 0; workerIndex < implementationWorkers; workerIndex++ {
			workerIndex := workerIndex
			repository := repository
			go runRepositoryWorker(ctx, cfg, orch, logger, repository, workerIndex, repositoryWorkerKindImplementation)
		}
		for workerIndex := 0; workerIndex < reviewWorkers; workerIndex++ {
			workerIndex := workerIndex
			repository := repository
			go runRepositoryWorker(ctx, cfg, orch, logger, repository, workerIndex, repositoryWorkerKindReview)
		}
	}
	return nil
}

func repositoryWorkersFromJobs(ctx context.Context, orch *orchestrator.Orchestrator) []config.MonitoredRepository {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	repositories := make([]config.MonitoredRepository, 0, len(jobs))
	for _, job := range jobs {
		repository := strings.TrimSpace(job.Repository)
		if repository == "" {
			continue
		}
		if _, ok := seen[repository]; ok {
			continue
		}
		seen[repository] = struct{}{}
		repositories = append(repositories, config.MonitoredRepository{
			Repository:            repository,
			ImplementationWorkers: 1,
			ReviewWorkers:         1,
		})
	}
	return repositories
}

func runRepositoryWorker(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger, repository config.MonitoredRepository, workerIndex int, kind repositoryWorkerKind) {
	workDir, err := prepareRepositoryWorkspace(ctx, cfg, repository.Repository, repository.WorkDir)
	if err != nil {
		if logger != nil {
			logger.Printf("repository workdir preparation failed repository=%s worker=%d kind=%s error=%v", repository.Repository, workerIndex, kind, err)
		}
		return
	}
	improvementWorkDir := ""
	if repository.ImprovementEnabled {
		improvementWorkDir, err = prepareRepositoryImprovementWorkspace(ctx, cfg, repository)
		if err != nil {
			if logger != nil {
				logger.Printf("repository improvement workdir preparation failed repository=%s worker=%d kind=%s error=%v", repository.Repository, workerIndex, kind, err)
			}
			return
		}
	}

	workerLogger, cleanup, err := newRepositoryWorkerLogger(cfg, logger, repository.Repository, workerIndex, time.Now())
	if err != nil {
		if logger != nil {
			logger.Printf("repository worker logger init failed repository=%s worker=%d kind=%s error=%v", repository.Repository, workerIndex, kind, err)
		}
		return
	}
	defer cleanup()

	workerLogger.Printf("worker started repository=%s worker=%d kind=%s", repository.Repository, workerIndex, kind)
	if repository.ImprovementEnabled {
		workerLogger.Printf("repository improvement workspace enabled repository=%s worker=%d kind=%s branch=%s work_dir=%s", repository.Repository, workerIndex, kind, config.ResolveImprovementBranch(repository), improvementWorkDir)
		if err := syncRepositoryImprovementWorkspace(ctx, cfg, repository, improvementWorkDir, workerLogger); err != nil {
			workerLogger.Printf("repository improvement workspace sync failed repository=%s worker=%d kind=%s error=%v", repository.Repository, workerIndex, kind, err)
			return
		}
	} else {
		workerLogger.Printf("repository improvement workspace disabled repository=%s worker=%d kind=%s", repository.Repository, workerIndex, kind)
	}
	workerLogger.Printf("repository base checkout ready repository=%s worker=%d kind=%s work_dir=%s", repository.Repository, workerIndex, kind, workDir)

	testRunner := executor.NewTestRunner()
	pusher, creator := newPRPublisher(cfg.App().Provider)
	commentSubmitter := newPRCommentSubmitter(cfg.App().Provider)
	commentFetcher := newPRCommentFetcher(cfg.App().Provider)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		workerLogger.Printf("worker polling started repository=%s worker=%d kind=%s", repository.Repository, workerIndex, kind)
		if err := runRepositoryWorkerCycle(ctx, cfg, orch, testRunner, pusher, creator, commentSubmitter, commentFetcher, workDir, improvementWorkDir, repository, workerIndex, kind, workerLogger); err != nil && ctx.Err() == nil {
			workerLogger.Printf("repository worker cycle failed repository=%s worker=%d kind=%s error=%v", repository.Repository, workerIndex, kind, err)
		} else {
			workerLogger.Printf("worker polling finished repository=%s worker=%d kind=%s", repository.Repository, workerIndex, kind)
		}

		select {
		case <-ctx.Done():
			workerLogger.Printf("worker stopped repository=%s worker=%d kind=%s reason=context_done", repository.Repository, workerIndex, kind)
			return
		case <-ticker.C:
		}
	}
}

func runRepositoryWorkerCycle(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, testRunner *executor.TestRunner, pusher BranchPusher, creator PRCreator, commentSubmitter PRCommentSubmitter, commentFetcher PRCommentFetcher, workDir string, improvementWorkDir string, repository config.MonitoredRepository, workerIndex int, kind repositoryWorkerKind, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	workerCount := repository.ImplementationWorkers
	allowedJobTypes := []domain.JobType{domain.JobTypeIssue, domain.JobTypePRFeedback}
	allowedStates := map[string]struct{}{
		string(domain.StateDetected):              {},
		string(domain.StateImplementationRunning): {},
		string(domain.StatePRCreating):            {},
	}
	phase := "implementation"
	if kind == repositoryWorkerKindReview {
		workerCount = repository.ReviewWorkers
		allowedJobTypes = []domain.JobType{domain.JobTypePRReview}
		allowedStates = map[string]struct{}{
			string(domain.StateCollectingContext): {},
		}
		phase = "review"
	}
	selectedJobs := jobsForRepositoryWorker(jobs, repository.Repository, workerIndex, workerCount)
	for _, job := range selectedJobs {
		if !jobTypeAllowed(job.Type, allowedJobTypes) {
			continue
		}
		if _, ok := allowedStates[string(job.State)]; !ok {
			continue
		}
		if kind == repositoryWorkerKindImplementation && workerReservedByJob(job) && !workerProcessesJobState(job) {
			if logger != nil {
				logger.Printf("worker reserved repository=%s worker=%d kind=%s job_id=%s state=%s", repository.Repository, workerIndex, kind, job.ID, job.State)
			}
			continue
		}
		if logger != nil {
			logger.Printf("job accepted repository=%s worker=%d kind=%s job_id=%s state=%s type=%s", repository.Repository, workerIndex, kind, job.ID, job.State, job.Type)
		}

		jobDir, err := cloneRepositoryWorkspaceForJob(ctx, cfg, repository.Repository, jobWorkspaceBranch(cfg, repository, job), workDir)
		if err != nil {
			if logger != nil {
				logger.Printf("repository worktree clone failed repository=%s worker=%d kind=%s job_id=%s error=%v", repository.Repository, workerIndex, kind, job.ID, err)
			}
			if interruptedErr := markRepositoryWorkerJobInterrupted(ctx, orch, job, err); interruptedErr != nil {
				return fmt.Errorf("mark interrupted failed: %w (cause: %v)", interruptedErr, err)
			}
			return err
		}

		switch job.State {
		case domain.StateDetected:
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d kind=%s job_id=%s phase=design", repository.Repository, workerIndex, kind, job.ID)
			}
			if err := processDesignJob(ctx, cfg, orch, job, workDir, improvementWorkDir, jobDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d kind=%s job_id=%s phase=design error=%v", repository.Repository, workerIndex, kind, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d kind=%s job_id=%s phase=design", repository.Repository, workerIndex, kind, job.ID)
			}
		case domain.StateImplementationRunning:
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d kind=%s job_id=%s phase=implementation", repository.Repository, workerIndex, kind, job.ID)
			}
			if err := processImplementationJob(ctx, cfg, orch, testRunner, job, workDir, improvementWorkDir, jobDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d kind=%s job_id=%s phase=implementation error=%v", repository.Repository, workerIndex, kind, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d kind=%s job_id=%s phase=implementation", repository.Repository, workerIndex, kind, job.ID)
			}
		case domain.StateCollectingContext:
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d kind=%s job_id=%s phase=%s", repository.Repository, workerIndex, kind, job.ID, phase)
			}
			if err := processReviewJob(ctx, cfg, orch, job, workDir, improvementWorkDir, jobDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d kind=%s job_id=%s phase=%s error=%v", repository.Repository, workerIndex, kind, job.ID, phase, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d kind=%s job_id=%s phase=%s", repository.Repository, workerIndex, kind, job.ID, phase)
			}
		case domain.StatePRCreating:
			if logger != nil {
				logger.Printf("job processing started repository=%s worker=%d kind=%s job_id=%s phase=pr", repository.Repository, workerIndex, kind, job.ID)
			}
			if err := processPRJob(ctx, cfg, orch, pusher, creator, commentSubmitter, commentFetcher, job, workDir, jobDir, logger); err != nil {
				if logger != nil {
					logger.Printf("job processing failed repository=%s worker=%d kind=%s job_id=%s phase=pr error=%v", repository.Repository, workerIndex, kind, job.ID, err)
				}
				return err
			}
			if logger != nil {
				logger.Printf("job processing finished repository=%s worker=%d kind=%s job_id=%s phase=pr", repository.Repository, workerIndex, kind, job.ID)
			}
		}
	}

	return nil
}

func jobTypeAllowed(jobType domain.JobType, allowed []domain.JobType) bool {
	for _, candidate := range allowed {
		if jobType == candidate {
			return true
		}
	}
	return false
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
	return selected
}

func markRepositoryWorkerJobInterrupted(ctx context.Context, orch *orchestrator.Orchestrator, job domain.Job, cause error) error {
	eventType, ok := repositoryWorkerInterruptedEventType(job.State)
	if !ok {
		return cause
	}
	return orch.UpdateJobState(ctx, job.ID, domain.StateInterrupted, eventType, map[string]any{
		"error": cause.Error(),
	})
}

func repositoryWorkerInterruptedEventType(state domain.JobState) (string, bool) {
	switch state {
	case domain.StateDetected, domain.StateDesignRunning, domain.StateDesignReady, domain.StateWaitingDesignApproval:
		return "design_interrupted", true
	case domain.StateImplementationRunning, domain.StateImplementationReady, domain.StateWaitingFinalApproval:
		return "implementation_interrupted", true
	case domain.StateTestRunning:
		return "test_interrupted", true
	case domain.StateCollectingContext, domain.StateReviewRunning, domain.StateReviewReady:
		return "review_interrupted", true
	case domain.StatePRCreating:
		return "pr_interrupted", true
	default:
		return "", false
	}
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

func processDesignJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, job domain.Job, workDir string, improvementWorkDir string, repoDir string, logger *log.Logger) error {
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
	if repositoryConfig, ok := resolveMonitoredRepository(cfg, job.Repository); ok {
		if err := syncRepositoryImprovementWorkspace(ctx, cfg, repositoryConfig, improvementWorkDir, logger); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
		}
	}

	contextData, err := buildRepositoryDesignContext(cfg, workDir, improvementWorkDir, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	runner := skill.NewRunner(repoDir, cfg.Root(), "", cfg.App().CopilotAllowTools).WithLogger(logger)
	skillName, err := resolveDesignSkillName(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}

	result, err := runner.RunDesign(ctx, skillName, contextData, execution)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "design_failed", map[string]any{"error": err.Error()})
	}
	saveJobSessionID(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, result.SessionID)
	if err := copyAIResultToWorkDir(workDir, artifacts.WorkerDesign, job, contextData.ArtifactDir); err != nil {
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

func processImplementationJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, testRunner *executor.TestRunner, job domain.Job, workDir string, improvementWorkDir string, repoDir string, logger *log.Logger) error {
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

	runSpec, err := resolveRepositoryImplementationRunSpec(cfg, workDir, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}
	if repositoryConfig, ok := resolveMonitoredRepository(cfg, job.Repository); ok {
		if err := syncRepositoryImprovementWorkspace(ctx, cfg, repositoryConfig, improvementWorkDir, logger); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
		}
	}

	contextData, err := buildRepositoryImplementationContext(cfg, workDir, improvementWorkDir, jobDetail, events, runSpec)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	runner := skill.NewRunner(repoDir, cfg.Root(), "", cfg.App().CopilotAllowTools).WithLogger(logger)
	execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}

	result, err := runner.RunImplementation(ctx, runSpec.SkillName, contextData, execution)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
	}
	saveJobSessionID(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, result.SessionID)
	if err := copyAIResultToWorkDir(workDir, filepath.Base(runSpec.ArtifactDir), job, contextData.ArtifactDir); err != nil {
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

func processReviewJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, job domain.Job, workDir string, improvementWorkDir string, repoDir string, logger *log.Logger) error {
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
	if repositoryConfig, ok := resolveMonitoredRepository(cfg, job.Repository); ok {
		if err := syncRepositoryImprovementWorkspace(ctx, cfg, repositoryConfig, improvementWorkDir, logger); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
		}
	}

	contextData, err := buildRepositoryReviewContext(cfg, workDir, improvementWorkDir, jobDetail, events)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	runner := skill.NewRunner(repoDir, cfg.Root(), "", cfg.App().CopilotAllowTools).WithLogger(logger)
	skillName, err := resolveReviewSkillName(cfg, jobDetail.WatchRuleID)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}

	result, err := runner.RunReview(ctx, skillName, contextData, execution)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "review_failed", map[string]any{"error": err.Error()})
	}
	saveJobSessionID(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, result.SessionID)
	if err := copyAIResultToWorkDir(workDir, artifacts.WorkerReview, job, contextData.ArtifactDir); err != nil {
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

func processPRJob(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, pusher BranchPusher, creator PRCreator, commentSubmitter PRCommentSubmitter, commentFetcher PRCommentFetcher, job domain.Job, workDir string, repoDir string, logger *log.Logger) error {
	if logger != nil {
		logger.Printf("pr job preparing request job_id=%s repo_dir=%s", job.ID, repoDir)
	}
	buildReq := buildRepositoryPRCreateRequest
	if job.Type == domain.JobTypePRFeedback {
		buildReq = buildRepositoryPRFeedbackPushRequest
	}
	req, err := buildReq(ctx, cfg, job, workDir)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}
	req.WorkDir = repoDir
	dummyRepository := domain.IsDummyRepository(job.Repository)
	if dummyRepository {
		if logger != nil {
			logger.Printf("pr job skipped external pr operations job_id=%s reason=dummy_repository", job.ID)
		}
		result := PRCreateResult{}
		if err := writePRCreateArtifact(req.ArtifactDir, result, req, false); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
		}
		transitionType := "pr_created"
		nextState := domain.StateCompleted
		if job.Type == domain.JobTypePRFeedback {
			transitionType = "pr_updated"
		}
		return orch.UpdateJobState(ctx, job.ID, nextState, transitionType, map[string]any{
			"url":        result.URL,
			"pullNumber": result.PullNumber,
			"title":      req.Title,
			"head":       req.BranchName,
		})
	}

	if logger != nil {
		logger.Printf("pr job pushing branch job_id=%s branch=%s artifact_dir=%s", job.ID, req.BranchName, req.ArtifactDir)
	}
	if err := pusher.Push(ctx, req); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_push_failed", map[string]any{"error": err.Error()})
	}

	if job.Type == domain.JobTypePRFeedback {
		result := PRCreateResult{URL: fmt.Sprintf("https://github.com/%s/pull/%d", job.Repository, job.GitHubNumber), PullNumber: job.GitHubNumber}
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
		if err := writePRCreateArtifact(req.ArtifactDir, result, req, true); err != nil {
			return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
		}
		if logger != nil {
			logger.Printf("pr feedback job completed job_id=%s url=%s", job.ID, result.URL)
		}
		return orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "pr_updated", map[string]any{
			"url":        result.URL,
			"pullNumber": result.PullNumber,
			"title":      req.Title,
			"head":       req.BranchName,
		})
	}

	if logger != nil {
		logger.Printf("pr job creating pull request job_id=%s branch=%s", job.ID, req.BranchName)
	}
	result, err := creator.Create(ctx, req)
	if err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}

	if err := writePRCreateArtifact(req.ArtifactDir, result, req, true); err != nil {
		return orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
	}
	if logger != nil {
		logger.Printf("pr job completed job_id=%s url=%s pull_number=%d", job.ID, result.URL, result.PullNumber)
	}

	if job.Type == domain.JobTypeIssue && commentFetcher != nil && result.PullNumber > 0 {
		if logger != nil {
			logger.Printf("pr job fetching comments job_id=%s pull_number=%d", job.ID, result.PullNumber)
		}
		if _, err := commentFetcher.Fetch(ctx, PRCommentFetchRequest{
			Repository:  job.Repository,
			PullNumber:  result.PullNumber,
			ArtifactDir: req.ArtifactDir,
		}); err != nil && logger != nil {
			logger.Printf("pr comment fetch failed job_id=%s pull_number=%d error=%v", job.ID, result.PullNumber, err)
		}
	}

	if err := orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "pr_created", map[string]any{
		"url":        result.URL,
		"pullNumber": result.PullNumber,
		"title":      req.Title,
		"head":       req.BranchName,
	}); err != nil {
		if logger != nil {
			logger.Printf("pr_created state transition failed for %s: %v", job.ID, err)
		}
		return err
	}

	if providerSupportsMockPRReviewBootstrap(cfg.App().Provider) && job.Type == domain.JobTypeIssue && result.PullNumber > 0 {
		if err := startPRReviewJobFromCreatedPR(ctx, cfg, orch, job, req, result); err != nil && logger != nil {
			logger.Printf("start pr review failed job_id=%s pull_number=%d error=%v", job.ID, result.PullNumber, err)
		}
	}
	return nil
}

func startPRReviewJobFromCreatedPR(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, sourceJob domain.Job, req PRCreateRequest, result PRCreateResult) error {
	rule, ok := resolveRepositoryPRReviewRule(cfg, sourceJob.Repository)
	if !ok {
		return nil
	}

	event := domain.DomainEvent{
		Type:     domain.DomainEventPRMatched,
		RuleID:   rule.ID,
		RuleName: rule.Name,
		Item: domain.RepositoryItem{
			Repository:   sourceJob.Repository,
			Number:       result.PullNumber,
			Title:        req.Title,
			Body:         req.Body,
			URL:          result.URL,
			UpdatedAt:    time.Now().UTC(),
			Target:       domain.TargetPullRequest,
			BranchName:   req.BranchName,
			BaseBranch:   req.BaseBranch,
			Labels:       append([]string(nil), rule.Labels...),
			Assignees:    []string{},
			Reviewers:    []string{},
			DefaultState: domain.StateCollectingContext,
		},
		MatchedAt: time.Now().UTC(),
	}
	return orch.ProcessMatch(ctx, cfg.App(), rule, event)
}

func resolveRepositoryPRReviewRule(cfg *config.Service, repository string) (config.WatchRule, bool) {
	for _, rule := range cfg.WatchRules().Rules {
		if !rule.Enabled {
			continue
		}
		if strings.TrimSpace(rule.Target) != string(domain.TargetPullRequest) {
			continue
		}
		if !repositoryListMatches(rule.Repositories, repository) {
			continue
		}
		return rule, true
	}
	return config.WatchRule{
		ID:           "default-pr-review",
		Name:         "Default PR Review Rule",
		Repositories: []string{repository},
		Target:       string(domain.TargetPullRequest),
		Labels:       []string{"ai:review"},
		SkillSet:     "default",
		TestProfile:  "go-default",
		Enabled:      true,
	}, true
}

func repositoryListMatches(values []string, repository string) bool {
	for _, value := range values {
		if repositoryMatches(repository, value) {
			return true
		}
	}
	return false
}

func prepareRepositoryWorkspaces(ctx context.Context, cfg *config.Service) error {
	for _, repository := range cfg.App().MonitoredRepositories {
		if strings.TrimSpace(repository.Repository) == "" {
			continue
		}
		if repository.ImplementationWorkers < 1 && repository.ReviewWorkers < 1 {
			continue
		}
		if _, err := prepareRepositoryWorkspace(ctx, cfg, repository.Repository, repository.WorkDir); err != nil {
			return err
		}
		if repository.ImprovementEnabled {
			if _, err := prepareRepositoryImprovementWorkspace(ctx, cfg, repository); err != nil {
				return err
			}
		}
	}
	return nil
}

func prepareRepositoryWorkspace(ctx context.Context, cfg *config.Service, repository string, workDirSetting string) (string, error) {
	var workDir string
	err := withRepositoryWorkspaceLock(repository, func() error {
		workDir = artifacts.RepositoryWorkerWorkDir(cfg.Root(), cfg.App().ArtifactsDir, repository, workDirSetting)
		if err := os.MkdirAll(filepath.Dir(workDir), 0o755); err != nil {
			return err
		}
		if domain.IsDummyRepository(repository) {
			if err := os.MkdirAll(workDir, 0o755); err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(workDir, ".git")); errors.Is(err, os.ErrNotExist) {
				if err := initializeRepositoryWorkerGitDir(ctx, workDir); err != nil {
					return err
				}
			}
			if err := removeRepositoryWorkerWorkspace(workDir, cfg.App().WorkspaceDir); err != nil {
				return err
			}
			return nil
		}
		if err := ensureRepositoryWorkerClone(ctx, workDir, repositoryCloneSource(repository), cfg.App().WorkspaceDir); err != nil {
			return err
		}
		if err := ensureRepositoryWorkerRemote(ctx, workDir, repositoryCloneSource(repository)); err != nil {
			return err
		}
		return nil
	})
	return workDir, err
}

func cloneRepositoryWorkspace(ctx context.Context, cfg *config.Service, repository string, workerIndex int, workDir ...string) (string, error) {
	branchName := strings.TrimSpace(resolveMonitoredRepositoryBranch(cfg, repository))
	if branchName == "" {
		branchName = "main"
	}
	return cloneRepositoryWorkspaceForBranch(ctx, cfg, repository, branchName, workDir...)
}

func cloneRepositoryWorkspaceForJob(ctx context.Context, cfg *config.Service, repository string, branch string, workDir string) (string, error) {
	return cloneRepositoryWorkspaceForBranch(ctx, cfg, repository, branch, workDir)
}

func jobWorkspaceBranch(cfg *config.Service, repository config.MonitoredRepository, job domain.Job) string {
	branch := strings.TrimSpace(job.BranchName)
	if branch != "" {
		return branch
	}
	branch = strings.TrimSpace(repository.Branch)
	if branch != "" {
		return branch
	}
	return strings.TrimSpace(resolveMonitoredRepositoryBranch(cfg, repository.Repository))
}

func cloneRepositoryWorkspaceForBranch(ctx context.Context, cfg *config.Service, repository string, branch string, workDir ...string) (string, error) {
	baseDir := ""
	if len(workDir) > 0 && strings.TrimSpace(workDir[0]) != "" {
		baseDir = workDir[0]
	} else {
		preparedDir, err := prepareRepositoryWorkspace(ctx, cfg, repository, resolveRepositoryConfiguredWorkDirSetting(cfg, repository))
		if err != nil {
			return "", err
		}
		baseDir = preparedDir
	}
	if domain.IsDummyRepository(repository) {
		return baseDir, nil
	}

	branchName := strings.TrimSpace(branch)
	if branchName == "" {
		branchName = "main"
	}
	var sourceDir string
	err := withRepositoryWorkspaceLock(repository, func() error {
		sourceDir = artifacts.RepositoryWorkerBranchDir(baseDir, branchName)
		if err := ensureRepositoryWorkerWorktree(ctx, baseDir, sourceDir, branchName, repositoryCloneSource(repository), cfg.App().WorkspaceDir); err != nil {
			return err
		}
		return nil
	})
	return sourceDir, err
}

func prepareRepositoryImprovementWorkspace(ctx context.Context, cfg *config.Service, repository config.MonitoredRepository) (string, error) {
	var improvementDir string
	err := withRepositoryWorkspaceLock(repository.Repository, func() error {
		preparedDir, err := prepareRepositoryImprovementWorkspaceLocked(ctx, cfg, repository)
		if err != nil {
			return err
		}
		improvementDir = preparedDir
		return nil
	})
	return improvementDir, err
}

func prepareRepositoryImprovementWorkspaceLocked(ctx context.Context, cfg *config.Service, repository config.MonitoredRepository) (string, error) {
	improvementBranch := config.ResolveImprovementBranch(repository)
	improvementDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, repository.Repository, improvementBranch)
	baseDir := artifacts.RepositoryWorkerWorkDir(cfg.Root(), cfg.App().ArtifactsDir, repository.Repository, repository.WorkDir)
	if err := os.MkdirAll(filepath.Dir(improvementDir), 0o755); err != nil {
		return "", err
	}
	if domain.IsDummyRepository(repository.Repository) {
		if err := os.MkdirAll(improvementDir, 0o755); err != nil {
			return "", err
		}
		return improvementDir, nil
	}
	if err := ensureRepositoryWorkerClone(ctx, baseDir, repositoryCloneSource(repository.Repository), cfg.App().WorkspaceDir); err != nil {
		return "", err
	}
	if err := ensureRepositoryWorkerRemote(ctx, baseDir, repositoryCloneSource(repository.Repository)); err != nil {
		return "", err
	}
	if err := ensureRepositoryImprovementWorktree(ctx, baseDir, improvementDir, repository, improvementBranch); err != nil {
		return "", err
	}
	return improvementDir, nil
}

func resolveRepositoryConfiguredWorkDirSetting(cfg *config.Service, repository string) string {
	for _, monitored := range cfg.App().MonitoredRepositories {
		if canonicalRepositoryID(monitored.Repository) != canonicalRepositoryID(repository) {
			continue
		}
		return monitored.WorkDir
	}
	return ""
}

func ensureRepositoryWorkerClone(ctx context.Context, targetDir string, source string, workspaceDir string) (err error) {
	if info, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
		if info.IsDir() {
			_ = removeRepositoryWorkerWorkspace(targetDir, workspaceDir)
			_, _ = runGitCommand(ctx, targetDir, "git", "checkout", "--detach")
			return nil
		}
		return fmt.Errorf("%s exists but is not a git repository", targetDir)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--quiet", source, targetDir)
	raw, cloneErr := cmd.CombinedOutput()
	if cloneErr != nil {
		err = fmt.Errorf("git clone failed: %w: %s", cloneErr, strings.TrimSpace(string(raw)))
		return err
	}
	if err := removeRepositoryWorkerWorkspace(targetDir, workspaceDir); err != nil {
		return err
	}
	if _, err := runGitCommand(ctx, targetDir, "git", "checkout", "--detach"); err != nil {
		return err
	}
	return nil
}

func ensureRepositoryWorkerWorktree(ctx context.Context, baseDir string, worktreeDir string, branch string, source string, workspaceDir string) error {
	if exists, stale, err := repositoryWorkerWorktreeStatus(ctx, baseDir, worktreeDir); err != nil {
		return err
	} else if exists {
		return nil
	} else if stale {
		if err := cleanupRepositoryWorkerWorktreeRegistration(ctx, baseDir, worktreeDir); err != nil {
			return err
		}
	}

	if info, err := os.Stat(filepath.Join(worktreeDir, ".git")); err == nil {
		if info.IsDir() {
			return removeRepositoryWorkerWorkspace(worktreeDir, workspaceDir)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.RemoveAll(worktreeDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return err
	}
	if _, err := runGitCommand(ctx, baseDir, "git", "fetch", "--prune", "origin"); err != nil {
		return err
	}

	if err := createRepositoryWorkerWorktree(ctx, baseDir, worktreeDir, branch); err != nil {
		if isRepositoryWorkerWorktreeRegistrationError(err) {
			if cleanupErr := cleanupRepositoryWorkerWorktreeRegistration(ctx, baseDir, worktreeDir); cleanupErr != nil {
				return fmt.Errorf("%w; cleanup failed: %v", err, cleanupErr)
			}
			if retryErr := createRepositoryWorkerWorktree(ctx, baseDir, worktreeDir, branch); retryErr != nil {
				return retryErr
			}
		} else {
			return err
		}
	}
	if err := ensureRepositoryWorkerRemote(ctx, worktreeDir, source); err != nil {
		return err
	}
	if err := removeRepositoryWorkerWorkspace(worktreeDir, workspaceDir); err != nil {
		return err
	}
	return nil
}

func repositoryWorkerWorktreeStatus(ctx context.Context, baseDir string, worktreeDir string) (bool, bool, error) {
	output, err := runGitCommand(ctx, baseDir, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return false, false, err
	}

	var currentWorktree string
	var listed bool
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			currentWorktree = ""
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			if sameRepositoryWorkerPath(currentWorktree, worktreeDir) {
				listed = true
				if _, err := os.Stat(worktreeDir); err == nil {
					return true, false, nil
				} else if os.IsNotExist(err) {
					return false, true, nil
				} else {
					return false, false, err
				}
			}
		}
	}
	return false, listed, nil
}

func sameRepositoryWorkerPath(left string, right string) bool {
	return normalizeRepositoryWorkerPath(left) == normalizeRepositoryWorkerPath(right)
}

func normalizeRepositoryWorkerPath(value string) string {
	return filepath.Clean(filepath.FromSlash(strings.TrimSpace(value)))
}

func repositoryWorkerWorktreeExists(ctx context.Context, baseDir string, worktreeDir string) (bool, error) {
	exists, _, err := repositoryWorkerWorktreeStatus(ctx, baseDir, worktreeDir)
	return exists, err
}

func cleanupRepositoryWorkerWorktreeRegistration(ctx context.Context, baseDir string, worktreeDir string) error {
	_, _ = runGitCommand(ctx, baseDir, "git", "worktree", "remove", "--force", worktreeDir)
	_, _ = runGitCommand(ctx, baseDir, "git", "worktree", "prune", "--expire", "now")
	if err := os.RemoveAll(worktreeDir); err != nil {
		return err
	}
	return nil
}

func createRepositoryWorkerWorktree(ctx context.Context, baseDir string, worktreeDir string, branch string) error {
	startPoint := branch
	if !gitRemoteBranchExists(ctx, baseDir, branch) {
		baseBranch, err := resolveRepositoryBaseBranch(ctx, baseDir, strings.TrimSpace(branch))
		if err != nil {
			return err
		}
		if !gitRemoteBranchExists(ctx, baseDir, baseBranch) {
			for _, fallback := range []string{"main", "master", "develop"} {
				if gitRemoteBranchExists(ctx, baseDir, fallback) {
					baseBranch = fallback
					break
				}
			}
		}
		if !gitRemoteBranchExists(ctx, baseDir, baseBranch) {
			return fmt.Errorf("repository worktree base branch not found")
		}
		startPoint = baseBranch
	}

	args := []string{"git", "worktree", "add", "-B", branch, worktreeDir, "origin/" + startPoint}
	if startPoint == branch {
		args[len(args)-1] = "origin/" + branch
	}
	if _, err := runGitCommand(ctx, baseDir, args...); err != nil {
		return err
	}
	return nil
}

func isRepositoryWorkerWorktreeRegistrationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already used by worktree") || strings.Contains(message, "is already checked out at")
}

func removeRepositoryWorkerWorkspace(targetDir string, workspaceDir string) error {
	trimmed := strings.TrimSpace(workspaceDir)
	if trimmed == "" {
		trimmed = ".workspace"
	}
	if filepath.IsAbs(trimmed) {
		return nil
	}
	if trimmed == "." {
		trimmed = ".workspace"
	}
	return os.RemoveAll(filepath.Join(targetDir, trimmed))
}

func syncRepositoryWorkspace(ctx context.Context, cfg *config.Service, job domain.Job, repoDir string, logger *log.Logger) error {
	return withRepositoryWorkspaceLock(job.Repository, func() error {
		if domain.IsDummyRepository(job.Repository) {
			if logger != nil {
				logger.Printf("syncing repository source checkout skipped job_id=%s repo_dir=%s reason=dummy_repository", job.ID, repoDir)
			}
			return nil
		}
		if job.Type == domain.JobTypePRFeedback {
			branchName := strings.TrimSpace(job.BranchName)
			if branchName == "" {
				return fmt.Errorf("resolve pull request branch: branch is empty")
			}
			if logger != nil {
				logger.Printf("syncing repository source checkout job_id=%s repo_dir=%s pull_request_branch=%s", job.ID, repoDir, branchName)
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

		branchName := strings.TrimSpace(job.BranchName)
		if branchName == "" {
			branchName = resolveMonitoredRepositoryBranch(cfg, job.Repository)
		}
		if branchName == "" {
			branchName = "main"
		}

		baseBranch := strings.TrimSpace(resolveMonitoredRepositoryBranch(cfg, job.Repository))
		var err error
		if baseBranch == "" {
			baseBranch, err = resolveRepositoryBaseBranch(ctx, repoDir, "")
			if err != nil {
				return err
			}
		}
		if baseBranch == "" {
			baseBranch = "main"
		}

		if logger != nil {
			logger.Printf("syncing repository source checkout job_id=%s repo_dir=%s branch=%s base_branch=%s", job.ID, repoDir, branchName, baseBranch)
		}

		if _, err := runGitCommand(ctx, repoDir, "git", "fetch", "--prune", "origin"); err != nil {
			return err
		}

		syncBranch := branchName
		if job.Type == domain.JobTypeIssue && !gitRemoteBranchExists(ctx, repoDir, branchName) {
			syncBranch = baseBranch
		}

		commands := [][]string{
			{"git", "checkout", "-f", "-B", branchName, "origin/" + syncBranch},
			{"git", "reset", "--hard", "origin/" + syncBranch},
			{"git", "clean", "-fd"},
		}
		for _, command := range commands {
			if _, err := runGitCommand(ctx, repoDir, command...); err != nil {
				return err
			}
		}
		return nil
	})
}

func syncRepositoryImprovementWorkspace(ctx context.Context, cfg *config.Service, repository config.MonitoredRepository, improvementDir string, logger *log.Logger) error {
	return withRepositoryWorkspaceLock(repository.Repository, func() error {
		if !repository.ImprovementEnabled {
			return nil
		}
		if domain.IsDummyRepository(repository.Repository) {
			if logger != nil {
				logger.Printf("syncing repository improvement checkout skipped repository=%s work_dir=%s reason=dummy_repository", repository.Repository, improvementDir)
			}
			return nil
		}

		if cfg == nil {
			return fmt.Errorf("repository improvement workspace sync requires config")
		}
		if _, err := os.Stat(filepath.Join(improvementDir, ".git")); errors.Is(err, os.ErrNotExist) {
			preparedDir, err := prepareRepositoryImprovementWorkspaceLocked(ctx, cfg, repository)
			if err != nil {
				return err
			}
			improvementDir = preparedDir
		}

		improvementBranch := config.ResolveImprovementBranch(repository)
		improvementWorkDir := artifacts.RepositoryWorkerImprovementWorkDir(improvementDir, repository.ImprovementDir)
		if err := os.MkdirAll(improvementWorkDir, 0o755); err != nil {
			return err
		}

		if logger != nil {
			logger.Printf("syncing repository improvement checkout repository=%s work_dir=%s branch=%s", repository.Repository, improvementDir, improvementBranch)
		}

		if _, err := runGitCommand(ctx, improvementDir, "git", "fetch", "--prune", "origin"); err != nil {
			return err
		}

		if gitRemoteBranchExists(ctx, improvementDir, improvementBranch) {
			commands := [][]string{
				{"git", "checkout", "-f", "-B", improvementBranch, "origin/" + improvementBranch},
				{"git", "reset", "--hard", "origin/" + improvementBranch},
			}
			for _, command := range commands {
				if _, err := runGitCommand(ctx, improvementDir, command...); err != nil {
					return err
				}
			}
		} else {
			baseBranch, err := resolveRepositoryBaseBranch(ctx, improvementDir, strings.TrimSpace(repository.Branch))
			if err != nil {
				return err
			}
			if !gitRemoteBranchExists(ctx, improvementDir, baseBranch) {
				for _, fallback := range []string{"main", "master", "develop"} {
					if gitRemoteBranchExists(ctx, improvementDir, fallback) {
						baseBranch = fallback
						break
					}
				}
			}
			if !gitRemoteBranchExists(ctx, improvementDir, baseBranch) {
				return fmt.Errorf("repository improvement workspace base branch not found")
			}
			if _, err := runGitCommand(ctx, improvementDir, "git", "checkout", "-f", "-B", improvementBranch, "origin/"+baseBranch); err != nil {
				return err
			}
		}

		cleanArgs := []string{"git", "clean", "-fd"}
		if relativeWorkDir, ok := repositoryImprovementWorkDirPattern(improvementDir, improvementWorkDir); ok {
			cleanArgs = append(cleanArgs, "-e", relativeWorkDir)
		}
		if _, err := runGitCommand(ctx, improvementDir, cleanArgs...); err != nil {
			return err
		}
		return nil
	})
}

func ensureRepositoryImprovementWorktree(ctx context.Context, repoDir string, worktreeDir string, repository config.MonitoredRepository, branch string) error {
	if info, err := os.Stat(filepath.Join(worktreeDir, ".git")); err == nil {
		if !info.IsDir() {
			return nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := ensureRepositoryWorkerWorktree(ctx, repoDir, worktreeDir, branch, repositoryCloneSource(repository.Repository), ".improvement"); err != nil {
		return err
	}
	return nil
}

func repositoryImprovementWorkDirPattern(workDir string, improvementWorkDir string) (string, bool) {
	relative, err := filepath.Rel(workDir, improvementWorkDir)
	if err != nil {
		return "", false
	}
	relative = filepath.ToSlash(strings.TrimSpace(relative))
	if relative == "" || relative == "." || strings.HasPrefix(relative, "../") {
		return "", false
	}
	if !strings.HasSuffix(relative, "/") {
		relative += "/"
	}
	return relative, true
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

func newRepositoryWorkerLogger(cfg *config.Service, fallback *log.Logger, repository string, workerIndex int, startedAt time.Time) (*log.Logger, func(), error) {
	logPath := artifacts.RepositoryWorkerLogPath(cfg.Root(), cfg.App().ArtifactsDir, repository, workerIndex, startedAt)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, func() {}, err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, func() {}, err
	}
	writer := &repositoryWorkerLogWriter{
		file:     file,
		fallback: fallback,
	}
	logger := log.New(writer, "", log.LstdFlags)
	return logger, func() { _ = file.Close() }, nil
}

type repositoryWorkerLogWriter struct {
	file     *os.File
	fallback *log.Logger
}

func (w *repositoryWorkerLogWriter) Write(p []byte) (int, error) {
	if w == nil || w.file == nil {
		return len(p), nil
	}
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}
	if w.fallback != nil && repositoryWorkerLogLineIsError(p) {
		w.fallback.Print(strings.TrimSpace(string(p)))
	}
	return n, nil
}

func repositoryWorkerLogLineIsError(p []byte) bool {
	line := strings.TrimSpace(string(p))
	if line == "" {
		return false
	}
	return strings.Contains(line, " error=") || strings.Contains(line, "error=") || strings.Contains(line, " failed") || strings.Contains(line, "failed:")
}

func repositoryWorkerSourceDir(cfg *config.Service, repository string, workerIndex int) string {
	branch := resolveMonitoredRepositoryBranch(cfg, repository)
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	return artifacts.RepositoryWorkerBranchWorkDir(cfg.Root(), cfg.App().ArtifactsDir, repository, branch)
}

func initializeRepositoryWorkerGitDir(ctx context.Context, workerDir string) error {
	if _, err := runGitCommand(ctx, workerDir, "git", "init", "--initial-branch=main"); err != nil {
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

func repositoryCloneSource(repository string) string {
	trimmed := strings.TrimSpace(repository)
	if trimmed == "" {
		return trimmed
	}
	if strings.Contains(trimmed, "://") || strings.HasPrefix(trimmed, "git@") || filepath.IsAbs(trimmed) {
		return trimmed
	}
	if _, err := os.Stat(trimmed); err == nil {
		if abs, err := filepath.Abs(trimmed); err == nil {
			return abs
		}
		return trimmed
	}
	if toolRoot := strings.TrimSpace(os.Getenv("KOROBOKCLE_TOOL_ROOT")); toolRoot != "" {
		if candidate := artifacts.RepositoryWorkerWorkDir(toolRoot, "artifacts", trimmed, ""); candidate != "" {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				if abs, err := filepath.Abs(candidate); err == nil {
					return abs
				}
				return candidate
			}
		}
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

func withRepositoryWorkspaceLock(repository string, fn func() error) error {
	key := canonicalRepositoryID(repository)
	if key == "" {
		key = strings.TrimSpace(repository)
	}
	if key == "" {
		return fn()
	}

	lockValue, _ := repositoryWorkspaceLocks.LoadOrStore(key, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}
