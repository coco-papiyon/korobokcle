package config

import (
	"reflect"
	"testing"
	"time"
)

func TestDefaultCopilotAllowTools(t *testing.T) {
	t.Parallel()

	want := []string{}
	if !reflect.DeepEqual(DefaultCopilotAllowTools, want) {
		t.Fatalf("expected default copilot allow tools %v, got %v", want, DefaultCopilotAllowTools)
	}
}

func TestDefaultScreenRefreshInterval(t *testing.T) {
	t.Parallel()

	if DefaultScreenRefreshInterval != 5*time.Second {
		t.Fatalf("expected default screen refresh interval 5s, got %s", DefaultScreenRefreshInterval)
	}
}

func TestProviderCatalogMatchesIssue(t *testing.T) {
	t.Parallel()

	want := []ProviderSpec{
		{Name: "copilot", Models: []string{"gpt-4.1"}},
		{Name: "codex", Models: []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"}},
		{Name: "mock", Models: []string{}},
	}
	if got := ProviderCatalog(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected provider catalog %v, got %v", want, got)
	}
}
