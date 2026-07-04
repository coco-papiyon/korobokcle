package app

import (
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func resolveConditionAISelection(settings domain.WatchSettings, condition domain.SearchCondition) (domain.AIProvider, string) {
	provider := settings.AIProvider
	if condition.AIProvider.IsValid() {
		provider = condition.AIProvider
	}
	model := selectedModel(settings, providerKey(provider))
	if condition.AIModel.Mode == domain.ModelModeCustom && strings.TrimSpace(condition.AIModel.Value) != "" {
		model = strings.TrimSpace(condition.AIModel.Value)
	}
	return provider, model
}

func resolveJobAISelection(settings domain.WatchSettings, job domain.Job) (domain.AIProvider, string) {
	return resolveJobAISelectionForRole(settings, job, "")
}

func verifierProviderForSettings(settings domain.WatchSettings) domain.AIProvider {
	provider := settings.VerificationAIProvider
	if !provider.IsValid() {
		provider = settings.AIProvider
	}
	if !provider.IsValid() {
		provider = domain.AIProviderCodex
	}
	return provider
}

func resolveJobAISelectionForRole(settings domain.WatchSettings, job domain.Job, role string) (domain.AIProvider, string) {
	if job.Kind == domain.JobKindIssueImplementation && strings.EqualFold(role, "verifier") {
		provider, model := implementationVerifierAISelection(settings)
		if job.AIProvider.IsValid() {
			provider = job.AIProvider
		}
		if strings.TrimSpace(job.AIModel) != "" {
			model = strings.TrimSpace(job.AIModel)
		}
		return provider, model
	}
	if job.Kind == domain.JobKindIssueImplementation {
		provider := settings.AIProvider
		if !provider.IsValid() {
			provider = domain.AIProviderCodex
		}
		model := selectedModel(settings, providerKey(provider))
		if job.AIProvider.IsValid() {
			provider = job.AIProvider
		}
		if strings.TrimSpace(job.AIModel) != "" {
			model = strings.TrimSpace(job.AIModel)
		}
		return provider, model
	}
	provider := settings.AIProvider
	if job.AIProvider.IsValid() {
		provider = job.AIProvider
	}
	model := strings.TrimSpace(job.AIModel)
	if model == "" {
		model = selectedModel(settings, providerKey(provider))
	}
	return provider, model
}

func implementationVerifierAISelection(settings domain.WatchSettings) (domain.AIProvider, string) {
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
	return provider, selectedModel(settings, providerKey(settings.AIProvider))
}
