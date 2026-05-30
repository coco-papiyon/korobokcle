package config

import "strings"

var providerCatalog = []ProviderSpec{
	{
		Name:   "copilot",
		Models: []string{"claude-sonnet-4.6", "claude-opus-4.6", "gpt-5.4", "gpt-5-mini", "gpt-4.1"},
	},
	{
		Name:   "claude",
		Models: []string{"claude-sonnet-4.6", "claude-opus-4.6"},
	},
	{
		Name:   "codex",
		Models: []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"},
	},
	{
		Name:   "mock",
		Models: []string{},
	},
}

func ProviderCatalog() []ProviderSpec {
	return cloneProviderSpecs(providerCatalog)
}

func ProviderNames() []string {
	names := make([]string, 0, len(providerCatalog))
	for _, provider := range providerCatalog {
		names = append(names, provider.Name)
	}
	return names
}

func ProviderSpecByName(name string) (ProviderSpec, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return ProviderSpec{}, false
	}
	for _, provider := range providerCatalog {
		if provider.Name == trimmed {
			return cloneProviderSpec(provider), true
		}
	}
	return ProviderSpec{}, false
}
