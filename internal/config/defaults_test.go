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

func TestDefaultWorkspaceDir(t *testing.T) {
	t.Parallel()

	if got := DefaultFiles().App.WorkspaceDir; got != ".workspace" {
		t.Fatalf("expected default workspace dir .workspace, got %q", got)
	}
}

func TestDefaultImprovementSettings(t *testing.T) {
	t.Parallel()

	app := DefaultFiles().App
	if len(app.MonitoredRepositories) != 1 {
		t.Fatalf("expected one default repository, got %d", len(app.MonitoredRepositories))
	}
	repository := app.MonitoredRepositories[0]
	if repository.ImprovementEnabled {
		t.Fatalf("expected improvement feature disabled by default")
	}
	if got := ResolveImprovementBranch(repository); got != DefaultImprovementBranch {
		t.Fatalf("expected default improvement branch %q, got %q", DefaultImprovementBranch, got)
	}
	if got := ResolveImprovementDir(repository); got != DefaultImprovementDir {
		t.Fatalf("expected default improvement dir %q, got %q", DefaultImprovementDir, got)
	}
}

func TestResolveImprovementSettingsUsesConfiguredValues(t *testing.T) {
	t.Parallel()

	repository := MonitoredRepository{
		ImprovementBranch: " custom-branch ",
		ImprovementDir:    " .rules/improvement ",
	}
	if got := ResolveImprovementBranch(repository); got != "custom-branch" {
		t.Fatalf("expected trimmed improvement branch, got %q", got)
	}
	if got := ResolveImprovementDir(repository); got != ".rules/improvement" {
		t.Fatalf("expected trimmed improvement dir, got %q", got)
	}
}

func TestDefaultNotificationChannelUsesWindowsDesktopNotification(t *testing.T) {
	t.Parallel()

	notifications := DefaultFiles().Notifications
	if len(notifications.Channels) != 1 {
		t.Fatalf("expected 1 notification channel, got %d", len(notifications.Channels))
	}

	channel := notifications.Channels[0]
	if channel.Name != "Windowsデスクトップ通知" {
		t.Fatalf("expected default channel name Windowsデスクトップ通知, got %q", channel.Name)
	}
	if channel.Type != "windows_toast" {
		t.Fatalf("expected default channel type windows_toast, got %q", channel.Type)
	}
}

func TestProviderCatalogMatchesIssue(t *testing.T) {
	t.Parallel()

	want := []ProviderSpec{
		{Name: "copilot", Models: []string{"claude-sonnet-4.6", "claude-opus-4.6", "gpt-5.4", "gpt-5-mini", "gpt-4.1"}},
		{Name: "claude", Models: []string{"claude-sonnet-4.6", "claude-opus-4.6"}},
		{Name: "codex", Models: []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"}},
		{Name: "mock", Models: []string{}},
	}
	if got := ProviderCatalog(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected provider catalog %v, got %v", want, got)
	}
}
