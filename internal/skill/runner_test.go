package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
)

func TestRunDesignWritesArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: design\nartifacts:\n  output_file: result.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("Title: {{ .Title }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}

	runner := NewRunner(root, "mock")
	artifactDir := artifacts.WorkerDir(root, "artifacts", "job-1", artifacts.WorkerDesign)
	_, err := runner.RunDesign(context.Background(), "design", DesignContext{
		Title:       "My Issue",
		ArtifactDir: artifactDir,
	}, ExecutionConfig{})
	if err != nil {
		t.Fatalf("RunDesign() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(artifactDir, "stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile(stdout.log) error = %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "mock provider executed") {
		t.Fatalf("expected mock provider stdout, got %q", content)
	}
}

func TestRunDesignUsesAppProviderWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: design\nprovider: mock\nartifacts:\n  output_file: result.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("Title: {{ .Title }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}

	runner := NewRunner(root, "mock")
	artifactDir := artifacts.WorkerDir(root, "artifacts", "job-2", artifacts.WorkerDesign)
	_, err := runner.RunDesign(context.Background(), "design", DesignContext{
		Title:       "My Issue",
		ArtifactDir: artifactDir,
	}, ExecutionConfig{})
	if err != nil {
		t.Fatalf("RunDesign() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(artifactDir, "stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile(stdout.log) error = %v", err)
	}
	if !strings.Contains(string(raw), "mock provider executed") {
		t.Fatalf("expected mock provider stdout, got %q", string(raw))
	}
}

func TestRunImplementationUsesArtifactDirAsWorkDir(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "implement")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: implement\nartifacts:\n  output_file: result.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("Title: {{ .Title }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}

	scriptPath := filepath.Join(root, "cwd-provider.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %cd%\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", `[]`)

	runner := NewRunner(root, "")
	artifactDir := artifacts.WorkerDir(root, "artifacts", "job-1", artifacts.WorkerImplementation)
	result, err := runner.RunImplementation(context.Background(), "implement", ImplementationContext{
		Title:             "My Issue",
		ArtifactDir:       artifactDir,
		DesignArtifact:    "approved design",
		DesignArtifactDir: artifacts.WorkerDir(root, "artifacts", "job-1", artifacts.WorkerDesign),
	}, ExecutionConfig{Provider: "codex"})
	if err != nil {
		t.Fatalf("RunImplementation() error = %v", err)
	}

	if got := strings.TrimSpace(result.Output); got != artifactDir {
		t.Fatalf("expected work dir %q, got %q", artifactDir, got)
	}
}
