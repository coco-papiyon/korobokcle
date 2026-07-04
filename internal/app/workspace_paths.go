package app

import (
	"path/filepath"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func implementationWorktreePath(workDir string, job domain.Job) string {
	repoDir := sanitizePart(strings.ReplaceAll(job.Repository, "/", "_"))
	return filepath.Join(workDir, "workspace", repoDir, job.ID, "worktree")
}

func jobWorkspaceDir(workDir string, job domain.Job) string {
	repoDir := sanitizePart(strings.ReplaceAll(job.Repository, "/", "_"))
	return filepath.Join(workDir, "workspace", repoDir, job.ID)
}

func jobLogDir(workDir string, job domain.Job) string {
	return filepath.Join(jobWorkspaceDir(workDir, job), "logs")
}

func implementationWorktreeBranchName(branch string, job domain.Job) string {
	base := strings.TrimSpace(branch)
	if base == "" {
		base = "issue"
	}
	return base + "__" + sanitizePart(job.ID)
}
