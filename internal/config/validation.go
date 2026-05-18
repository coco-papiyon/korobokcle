package config

import (
	"fmt"
	"strings"
)

func ValidateModelForProvider(provider ProviderSpec, model string) (string, error) {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		return "", nil
	}
	if len(provider.Models) == 0 {
		return "", fmt.Errorf("model must be empty for provider %q", provider.Name)
	}
	for _, candidate := range provider.Models {
		if candidate == trimmedModel {
			return trimmedModel, nil
		}
	}
	return "", fmt.Errorf("model must be one of %s", strings.Join(modelNames(provider), ", "))
}

func modelNames(provider ProviderSpec) []string {
	names := []string{}
	for _, model := range provider.Models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" || containsString(names, trimmed) {
			continue
		}
		names = append(names, trimmed)
	}
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
