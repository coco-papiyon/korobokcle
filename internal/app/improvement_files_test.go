package app

import (
	"path/filepath"
	"testing"
)

func TestRepositoryImprovementWorkFilesUseConfiguredDir(t *testing.T) {
	t.Parallel()

	workDir := filepath.Join("workspace", "tool", "artifacts", "owner-repo", "workspace")
	files := repositoryImprovementWorkFiles(workDir, ".draft-improvements")

	if files.Dir != filepath.Join(workDir, ".draft-improvements") {
		t.Fatalf("unexpected dir: %q", files.Dir)
	}
	if files.InputPath != filepath.Join(workDir, ".draft-improvements", "input.md") {
		t.Fatalf("unexpected input path: %q", files.InputPath)
	}
	if files.ContextPath != filepath.Join(workDir, ".draft-improvements", "context.json") {
		t.Fatalf("unexpected context path: %q", files.ContextPath)
	}
	if files.DraftDir != filepath.Join(workDir, ".draft-improvements", "draft") {
		t.Fatalf("unexpected draft dir: %q", files.DraftDir)
	}
	if files.DraftPath != filepath.Join(workDir, ".draft-improvements", "draft", "draft.md") {
		t.Fatalf("unexpected draft path: %q", files.DraftPath)
	}
	if files.NotesPath != filepath.Join(workDir, ".draft-improvements", "notes.md") {
		t.Fatalf("unexpected notes path: %q", files.NotesPath)
	}
	if files.ImplementationPromptPath != filepath.Join(workDir, ".draft-improvements", "implementation-prompt.md") {
		t.Fatalf("unexpected implementation prompt path: %q", files.ImplementationPromptPath)
	}
}

func TestRepositoryImprovementArtifactFilesUseImprovementDir(t *testing.T) {
	t.Parallel()

	root := filepath.Join("workspace", "tool")
	files := repositoryImprovementArtifactFiles(root, "artifacts", "owner/repo", 42)
	wantDir := filepath.Join(root, "artifacts", "owner-repo", "jobs", "issue_42", "improvement")

	if files.Dir != wantDir {
		t.Fatalf("unexpected dir: %q", files.Dir)
	}
	if files.InputPath != filepath.Join(wantDir, "input.md") {
		t.Fatalf("unexpected input path: %q", files.InputPath)
	}
	if files.ContextPath != filepath.Join(wantDir, "context.json") {
		t.Fatalf("unexpected context path: %q", files.ContextPath)
	}
	if files.DraftDir != filepath.Join(wantDir, "draft") {
		t.Fatalf("unexpected draft dir: %q", files.DraftDir)
	}
	if files.DraftPath != filepath.Join(wantDir, "draft", "draft.md") {
		t.Fatalf("unexpected draft path: %q", files.DraftPath)
	}
	if files.NotesPath != filepath.Join(wantDir, "notes.md") {
		t.Fatalf("unexpected notes path: %q", files.NotesPath)
	}
	if files.ImplementationPromptPath != filepath.Join(wantDir, "implementation-prompt.md") {
		t.Fatalf("unexpected implementation prompt path: %q", files.ImplementationPromptPath)
	}
	if files.ResultPath != filepath.Join(wantDir, "result.md") {
		t.Fatalf("unexpected result path: %q", files.ResultPath)
	}
	if files.ApprovalPath != filepath.Join(wantDir, "approval.json") {
		t.Fatalf("unexpected approval path: %q", files.ApprovalPath)
	}
	if files.DecisionPath != filepath.Join(wantDir, "decision.json") {
		t.Fatalf("unexpected decision path: %q", files.DecisionPath)
	}
}
