package app

import (
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestResolveConditionAISelection(t *testing.T) {
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		AIProvider: domain.AIProviderGitHubCopilot,
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.5"},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "claude-opus-4.6"},
		},
	})

	provider, model := resolveConditionAISelection(settings, domain.SearchCondition{})
	if provider != domain.AIProviderGitHubCopilot {
		t.Fatalf("provider = %q, want %q", provider, domain.AIProviderGitHubCopilot)
	}
	if model != "claude-opus-4.6" {
		t.Fatalf("model = %q, want claude-opus-4.6", model)
	}

	override := domain.SearchCondition{
		AIProvider: domain.AIProviderCodex,
		AIModel:    domain.ModelSelection{Mode: domain.ModelModeDefault},
	}
	provider, model = resolveConditionAISelection(settings, override)
	if provider != domain.AIProviderCodex {
		t.Fatalf("override provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "gpt-5.5" {
		t.Fatalf("override model = %q, want gpt-5.5", model)
	}

	override.AIModel = domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "custom-1"}
	provider, model = resolveConditionAISelection(settings, override)
	if provider != domain.AIProviderCodex {
		t.Fatalf("custom provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "custom-1" {
		t.Fatalf("custom model = %q, want custom-1", model)
	}
}

func TestResolveJobAISelection(t *testing.T) {
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		AIProvider: domain.AIProviderCodex,
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.4-mini"},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeDefault},
		},
	})

	provider, model := resolveJobAISelection(settings, domain.Job{})
	if provider != domain.AIProviderCodex {
		t.Fatalf("provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "gpt-5.4-mini" {
		t.Fatalf("model = %q, want gpt-5.4-mini", model)
	}

	job := domain.Job{
		AIProvider: domain.AIProviderGitHubCopilot,
		AIModel:    "claude-sonnet-4.6",
	}
	provider, model = resolveJobAISelection(settings, job)
	if provider != domain.AIProviderGitHubCopilot {
		t.Fatalf("job provider = %q, want %q", provider, domain.AIProviderGitHubCopilot)
	}
	if model != "claude-sonnet-4.6" {
		t.Fatalf("job model = %q, want claude-sonnet-4.6", model)
	}
}
