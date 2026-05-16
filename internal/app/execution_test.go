package app

import (
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestResolveExecutionConfigUsesAppSettingsByDefault(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		App: config.App{
			Provider: "codex",
			Model:    "gpt-4.1",
			Providers: []config.ProviderSpec{
				{
					Name:   "codex",
					Models: []string{"gpt-4.1", "gpt-4o"},
				},
			},
		},
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1"},
			},
		},
	})

	got, err := resolveExecutionConfig(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveExecutionConfig() error = %v", err)
	}
	if got.Provider != "codex" {
		t.Fatalf("expected provider codex, got %q", got.Provider)
	}
	if got.Model != "gpt-4.1" {
		t.Fatalf("expected model gpt-4.1, got %q", got.Model)
	}
}

func TestResolveExecutionConfigUsesWatchRuleOverrides(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		App: config.App{
			Provider: "codex",
			Model:    "gpt-4.1",
			Providers: []config.ProviderSpec{
				{
					Name:   "codex",
					Models: []string{"gpt-4.1", "gpt-4o"},
				},
				{
					Name:   "copilot",
					Models: []string{"gpt-4.1", "gpt-4o"},
				},
			},
		},
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{
					ID:       "rule-1",
					Provider: "copilot",
					Model:    "o4-mini",
				},
			},
		},
	})

	got, err := resolveExecutionConfig(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveExecutionConfig() error = %v", err)
	}
	if got.Provider != "copilot" {
		t.Fatalf("expected provider copilot, got %q", got.Provider)
	}
	if got.Model != "o4-mini" {
		t.Fatalf("expected model o4-mini, got %q", got.Model)
	}
}

func TestResolveExecutionConfigUsesEmptyModelWhenAppModelEmpty(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		App: config.App{
			Provider: "codex",
			Providers: []config.ProviderSpec{
				{
					Name:   "codex",
					Models: []string{"gpt-4.1", "gpt-4o"},
				},
			},
		},
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1"},
			},
		},
	})

	got, err := resolveExecutionConfig(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveExecutionConfig() error = %v", err)
	}
	if got.Model != "" {
		t.Fatalf("expected empty model, got %q", got.Model)
	}
}
