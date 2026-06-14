package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestLoadRepositoryImprovementInstructionsFiltersByPhaseAndStatus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, "artifacts", "owner-repo", "")
	improvementsDir := artifacts.RepositoryWorkerImprovementsDir(workDir, ".improvement")
	if err := os.MkdirAll(improvementsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(improvementsDir) error = %v", err)
	}

	activeDesign := ImprovementDocument{
		FrontMatter: ImprovementFrontMatter{
			ID:        "design-policy",
			Title:     "Design policy",
			Scope:     "repository",
			Phases:    []string{"design"},
			Status:    "active",
			UpdatedAt: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		},
		Body: "- Keep the design doc concise.",
	}
	rawActiveDesign, err := activeDesign.MarshalMarkdown()
	if err != nil {
		t.Fatalf("MarshalMarkdown() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(improvementsDir, "design-policy.md"), rawActiveDesign, 0o644); err != nil {
		t.Fatalf("WriteFile(design-policy.md) error = %v", err)
	}

	activeFix := ImprovementDocument{
		FrontMatter: ImprovementFrontMatter{
			ID:        "fix-policy",
			Title:     "Fix policy",
			Scope:     "repository",
			Phases:    []string{"fix"},
			Status:    "active",
			UpdatedAt: time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC),
		},
		Body: "- Keep fixes small.",
	}
	rawActiveFix, err := activeFix.MarshalMarkdown()
	if err != nil {
		t.Fatalf("MarshalMarkdown() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(improvementsDir, "fix-policy.md"), rawActiveFix, 0o644); err != nil {
		t.Fatalf("WriteFile(fix-policy.md) error = %v", err)
	}

	inactiveReview := ImprovementDocument{
		FrontMatter: ImprovementFrontMatter{
			ID:        "review-policy",
			Title:     "Review policy",
			Scope:     "repository",
			Phases:    []string{"review"},
			Status:    "inactive",
			UpdatedAt: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
		},
		Body: "- Keep reviews focused.",
	}
	rawInactiveReview, err := inactiveReview.MarshalMarkdown()
	if err != nil {
		t.Fatalf("MarshalMarkdown() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(improvementsDir, "review-policy.md"), rawInactiveReview, 0o644); err != nil {
		t.Fatalf("WriteFile(review-policy.md) error = %v", err)
	}

	svc := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:         "owner/repository",
				ImprovementEnabled: true,
				ImprovementDir:     ".improvement",
			}},
		},
	})

	designInstructions, err := loadRepositoryImprovementInstructions(svc, workDir, "owner/repository", "design")
	if err != nil {
		t.Fatalf("loadRepositoryImprovementInstructions(design) error = %v", err)
	}
	if len(designInstructions) != 1 || designInstructions[0].ID != "design-policy" {
		t.Fatalf("unexpected design instructions: %#v", designInstructions)
	}

	fixInstructions, err := loadRepositoryImprovementInstructions(svc, workDir, "owner/repository", "review_fix")
	if err != nil {
		t.Fatalf("loadRepositoryImprovementInstructions(fix) error = %v", err)
	}
	if len(fixInstructions) != 1 || fixInstructions[0].ID != "fix-policy" {
		t.Fatalf("unexpected fix instructions: %#v", fixInstructions)
	}
}
