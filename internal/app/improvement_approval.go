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

const (
	improvementApprovalApproved = "approved"
	improvementApprovalRejected = "rejected"
)

type improvementApprovalRecord struct {
	Status     string `json:"status"`
	Comment    string `json:"comment,omitempty"`
	ApprovedAt string `json:"approvedAt"`
}

type improvementApprovalRequest struct {
	Status     string
	Comment    string
	ResultBody string
}

func applyImprovementApproval(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, jobID string, req improvementApprovalRequest, logger *log.Logger) error {
	job, _, err := orch.JobDetail(ctx, jobID)
	if err != nil {
		return err
	}

	repositoryConfig, ok := resolveMonitoredRepository(cfg, job.Repository)
	if !ok || !repositoryConfig.ImprovementEnabled {
		return fmt.Errorf("improvement feature is disabled for repository %q", job.Repository)
	}

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, config.ResolveImprovementBranch(repositoryConfig))
	workFiles := repositoryImprovementWorkFiles(workDir, repositoryConfig.ImprovementDir, job.ID, job.Title)
	artifactFiles := repositoryImprovementArtifactFiles(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber)

	draftRaw, err := os.ReadFile(workFiles.DraftPath)
	if err != nil {
		return err
	}
	resultBody := strings.TrimSpace(req.ResultBody)
	if resultBody == "" {
		resultBody = strings.TrimSpace(string(draftRaw))
	}

	approval := improvementApprovalRecord{
		Status:     strings.TrimSpace(req.Status),
		Comment:    strings.TrimSpace(req.Comment),
		ApprovedAt: time.Now().UTC().Format(time.RFC3339),
	}
	switch approval.Status {
	case improvementApprovalApproved, improvementApprovalRejected:
	default:
		return fmt.Errorf("unsupported improvement approval status %q", approval.Status)
	}

	approvalRaw, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return err
	}
	if err := writeImprovementFile(artifactFiles.ApprovalPath, approvalRaw); err != nil {
		return err
	}
	if err := writeImprovementFile(artifactFiles.ResultPath, []byte(resultBody+"\n")); err != nil {
		return err
	}

	decision := improvementDecision{
		Decision:    approval.Status,
		Reason:      approval.Comment,
		UpdatedAt:   approval.ApprovedAt,
		SourceEvent: "",
	}
	if err := writeImprovementDecisionFiles(workFiles, artifactFiles, decision); err != nil {
		return err
	}

	if approval.Status == improvementApprovalRejected {
		if logger != nil {
			logger.Printf("improvement draft rejected job_id=%s", jobID)
		}
		return nil
	}

	contextRaw, err := os.ReadFile(artifactFiles.ContextPath)
	if err != nil {
		return err
	}
	var contextData improvementContextData
	if err := json.Unmarshal(contextRaw, &contextData); err != nil {
		return err
	}

	phaseNames := improvementPhaseFileNames(contextData.Phases)
	phaseName := improvementPrimaryPhaseName(phaseNames)
	targetPath := artifacts.RepositoryWorkerImprovementPhaseFile(workDir, repositoryConfig.ImprovementDir, phaseName)
	if err := writeImprovementImplementationPrompt(cfg, job, contextData, resultBody, phaseNames, targetPath, artifactFiles); err != nil {
		return err
	}
	implementationOutput, err := runImprovementImplementation(ctx, cfg, repositoryConfig, job, contextData, resultBody, targetPath, phaseNames, artifactFiles, logger)
	if err != nil {
		return err
	}
	if err := writeImprovementFile(targetPath, []byte(strings.TrimSpace(implementationOutput)+"\n")); err != nil {
		return err
	}

	if err := prepareImprovementBranch(ctx, workDir, config.ResolveImprovementBranch(repositoryConfig), artifactFiles.Dir); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("improvement draft approved job_id=%s path=%s prepared=true", jobID, targetPath)
	}
	return nil
}

type improvementImplementationPromptContext struct {
	JobID        string                 `json:"jobId"`
	Repository   string                 `json:"repository"`
	IssueNumber  int                    `json:"issueNumber"`
	Title        string                 `json:"title"`
	WorkDir      string                 `json:"workDir"`
	TargetPath   string                 `json:"targetPath"`
	Phases       []string               `json:"phases"`
	Source       improvementSourceInput `json:"source"`
	ApprovedBody string                 `json:"approvedBody"`
}

func writeImprovementImplementationPrompt(cfg *config.Service, job domain.Job, contextData improvementContextData, approvedBody string, phaseNames []string, targetPath string, artifactFiles improvementArtifactFiles) error {
	repositoryConfig, _ := resolveMonitoredRepository(cfg, job.Repository)
	promptContext := improvementImplementationPromptContext{
		JobID:        job.ID,
		Repository:   job.Repository,
		IssueNumber:  job.GitHubNumber,
		Title:        contextData.Title,
		WorkDir:      artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, config.ResolveImprovementBranch(repositoryConfig)),
		TargetPath:   targetPath,
		Phases:       append([]string(nil), phaseNames...),
		Source:       contextData.Source,
		ApprovedBody: approvedBody,
	}
	prompt, err := skill.RenderSkillPrompt(cfg.Root(), "default/improvement_implementation", promptContext)
	if err != nil {
		return err
	}
	if err := writeImprovementFile(artifactFiles.ImplementationPromptPath, []byte(prompt)); err != nil {
		return err
	}
	return nil
}

func improvementPhaseFileNames(phases []string) []string {
	seen := make(map[string]struct{}, len(phases))
	out := make([]string, 0, len(phases))
	for _, phase := range phases {
		trimmed := strings.TrimSpace(phase)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		out = append(out, "improvement")
	}
	return out
}

func improvementPrimaryPhaseName(phases []string) string {
	phaseNames := improvementPhaseFileNames(phases)
	if len(phaseNames) == 0 {
		return "improvement"
	}
	return phaseNames[0]
}

func runImprovementImplementation(ctx context.Context, cfg *config.Service, repositoryConfig config.MonitoredRepository, job domain.Job, contextData improvementContextData, approvedBody string, targetPath string, phaseNames []string, artifactFiles improvementArtifactFiles, logger *log.Logger) (string, error) {
	if err := syncRepositoryImprovementWorkspace(ctx, cfg, repositoryConfig, artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, config.ResolveImprovementBranch(repositoryConfig)), logger); err != nil {
		return "", err
	}

	execution, err := resolveImprovementExecutionConfig(cfg, job.WatchRuleID)
	if err != nil {
		return "", err
	}

	promptContext := improvementImplementationPromptContext{
		JobID:        job.ID,
		Repository:   job.Repository,
		IssueNumber:  job.GitHubNumber,
		Title:        contextData.Title,
		WorkDir:      artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, config.ResolveImprovementBranch(repositoryConfig)),
		TargetPath:   targetPath,
		Phases:       append([]string(nil), phaseNames...),
		Source:       contextData.Source,
		ApprovedBody: approvedBody,
	}

	prompt, err := skill.RenderSkillPrompt(cfg.Root(), "default/improvement_implementation", promptContext)
	if err != nil {
		return "", err
	}
	if err := writeImprovementFile(artifactFiles.ImplementationPromptPath, []byte(prompt)); err != nil {
		return "", err
	}

	provider, err := skill.ProviderFor(execution.Provider)
	if err != nil {
		return "", err
	}
	request := skill.AIRequest{
		SkillName:         "improvement_implementation",
		Prompt:            prompt,
		Model:             execution.Model,
		WorkDir:           artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, config.ResolveImprovementBranch(repositoryConfig)),
		ArtifactDir:       artifactFiles.Dir,
		OutputPath:        targetPath,
		StdoutLogPath:     filepath.Join(artifactFiles.Dir, "stdout.log"),
		StderrLogPath:     filepath.Join(artifactFiles.Dir, "stderr.log"),
		CopilotAllowTools: cfg.App().CopilotAllowTools,
	}
	if logger != nil {
		logger.Printf(
			"ai execution started phase=%s skill=%s provider=%s model=%s workdir=%s artifact_dir=%s output_path=%s",
			"improvement_implementation",
			request.SkillName,
			execution.Provider,
			execution.Model,
			request.WorkDir,
			request.ArtifactDir,
			request.OutputPath,
		)
	}
	result, err := provider.Run(ctx, request)
	if err != nil {
		if logger != nil {
			logger.Printf(
				"ai execution failed phase=%s skill=%s provider=%s model=%s workdir=%s artifact_dir=%s output_path=%s error=%v",
				"improvement_implementation",
				request.SkillName,
				execution.Provider,
				execution.Model,
				request.WorkDir,
				request.ArtifactDir,
				request.OutputPath,
				err,
			)
		}
		return "", err
	}
	if err := os.WriteFile(filepath.Join(artifactFiles.Dir, "stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(artifactFiles.Dir, "stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return "", err
	}
	if logger != nil {
		logger.Printf(
			"ai execution completed phase=%s skill=%s provider=%s model=%s workdir=%s artifact_dir=%s output_path=%s stdout_bytes=%d stderr_bytes=%d output_bytes=%d",
			"improvement_implementation",
			request.SkillName,
			execution.Provider,
			execution.Model,
			request.WorkDir,
			request.ArtifactDir,
			request.OutputPath,
			len(result.Stdout),
			len(result.Stderr),
			len(result.Output),
		)
	}
	output := strings.TrimSpace(result.Output)
	if output == "" {
		return "", fmt.Errorf("improvement implementation provider returned empty output")
	}
	return output, nil
}

func resolveImprovementExecutionConfig(cfg *config.Service, watchRuleID string) (skill.ExecutionConfig, error) {
	if trimmed := strings.TrimSpace(watchRuleID); trimmed != "" {
		if execution, err := resolveExecutionConfig(cfg, trimmed); err == nil {
			return execution, nil
		}
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.App().Provider))
	if provider == "" {
		provider = "mock"
	}
	spec, ok := cfg.ProviderByName(provider)
	if !ok {
		return skill.ExecutionConfig{}, fmt.Errorf("provider %q not found", provider)
	}

	model := strings.TrimSpace(cfg.App().Model)
	if model != "" {
		validatedModel, err := config.ValidateModelForProvider(spec, model)
		if err != nil {
			return skill.ExecutionConfig{}, fmt.Errorf("%w", err)
		}
		model = validatedModel
	}
	return skill.ExecutionConfig{
		Provider: provider,
		Model:    model,
	}, nil
}

func prepareImprovementBranch(ctx context.Context, workDir string, branch string, artifactDir string) error {
	fetchOutput, fetchErr := runGitCommandOutput(ctx, workDir, "git", "fetch", "--prune", "origin")
	if err := writeGitLog(artifactDir, "git-fetch.log", fetchOutput, fetchErr); err != nil {
		return err
	}
	if fetchErr != nil {
		return fetchErr
	}

	hasRemoteBranch := gitRemoteBranchExists(ctx, workDir, branch)
	checkoutArgs := []string{"git", "checkout", "-B", branch}
	if hasRemoteBranch {
		checkoutArgs = []string{"git", "checkout", "-B", branch, "origin/" + branch}
	}
	checkoutOutput, checkoutErr := runGitCommandOutput(ctx, workDir, checkoutArgs...)
	if err := writeGitLog(artifactDir, "git-checkout.log", checkoutOutput, checkoutErr); err != nil {
		return err
	}
	if checkoutErr != nil {
		return checkoutErr
	}
	if hasRemoteBranch {
		resetOutput, resetErr := runGitCommandOutput(ctx, workDir, "git", "reset", "--hard", "origin/"+branch)
		if err := writeGitLog(artifactDir, "git-reset.log", resetOutput, resetErr); err != nil {
			return err
		}
		if resetErr != nil {
			return resetErr
		}
	}

	addOutput, addErr := runGitCommandOutput(ctx, workDir, "git", "add", "-A", "--", ".", ":(exclude).improvement/draft/**")
	if err := writeGitLog(artifactDir, "git-add.log", addOutput, addErr); err != nil {
		return err
	}
	if addErr != nil {
		return addErr
	}
	commitOutput, commitErr := runGitCommandOutput(ctx, workDir, "git", "commit", "--allow-empty", "-m", "Update improvement workspace")
	if err := writeGitLog(artifactDir, "git-commit.log", commitOutput, commitErr); err != nil {
		return err
	}
	if commitErr != nil {
		return commitErr
	}

	return nil
}

func pushImprovementBranch(ctx context.Context, workDir string, branch string, artifactDir string) error {
	statusOutput, statusErr := runGitCommandOutput(ctx, workDir, "git", "status", "--porcelain", "--untracked-files=no")
	if err := writeGitLog(artifactDir, "git-status.log", statusOutput, statusErr); err != nil {
		return err
	}
	if statusErr != nil {
		return statusErr
	}

	if strings.TrimSpace(statusOutput) != "" {
		addOutput, addErr := runGitCommandOutput(ctx, workDir, "git", "add", "-A", "--", ".", ":(exclude).improvement/draft/**")
		if err := writeGitLog(artifactDir, "git-add.log", addOutput, addErr); err != nil {
			return err
		}
		if addErr != nil {
			return addErr
		}

		commitOutput, commitErr := runGitCommandOutput(ctx, workDir, "git", "commit", "--allow-empty", "-m", "Update improvement workspace")
		if err := writeGitLog(artifactDir, "git-commit.log", commitOutput, commitErr); err != nil {
			return err
		}
		if commitErr != nil {
			return commitErr
		}
	}

	pushOutput, pushErr := runGitCommandOutput(ctx, workDir, "git", "push", "origin", branch)
	if pushErr == nil {
		return writeGitLog(artifactDir, "git-push.log", pushOutput, nil)
	}
	if err := writeGitLog(artifactDir, "git-push.log", pushOutput, pushErr); err != nil {
		return err
	}

	retryOutput, retryErr := runGitCommandOutput(ctx, workDir, "git", "push", "origin", branch)
	if err := writeGitLog(artifactDir, "git-push-retry.log", retryOutput, retryErr); err != nil {
		return err
	}
	if retryErr != nil {
		return retryErr
	}
	return nil
}

func gitRemoteBranchExists(ctx context.Context, workDir string, branch string) bool {
	_, err := runGitCommandOutput(ctx, workDir, "git", "ls-remote", "--exit-code", "--heads", "origin", branch)
	return err == nil
}

func runGitCommandOutput(ctx context.Context, repoDir string, args ...string) (string, error) {
	output, err := runGitCommand(ctx, repoDir, args...)
	if err != nil {
		return output, err
	}
	return output, nil
}

func writeGitLog(artifactDir string, name string, output string, err error) error {
	if err != nil {
		output = strings.TrimSpace(output + "\n" + err.Error())
	}
	return writeImprovementFile(filepath.Join(artifactDir, name), []byte(strings.TrimSpace(output)))
}
