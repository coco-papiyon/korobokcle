package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestCloneRepositoryWorkspaceClonesLocalRepository(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("clone test"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	workerDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerDir, ".git")); err != nil {
		t.Fatalf("expected cloned git repository: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerDir, "README.md")); err != nil {
		t.Fatalf("expected cloned file: %v", err)
	}
	if workerDir != artifacts.RepositoryWorkerDir(root, cfg.App().ArtifactsDir, source, 0) {
		t.Fatalf("unexpected worker dir: %s", workerDir)
	}
}

func TestJobAssignedToWorkerDeterministic(t *testing.T) {
	t.Parallel()

	job := domain.Job{ID: "issue-owner-repository-1"}
	first := jobAssignedToWorker(job, "owner/repository", 0, 2)
	second := jobAssignedToWorker(job, "owner/repository", 0, 2)
	if first != second {
		t.Fatalf("expected deterministic worker assignment, got %v and %v", first, second)
	}
	other := jobAssignedToWorker(job, "owner/repository", 1, 2)
	if first == other {
		t.Fatalf("expected job to map to a single worker index, got duplicate assignment")
	}
}

func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
	return nil
}
