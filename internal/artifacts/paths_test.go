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

func TestRepositoryWorkerPathsUseWorkerRootAndWorkspace(t *testing.T) {
	t.Parallel()

	root := filepath.Join("workspace", "tool")
	workerDir := RepositoryWorkerDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 2)
	wantWorkerDir := filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "worker-2")
	if workerDir != wantWorkerDir {
		t.Fatalf("RepositoryWorkerDir() = %q, want %q", workerDir, wantWorkerDir)
	}

	workspaceDir := RepositoryWorkerWorkspaceDir(workerDir, ".workspace")
	wantWorkspaceDir := filepath.Join(wantWorkerDir, ".workspace")
	if workspaceDir != wantWorkspaceDir {
		t.Fatalf("RepositoryWorkerWorkspaceDir() = %q, want %q", workspaceDir, wantWorkspaceDir)
	}

	issueDir := RepositoryWorkerIssueDir(workerDir, ".workspace", 42)
	wantIssueDir := filepath.Join(wantWorkspaceDir, "issue_42")
	if issueDir != wantIssueDir {
		t.Fatalf("RepositoryWorkerIssueDir() = %q, want %q", issueDir, wantIssueDir)
	}

	artifactDir := RepositoryWorkerArtifactDir(workerDir, ".workspace", 42, "design")
	wantArtifactDir := filepath.Join(wantIssueDir, "design")
	if artifactDir != wantArtifactDir {
		t.Fatalf("RepositoryWorkerArtifactDir() = %q, want %q", artifactDir, wantArtifactDir)
	}
}
