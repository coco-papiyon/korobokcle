package app

import (
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func resolveJobAISelection(settings domain.WatchSettings, job domain.Job) (domain.AIProvider, string) {
	return resolveJobAISelectionForRole(settings, job, "")
}

func resolveJobAISelectionForRole(settings domain.WatchSettings, job domain.Job, role string) (domain.AIProvider, string) {
	switch {
	case job.Kind == domain.JobKindIssueImplementation && strings.EqualFold(role, "verifier"):
		return verifierAISelection(settings)
	case job.Kind == domain.JobKindPRReview:
		return reviewerAISelection(settings)
	default:
		return implementerAISelection(settings)
	}
}

func implementerAISelection(settings domain.WatchSettings) (domain.AIProvider, string) {
	provider := settings.AIProvider
	if !provider.IsValid() {
		provider = domain.AIProviderCodex
	}
	return provider, selectedModel(settings, providerKey(provider))
}

func verifierAISelection(settings domain.WatchSettings) (domain.AIProvider, string) {
	provider := settings.VerificationAIProvider
	if !provider.IsValid() {
		provider = settings.AIProvider
	}
	if !provider.IsValid() {
		provider = domain.AIProviderCodex
	}
	model := settings.VerificationAIModel
	if model.Mode == domain.ModelModeCustom && strings.TrimSpace(model.Value) != "" {
		return provider, strings.TrimSpace(model.Value)
	}
	return provider, selectedModel(settings, providerKey(provider))
}

func reviewerAISelection(settings domain.WatchSettings) (domain.AIProvider, string) {
	provider := settings.ReviewerAIProvider
	if !provider.IsValid() {
		provider = settings.AIProvider
	}
	if !provider.IsValid() {
		provider = domain.AIProviderCodex
	}
	model := settings.ReviewerAIModel
	if model.Mode == domain.ModelModeCustom && strings.TrimSpace(model.Value) != "" {
		return provider, strings.TrimSpace(model.Value)
	}
	return provider, selectedModel(settings, providerKey(provider))
}
