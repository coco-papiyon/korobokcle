package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/issuebody"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func unmarshalEventPayload(payload string, out any) error {
	return json.Unmarshal([]byte(payload), out)
}

func repositoryWorkerArtifactDir(cfg *config.Service, repository string, issueNumber int, phase string) string {
	return artifacts.RepositoryWorkerJobPhaseDir(cfg.Root(), cfg.App().ArtifactsDir, repository, issueNumber, phase)
}

func readRepositoryWorkerArtifactFile(cfg *config.Service, repository string, issueNumber int, phase string, names ...string) ([]byte, error) {
	dir := repositoryWorkerArtifactDir(cfg, repository, issueNumber, phase)
	return readFirstArtifactFile(dir, names...)
}

func buildRepositoryDesignContext(cfg *config.Service, workDir string, improvementWorkDir string, job domain.Job, events []domain.Event) (skill.DesignContext, error) {
	ctxData := skill.DesignContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerDesign),
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

	if existingDesign, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerDesign, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerDesign), "result.md", "design.md"); err == nil {
		ctxData.ExistingDesign = string(existingDesign)
	} else if !errors.Is(err, os.ErrNotExist) {
		return skill.DesignContext{}, err
	}

	snapshot, err := issuebody.Resolve(events)
	if err != nil {
		return skill.DesignContext{}, err
	}
	ctxData.Body = snapshot.Body
	ctxData.Author = snapshot.Author
	ctxData.Labels = snapshot.Labels
	ctxData.Assignees = snapshot.Assignees

	if instructions, err := loadRepositoryImprovementInstructions(cfg, improvementWorkDir, job.Repository, "design"); err != nil {
		return skill.DesignContext{}, err
	} else {
		ctxData.ManagedInstructions = instructions
	}

	return ctxData, nil
}

func resolveRepositoryImplementationRunSpec(cfg *config.Service, workDir string, job domain.Job, events []domain.Event) (implementationRunSpec, error) {
	isFix := false
	artifactPhase := artifacts.WorkerImplementation

	sourceEventType, err := latestImplementationRerunSourceEventType(events)
	if err != nil {
		return implementationRunSpec{}, err
	}
	if sourceEventType == "test_failed" {
		isFix = true
		artifactPhase = artifacts.WorkerFix
	}

	skillName, err := resolveImplementationSkillName(cfg, job, isFix)
	if err != nil {
		return implementationRunSpec{}, err
	}

	return implementationRunSpec{
		SkillName:   skillName,
		ArtifactDir: repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifactPhase),
	}, nil
}

func buildRepositoryImplementationContext(cfg *config.Service, workDir string, improvementWorkDir string, job domain.Job, events []domain.Event, runSpec implementationRunSpec) (skill.ImplementationContext, error) {
	if job.Type == domain.JobTypePRFeedback {
		return buildRepositoryPRFeedbackImplementationContext(cfg, workDir, improvementWorkDir, job, events, runSpec)
	}

	designArtifactDir := repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	designArtifactRaw, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerDesign, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerDesign), "result.md", "design.md")
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

	rerunComment, previousFailure, previousTestReport, err := loadRepositoryImplementationRetryContext(cfg, workDir, job, events)
	if err != nil {
		return skill.ImplementationContext{}, err
	}
	ctxData.RerunComment = rerunComment
	if strings.TrimSpace(ctxData.RerunComment) != "" {
		implementationArtifact, err := readRepositoryWorkerArtifactFile(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation, "result.md", "implement.md", "summary.md", "stdout.log")
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

	if instructions, err := loadRepositoryImprovementInstructions(cfg, improvementWorkDir, job.Repository, runSpec.SkillName); err != nil {
		return skill.ImplementationContext{}, err
	} else {
		ctxData.ManagedInstructions = instructions
	}

	return ctxData, nil
}

func buildRepositoryPRFeedbackImplementationContext(cfg *config.Service, workDir string, improvementWorkDir string, job domain.Job, events []domain.Event, runSpec implementationRunSpec) (skill.ImplementationContext, error) {
	ctxData := skill.ImplementationContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: runSpec.ArtifactDir,
	}

	implementationArtifact, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerImplementation, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation), "result.md", "review_fix.md", "implement.md", "summary.md", "stdout.log")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return skill.ImplementationContext{}, err
	}
	if err == nil {
		ctxData.ImplementationArtifact = string(implementationArtifact)
	}

	rerunComment, previousFailure, previousTestReport, err := loadRepositoryImplementationRetryContext(cfg, workDir, job, events)
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
		if err := unmarshalEventPayload(events[i].Payload, &payload); err != nil {
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

	if instructions, err := loadRepositoryImprovementInstructions(cfg, workDir, job.Repository, runSpec.SkillName); err != nil {
		return skill.ImplementationContext{}, err
	} else {
		ctxData.ManagedInstructions = instructions
	}

	return ctxData, nil
}

func buildRepositoryReviewContext(cfg *config.Service, workDir string, improvementWorkDir string, job domain.Job, events []domain.Event) (skill.ReviewContext, error) {
	ctxData := skill.ReviewContext{
		JobID:       job.ID,
		Repository:  job.Repository,
		PullNumber:  job.GitHubNumber,
		Title:       job.Title,
		WatchRuleID: job.WatchRuleID,
		BranchName:  job.BranchName,
		ArtifactDir: repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerReview),
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
		if err := unmarshalEventPayload(event.Payload, &payload); err != nil {
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

	if instructions, err := loadRepositoryImprovementInstructions(cfg, improvementWorkDir, job.Repository, "review"); err != nil {
		return skill.ReviewContext{}, err
	} else {
		ctxData.ManagedInstructions = instructions
	}

	return ctxData, nil
}

func loadRepositoryImplementationRetryContext(cfg *config.Service, workDir string, job domain.Job, events []domain.Event) (string, string, string, error) {
	var rerunComment string
	var previousFailure string
	var previousTestReport string

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if rerunComment == "" && event.EventType == "implementation_rerun_requested" {
			var payload struct {
				Comment string `json:"comment"`
			}
			if err := unmarshalEventPayload(event.Payload, &payload); err != nil {
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
			if err := unmarshalEventPayload(event.Payload, &payload); err != nil {
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
		reportDir := repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
		if raw, err := os.ReadFile(filepath.Join(reportDir, "test-report.json")); err == nil {
			previousTestReport = string(raw)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", "", fmt.Errorf("read previous test report: %w", err)
		}
	}

	return rerunComment, previousFailure, previousTestReport, nil
}
