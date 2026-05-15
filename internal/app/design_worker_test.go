package app

import (
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestResolveDesignSkillNameDefault(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "default"},
			},
		},
	})

	got, err := resolveDesignSkillName(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveDesignSkillName() error = %v", err)
	}
	if got != "design" {
		t.Fatalf("expected design, got %q", got)
	}
}

func TestResolveDesignSkillNameFromSkillSet(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "team-a"},
			},
		},
	})

	got, err := resolveDesignSkillName(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveDesignSkillName() error = %v", err)
	}
	if got != "team-a/design" {
		t.Fatalf("expected team-a/design, got %q", got)
	}
}
