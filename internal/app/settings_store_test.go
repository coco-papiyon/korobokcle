package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestNewFileSettingsStoreCreatesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "settings.json")
	defaults := domain.WatchSettings{
		Repository:             "owner/repo",
		BuiltinAllowedCommands: domain.DefaultAllowedCommands(),
	}

	store, err := NewFileSettingsStore(path, defaults)
	if err != nil {
		t.Fatalf("NewFileSettingsStore() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected settings file: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var persisted domain.WatchSettings
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(persisted.BuiltinAllowedCommands) != len(defaults.BuiltinAllowedCommands) {
		t.Fatalf("builtin allowed commands = %d, want %d", len(persisted.BuiltinAllowedCommands), len(defaults.BuiltinAllowedCommands))
	}
	if persisted.BuiltinAllowedCommands[0] != defaults.BuiltinAllowedCommands[0] {
		t.Fatalf("builtin allowed commands[0] = %q, want %q", persisted.BuiltinAllowedCommands[0], defaults.BuiltinAllowedCommands[0])
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.BuiltinAllowedCommands) != len(defaults.BuiltinAllowedCommands) {
		t.Fatalf("loaded builtin allowed commands = %d, want %d", len(loaded.BuiltinAllowedCommands), len(defaults.BuiltinAllowedCommands))
	}
}
