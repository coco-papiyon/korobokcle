package app

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func copyAIResultToWorkDir(workDir string, phase string, job domain.Job, artifactDir string) error {
	raw, err := readFirstArtifactFile(artifactDir, "result.md")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	targetPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, phase, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, raw, 0o644)
}

func readPreferredWorkingArtifact(workDir string, phase string, job domain.Job, fallbackDir string, names ...string) ([]byte, error) {
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, phase, job.GitHubNumber, job.Title)
	if raw, err := os.ReadFile(workingPath); err == nil {
		return raw, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return readFirstArtifactFile(fallbackDir, names...)
}
