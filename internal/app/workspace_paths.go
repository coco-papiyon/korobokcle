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

func implementationWorktreeBranchName(branch string, job domain.Job) string {
	base := strings.TrimSpace(branch)
	if base == "" {
		base = "issue"
	}
	return base + "__" + sanitizePart(job.ID)
}
