package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestLoadImprovementDetailSkipsIssueWithoutImprovementArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{
			Repository:         "owner/repository",
			ImprovementEnabled: true,
		},
	}
	s := &Server{config: config.NewService(root, files)}

	emptyIssueDir := artifacts.RepositoryWorkerJobDir(root, files.App.ArtifactsDir, "owner/repository", 101)
	if err := os.MkdirAll(emptyIssueDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if _, err := s.loadImprovementDetail("owner/repository", 101); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestApproveImprovementSupportsNoImprovementNeeded(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{
			Repository:         "owner/repository",
			ImprovementEnabled: true,
		},
	}
	s := &Server{config: config.NewService(root, files)}

	improvementDir := filepath.Join(artifacts.RepositoryWorkerJobDir(root, files.App.ArtifactsDir, "owner/repository", 102), "improvement")
	if err := os.MkdirAll(improvementDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(improvementDir, "input.md"), []byte("temporary request"), 0o644); err != nil {
		t.Fatalf("WriteFile(input.md) error = %v", err)
	}

	if err := s.approveImprovement("owner/repository", 102, "no_improvement_needed", "恒久改善は不要"); err != nil {
		t.Fatalf("approveImprovement() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(improvementDir, "decision.json"))
	if err != nil {
		t.Fatalf("ReadFile(decision.json) error = %v", err)
	}
	if string(raw) == "" || !containsAll(string(raw), "no_improvement_needed", "恒久改善は不要") {
		t.Fatalf("unexpected decision.json: %q", string(raw))
	}
}

func containsAll(text string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			return false
		}
	}
	return true
}
