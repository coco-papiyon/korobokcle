package config

import "testing"

func TestValidateModelForProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ProviderSpec
		model    string
		want     string
		wantErr  bool
	}{
		{
			name:     "empty model is allowed",
			provider: ProviderSpec{Name: "mock"},
			model:    "   ",
			want:     "",
		},
		{
			name:     "valid model is returned trimmed",
			provider: ProviderSpec{Name: "claude", Models: []string{"claude-sonnet-4.6", "claude-opus-4.6"}},
			model:    " claude-sonnet-4.6 ",
			want:     "claude-sonnet-4.6",
		},
		{
			name:     "provider without models rejects explicit model",
			provider: ProviderSpec{Name: "mock"},
			model:    "gpt-5.4",
			wantErr:  true,
		},
		{
			name:     "invalid model reports choices",
			provider: ProviderSpec{Name: "copilot", Models: []string{"gpt-5.4", "gpt-5.4-mini", "gpt-5.4"}},
			model:    "gpt-5.3",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ValidateModelForProvider(tt.provider, tt.model)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateModelForProvider() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateModelForProvider() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ValidateModelForProvider() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModelNamesAndContainsString(t *testing.T) {
	t.Parallel()

	provider := ProviderSpec{
		Name:   "example",
		Models: []string{"  gpt-5.4  ", "", "gpt-5.4-mini", "gpt-5.4"},
	}
	got := modelNames(provider)
	want := []string{"gpt-5.4", "gpt-5.4-mini"}
	if len(got) != len(want) {
		t.Fatalf("modelNames() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("modelNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if !containsString(got, "gpt-5.4-mini") {
		t.Fatal("expected containsString() to find existing model")
	}
	if containsString(got, "claude") {
		t.Fatal("expected containsString() to return false for missing model")
	}
}

func TestResolveImprovementBranchAndDir(t *testing.T) {
	t.Parallel()

	repository := MonitoredRepository{
		ImprovementBranch: "  feature-ai  ",
		ImprovementDir:    "  .improvement-custom  ",
	}
	if got := ResolveImprovementBranch(repository); got != "feature-ai" {
		t.Fatalf("ResolveImprovementBranch() = %q, want %q", got, "feature-ai")
	}
	if got := ResolveImprovementDir(repository); got != ".improvement-custom" {
		t.Fatalf("ResolveImprovementDir() = %q, want %q", got, ".improvement-custom")
	}

	repository.ImprovementBranch = "   "
	repository.ImprovementDir = "\t"
	if got := ResolveImprovementBranch(repository); got != DefaultImprovementBranch {
		t.Fatalf("ResolveImprovementBranch() = %q, want default %q", got, DefaultImprovementBranch)
	}
	if got := ResolveImprovementDir(repository); got != DefaultImprovementDir {
		t.Fatalf("ResolveImprovementDir() = %q, want default %q", got, DefaultImprovementDir)
	}
}
