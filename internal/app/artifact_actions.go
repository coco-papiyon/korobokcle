package app

import (
	"context"
	"encoding/json"
	"errors"
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
	RequestChanges(context.Context, string, string) (domain.Job, error)
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
		if err := s.createPullRequest(ctx, job, buildPullRequestBody(job, artifact.Content, userComment)); err != nil {
			return domain.Job{}, err
		}
		latestLabels := []string{domain.MustLabel(domain.StatePRCreated)}
		if err := s.updateTargetLabels(ctx, job, latestLabels, stateLabelsExcept(latestLabels...)); err != nil {
			return domain.Job{}, err
		}
	} else if job.Kind == domain.JobKindPRConflict {
		if err := s.prepareImplementationBranch(ctx, job); err != nil {
			return domain.Job{}, err
		}
		repoDir, err := s.jobRepoDir(ctx, job)
		if err != nil {
			return domain.Job{}, err
		}
		branch, err := currentBranch(ctx, repoDir)
		if err != nil {
			return domain.Job{}, err
		}
		if err := publishBranch(ctx, repoDir, branch); err != nil {
			return domain.Job{}, err
		}
		if err := s.postTargetComment(ctx, job, artifact.Content, userComment); err != nil {
			return domain.Job{}, err
		}
		latestLabels := []string{domain.MustLabel(domain.StatePRConflictResolved)}
		if err := s.updateTargetLabels(ctx, job, latestLabels, stateLabelsExcept(latestLabels...)); err != nil {
			return domain.Job{}, err
		}
	} else if job.Kind == domain.JobKindPRReview {
		if err := s.postTargetComment(ctx, job, artifact.Content, userComment); err != nil {
			return domain.Job{}, err
		}
		job = markJobState(job, domain.StateReviewApproved)
	} else {
		if err := s.postTargetComment(ctx, job, artifact.Content, userComment); err != nil {
			return domain.Job{}, err
		}
		approvedState := domain.ApprovedStateForReadyState(job.State)
		if job.Kind == domain.JobKindPRFeedback && job.State == domain.StateReviewFixImplementationReady {
			approvedState = domain.StateReviewFixed
		}
		latestLabels := []string{domain.MustLabel(approvedState)}
		if err := s.updateTargetLabels(ctx, job, latestLabels, stateLabelsExcept(latestLabels...)); err != nil {
			return domain.Job{}, err
		}
	}
	if s.feedback != nil {
		if err := s.feedback.Delete(ctx, job.ID); err != nil {
			return domain.Job{}, err
		}
	}
	if job.Kind != domain.JobKindPRReview {
		job = markJobState(job, domain.StateCompleted)
	}
	if err := s.completeApproval(ctx, job); err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (s *ArtifactActionService) RequestChanges(ctx context.Context, id, userComment string) (domain.Job, error) {
	job, ok, err := s.getJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if !ok {
		return domain.Job{}, fmt.Errorf("job not found")
	}
	if job.Kind != domain.JobKindPRReview || job.State != domain.StateReviewReady {
		return domain.Job{}, fmt.Errorf("job is not ready for review feedback")
	}

	artifact, err := s.GetArtifact(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if err := s.postTargetComment(ctx, job, artifact.Content, userComment); err != nil {
		return domain.Job{}, err
	}
	latestLabels := []string{domain.MustLabel(domain.StatePRReviewComment)}
	if err := s.updateTargetLabels(ctx, job, latestLabels, stateLabelsExcept(latestLabels...)); err != nil {
		return domain.Job{}, err
	}
	if s.feedback != nil {
		if err := s.feedback.Delete(ctx, job.ID); err != nil {
			return domain.Job{}, err
		}
	}
	job = markJobState(job, domain.StateCompleted)
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
	runningState := rerunRunningState(job)
	if runningState == domain.StateFailed {
		return domain.Job{}, fmt.Errorf("job is not ready for rerun")
	}
	if s.feedback != nil {
		if err := s.feedback.Save(ctx, job.ID, userComment); err != nil {
			return domain.Job{}, err
		}
	}
	job.ErrorMessage = ""
	job = markJobState(job, runningState)
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

func rerunRunningState(job domain.Job) domain.JobState {
	if isReadyState(job.State) {
		return domain.RunningStateForReadyState(job.State)
	}
	if job.State != domain.StateFailed {
		return domain.StateFailed
	}
	switch job.FailedFromState {
	case domain.StateDesignRunning,
		domain.StateImplementationRunning,
		domain.StateReviewRunning,
		domain.StateReviewFixDesignRunning,
		domain.StateReviewFixImplementationRunning,
		domain.StatePRConflictRunning:
		return job.FailedFromState
	default:
		return domain.RunningStateForKind(job.Kind, job.State)
	}
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
	return runGH(ctx, append(githubCommentArgs(job), "--body", buildResultBody(job, artifact, userComment))...)
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
	if job.Kind != domain.JobKindIssueImplementation && job.Kind != domain.JobKindPRConflict {
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
	currentLabels, err := currentTargetLabels(ctx, job)
	if err != nil {
		return err
	}
	remove = existingLabelsOnly(remove, currentLabels, add)
	args := githubEditArgs(job)
	for _, label := range add {
		args = append(args, "--add-label", label)
	}
	for _, label := range remove {
		args = append(args, "--remove-label", label)
	}
	return runGH(ctx, args...)
}

func currentTargetLabels(ctx context.Context, job domain.Job) ([]string, error) {
	args := []string{}
	switch domain.ResultCommentTarget(job.Kind) {
	case "pr":
		args = append(args, "pr", "view")
	default:
		args = append(args, "issue", "view")
	}
	args = append(args, "--repo", job.Repository, fmt.Sprintf("%d", job.Number), "--json", "labels")
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	var payload struct {
		Labels []ghLabel `json:"labels"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("decode gh labels: %w", err)
	}
	return labelNames(payload.Labels), nil
}

func existingLabelsOnly(remove []string, current []string, add []string) []string {
	currentSet := make(map[string]string, len(current))
	for _, label := range current {
		currentSet[strings.ToLower(strings.TrimSpace(label))] = label
	}
	addSet := make(map[string]struct{}, len(add))
	for _, label := range add {
		addSet[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}
	out := make([]string, 0, len(remove))
	seen := make(map[string]struct{}, len(remove))
	for _, label := range remove {
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" {
			continue
		}
		if _, ok := addSet[key]; ok {
			continue
		}
		currentLabel, ok := currentSet[key]
		if !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, currentLabel)
	}
	return out
}

func stateLabelsExcept(keep ...string) []string {
	keepSet := make(map[string]struct{}, len(keep))
	for _, label := range keep {
		keepSet[label] = struct{}{}
	}
	labels := domain.AllStateLabels()
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if _, ok := keepSet[label]; ok {
			continue
		}
		out = append(out, label)
	}
	return out
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
	exists, err := remoteBranchExists(ctx, baseDir, branch)
	if err != nil {
		return err
	}
	if exists {
		if err := runGit(ctx, baseDir, "pull", "--rebase", "origin", branch); err != nil {
			return fmt.Errorf("rebase remote branch before push: %w", err)
		}
	}
	return runGit(ctx, baseDir, "push", "-u", "origin", branch)
}

func remoteBranchExists(ctx context.Context, baseDir, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", baseDir, "ls-remote", "--exit-code", "--heads", "origin", branch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return false, nil
	}
	return false, fmt.Errorf("git ls-remote --heads origin %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
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
		domain.StateReviewFixImplementationReady,
		domain.StatePRConflictReady:
		return true
	default:
		return false
	}
}

func buildResultBody(job domain.Job, artifact string, userComment string) string {
	lines := []string{
		"# " + resultTitle(job),
		"",
		stripLeadingH1(artifact),
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

func buildPullRequestBody(job domain.Job, artifact string, userComment string) string {
	return buildResultBody(job, artifact, userComment) + "\n\nCloses #" + strconv.Itoa(job.Number)
}

func resultTitle(job domain.Job) string {
	switch job.Kind {
	case domain.JobKindIssueDesign:
		return "設計結果"
	case domain.JobKindIssueImplementation:
		return "実装結果"
	case domain.JobKindPRReview:
		return "レビュー結果"
	case domain.JobKindPRFeedback:
		return "レビュー指摘修正結果"
	case domain.JobKindPRConflict:
		return "コンフリクト解消結果"
	default:
		return "結果"
	}
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
