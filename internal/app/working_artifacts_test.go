package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestCopyAIResultToWorkDirCopiesResultMarkdown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	resultDir := filepath.Join(root, "artifacts", "workers", "owner-repository", "jobs", "issue_42", artifacts.WorkerDesign)
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(resultDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(resultDir, "result.md"), []byte("design result"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 42, Title: "設計結果 / draft"}
	if err := copyAIResultToWorkDir(workDir, artifacts.WorkerDesign, job, resultDir); err != nil {
		t.Fatalf("copyAIResultToWorkDir() error = %v", err)
	}

	gotPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerDesign, job.GitHubNumber, job.Title)
	raw, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("ReadFile(gotPath) error = %v", err)
	}
	if string(raw) != "design result" {
		t.Fatalf("expected copied content, got %q", string(raw))
	}
}

func TestReadPreferredWorkingArtifactPrefersWorkingCopy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	phase := artifacts.WorkerImplementation
	job := domain.Job{Repository: "owner/repository", GitHubNumber: 42, Title: "実装済み"}
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, phase, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(workingPath dir) error = %v", err)
	}
	if err := os.WriteFile(workingPath, []byte("working copy"), 0o644); err != nil {
		t.Fatalf("WriteFile(workingPath) error = %v", err)
	}

	fallbackDir := filepath.Join(root, "artifacts", "workers", "owner-repository", "jobs", "issue_42", phase)
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fallbackDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fallbackDir, "result.md"), []byte("fallback"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	raw, err := readPreferredWorkingArtifact(workDir, phase, job, fallbackDir, "result.md")
	if err != nil {
		t.Fatalf("readPreferredWorkingArtifact() error = %v", err)
	}
	if string(raw) != "working copy" {
		t.Fatalf("expected working copy to win, got %q", string(raw))
	}
}

func TestCopyAIResultToWorkDirSkipsMissingArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	artifactDir := filepath.Join(root, "artifacts", "workers", "owner-repository", "jobs", "issue_42", artifacts.WorkerDesign)
	job := domain.Job{Repository: "owner/repository", GitHubNumber: 42, Title: "設計結果 / draft"}

	if err := copyAIResultToWorkDir(workDir, artifacts.WorkerDesign, job, artifactDir); err != nil {
		t.Fatalf("copyAIResultToWorkDir() error = %v", err)
	}

	gotPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerDesign, job.GitHubNumber, job.Title)
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Fatalf("expected no working file to be created, got err=%v", err)
	}
}

func TestReadPreferredWorkingArtifactFallsBackToArtifactDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	phase := artifacts.WorkerImplementation
	job := domain.Job{Repository: "owner/repository", GitHubNumber: 42, Title: "実装済み"}
	fallbackDir := filepath.Join(root, "artifacts", "workers", "owner-repository", "jobs", "issue_42", phase)
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fallbackDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fallbackDir, "result.md"), []byte("fallback"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	raw, err := readPreferredWorkingArtifact(workDir, phase, job, fallbackDir, "result.md")
	if err != nil {
		t.Fatalf("readPreferredWorkingArtifact() error = %v", err)
	}
	if string(raw) != "fallback" {
		t.Fatalf("expected fallback artifact to be returned, got %q", string(raw))
	}
}
