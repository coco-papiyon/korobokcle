package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestBuildFixturesIncludeImprovementScenarios(t *testing.T) {
	t.Parallel()

	fixtures := buildFixtures("artifacts")
	byJobID := make(map[string]jobFixture, len(fixtures))
	for _, fixture := range fixtures {
		byJobID[fixture.job.ID] = fixture
	}

	draftFixture, ok := byJobID["fixture-pr-created"]
	if !ok {
		t.Fatalf("fixture-pr-created not found")
	}
	assertArtifactExists(t, draftFixture.artifacts, artifacts.WorkerImprovement, "decision.json", `"decision":"draft_created"`)
	assertWorkspaceFileExists(t, draftFixture.workspaceFiles, filepath.Join(".improvement", "draft", "draft.md"))

	approvedFixture, ok := byJobID["fixture-pr-comment-analysis-ready"]
	if !ok {
		t.Fatalf("fixture-pr-comment-analysis-ready not found")
	}
	assertArtifactExists(t, approvedFixture.artifacts, artifacts.WorkerImprovement, "approval.json", `"status":"approved"`)
	assertWorkspaceFileExists(t, approvedFixture.workspaceFiles, filepath.Join(".improvements", "複雑な条件分岐は-helper-に抽出する.md"))

	noImprovementFixture, ok := byJobID["fixture-failed"]
	if !ok {
		t.Fatalf("fixture-failed not found")
	}
	assertArtifactExists(t, noImprovementFixture.artifacts, artifacts.WorkerImprovement, "decision.json", `"decision":"no_improvement_needed"`)
}

func TestWriteRepositoryArtifactsWritesImprovementWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repository := "owner/repository"
	workDirSetting := "artifacts/owner-repository/workspace"
	workspaceFiles := []workspaceFile{
		{path: filepath.Join(".improvement", "draft", "draft.md"), content: "draft content"},
		{path: filepath.Join(".improvements", "rule.md"), content: "approved content"},
	}

	if err := writeRepositoryArtifacts(root, "artifacts", workDirSetting, repository, 42, "Issue", nil, workspaceFiles); err != nil {
		t.Fatalf("writeRepositoryArtifacts() error = %v", err)
	}

	workDir := artifacts.RepositoryWorkerWorkDir(root, "artifacts", repository, workDirSetting)
	for _, relativePath := range []string{
		filepath.Join(".improvement", "draft", "draft.md"),
		filepath.Join(".improvements", "rule.md"),
	} {
		path := filepath.Join(workDir, relativePath)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected workspace file %s: %v", path, err)
		}
	}
}

func TestDefaultFixtureConfigEnablesImprovementFeature(t *testing.T) {
	t.Parallel()

	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{
			Repository:         "coco-papiyon/korobokcle",
			ImprovementEnabled: true,
			ImprovementBranch:  "improvement",
			ImprovementDir:     ".improvements",
			ImprovementWorkDir: ".improvement",
		},
	}

	repository := files.App.MonitoredRepositories[0]
	if !repository.ImprovementEnabled {
		t.Fatalf("expected improvement feature enabled")
	}
	if repository.ImprovementBranch != "improvement" || repository.ImprovementDir != ".improvements" || repository.ImprovementWorkDir != ".improvement" {
		t.Fatalf("unexpected improvement config: %+v", repository)
	}
}

func assertArtifactExists(t *testing.T, files []artifactFile, worker string, name string, contentSubstring string) {
	t.Helper()
	for _, file := range files {
		if file.worker == worker && file.name == name {
			if contentSubstring != "" && !strings.Contains(file.content, contentSubstring) {
				t.Fatalf("artifact %s/%s missing %q in %q", worker, name, contentSubstring, file.content)
			}
			return
		}
	}
	t.Fatalf("artifact %s/%s not found", worker, name)
}

func assertWorkspaceFileExists(t *testing.T, files []workspaceFile, path string) {
	t.Helper()
	for _, file := range files {
		if file.path == path {
			return
		}
	}
	t.Fatalf("workspace file %s not found", path)
}
