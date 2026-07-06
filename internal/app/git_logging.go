package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type commandDebugLogger interface {
	Debugf(string, ...any)
}

func relativePathForLog(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func gitLogPrefix(baseDir string, note string) string {
	if strings.TrimSpace(note) == "" {
		return fmt.Sprintf("git -C %s", baseDir)
	}
	return fmt.Sprintf("git -C %s (%s)", baseDir, note)
}

func runGHLogged(ctx context.Context, logger commandDebugLogger, args ...string) error {
	if logger != nil {
		logger.Debugf("gh %s", strings.Join(args, " "))
	}
	if err := runGH(ctx, args...); err != nil {
		if logger != nil {
			logger.Debugf("gh failed: %s: %v", strings.Join(args, " "), err)
		}
		return err
	}
	return nil
}

func currentBranchLogged(ctx context.Context, logger commandDebugLogger, baseDir string, note string) (string, error) {
	if logger != nil {
		logger.Debugf("%s branch --show-current", gitLogPrefix(baseDir, note))
	}
	return currentBranch(ctx, baseDir)
}

func checkoutOrCreateBranchLogged(ctx context.Context, logger commandDebugLogger, baseDir, note, branch string) error {
	if logger != nil {
		logger.Debugf("%s check-ref-format --branch %s", gitLogPrefix(baseDir, note), branch)
	}
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	if err := runGitLogged(ctx, logger, baseDir, note, "check-ref-format", "--branch", branch); err != nil {
		return err
	}
	if logger != nil {
		logger.Debugf("%s rev-parse --verify --quiet refs/heads/%s", gitLogPrefix(baseDir, note), branch)
	}
	if err := runGit(ctx, baseDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return runGitLogged(ctx, logger, baseDir, note, "checkout", branch)
	}
	return runGitLogged(ctx, logger, baseDir, note, "checkout", "-b", branch)
}

func ensureBranchHasCommitLogged(ctx context.Context, logger commandDebugLogger, baseDir, note, branch string) error {
	if logger != nil {
		logger.Debugf("%s rev-list --count main..%s", gitLogPrefix(baseDir, note), branch)
	}
	count, err := gitCommitCount(ctx, baseDir, "main.."+branch)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return runGitLogged(ctx, logger, baseDir, note, "commit", "--allow-empty", "-m", "chore: prepare PR for "+branch)
}

func publishBranchLogged(ctx context.Context, logger commandDebugLogger, baseDir, note, localBranch, remoteBranch string) error {
	localBranch = strings.TrimSpace(localBranch)
	if localBranch == "" {
		return fmt.Errorf("branch name is required")
	}
	remoteBranch = strings.TrimSpace(remoteBranch)
	if remoteBranch == "" {
		remoteBranch = localBranch
	}
	exists, err := remoteBranchExistsLogged(ctx, logger, baseDir, note, remoteBranch)
	if err != nil {
		return err
	}
	if exists {
		if err := runGitLogged(ctx, logger, baseDir, note, "pull", "--rebase", "origin", remoteBranch); err != nil {
			return fmt.Errorf("rebase remote branch before push: %w", err)
		}
	}
	return runGitLogged(ctx, logger, baseDir, note, "push", "-u", "origin", localBranch+":"+remoteBranch)
}

func remoteBranchExistsLogged(ctx context.Context, logger commandDebugLogger, baseDir, note, branch string) (bool, error) {
	if logger != nil {
		logger.Debugf("%s ls-remote --exit-code --heads origin %s", gitLogPrefix(baseDir, note), branch)
	}
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

func stageAndCommitIfNeededLogged(ctx context.Context, logger commandDebugLogger, repoDir, note string, message string) error {
	dirty, err := gitHasChangesLogged(ctx, logger, repoDir, note)
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	if err := runGitLogged(ctx, logger, repoDir, note, "add", "-A"); err != nil {
		return err
	}
	return runGitLogged(ctx, logger, repoDir, note, "commit", "-m", message)
}

func gitHasChangesLogged(ctx context.Context, logger commandDebugLogger, repoDir, note string) (bool, error) {
	if logger != nil {
		logger.Debugf("%s status --porcelain", gitLogPrefix(repoDir, note))
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status --porcelain: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func gitWorkingTreeDiffLogged(ctx context.Context, logger commandDebugLogger, repoDir, note string) (string, error) {
	cached, err := runGitOutputLogged(ctx, logger, repoDir, note, "diff", "--cached", "--no-ext-diff", "-U4")
	if err != nil {
		return "", err
	}
	working, err := runGitOutputLogged(ctx, logger, repoDir, note, "diff", "--no-ext-diff", "-U4")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Join([]string{strings.TrimSpace(cached), strings.TrimSpace(working)}, "\n\n")), nil
}

func gitDiffAgainstBaseLogged(ctx context.Context, logger commandDebugLogger, repoDir, note, baseBranch string) (string, error) {
	candidates := []string{
		strings.TrimSpace(baseBranch),
		"origin/" + strings.TrimSpace(baseBranch),
	}
	var lastErr error
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		diff, err := runGitOutputLogged(ctx, logger, repoDir, note, "diff", "--no-ext-diff", "-U4", candidate+"...HEAD")
		if err == nil {
			return strings.TrimSpace(diff), nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", nil
}

func runGitLogged(ctx context.Context, logger commandDebugLogger, baseDir, note string, args ...string) error {
	if logger != nil {
		logger.Debugf("%s %s", gitLogPrefix(baseDir, note), strings.Join(args, " "))
	}
	if err := runGit(ctx, baseDir, args...); err != nil {
		if logger != nil {
			logger.Debugf("git failed: %s %s: %v", gitLogPrefix(baseDir, note), strings.Join(args, " "), err)
		}
		return err
	}
	return nil
}

func runGitOutputLogged(ctx context.Context, logger commandDebugLogger, baseDir, note string, args ...string) (string, error) {
	if logger != nil {
		logger.Debugf("%s %s", gitLogPrefix(baseDir, note), strings.Join(args, " "))
	}
	out, err := runGitOutput(ctx, baseDir, args...)
	if err != nil && logger != nil {
		logger.Debugf("git failed: %s %s: %v", gitLogPrefix(baseDir, note), strings.Join(args, " "), err)
	}
	return out, err
}

func gitCommitCountLogged(ctx context.Context, logger commandDebugLogger, baseDir, note, revRange string) (int, error) {
	if logger != nil {
		logger.Debugf("%s rev-list --count %s", gitLogPrefix(baseDir, note), revRange)
	}
	count, err := gitCommitCount(ctx, baseDir, revRange)
	if err != nil && logger != nil {
		logger.Debugf("git failed: %s rev-list --count %s: %v", gitLogPrefix(baseDir, note), revRange, err)
	}
	return count, err
}

func ensureGHLabelLogged(ctx context.Context, logger commandDebugLogger, repository, label string) error {
	if logger != nil {
		logger.Debugf("gh label create %s --repo %s --color 0E8A16 --description korobokcle state label --force", label, repository)
	}
	return ensureGHLabel(ctx, repository, label)
}

func currentTargetLabelsLogged(ctx context.Context, logger commandDebugLogger, job domain.Job) ([]string, error) {
	args := []string{}
	switch domain.ResultCommentTarget(job.Kind) {
	case "pr":
		args = append(args, "pr", "view")
	default:
		args = append(args, "issue", "view")
	}
	args = append(args, "--repo", job.Repository, strconv.Itoa(job.Number), "--json", "labels")
	if logger != nil {
		logger.Debugf("gh %s", strings.Join(args, " "))
	}
	return currentTargetLabels(ctx, job)
}
