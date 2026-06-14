package app

import (
	"fmt"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func resolveExecutionConfig(cfg *config.Service, watchRuleID string) (skill.ExecutionConfig, error) {
	rule, ok := cfg.WatchRuleByID(watchRuleID)
	provider := firstNonEmpty(rule.Provider, cfg.App().Provider)
	spec, ok := cfg.ProviderByName(strings.ToLower(strings.TrimSpace(provider)))
	if !ok {
		return skill.ExecutionConfig{}, fmt.Errorf("provider %q not found", provider)
	}

	model := firstNonEmpty(rule.Model, cfg.App().Model)
	if trimmedModel := strings.TrimSpace(model); trimmedModel != "" {
		validatedModel, err := config.ValidateModelForProvider(spec, trimmedModel)
		if err != nil {
			return skill.ExecutionConfig{}, fmt.Errorf("%w", err)
		}
		model = validatedModel
	}
	return skill.ExecutionConfig{
		Provider: strings.ToLower(strings.TrimSpace(provider)),
		Model:    strings.TrimSpace(model),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
