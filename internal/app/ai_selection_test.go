package app

import (
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestResolveJobAISelectionUsesImplementerDefaults(t *testing.T) {
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		AIProvider: domain.AIProviderCodex,
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.4-mini"},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "claude-sonnet-4.6"},
		},
	})

	provider, model := resolveJobAISelection(settings, domain.Job{Kind: domain.JobKindIssueDesign})
	if provider != domain.AIProviderCodex {
		t.Fatalf("provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "gpt-5.4-mini" {
		t.Fatalf("model = %q, want gpt-5.4-mini", model)
	}
}

func TestResolveJobAISelectionForRoleUsesVerifierOverrides(t *testing.T) {
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		AIProvider: domain.AIProviderCodex,
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.4-mini"},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "claude-sonnet-4.6"},
		},
		VerificationAIProvider: domain.AIProviderGitHubCopilot,
		VerificationAIModel:    domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "claude-opus-4.6"},
	})

	provider, model := resolveJobAISelectionForRole(settings, domain.Job{Kind: domain.JobKindIssueImplementation}, "agent")
	if provider != domain.AIProviderCodex {
		t.Fatalf("implementer provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "gpt-5.4-mini" {
		t.Fatalf("implementer model = %q, want gpt-5.4-mini", model)
	}

	provider, model = resolveJobAISelectionForRole(settings, domain.Job{Kind: domain.JobKindIssueImplementation}, "verifier")
	if provider != domain.AIProviderGitHubCopilot {
		t.Fatalf("verifier provider = %q, want %q", provider, domain.AIProviderGitHubCopilot)
	}
	if model != "claude-opus-4.6" {
		t.Fatalf("verifier model = %q, want claude-opus-4.6", model)
	}

	settings.VerificationAIProvider = ""
	settings.VerificationAIModel = domain.ModelSelection{}
	provider, model = resolveJobAISelectionForRole(settings, domain.Job{Kind: domain.JobKindIssueImplementation}, "verifier")
	if provider != domain.AIProviderCodex {
		t.Fatalf("fallback verifier provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "gpt-5.4-mini" {
		t.Fatalf("fallback verifier model = %q, want gpt-5.4-mini", model)
	}
}

func TestResolveJobAISelectionForRoleUsesReviewerOverrides(t *testing.T) {
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		AIProvider: domain.AIProviderCodex,
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.4-mini"},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "claude-sonnet-4.6"},
		},
		ReviewerAIProvider: domain.AIProviderGitHubCopilot,
		ReviewerAIModel:    domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-4.1"},
	})

	provider, model := resolveJobAISelectionForRole(settings, domain.Job{Kind: domain.JobKindPRReview}, "agent")
	if provider != domain.AIProviderGitHubCopilot {
		t.Fatalf("reviewer provider = %q, want %q", provider, domain.AIProviderGitHubCopilot)
	}
	if model != "gpt-4.1" {
		t.Fatalf("reviewer model = %q, want gpt-4.1", model)
	}

	settings.ReviewerAIProvider = ""
	settings.ReviewerAIModel = domain.ModelSelection{}
	provider, model = resolveJobAISelectionForRole(settings, domain.Job{Kind: domain.JobKindPRReview}, "agent")
	if provider != domain.AIProviderCodex {
		t.Fatalf("fallback reviewer provider = %q, want %q", provider, domain.AIProviderCodex)
	}
	if model != "gpt-5.4-mini" {
		t.Fatalf("fallback reviewer model = %q, want gpt-5.4-mini", model)
	}
}
