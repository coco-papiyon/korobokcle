package config

import (
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
}
