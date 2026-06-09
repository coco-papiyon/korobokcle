package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
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

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository)
	workFiles := repositoryImprovementWorkFiles(workDir, repositoryConfig.ImprovementWorkDir)
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

	contextRaw, err := os.ReadFile(workFiles.ContextPath)
	if err != nil {
		return err
	}
	var contextData improvementContextData
	if err := json.Unmarshal(contextRaw, &contextData); err != nil {
		return err
	}

	document := buildApprovedImprovementDocument(jobID, contextData, resultBody)
	if err := writeImprovementImplementationPrompt(cfg, repositoryConfig, job, contextData, document, resultBody, workFiles, artifactFiles); err != nil && logger != nil {
		logger.Printf("improvement implementation prompt generation failed job_id=%s error=%v", jobID, err)
	}
	documentRaw, err := document.MarshalMarkdown()
	if err != nil {
		return err
	}
	if err := writeApprovedImprovementPhaseFiles(workDir, artifactFiles.Dir, repositoryConfig.ImprovementWorkDir, contextData.Phases, documentRaw); err != nil {
		return err
	}

	repoRelativePath := filepath.ToSlash(filepath.Join(config.ResolveImprovementDir(repositoryConfig), document.FrontMatter.ID+".md"))
	targetPath := filepath.Join(workDir, filepath.FromSlash(repoRelativePath))
	if err := updateImprovementBranch(ctx, workDir, config.ResolveImprovementBranch(repositoryConfig), artifactFiles.Dir, repoRelativePath, targetPath, documentRaw); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("improvement draft approved job_id=%s path=%s", jobID, targetPath)
	}
	return nil
}

type improvementImplementationPromptContext struct {
	JobID        string                 `json:"jobId"`
	Repository   string                 `json:"repository"`
	IssueNumber  int                    `json:"issueNumber"`
	Title        string                 `json:"title"`
	TargetPath   string                 `json:"targetPath"`
	DocumentID   string                 `json:"documentId"`
	Phases       []string               `json:"phases"`
	Source       improvementSourceInput `json:"source"`
	ApprovedBody string                 `json:"approvedBody"`
}

func writeImprovementImplementationPrompt(cfg *config.Service, repositoryConfig config.MonitoredRepository, job domain.Job, contextData improvementContextData, document ImprovementDocument, approvedBody string, workFiles improvementWorkFiles, artifactFiles improvementArtifactFiles) error {
	targetPath := filepath.ToSlash(filepath.Join(config.ResolveImprovementDir(repositoryConfig), document.FrontMatter.ID+".md"))
	promptContext := improvementImplementationPromptContext{
		JobID:        job.ID,
		Repository:   job.Repository,
		IssueNumber:  job.GitHubNumber,
		Title:        document.FrontMatter.Title,
		TargetPath:   targetPath,
		DocumentID:   document.FrontMatter.ID,
		Phases:       append([]string(nil), contextData.Phases...),
		Source:       contextData.Source,
		ApprovedBody: approvedBody,
	}
	prompt, err := skill.RenderSkillPrompt(cfg.Root(), "default/improvement_implementation", promptContext)
	if err != nil {
		return err
	}
	if err := writeImprovementFile(workFiles.ImplementationPromptPath, []byte(prompt)); err != nil {
		return err
	}
	if err := writeImprovementFile(artifactFiles.ImplementationPromptPath, []byte(prompt)); err != nil {
		return err
	}
	return nil
}

func buildApprovedImprovementDocument(jobID string, contextData improvementContextData, body string) ImprovementDocument {
	title := improvementDocumentTitle(body, contextData.Title)
	now := time.Now().UTC()
	return ImprovementDocument{
		FrontMatter: ImprovementFrontMatter{
			ID:        improvementDocumentID(title, jobID),
			Title:     title,
			Scope:     "repository",
			Phases:    append([]string(nil), contextData.Phases...),
			Status:    "active",
			UpdatedAt: now,
			Source: ImprovementSource{
				JobID:       contextData.JobID,
				IssueNumber: contextData.IssueNumber,
				Repository:  contextData.Repository,
				Event:       contextData.Source.EventType,
			},
		},
	Body: improvementDocumentBody(body),
	}
}

func writeApprovedImprovementPhaseFiles(workDir string, artifactDir string, configuredWorkDir string, phases []string, raw []byte) error {
	phaseNames := improvementPhaseFileNames(phases)
	for _, phase := range phaseNames {
		workPath := artifacts.RepositoryWorkerImprovementPhaseFile(workDir, configuredWorkDir, phase)
		artifactPath := filepath.Join(artifactDir, phase+".md")
		if err := writeImprovementFile(workPath, raw); err != nil {
			return err
		}
		if err := writeImprovementFile(artifactPath, raw); err != nil {
			return err
		}
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

func improvementDocumentTitle(body string, fallback string) string {
	lines := strings.Split(body, "\n")
	inTitle := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## タイトル" || trimmed == "# タイトル" {
			inTitle = true
			continue
		}
		if inTitle {
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				break
			}
			return trimmed
		}
	}
	trimmedFallback := strings.TrimSpace(fallback)
	if trimmedFallback != "" {
		return trimmedFallback
	}
	return "改善方針"
}

func improvementDocumentBody(body string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## 汎化した方針案" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return strings.TrimSpace(body)
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n"))
}

func improvementDocumentID(title string, fallback string) string {
	trimmed := strings.TrimSpace(strings.ToLower(title))
	if trimmed == "" {
		trimmed = strings.TrimSpace(strings.ToLower(fallback))
	}
	if trimmed == "" {
		return "improvement"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "@", "-", "?", "-", "#", "-")
	normalized := replacer.Replace(trimmed)
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		normalized = "improvement"
	}
	if len(normalized) <= 64 {
		return normalized
	}
	sum := sha1.Sum([]byte(normalized))
	return normalized[:48] + "-" + hex.EncodeToString(sum[:4])
}

func updateImprovementBranch(ctx context.Context, workDir string, branch string, artifactDir string, repoRelativePath string, targetPath string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
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

	if err := writeImprovementFile(targetPath, raw); err != nil {
		return err
	}
	addOutput, addErr := runGitCommandOutput(ctx, workDir, "git", "add", repoRelativePath)
	if err := writeGitLog(artifactDir, "git-add.log", addOutput, addErr); err != nil {
		return err
	}
	if addErr != nil {
		return addErr
	}
	commitOutput, commitErr := runGitCommandOutput(ctx, workDir, "git", "commit", "--allow-empty", "-m", "Update improvement "+repoRelativePath)
	if err := writeGitLog(artifactDir, "git-commit.log", commitOutput, commitErr); err != nil {
		return err
	}
	if commitErr != nil {
		return commitErr
	}

	pushOutput, pushErr := runGitCommandOutput(ctx, workDir, "git", "push", "origin", branch)
	if pushErr == nil {
		return writeGitLog(artifactDir, "git-push.log", pushOutput, nil)
	}
	if err := writeGitLog(artifactDir, "git-push.log", pushOutput, pushErr); err != nil {
		return err
	}

	retryFetchOutput, retryFetchErr := runGitCommandOutput(ctx, workDir, "git", "fetch", "--prune", "origin")
	if err := writeGitLog(artifactDir, "git-fetch-retry.log", retryFetchOutput, retryFetchErr); err != nil {
		return err
	}
	if retryFetchErr != nil {
		return retryFetchErr
	}
	remoteRaw, remoteErr := readGitFile(ctx, workDir, "origin/"+branch, repoRelativePath)
	if remoteErr == nil && !sameBytes(remoteRaw, raw) {
		return fmt.Errorf("improvement branch conflict: remote file %s changed", repoRelativePath)
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

func readGitFile(ctx context.Context, repoDir string, ref string, path string) ([]byte, error) {
	output, err := runGitCommand(ctx, repoDir, "git", "show", ref+":"+filepath.ToSlash(path))
	if err != nil {
		if strings.Contains(err.Error(), "exists on disk, but not in") || strings.Contains(err.Error(), "path") {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	return []byte(output), nil
}

func sameBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
