package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/web"
)

type ArtifactActions interface {
	GetArtifact(context.Context, string) (web.DesignArtifact, error)
	ApproveArtifact(context.Context, string, string) (domain.Job, error)
	RerunArtifact(context.Context, string, string) (domain.Job, error)
}

type RepositoryMonitor interface {
	PollNow(context.Context) error
}

type ArtifactActionService struct {
	store    JobStore
	settings SettingsStore
	manager  *WorkerManager
	feedback DesignFeedbackStore
	baseDir  string
	toolDir  string
	logger   workflowLogger
	monitor  RepositoryMonitor
}

func NewArtifactActionService(store JobStore, settings SettingsStore, manager *WorkerManager, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger, monitor RepositoryMonitor) *ArtifactActionService {
	return &ArtifactActionService{
		store:    store,
		settings: settings,
		manager:  manager,
		feedback: feedback,
		baseDir:  baseDir,
		toolDir:  toolDir,
		logger:   logger,
		monitor:  monitor,
	}
}

func (s *ArtifactActionService) GetArtifact(ctx context.Context, id string) (web.DesignArtifact, error) {
	job, ok, err := s.getJob(ctx, id)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	if !ok {
		return web.DesignArtifact{}, fmt.Errorf("job not found")
	}
	path, err := s.artifactPath(job)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	return web.DesignArtifact{Content: string(raw), Path: path}, nil
}

func (s *ArtifactActionService) ApproveArtifact(ctx context.Context, id, userComment string) (domain.Job, error) {
	job, ok, err := s.getJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if !ok {
		return domain.Job{}, fmt.Errorf("job not found")
	}
	if !isReadyState(job.State) {
		return domain.Job{}, fmt.Errorf("job is not ready for approval")
	}

	artifact, err := s.GetArtifact(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}

	if job.Kind == domain.JobKindIssueImplementation {
		if err := s.prepareImplementationBranch(ctx, job); err != nil {
			return domain.Job{}, err
		}
		if err := s.createPullRequest(ctx, job, buildResultBody(artifact.Content, userComment)); err != nil {
			return domain.Job{}, err
		}
		if err := s.updateTargetLabels(ctx, job, []string{
			domain.MustLabel(domain.StateImplementationApproved),
			domain.MustLabel(domain.StatePRCreated),
		}, []string{domain.MustLabel(domain.StateDesignApproved)}); err != nil {
			return domain.Job{}, err
		}
	} else {
		if err := s.postTargetComment(ctx, job, artifact.Content, userComment); err != nil {
			return domain.Job{}, err
		}
		if err := s.updateTargetLabels(ctx, job, []string{domain.MustLabel(domain.ApprovedStateForReadyState(job.State))}, nil); err != nil {
			return domain.Job{}, err
		}
	}
	if s.feedback != nil {
		if err := s.feedback.Delete(ctx, job.ID); err != nil {
			return domain.Job{}, err
		}
	}
	job.State = domain.StateCompleted
	if err := s.completeApproval(ctx, job); err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (s *ArtifactActionService) completeApproval(ctx context.Context, job domain.Job) error {
	if err := s.store.Upsert(ctx, job); err != nil {
		return err
	}
	if s.monitor != nil {
		if err := s.monitor.PollNow(ctx); err != nil {
			return fmt.Errorf("refresh repository after approval: %w", err)
		}
	}
	return nil
}

func (s *ArtifactActionService) RerunArtifact(ctx context.Context, id, userComment string) (domain.Job, error) {
	job, ok, err := s.getJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if !ok {
		return domain.Job{}, fmt.Errorf("job not found")
	}
	if !isReadyState(job.State) {
		return domain.Job{}, fmt.Errorf("job is not ready for rerun")
	}
	if s.feedback != nil {
		if err := s.feedback.Save(ctx, job.ID, userComment); err != nil {
			return domain.Job{}, err
		}
	}
	job.State = domain.RunningStateForReadyState(job.State)
	if err := s.store.Upsert(ctx, job); err != nil {
		return domain.Job{}, err
	}
	if s.manager == nil {
		return domain.Job{}, fmt.Errorf("worker manager not configured")
	}
	if err := s.manager.Submit(job); err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (s *ArtifactActionService) getJob(ctx context.Context, id string) (domain.Job, bool, error) {
	if s.store == nil {
		return domain.Job{}, false, fmt.Errorf("job store not configured")
	}
	return s.store.Get(ctx, id)
}

func (s *ArtifactActionService) artifactPath(job domain.Job) (string, error) {
	if artifactSubdir(job) == "" {
		return "", fmt.Errorf("job is not supported")
	}
	return filepath.Join(s.baseDir, ".workspace", artifactSubdir(job), fmt.Sprintf("%d_%s.md", job.Number, sanitizePart(job.Title))), nil
}

func (s *ArtifactActionService) postTargetComment(ctx context.Context, job domain.Job, artifact string, userComment string) error {
	return runGH(ctx, append(githubCommentArgs(job), "--body", buildResultBody(artifact, userComment))...)
}

func (s *ArtifactActionService) createPullRequest(ctx context.Context, job domain.Job, body string) error {
	repoDir, err := s.jobRepoDir(ctx, job)
	if err != nil {
		return err
	}
	branch, err := currentBranch(ctx, repoDir)
	if err != nil {
		return err
	}
	if err := ensureBranchHasCommit(ctx, repoDir, branch); err != nil {
		return err
	}
	baseBranch := "main"
	if s.settings != nil {
		settings, err := s.settings.Load(ctx)
		if err == nil && strings.TrimSpace(settings.BaseBranch) != "" {
			baseBranch = strings.TrimSpace(settings.BaseBranch)
		}
	}
	if err := publishBranch(ctx, repoDir, branch); err != nil {
		return err
	}
	args := []string{
		"pr", "create",
		"--repo", job.Repository,
		"--base", baseBranch,
		"--title", job.Title,
		"--body", body,
		"--head", branch,
	}
	return runGH(ctx, args...)
}

func (s *ArtifactActionService) prepareImplementationBranch(ctx context.Context, job domain.Job) error {
	repoDir, err := s.jobRepoDir(ctx, job)
	if err != nil {
		return err
	}
	if err := stageAndCommitIfNeeded(ctx, repoDir, fmt.Sprintf("feat: implement #%d %s", job.Number, job.Title)); err != nil {
		return err
	}
	return nil
}

func (s *ArtifactActionService) ensureBranch(ctx context.Context, job domain.Job) (string, error) {
	pattern := "issue_#<issue番号>"
	if s.settings != nil {
		settings, err := s.settings.Load(ctx)
		if err == nil {
			if trimmed := strings.TrimSpace(settings.BranchNamePattern); trimmed != "" {
				pattern = trimmed
			}
		}
	}
	branch := renderBranchName(pattern, job.Number)
	if err := checkoutOrCreateBranch(ctx, s.baseDir, branch); err != nil {
		return "", err
	}
	return branch, nil
}

func (s *ArtifactActionService) jobRepoDir(ctx context.Context, job domain.Job) (string, error) {
	if job.Kind != domain.JobKindIssueImplementation {
		return s.baseDir, nil
	}
	return implementationWorktreePath(s.toolDir, job), nil
}

func (s *ArtifactActionService) updateTargetLabels(ctx context.Context, job domain.Job, add []string, remove []string) error {
	for _, label := range add {
		if err := ensureGHLabel(ctx, job.Repository, label); err != nil {
			return err
		}
	}
	args := githubEditArgs(job)
	for _, label := range add {
		args = append(args, "--add-label", label)
	}
	for _, label := range remove {
		args = append(args, "--remove-label", label)
	}
	return runGH(ctx, args...)
}

func githubCommentArgs(job domain.Job) []string {
	args := []string{}
	switch domain.ResultCommentTarget(job.Kind) {
	case "pr":
		args = append(args, "pr", "comment")
	default:
		args = append(args, "issue", "comment")
	}
	args = append(args, "--repo", job.Repository, fmt.Sprintf("%d", job.Number))
	return args
}

func githubEditArgs(job domain.Job) []string {
	args := []string{}
	switch domain.ResultCommentTarget(job.Kind) {
	case "pr":
		args = append(args, "pr", "edit")
	default:
		args = append(args, "issue", "edit")
	}
	args = append(args, "--repo", job.Repository, fmt.Sprintf("%d", job.Number))
	return args
}

func runGH(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func checkoutOrCreateBranch(ctx context.Context, baseDir, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	if err := runGit(ctx, baseDir, "check-ref-format", "--branch", branch); err != nil {
		return err
	}
	if err := runGit(ctx, baseDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return runGit(ctx, baseDir, "checkout", branch)
	}
	return runGit(ctx, baseDir, "checkout", "-b", branch)
}

func ensureBranchHasCommit(ctx context.Context, baseDir, branch string) error {
	count, err := gitCommitCount(ctx, baseDir, "main.."+branch)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return runGit(ctx, baseDir, "commit", "--allow-empty", "-m", "chore: prepare PR for "+branch)
}

func publishBranch(ctx context.Context, baseDir, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	return runGit(ctx, baseDir, "push", "-u", "origin", branch)
}

func stageAndCommitIfNeeded(ctx context.Context, repoDir string, message string) error {
	dirty, err := gitHasChanges(ctx, repoDir)
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	if err := runGit(ctx, repoDir, "add", "-A"); err != nil {
		return err
	}
	return runGit(ctx, repoDir, "commit", "-m", message)
}

func gitHasChanges(ctx context.Context, repoDir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status --porcelain: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func runGit(ctx context.Context, baseDir string, args ...string) error {
	fullArgs := append([]string{"-C", baseDir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitCommitCount(ctx context.Context, baseDir, revRange string) (int, error) {
	fullArgs := []string{"-C", baseDir, "rev-list", "--count", revRange}
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count %s: %w: %s", revRange, err, strings.TrimSpace(string(out)))
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse git rev-list count %q: %w", strings.TrimSpace(string(out)), err)
	}
	return count, nil
}

func ensureGHLabel(ctx context.Context, repository, label string) error {
	args := []string{
		"label", "create", label,
		"--repo", repository,
		"--color", "0E8A16",
		"--description", "korobokcle state label",
		"--force",
	}
	return runGH(ctx, args...)
}

func renderBranchName(pattern string, issueNumber int) string {
	branch := strings.TrimSpace(pattern)
	if branch == "" {
		branch = "issue_#<issue番号>"
	}
	issueNumberText := strconv.Itoa(issueNumber)
	branch = strings.NewReplacer(
		"<issue番号>", issueNumberText,
		"<issueNumber>", issueNumberText,
		"{issue番号}", issueNumberText,
		"{issueNumber}", issueNumberText,
	).Replace(branch)
	return strings.TrimSpace(branch)
}

func isReadyState(state domain.JobState) bool {
	switch state {
	case domain.StateDesignReady,
		domain.StateImplementationReady,
		domain.StateReviewReady,
		domain.StateReviewFixDesignReady,
		domain.StateReviewFixImplementationReady:
		return true
	default:
		return false
	}
}

func buildResultBody(artifact string, userComment string) string {
	lines := []string{
		"### 結果",
		strings.TrimSpace(artifact),
	}
	if comment := strings.TrimSpace(userComment); comment != "" {
		lines = append(lines,
			"",
			"### ユーザコメント",
			comment,
		)
	}
	return strings.Join(lines, "\n")
}

func currentBranch(ctx context.Context, baseDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", baseDir, "branch", "--show-current")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w: %s", err, strings.TrimSpace(string(out)))
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("current branch not found")
	}
	return branch, nil
}
