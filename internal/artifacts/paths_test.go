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
	wantWorkerDir := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "workers", "worker-2")
	if workerDir != wantWorkerDir {
		t.Fatalf("RepositoryWorkerDir() = %q, want %q", workerDir, wantWorkerDir)
	}

	jobDir := RepositoryWorkerJobDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 42)
	wantJobDir := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "jobs", "issue_42")
	if jobDir != wantJobDir {
		t.Fatalf("RepositoryWorkerJobDir() = %q, want %q", jobDir, wantJobDir)
	}

	phaseDir := RepositoryWorkerJobPhaseDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 42, "design")
	wantPhaseDir := filepath.Join(wantJobDir, "design")
	if phaseDir != wantPhaseDir {
		t.Fatalf("RepositoryWorkerJobPhaseDir() = %q, want %q", phaseDir, wantPhaseDir)
	}

	workArtifactDir := RepositoryWorkerWorkArtifactDir(filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "workspace"), "design")
	wantWorkArtifactDir := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "workspace", "design")
	if workArtifactDir != wantWorkArtifactDir {
		t.Fatalf("RepositoryWorkerWorkArtifactDir() = %q, want %q", workArtifactDir, wantWorkArtifactDir)
	}

	workArtifactFile := RepositoryWorkerWorkArtifactFileName(42, "設計結果 / draft")
	wantWorkArtifactFile := "issue_42_設計結果 - draft.md"
	if workArtifactFile != wantWorkArtifactFile {
		t.Fatalf("RepositoryWorkerWorkArtifactFileName() = %q, want %q", workArtifactFile, wantWorkArtifactFile)
	}

	workArtifactPath := RepositoryWorkerWorkArtifactPath(filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "workspace"), "design", 42, "設計結果 / draft")
	wantWorkArtifactPath := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "workspace", "design", wantWorkArtifactFile)
	if workArtifactPath != wantWorkArtifactPath {
		t.Fatalf("RepositoryWorkerWorkArtifactPath() = %q, want %q", workArtifactPath, wantWorkArtifactPath)
	}

	improvementWorkspaceDir := RepositoryWorkerImprovementWorkspaceDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git")
	wantImprovementWorkspaceDir := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "improvement")
	if improvementWorkspaceDir != wantImprovementWorkspaceDir {
		t.Fatalf("RepositoryWorkerImprovementWorkspaceDir() = %q, want %q", improvementWorkspaceDir, wantImprovementWorkspaceDir)
	}
}

func TestRepositoryWorkerImprovementPathsUseDefaults(t *testing.T) {
	t.Parallel()

	root := filepath.Join("workspace", "tool")
	workDir := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "workspace")

	improvementsDir := RepositoryWorkerImprovementsDir(workDir, "")
	wantImprovementsDir := filepath.Join(workDir, ".improvements")
	if improvementsDir != wantImprovementsDir {
		t.Fatalf("RepositoryWorkerImprovementsDir() = %q, want %q", improvementsDir, wantImprovementsDir)
	}

	improvementWorkDir := RepositoryWorkerImprovementWorkDir(workDir, "")
	wantImprovementWorkDir := filepath.Join(workDir, ".improvement")
	if improvementWorkDir != wantImprovementWorkDir {
		t.Fatalf("RepositoryWorkerImprovementWorkDir() = %q, want %q", improvementWorkDir, wantImprovementWorkDir)
	}

	improvementPhaseFile := RepositoryWorkerImprovementPhaseFile(workDir, "", "design")
	wantImprovementPhaseFile := filepath.Join(workDir, ".improvement", "design.md")
	if improvementPhaseFile != wantImprovementPhaseFile {
		t.Fatalf("RepositoryWorkerImprovementPhaseFile() = %q, want %q", improvementPhaseFile, wantImprovementPhaseFile)
	}

	improvementArtifactDir := RepositoryWorkerImprovementArtifactDir(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 42)
	wantImprovementArtifactDir := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "jobs", "issue_42", "improvement")
	if improvementArtifactDir != wantImprovementArtifactDir {
		t.Fatalf("RepositoryWorkerImprovementArtifactDir() = %q, want %q", improvementArtifactDir, wantImprovementArtifactDir)
	}
}

func TestRepositoryWorkerImprovementPathsUseConfiguredValues(t *testing.T) {
	t.Parallel()

	workDir := filepath.Join("workspace", "tool", "artifacts", "coco-papiyon-korobokcle", "workspace")

	improvementsDir := RepositoryWorkerImprovementsDir(workDir, ".repo-improvements")
	wantImprovementsDir := filepath.Join(workDir, ".repo-improvements")
	if improvementsDir != wantImprovementsDir {
		t.Fatalf("RepositoryWorkerImprovementsDir() = %q, want %q", improvementsDir, wantImprovementsDir)
	}

	improvementWorkDir := RepositoryWorkerImprovementWorkDir(workDir, ".repo-improvement")
	wantImprovementWorkDir := filepath.Join(workDir, ".repo-improvement")
	if improvementWorkDir != wantImprovementWorkDir {
		t.Fatalf("RepositoryWorkerImprovementWorkDir() = %q, want %q", improvementWorkDir, wantImprovementWorkDir)
	}

	improvementWorkFile := RepositoryWorkerImprovementWorkFile(workDir, ".repo-improvement", "draft.md")
	wantImprovementWorkFile := filepath.Join(workDir, ".repo-improvement", "draft.md")
	if improvementWorkFile != wantImprovementWorkFile {
		t.Fatalf("RepositoryWorkerImprovementWorkFile() = %q, want %q", improvementWorkFile, wantImprovementWorkFile)
	}

	improvementsFile := RepositoryWorkerImprovementsFile(workDir, ".repo-improvements", "ui-layout-policy.md")
	wantImprovementsFile := filepath.Join(workDir, ".repo-improvements", "ui-layout-policy.md")
	if improvementsFile != wantImprovementsFile {
		t.Fatalf("RepositoryWorkerImprovementsFile() = %q, want %q", improvementsFile, wantImprovementsFile)
	}
}
