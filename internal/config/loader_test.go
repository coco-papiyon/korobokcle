package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	if files.App.ShutdownTimeout != 10*time.Second {
		t.Fatalf("expected default shutdown timeout 10s, got %s", files.App.ShutdownTimeout)
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
	if !bytes.Contains(raw, []byte("screenRefreshInterval: 5")) {
		t.Fatalf("expected default app config to store screenRefreshInterval as seconds, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("shutdownTimeout: 10")) {
		t.Fatalf("expected default app config to store shutdownTimeout as seconds, got %s", string(raw))
	}
	if bytes.Contains(raw, []byte("providers:")) {
		t.Fatalf("expected default app config to omit providers, got %s", string(raw))
	}
}
