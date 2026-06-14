package config

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestAppMarshalYAMLUsesSecondsForTimingFields(t *testing.T) {
	t.Parallel()

	app := DefaultFiles().App
	app.PollInterval = 90 * time.Second
	app.ScreenRefreshInterval = 15 * time.Second
	app.ShutdownTimeout = 42 * time.Second

	raw, err := yaml.Marshal(app)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	text := string(raw)
	for _, expected := range []string{"pollInterval: 90", "screenRefreshInterval: 15", "shutdownTimeout: 42"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in yaml, got %s", expected, text)
		}
	}
	if strings.Contains(text, "5s") || strings.Contains(text, "10s") {
		t.Fatalf("expected yaml to omit duration strings, got %s", text)
	}
}

func TestAppUnmarshalYAMLUsesIntegerSecondsAndFallsBackForInvalidValues(t *testing.T) {
	t.Parallel()

	var app App
	app = DefaultFiles().App

	if err := yaml.Unmarshal([]byte("pollInterval: 120\nscreenRefreshInterval: 7\nshutdownTimeout: 30\n"), &app); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if got := app.PollInterval; got != 2*time.Minute {
		t.Fatalf("expected poll interval 2m, got %s", got)
	}
	if got := app.ScreenRefreshInterval; got != 7*time.Second {
		t.Fatalf("expected screen refresh interval 7s, got %s", got)
	}
	if got := app.ShutdownTimeout; got != 30*time.Second {
		t.Fatalf("expected shutdown timeout 30s, got %s", got)
	}

	if err := yaml.Unmarshal([]byte("pollInterval: 2m0s\nscreenRefreshInterval: 5s\nshutdownTimeout: 1m\n"), &app); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if got := app.PollInterval; got != 2*time.Minute {
		t.Fatalf("expected invalid poll interval to keep previous value, got %s", got)
	}
	if got := app.ScreenRefreshInterval; got != 7*time.Second {
		t.Fatalf("expected invalid screen refresh interval to keep previous value, got %s", got)
	}
	if got := app.ShutdownTimeout; got != 30*time.Second {
		t.Fatalf("expected invalid shutdown timeout to keep previous value, got %s", got)
	}
}

func TestAppMonitoredRepositoryBranchRoundTrip(t *testing.T) {
	t.Parallel()

	app := DefaultFiles().App
	app.MonitoredRepositories = []MonitoredRepository{
		{
			Repository:         "owner/repo",
			Branch:             "release/1.x",
			Workers:            2,
			ImprovementEnabled: true,
			ImprovementBranch:  "develop-ai",
			ImprovementDir:     ".repo-improvement",
		},
	}

	raw, err := yaml.Marshal(app)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if !strings.Contains(string(raw), "branch: release/1.x") {
		t.Fatalf("expected branch in yaml, got %s", string(raw))
	}
	for _, expected := range []string{
		"improvementEnabled: true",
		"improvementBranch: develop-ai",
		"improvementDir: .repo-improvement",
	} {
		if !strings.Contains(string(raw), expected) {
			t.Fatalf("expected %q in yaml, got %s", expected, string(raw))
		}
	}

	var decoded App
	if err := yaml.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if len(decoded.MonitoredRepositories) != 1 {
		t.Fatalf("expected one monitored repository, got %d", len(decoded.MonitoredRepositories))
	}
	if got := decoded.MonitoredRepositories[0].Branch; got != "release/1.x" {
		t.Fatalf("expected monitored repository branch release/1.x, got %q", got)
	}
	if !decoded.MonitoredRepositories[0].ImprovementEnabled {
		t.Fatalf("expected improvementEnabled to round-trip")
	}
	if got := decoded.MonitoredRepositories[0].ImprovementBranch; got != "develop-ai" {
		t.Fatalf("expected improvement branch develop-ai, got %q", got)
	}
	if got := decoded.MonitoredRepositories[0].ImprovementDir; got != ".repo-improvement" {
		t.Fatalf("expected improvement dir .repo-improvement, got %q", got)
	}
}
