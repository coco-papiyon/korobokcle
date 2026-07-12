package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func ensureJobWorktree(ctx context.Context, baseDir, toolDir string, logger workflowLogger, job domain.Job, branch, baseBranch string, prepareMerge bool) (string, string, error) {
	worktreeBranch := strings.TrimSpace(branch)
	if worktreeBranch == "" {
		return "", "", fmt.Errorf("branch name is required")
	}
	worktreePath := implementationWorktreePath(toolDir, job)
	worktreeNote := ""
	if toolDir != "" {
		worktreeNote = "worktree=" + relativePathForLog(toolDir, worktreePath)
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return "", "", fmt.Errorf("create worktree parent: %w", err)
	}
	if _, err := os.Stat(filepath.Join(worktreePath, ".git")); err == nil {
		if prepareMerge && mergeInProgressLogged(ctx, logger, worktreePath, worktreeNote) {
			return worktreePath, branch, nil
		}
		currentBranchName, currentErr := currentBranchLogged(ctx, logger, worktreePath, worktreeNote)
		if currentErr == nil && strings.TrimSpace(currentBranchName) != "" {
			dirty, dirtyErr := gitHasChangesLogged(ctx, logger, worktreePath, worktreeNote)
			if dirtyErr != nil {
				return "", "", dirtyErr
			}
			if dirty {
				if logger != nil {
					logger.Infof("workflow reuse dirty worktree job=%s path=%s branch=%s", job.ID, worktreePath, currentBranchName)
				}
				return worktreePath, currentBranchName, nil
			}
			if err := syncBranchFromRemoteLogged(ctx, logger, worktreePath, worktreeNote, currentBranchName); err != nil {
				return "", "", err
			}
			if prepareMerge {
				if err := prepareConflictMergeLogged(ctx, logger, worktreePath, worktreeNote, baseBranch); err != nil {
					return "", "", err
				}
			}
			return worktreePath, currentBranchName, nil
		}
		if err := syncBranchFromRemoteLogged(ctx, logger, worktreePath, worktreeNote, worktreeBranch); err != nil {
			return "", "", err
		}
		return worktreePath, worktreeBranch, nil
	}
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if pruneErr := runGitLogged(ctx, logger, baseDir, "", "worktree", "prune"); pruneErr != nil {
			return "", "", fmt.Errorf("prune stale worktrees: %w", pruneErr)
		}
	}
	if err := addImplementationWorktreeLogged(ctx, logger, baseDir, worktreeBranch, worktreePath); err != nil {
		if !strings.Contains(err.Error(), "already used by worktree") {
			return "", "", fmt.Errorf("create worktree: %w", err)
		}
		worktreeBranch = implementationWorktreeBranchName(branch, job)
		if retryErr := addImplementationWorktreeLogged(ctx, logger, baseDir, worktreeBranch, worktreePath); retryErr != nil {
			return "", "", fmt.Errorf("create worktree: %w", retryErr)
		}
	}
	if err := syncBranchFromRemoteLogged(ctx, logger, worktreePath, worktreeNote, worktreeBranch); err != nil {
		return "", "", err
	}
	if prepareMerge {
		if err := prepareConflictMergeLogged(ctx, logger, worktreePath, worktreeNote, baseBranch); err != nil {
			return "", "", err
		}
	}
	return worktreePath, worktreeBranch, nil
}
