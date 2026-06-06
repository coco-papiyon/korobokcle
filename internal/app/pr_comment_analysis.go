package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func processPRCommentAnalysis(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, jobID string, selectedComment PRComment, logger *log.Logger) error {
	return processPRCommentAnalysisWithDeps(ctx, cfg, orch, jobID, selectedComment, logger, func(workDir string) *skill.Runner {
		return skill.NewRunner(workDir, cfg.Root(), "", cfg.App().CopilotAllowTools).WithLogger(logger)
	})
}

func processPRCommentAnalysisWithDeps(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, jobID string, selectedComment PRComment, logger *log.Logger, runnerFactory func(workDir string) *skill.Runner) error {
	job, events, err := orch.JobDetail(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Type != domain.JobTypeIssue {
		return fmt.Errorf("job %q is not an issue job", jobID)
	}
	if strings.TrimSpace(selectedComment.Body) == "" {
		return fmt.Errorf("selected comment body is empty")
	}

	pullNumber, err := resolveRepositoryWorkerPullNumber(cfg, job, events)
	if err != nil {
		return err
	}
	if pullNumber < 1 {
		return fmt.Errorf("pull number is not available for job %q", jobID)
	}

	artifactDir := repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	configuredWorkDir := resolveRepositoryConfiguredWorkDirSetting(cfg, job.Repository)
	workDir := artifacts.RepositoryWorkerWorkDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, configuredWorkDir)
	if runnerFactory == nil {
		return fmt.Errorf("pr comment runner is not configured")
	}
	runner := runnerFactory(workDir)
	execution, err := resolveExecutionConfig(cfg, job.WatchRuleID)
	if err != nil {
		return err
	}

	contextData, err := buildPRCommentAnalysisContext(cfg, workDir, job, events, selectedComment)
	if err != nil {
		return err
	}

	if logger != nil {
		logger.Printf("pr comment analysis started job_id=%s pull_number=%d author=%s", jobID, pullNumber, selectedComment.Author)
	}
	if _, err := runner.RunImplementation(ctx, reviewFixSkillName, contextData, execution); err != nil {
		return err
	}
	if err := copyAIResultToWorkDir(workDir, artifacts.WorkerReview, job, artifactDir); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"artifactDir": artifactDir,
		"pullNumber":  pullNumber,
		"comment":     selectedComment,
	})
	if err != nil {
		return err
	}
	if err := orch.UpdateJobState(ctx, job.ID, domain.StateWaitingDesignApproval, "pr_comment_analysis_ready", map[string]any{
		"artifactDir": artifactDir,
		"pullNumber":  pullNumber,
		"comment":     selectedComment,
	}); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("pr comment analysis completed job_id=%s pull_number=%d comment_count=1", jobID, pullNumber)
	}
	_ = payload
	return nil
}

func buildPRCommentAnalysisContext(cfg *config.Service, workDir string, job domain.Job, events []domain.Event, selectedComment PRComment) (skill.ImplementationContext, error) {
	ctxData := skill.ImplementationContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerPR),
	}

	for _, event := range events {
		switch event.EventType {
		case string(domain.DomainEventIssueMatched):
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
		case "pr_created", "pr_updated":
			var payload struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal([]byte(event.Payload), &payload); err == nil && strings.TrimSpace(payload.URL) != "" {
				ctxData.SourceURL = payload.URL
			}
		}
	}

	if raw, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerImplementation, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation), "result.md", "review_fix.md", "implement.md", "summary.md", "stdout.log"); err == nil {
		ctxData.ImplementationArtifact = string(raw)
	} else if !os.IsNotExist(err) {
		return skill.ImplementationContext{}, err
	}

	rerunComment, previousFailure, previousTestReport, err := loadRepositoryImplementationRetryContext(cfg, workDir, job, events)
	if err != nil {
		return skill.ImplementationContext{}, err
	}
	ctxData.RerunComment = rerunComment
	ctxData.PreviousFailure = previousFailure
	ctxData.PreviousTestReport = previousTestReport
	ctxData.ReviewComments = []skill.ReviewComment{{
		Author: selectedComment.Author,
		Body:   selectedComment.Body,
		URL:    selectedComment.URL,
	}}

	return ctxData, nil
}

func resolveRepositoryWorkerPullNumber(cfg *config.Service, job domain.Job, events []domain.Event) (int, error) {
	if job.Type == domain.JobTypePRFeedback && job.GitHubNumber > 0 {
		return job.GitHubNumber, nil
	}
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].EventType {
		case "pr_created", "pr_updated":
			var payload struct {
				PullNumber int `json:"pullNumber"`
			}
			if err := json.Unmarshal([]byte(events[i].Payload), &payload); err == nil && payload.PullNumber > 0 {
				return payload.PullNumber, nil
			}
		}
	}
	if artifact, err := readRepositoryWorkerArtifactFile(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerPR, "result.json"); err == nil {
		var payload struct {
			PullNumber int `json:"pullNumber"`
		}
		if err := json.Unmarshal(artifact, &payload); err == nil && payload.PullNumber > 0 {
			return payload.PullNumber, nil
		}
	}
	return 0, os.ErrNotExist
}
