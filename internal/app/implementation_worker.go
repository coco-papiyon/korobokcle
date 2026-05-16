package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/executor"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func startImplementationWorker(ctx context.Context, root string, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	runner := skill.NewRunner(root, "")
	testRunner := executor.NewTestRunner()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			if err := runPendingImplementations(ctx, root, cfg, orch, runner, testRunner, logger); err != nil && ctx.Err() == nil {
				logger.Printf("implementation worker error: %v", err)
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

func runPendingImplementations(ctx context.Context, root string, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, testRunner *executor.TestRunner, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if job.Type != domain.JobTypeIssue || job.State != domain.StateImplementationRunning {
			continue
		}

		jobDetail, events, err := orch.JobDetail(ctx, job.ID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		contextData, err := buildImplementationContext(cfg, jobDetail, events)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := runImplementationAttempts(ctx, root, cfg, orch, runner, testRunner, job, contextData, execution, logger); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateImplementationReady, "implementation_ready", map[string]any{
			"artifactDir": contextData.ArtifactDir,
		}); err != nil {
			logger.Printf("implementation_ready state transition failed for %s: %v", job.ID, err)
			continue
		}
		if err := orch.UpdateJobState(ctx, job.ID, domain.StateWaitingFinalApproval, "waiting_final_approval", map[string]any{
			"artifactDir": contextData.ArtifactDir,
		}); err != nil {
			logger.Printf("waiting_final_approval state transition failed for %s: %v", job.ID, err)
			continue
		}
	}

	return nil
}

func runImplementationAttempts(ctx context.Context, root string, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, testRunner *executor.TestRunner, job domain.Job, baseContext skill.ImplementationContext, execution skill.ExecutionConfig, logger *log.Logger) error {
	const maxAttempts = 3

	var previousFailure string
	var previousTestReport string
	var currentDiff string

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptContext := baseContext
		attemptContext.Attempt = attempt
		attemptContext.PreviousFailure = previousFailure
		attemptContext.PreviousTestReport = previousTestReport
		attemptContext.CurrentDiff = currentDiff

		result, err := runner.RunImplementation(ctx, "implement", attemptContext, execution)
		if err != nil {
			previousFailure = fmt.Sprintf("implementation attempt %d failed: %v", attempt, err)
			currentDiff, _ = captureWorkingDiff(ctx, root)
			if err := writeImplementationAttemptArtifact(baseContext.ArtifactDir, attempt, result, previousFailure, previousTestReport, currentDiff); err != nil {
				logger.Printf("implementation attempt artifact write failed for %s: %v", job.ID, err)
			}
			continue
		}

		if err := applyImplementationPatch(ctx, root, baseContext.ArtifactDir, result.Output); err != nil {
			previousFailure = fmt.Sprintf("implementation attempt %d patch application failed: %v", attempt, err)
			currentDiff, _ = captureWorkingDiff(ctx, root)
			if err := writeImplementationAttemptArtifact(baseContext.ArtifactDir, attempt, result, previousFailure, previousTestReport, currentDiff); err != nil {
				logger.Printf("implementation attempt artifact write failed for %s: %v", job.ID, err)
			}
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateTestRunning, "test_started", map[string]any{
			"artifactDir": baseContext.ArtifactDir,
			"attempt":     attempt,
		}); err != nil {
			logger.Printf("test_started state transition failed for %s: %v", job.ID, err)
			return err
		}

		report, err := runTestsForJob(ctx, cfg, testRunner, job, baseContext.ArtifactDir)
		if err != nil {
			previousFailure = fmt.Sprintf("implementation attempt %d test runner failed: %v", attempt, err)
			previousTestReport = ""
			currentDiff, _ = captureWorkingDiff(ctx, root)
			if err := writeImplementationAttemptArtifact(baseContext.ArtifactDir, attempt, result, previousFailure, previousTestReport, currentDiff); err != nil {
				logger.Printf("implementation attempt artifact write failed for %s: %v", job.ID, err)
			}
			continue
		}
		if !report.Success {
			previousFailure = fmt.Sprintf("implementation attempt %d tests failed", attempt)
			previousTestReport = stringifyTestReport(report)
			currentDiff, _ = captureWorkingDiff(ctx, root)
			if err := writeImplementationAttemptArtifact(baseContext.ArtifactDir, attempt, result, previousFailure, previousTestReport, currentDiff); err != nil {
				logger.Printf("implementation attempt artifact write failed for %s: %v", job.ID, err)
			}
			continue
		}

		return nil
	}

	if strings.TrimSpace(previousFailure) == "" {
		return fmt.Errorf("implementation attempts exhausted")
	}
	return fmt.Errorf(previousFailure)
}

func buildImplementationContext(cfg *config.Service, job domain.Job, events []domain.Event) (skill.ImplementationContext, error) {
	designArtifactDir := filepath.Join(cfg.Root(), cfg.App().ArtifactsDir, "designs", job.ID)
	designArtifactPath := filepath.Join(designArtifactDir, "design.md")
	designArtifactRaw, err := os.ReadFile(designArtifactPath)
	if err != nil {
		return skill.ImplementationContext{}, err
	}

	ctxData := skill.ImplementationContext{
		JobID:             job.ID,
		Repository:        job.Repository,
		IssueNumber:       job.GitHubNumber,
		Title:             job.Title,
		WatchRuleID:       job.WatchRuleID,
		BranchName:        job.BranchName,
		DesignArtifact:    string(designArtifactRaw),
		DesignArtifactDir: designArtifactDir,
		ArtifactDir:       filepath.Join(cfg.Root(), cfg.App().ArtifactsDir, "changes", job.ID),
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
			return skill.ImplementationContext{}, err
		}
		ctxData.Body = payload.Body
		ctxData.Author = payload.Author
		ctxData.Labels = payload.Labels
		ctxData.Assignees = payload.Assignees
		break
	}

	return ctxData, nil
}

func writeImplementationAttemptArtifact(artifactDir string, attempt int, result skill.AIResult, failure string, testReport string, currentDiff string) error {
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}

	payload := map[string]any{
		"attempt":            attempt,
		"stdout":             result.Stdout,
		"stderr":             result.Stderr,
		"failure":            failure,
		"previousTestReport": testReport,
		"currentDiff":        currentDiff,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(artifactDir, fmt.Sprintf("implementation-attempt-%d.json", attempt)), raw, 0o644)
}

func stringifyTestReport(report executor.TestReport) string {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return ""
	}
	return string(raw)
}

func runTestsForJob(ctx context.Context, cfg *config.Service, testRunner *executor.TestRunner, job domain.Job, artifactDir string) (executor.TestReport, error) {
	rule, ok := cfg.WatchRuleByID(job.WatchRuleID)
	if !ok {
		return executor.TestReport{}, os.ErrNotExist
	}

	var profile config.TestProfile
	found := false
	for _, candidate := range cfg.TestProfiles().Profiles {
		if candidate.Name == rule.TestProfile {
			profile = candidate
			found = true
			break
		}
	}
	if !found {
		return executor.TestReport{}, os.ErrNotExist
	}

	report := testRunner.Run(ctx, executor.TestProfile{
		Name:     profile.Name,
		Commands: profile.Commands,
	}, cfg.Root())

	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return executor.TestReport{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "test-report.json"), raw, 0o644); err != nil {
		return executor.TestReport{}, err
	}
	return report, nil
}
