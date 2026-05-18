package config

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestAppMarshalYAMLUsesSecondsForPollInterval(t *testing.T) {
	t.Parallel()

	app := DefaultFiles().App
	app.PollInterval = 90 * time.Second

	raw, err := yaml.Marshal(app)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	text := string(raw)
	if !strings.Contains(text, "pollInterval: 90") {
		t.Fatalf("expected poll interval to be stored as seconds, got %s", text)
	}
	if strings.Contains(text, "providers:") {
		t.Fatalf("expected app yaml to omit providers, got %s", text)
	}
}

func TestAppUnmarshalYAMLAcceptsLegacyDurationAndSeconds(t *testing.T) {
	t.Parallel()

	t.Run("legacy duration", func(t *testing.T) {
		t.Parallel()

		var app App
		app = DefaultFiles().App

		if err := yaml.Unmarshal([]byte("pollInterval: 2m0s\n"), &app); err != nil {
			t.Fatalf("yaml.Unmarshal() error = %v", err)
		}
		if got := app.PollInterval; got != 2*time.Minute {
			t.Fatalf("expected 2m0s to decode as 2m, got %s", got)
		}
	})

	t.Run("seconds", func(t *testing.T) {
		t.Parallel()

		var app App
		app = DefaultFiles().App

		if err := yaml.Unmarshal([]byte("pollInterval: 120\n"), &app); err != nil {
			t.Fatalf("yaml.Unmarshal() error = %v", err)
		}
		if got := app.PollInterval; got != 2*time.Minute {
			t.Fatalf("expected 120 to decode as 2m, got %s", got)
		}
	})
}
