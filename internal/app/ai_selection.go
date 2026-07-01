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
