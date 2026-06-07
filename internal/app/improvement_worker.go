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

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

const improvementSkillName = "improvement"

func generateImprovementDraft(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, jobID string, comment string, logger *log.Logger) error {
	job, events, err := orch.JobDetail(ctx, jobID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(comment) == "" {
		return fmt.Errorf("comment is empty")
	}
	repoConfig, ok := findRepositoryConfig(cfg.App(), job.Repository)
	if !ok || !repoConfig.ImprovementEnabled {
		return fmt.Errorf("improvement is disabled for repository %q", job.Repository)
	}

	workDir := artifacts.RepositoryWorkerWorkDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, repoConfig.WorkDir)
	improvementDir := artifacts.RepositoryWorkerImprovementDir(workDir, repoConfig.ImprovementWorkDir)
	jobImprovementDir := filepath.Join(artifacts.RepositoryWorkerJobDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber), "improvement")
	if err := os.MkdirAll(improvementDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(jobImprovementDir, 0o755); err != nil {
		return err
	}

	inputPath := filepath.Join(improvementDir, "input.md")
	if err := os.WriteFile(inputPath, []byte(comment), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(jobImprovementDir, "input.md"), []byte(comment), 0o644); err != nil {
		return err
	}
	if err := writeImprovementContext(filepath.Join(jobImprovementDir, "context.json"), job, comment, time.Now().UTC()); err != nil {
		return err
	}

	execution, err := resolveExecutionConfig(cfg, job.WatchRuleID)
	if err != nil {
		return err
	}

	contextData, err := buildImprovementContext(cfg, workDir, job, events, comment, inputPath)
	if err != nil {
		return err
	}
	runner := skill.NewRunner(workDir, cfg.Root(), "", cfg.App().CopilotAllowTools).WithLogger(logger)
	if _, err := runner.RunImprovement(ctx, improvementSkillName, contextData, execution); err != nil {
		return err
	}

	rawResult, err := os.ReadFile(filepath.Join(jobImprovementDir, "result.md"))
	if err != nil {
		return err
	}
	result := strings.TrimSpace(string(rawResult))
	now := time.Now().UTC().Format(time.RFC3339)
	if isNoImprovementNeeded(result) {
		reason := strings.TrimSpace(strings.TrimPrefix(result, "NO_IMPROVEMENT_NEEDED"))
		if err := writeImprovementDecision(filepath.Join(jobImprovementDir, "decision.json"), "no_improvement_needed", reason, now); err != nil {
			return err
		}
		return nil
	}

	draftPath := artifacts.RepositoryWorkerImprovementDraftPath(workDir, repoConfig.ImprovementWorkDir, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(draftPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(draftPath, []byte(result), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(jobImprovementDir, "draft.md"), []byte(result), 0o644); err != nil {
		return err
	}
	return writeImprovementDecision(filepath.Join(jobImprovementDir, "decision.json"), "draft_created", "", now)
}

func buildImprovementContext(cfg *config.Service, workDir string, job domain.Job, events []domain.Event, comment string, inputPath string) (skill.ImprovementContext, error) {
	ctxData := skill.ImprovementContext{
		JobID:             job.ID,
		Repository:        job.Repository,
		IssueNumber:       job.GitHubNumber,
		Title:             job.Title,
		JobType:           string(job.Type),
		Comment:           comment,
		InputArtifactPath: inputPath,
		ArtifactDir:       filepath.Join(artifacts.RepositoryWorkerJobDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber), "improvement"),
	}

	ctxData.ExistingImprovements = loadExistingImprovements(cfg, job.Repository)
	if raw, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerDesign, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerDesign), "result.md"); err == nil {
		ctxData.ExistingDesign = string(raw)
	}
	if raw, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerImplementation, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation), "result.md"); err == nil {
		ctxData.ExistingImplementation = string(raw)
	}
	if raw, err := readPreferredWorkingArtifact(workDir, artifacts.WorkerReview, job, repositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerReview), "result.md"); err == nil {
		ctxData.ExistingReview = string(raw)
	}
	if raw, err := readRepositoryWorkerArtifactFile(cfg, job.Repository, job.GitHubNumber, artifacts.WorkerPR, "result.md"); err == nil {
		ctxData.ExistingPRCommentResult = string(raw)
	}
	return ctxData, nil
}

func loadExistingImprovements(cfg *config.Service, repository string) string {
	repoConfig, ok := findRepositoryConfig(cfg.App(), repository)
	if !ok {
		return ""
	}
	workDir := artifacts.RepositoryWorkerWorkDir(cfg.Root(), cfg.App().ArtifactsDir, repository, repoConfig.WorkDir)
	approvedDir := artifacts.RepositoryWorkerImprovementApprovedDir(workDir, repoConfig.ImprovementDir)
	entries, err := os.ReadDir(approvedDir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(approvedDir, entry.Name()))
		if err != nil {
			continue
		}
		parts = append(parts, "## "+entry.Name()+"\n\n"+string(raw))
	}
	return strings.Join(parts, "\n\n")
}

func findRepositoryConfig(app config.App, repository string) (config.MonitoredRepository, bool) {
	for _, repo := range app.MonitoredRepositories {
		if strings.TrimSpace(repo.Repository) == strings.TrimSpace(repository) {
			return repo, true
		}
	}
	return config.MonitoredRepository{}, false
}

func isNoImprovementNeeded(result string) bool {
	return strings.HasPrefix(strings.TrimSpace(result), "NO_IMPROVEMENT_NEEDED")
}

func writeImprovementDecision(path string, decision string, reason string, updatedAt string) error {
	payload := map[string]any{
		"decision":  decision,
		"reason":    strings.TrimSpace(reason),
		"updatedAt": updatedAt,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func writeImprovementContext(path string, job domain.Job, comment string, createdAt time.Time) error {
	payload := map[string]any{
		"jobId":       job.ID,
		"repository":  job.Repository,
		"issueNumber": job.GitHubNumber,
		"title":       job.Title,
		"jobType":     string(job.Type),
		"comment":     strings.TrimSpace(comment),
		"createdAt":   createdAt.UTC().Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
