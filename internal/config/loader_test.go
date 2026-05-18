package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrInitCreatesDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files, err := LoadOrInit(root)
	if err != nil {
		t.Fatalf("LoadOrInit() error = %v", err)
	}

	if files.App.HTTPPort == 0 {
		t.Fatal("expected default http port")
	}
	if files.App.ScreenRefreshInterval != DefaultScreenRefreshInterval {
		t.Fatalf("expected default screen refresh interval %s, got %s", DefaultScreenRefreshInterval, files.App.ScreenRefreshInterval)
	}

	for _, path := range []string{
		"config/app.yaml",
		"config/watch-rules.yaml",
		"config/notifications.yaml",
		"config/test-profiles.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("expected file %s to exist: %v", path, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "app.yaml"))
	if err != nil {
		t.Fatalf("read app.yaml: %v", err)
	}
	if !bytes.Contains(raw, []byte("pollInterval: 120")) {
		t.Fatalf("expected default app config to store pollInterval as seconds, got %s", string(raw))
	}
	if bytes.Contains(raw, []byte("providers:")) {
		t.Fatalf("expected default app config to omit providers, got %s", string(raw))
	}
}
