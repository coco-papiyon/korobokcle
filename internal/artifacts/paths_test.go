package artifacts

import (
	"path/filepath"
	"testing"
)

func TestJobDirResolvesRelativeArtifactsDirAgainstRoot(t *testing.T) {
	t.Parallel()

	got := JobDir(filepath.Join("workspace", "tool"), "artifacts", "job-1")
	want := filepath.Join("workspace", "tool", "artifacts", "jobs", "job-1")
	if got != want {
		t.Fatalf("JobDir() = %q, want %q", got, want)
	}
}

func TestJobDirPreservesAbsoluteArtifactsDir(t *testing.T) {
	t.Parallel()

	absoluteArtifactsDir := filepath.Join(t.TempDir(), "artifacts")

	got := JobDir(filepath.Join("workspace", "tool"), absoluteArtifactsDir, "job-1")
	want := filepath.Join(absoluteArtifactsDir, "jobs", "job-1")
	if got != want {
		t.Fatalf("JobDir() = %q, want %q", got, want)
	}
}

func TestRepositoryWorkerPathsUseJobDirs(t *testing.T) {
	t.Parallel()

	root := filepath.Join("workspace", "tool")
	workerDir := RepositoryWorkerDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 2)
	wantWorkerDir := filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "worker-2")
	if workerDir != wantWorkerDir {
		t.Fatalf("RepositoryWorkerDir() = %q, want %q", workerDir, wantWorkerDir)
	}

	jobDir := RepositoryWorkerJobDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 42)
	wantJobDir := filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "jobs", "issue_42")
	if jobDir != wantJobDir {
		t.Fatalf("RepositoryWorkerJobDir() = %q, want %q", jobDir, wantJobDir)
	}

	phaseDir := RepositoryWorkerJobPhaseDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 42, "design")
	wantPhaseDir := filepath.Join(wantJobDir, "design")
	if phaseDir != wantPhaseDir {
		t.Fatalf("RepositoryWorkerJobPhaseDir() = %q, want %q", phaseDir, wantPhaseDir)
	}

	workArtifactDir := RepositoryWorkerWorkArtifactDir(filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "work"), "design")
	wantWorkArtifactDir := filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "work", "design")
	if workArtifactDir != wantWorkArtifactDir {
		t.Fatalf("RepositoryWorkerWorkArtifactDir() = %q, want %q", workArtifactDir, wantWorkArtifactDir)
	}

	workArtifactFile := RepositoryWorkerWorkArtifactFileName(42, "設計結果 / draft")
	wantWorkArtifactFile := "issue_42_設計結果 - draft.md"
	if workArtifactFile != wantWorkArtifactFile {
		t.Fatalf("RepositoryWorkerWorkArtifactFileName() = %q, want %q", workArtifactFile, wantWorkArtifactFile)
	}

	workArtifactPath := RepositoryWorkerWorkArtifactPath(filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "work"), "design", 42, "設計結果 / draft")
	wantWorkArtifactPath := filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "work", "design", wantWorkArtifactFile)
	if workArtifactPath != wantWorkArtifactPath {
		t.Fatalf("RepositoryWorkerWorkArtifactPath() = %q, want %q", workArtifactPath, wantWorkArtifactPath)
	}
}
