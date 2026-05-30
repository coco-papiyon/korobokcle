package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceUpdateTestProfilesPersistsAndClones(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	svc := NewService(root, DefaultFiles())
	file := TestProfiles{
		Profiles: []TestProfile{
			{
				Name:     "go-default",
				Commands: []string{"go test ./...", "go test ./internal/..."},
			},
		},
	}

	if err := svc.UpdateTestProfiles(file); err != nil {
		t.Fatalf("UpdateTestProfiles() error = %v", err)
	}

	file.Profiles[0].Name = "changed"
	file.Profiles[0].Commands[0] = "modified"

	got := svc.TestProfiles()
	if len(got.Profiles) != 1 {
		t.Fatalf("expected one profile, got %d", len(got.Profiles))
	}
	if got.Profiles[0].Name != "go-default" {
		t.Fatalf("expected cached profile name to remain go-default, got %q", got.Profiles[0].Name)
	}
	if got.Profiles[0].Commands[0] != "go test ./..." {
		t.Fatalf("expected cached command to remain unchanged, got %q", got.Profiles[0].Commands[0])
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "test-profiles.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range [][]byte{[]byte("name: go-default"), []byte("- go test ./..."), []byte("- go test ./internal/...")} {
		if !bytes.Contains(raw, expected) {
			t.Fatalf("expected saved yaml to contain %q, got %s", expected, string(raw))
		}
	}
}

func TestServiceUpdateToolCommandsPersistsAndClones(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	svc := NewService(root, DefaultFiles())
	file := ToolCommands{
		Commands: []ToolCommand{
			{
				Name:     "preview",
				Command:  "npm run dev",
				Resident: true,
			},
		},
	}

	if err := svc.UpdateToolCommands(file); err != nil {
		t.Fatalf("UpdateToolCommands() error = %v", err)
	}

	file.Commands[0].Name = "changed"
	file.Commands[0].Command = "modified"

	got := svc.ToolCommands()
	if len(got.Commands) != 1 {
		t.Fatalf("expected one command, got %d", len(got.Commands))
	}
	if got.Commands[0].Name != "preview" {
		t.Fatalf("expected cached tool command name to remain preview, got %q", got.Commands[0].Name)
	}
	if got.Commands[0].Command != "npm run dev" {
		t.Fatalf("expected cached tool command to remain unchanged, got %q", got.Commands[0].Command)
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "tool-commands.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range [][]byte{[]byte("name: preview"), []byte("command: npm run dev"), []byte("resident: true")} {
		if !bytes.Contains(raw, expected) {
			t.Fatalf("expected saved yaml to contain %q, got %s", expected, string(raw))
		}
	}
}
