package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/executor"
	"github.com/coco-papiyon/korobokcle/internal/issuebody"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

const (
	implementationSkillName = "implement"
	implementFixSkillName   = "implement_fix"
	reviewFixSkillName      = "review_fix"
)

type implementationRunSpec struct {
	SkillName   string
	ArtifactDir string
}

func startImplementationWorker(ctx context.Context, repoRoot string, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	runner := skill.NewRunner(repoRoot, cfg.Root(), "", cfg.App().CopilotAllowTools)
	testRunner := executor.NewTestRunner()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			if err := runPendingImplementations(ctx, repoRoot, cfg, orch, runner, testRunner, logger); err != nil && ctx.Err() == nil {
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

func runPendingImplementations(ctx context.Context, repoRoot string, cfg *config.Service, orch *orchestrator.Orchestrator, runner *skill.Runner, testRunner *executor.TestRunner, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if (job.Type != domain.JobTypeIssue && job.Type != domain.JobTypePRFeedback) || job.State != domain.StateImplementationRunning {
			continue
		}

		jobDetail, events, err := orch.JobDetail(ctx, job.ID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		runSpec, err := resolveImplementationRunSpec(cfg, jobDetail, events)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		contextData, err := buildImplementationContext(cfg, repoRoot, jobDetail, events, runSpec)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		execution, err := resolveExecutionConfig(cfg, jobDetail.WatchRuleID)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		if _, err := runner.RunImplementation(ctx, runSpec.SkillName, contextData, execution); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}
		if err := copyAIResultToWorkDir(repoRoot, filepath.Base(runSpec.ArtifactDir), jobDetail, contextData.ArtifactDir); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "implementation_failed", map[string]any{"error": err.Error()})
			continue
		}

		shouldRunTests, err := jobHasRunnableTestProfile(cfg, job)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{"error": err.Error()})
			continue
		}
		if shouldRunTests {
			if err := orch.UpdateJobState(ctx, job.ID, domain.StateTestRunning, "test_started", map[string]any{
				"artifactDir": contextData.ArtifactDir,
			}); err != nil {
				logger.Printf("test_started state transition failed for %s: %v", job.ID, err)
				continue
			}

			report, err := runTestsForJob(ctx, cfg, testRunner, job, contextData.ArtifactDir, repoRoot)
			if err != nil {
				_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{"error": err.Error()})
				continue
			}
			if !report.Success {
				_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "test_failed", map[string]any{
					"reportPath": filepath.Join(contextData.ArtifactDir, "test-report.json"),
				})
				continue
			}
		} else {
			logger.Printf("tests skipped for %s: empty test profile", job.ID)
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

func buildImplementationContext(cfg *config.Service, workDir string, job domain.Job, events []domain.Event, runSpec implementationRunSpec) (skill.ImplementationContext, error) {
	if job.Type == domain.JobTypePRFeedback {
		return buildPRFeedbackImplementationContext(cfg, workDir, job, events, runSpec)
	}

	designArtifactDir := artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	designArtifactRaw, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerDesign, job, designArtifactDir, "result.md", "design.md")
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
		ArtifactDir:       runSpec.ArtifactDir,
	}

	ctxData.DesignApprovalComment, err = loadDesignApprovalComment(events)
	if err != nil {
		return skill.ImplementationContext{}, err
	}

	rerunComment, previousFailure, previousTestReport, err := loadImplementationRetryContext(cfg, job, events)
	if err != nil {
		return skill.ImplementationContext{}, err
	}
	ctxData.RerunComment = rerunComment
	if strings.TrimSpace(ctxData.RerunComment) != "" {
		implementationArtifact, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerImplementation, job, artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation), "result.md", "implement.md", "summary.md", "stdout.log")
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return skill.ImplementationContext{}, err
		}
		if err == nil {
			ctxData.ImplementationArtifact = string(implementationArtifact)
		}
		ctxData.PreviousFailure = previousFailure
		ctxData.PreviousTestReport = previousTestReport
	}

	snapshot, err := issuebody.Resolve(events)
	if err != nil {
		return skill.ImplementationContext{}, err
	}
	ctxData.Body = snapshot.Body
	ctxData.Author = snapshot.Author
	ctxData.Labels = snapshot.Labels
	ctxData.Assignees = snapshot.Assignees

	return ctxData, nil
}

func loadDesignApprovalComment(events []domain.Event) (string, error) {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.EventType != "design_approved" {
			continue
		}

		var payload struct {
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return "", err
		}
		return strings.TrimSpace(payload.Comment), nil
	}

	return "", nil
}

func readFirstArtifactFile(dir string, names ...string) ([]byte, error) {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		paths = append(paths, path)
		raw, err := os.ReadFile(path)
		if err == nil {
			return raw, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: searched %s", os.ErrNotExist, strings.Join(paths, ", "))
}

func resolveImplementationRunSpec(cfg *config.Service, job domain.Job, events []domain.Event) (implementationRunSpec, error) {
	isFix := false
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)

	sourceEventType, err := latestImplementationRerunSourceEventType(events)
	if err != nil {
		return implementationRunSpec{}, err
	}
	if sourceEventType == "test_failed" {
		isFix = true
		artifactDir = artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	}

	skillName, err := resolveImplementationSkillName(cfg, job, isFix)
	if err != nil {
		return implementationRunSpec{}, err
	}

	return implementationRunSpec{
		SkillName:   skillName,
		ArtifactDir: artifactDir,
	}, nil
}

func resolveImplementationSkillName(cfg *config.Service, job domain.Job, isFix bool) (string, error) {
	rule, ok := cfg.WatchRuleByID(job.WatchRuleID)
	if !ok {
		return "", os.ErrNotExist
	}

	if job.Type == domain.JobTypePRFeedback {
		skillSet := strings.TrimSpace(rule.SkillSet)
		if skillSet == "" || skillSet == "default" {
			return reviewFixSkillName, nil
		}
		return filepath.ToSlash(filepath.Join(skillSet, reviewFixSkillName)), nil
	}

	baseName := implementationSkillName
	if isFix {
		baseName = implementFixSkillName
	}

	skillSet := strings.TrimSpace(rule.SkillSet)
	if skillSet == "" || skillSet == "default" {
		return baseName, nil
	}
	return filepath.ToSlash(filepath.Join(skillSet, baseName)), nil
}

func buildPRFeedbackImplementationContext(cfg *config.Service, workDir string, job domain.Job, events []domain.Event, runSpec implementationRunSpec) (skill.ImplementationContext, error) {
	ctxData := skill.ImplementationContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: runSpec.ArtifactDir,
	}

	implementationArtifact, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerImplementation, job, artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation), "result.md", "review_fix.md", "implement.md", "summary.md", "stdout.log")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return skill.ImplementationContext{}, err
	}
	if err == nil {
		ctxData.ImplementationArtifact = string(implementationArtifact)
	}

	rerunComment, previousFailure, previousTestReport, err := loadImplementationRetryContext(cfg, job, events)
	if err != nil {
		return skill.ImplementationContext{}, err
	}
	ctxData.RerunComment = rerunComment
	ctxData.PreviousFailure = previousFailure
	ctxData.PreviousTestReport = previousTestReport

	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != string(domain.DomainEventPRReviewMatched) {
			continue
		}

		var payload struct {
			Body           string                 `json:"body"`
			Author         string                 `json:"author"`
			Labels         []string               `json:"labels"`
			Assignees      []string               `json:"assignees"`
			URL            string                 `json:"url"`
			ReviewComments []domain.ReviewComment `json:"reviewComments"`
		}
		if err := json.Unmarshal([]byte(events[i].Payload), &payload); err != nil {
			return skill.ImplementationContext{}, err
		}
		ctxData.Body = payload.Body
		ctxData.Author = payload.Author
		ctxData.Labels = payload.Labels
		ctxData.Assignees = payload.Assignees
		ctxData.SourceURL = payload.URL
		ctxData.ReviewComments = make([]skill.ReviewComment, 0, len(payload.ReviewComments))
		for _, comment := range payload.ReviewComments {
			ctxData.ReviewComments = append(ctxData.ReviewComments, skill.ReviewComment{
				Author: comment.Author,
				Body:   comment.Body,
				Path:   comment.Path,
				Line:   comment.Line,
				URL:    comment.URL,
			})
		}
		break
	}

	return ctxData, nil
}

func latestImplementationRerunSourceEventType(events []domain.Event) (string, error) {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.EventType != "implementation_rerun_requested" {
			continue
		}

		var payload struct {
			EventID *int64 `json:"eventId"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return "", err
		}
		if payload.EventID == nil {
			return "", nil
		}
		for j := i - 1; j >= 0; j-- {
			if events[j].ID == *payload.EventID {
				return events[j].EventType, nil
			}
		}
		return "", nil
	}
	return "", nil
}

func loadImplementationRetryContext(cfg *config.Service, job domain.Job, events []domain.Event) (string, string, string, error) {
	var rerunComment string
	var previousFailure string
	var previousTestReport string

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if rerunComment == "" && event.EventType == "implementation_rerun_requested" {
			var payload struct {
				Comment string `json:"comment"`
			}
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				return "", "", "", err
			}
			rerunComment = strings.TrimSpace(payload.Comment)
			continue
		}

		switch event.EventType {
		case "test_failed", "implementation_failed":
			var payload struct {
				Error      string `json:"error"`
				ReportPath string `json:"reportPath"`
			}
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				return "", "", "", err
			}
			previousFailure = strings.TrimSpace(payload.Error)
			if previousFailure == "" {
				previousFailure = event.EventType
			}
			if strings.TrimSpace(payload.ReportPath) != "" {
				if raw, err := os.ReadFile(payload.ReportPath); err == nil {
					previousTestReport = string(raw)
				}
			}
			break
		}
	}

	if previousTestReport == "" {
		reportPaths := []string{
			filepath.Join(resolveImplementationRetryArtifactDir(cfg, job, events), "test-report.json"),
		}
		fallbackPath := filepath.Join(artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation), "test-report.json")
		if fallbackPath != reportPaths[0] {
			reportPaths = append(reportPaths, fallbackPath)
		}
		for _, reportPath := range reportPaths {
			if raw, err := os.ReadFile(reportPath); err == nil {
				previousTestReport = string(raw)
				break
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", "", fmt.Errorf("read previous test report: %w", err)
			}
		}
	}

	return rerunComment, previousFailure, previousTestReport, nil
}

func resolveImplementationRetryArtifactDir(cfg *config.Service, job domain.Job, events []domain.Event) string {
	sourceEventType, err := latestImplementationRerunSourceEventType(events)
	if err == nil && sourceEventType == "test_failed" {
		return artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	}
	return artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
}

func runTestsForJob(ctx context.Context, cfg *config.Service, testRunner *executor.TestRunner, job domain.Job, artifactDir string, repoRoot string) (executor.TestReport, error) {
	profile, shouldRun, err := resolveJobTestProfile(cfg, job)
	if err != nil {
		return executor.TestReport{}, err
	}
	if !shouldRun {
		return executor.TestReport{}, nil
	}

	report := testRunner.Run(ctx, profile, repoRoot)

	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return executor.TestReport{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "test-report.json"), raw, 0o644); err != nil {
		return executor.TestReport{}, err
	}
	return report, nil
}

func jobHasRunnableTestProfile(cfg *config.Service, job domain.Job) (bool, error) {
	_, shouldRun, err := resolveJobTestProfile(cfg, job)
	return shouldRun, err
}

func resolveJobTestProfile(cfg *config.Service, job domain.Job) (executor.TestProfile, bool, error) {
	rule, ok := cfg.WatchRuleByID(job.WatchRuleID)
	if !ok {
		return executor.TestProfile{}, false, os.ErrNotExist
	}

	profileName := strings.TrimSpace(rule.TestProfile)
	if profileName == "" {
		return executor.TestProfile{}, false, nil
	}

	for _, candidate := range cfg.TestProfiles().Profiles {
		if candidate.Name != profileName {
			continue
		}
		return executor.TestProfile{
			Name:     candidate.Name,
			Commands: append([]string(nil), candidate.Commands...),
		}, true, nil
	}

	return executor.TestProfile{}, false, os.ErrNotExist
}
