package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractImplementationPatchFromFencedBlock(t *testing.T) {
	t.Parallel()

	output := "## Summary\n\nChanged files.\n\n## Patch\n\n```diff\ndiff --git a/example.txt b/example.txt\n--- a/example.txt\n+++ b/example.txt\n@@ -1 +1 @@\n-old\n+new\n```\n"

	patch, err := extractImplementationPatch(output)
	if err != nil {
		t.Fatalf("extractImplementationPatch() error = %v", err)
	}
	if !strings.Contains(patch, "diff --git a/example.txt b/example.txt") {
		t.Fatalf("expected diff content, got %q", patch)
	}

	summary := trimImplementationSummary(output)
	if strings.Contains(summary, "diff --git") {
		t.Fatalf("expected summary without patch, got %q", summary)
	}
}

func TestApplyImplementationPatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactDir := filepath.Join(root, "artifacts", "changes", "job-1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	if err := runGitCommand(root, "init"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	targetPath := filepath.Join(root, "example.txt")
	if err := os.WriteFile(targetPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	output := "## Summary\n\nChanged a file.\n\n## Patch\n\n```diff\ndiff --git a/example.txt b/example.txt\n--- a/example.txt\n+++ b/example.txt\n@@ -1 +1 @@\n-old\n+new\n```\n"

	if err := applyImplementationPatch(context.Background(), root, artifactDir, output); err != nil {
		t.Fatalf("applyImplementationPatch() error = %v", err)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(target) error = %v", err)
	}
	if strings.TrimSpace(string(raw)) != "new" {
		t.Fatalf("expected patched file content, got %q", string(raw))
	}

	patchArtifact, err := os.ReadFile(filepath.Join(artifactDir, implementationPatchArtifact))
	if err != nil {
		t.Fatalf("ReadFile(patch artifact) error = %v", err)
	}
	if !strings.Contains(string(patchArtifact), "diff --git a/example.txt b/example.txt") {
		t.Fatalf("expected patch artifact content, got %q", string(patchArtifact))
	}
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	_ = output
	return nil
}
