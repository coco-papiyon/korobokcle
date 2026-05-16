package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDesignWritesArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: design\nartifacts:\n  output_file: design.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("Title: {{ .Title }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}

	runner := NewRunner(root, "mock")
	artifactDir := filepath.Join(root, "artifacts", "designs", "job-1")
	_, err := runner.RunDesign(context.Background(), "design", DesignContext{
		Title:       "My Issue",
		ArtifactDir: artifactDir,
	})
	if err != nil {
		t.Fatalf("RunDesign() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(artifactDir, "ai-stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile(ai-stdout.log) error = %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "mock provider executed") {
		t.Fatalf("expected mock provider stdout, got %q", content)
	}
}

func TestRunDesignUsesAppProviderWhenConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: design\nprovider: mock\nartifacts:\n  output_file: design.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("Title: {{ .Title }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}

	runner := NewRunner(root, "mock")
	artifactDir := filepath.Join(root, "artifacts", "designs", "job-2")
	_, err := runner.RunDesign(context.Background(), "design", DesignContext{
		Title:       "My Issue",
		ArtifactDir: artifactDir,
	})
	if err != nil {
		t.Fatalf("RunDesign() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(artifactDir, "ai-stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile(ai-stdout.log) error = %v", err)
	}
	if !strings.Contains(string(raw), "mock provider executed") {
		t.Fatalf("expected mock provider stdout, got %q", string(raw))
	}
}
