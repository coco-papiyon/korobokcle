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
				ID:       "profile-1",
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
	if got.Profiles[0].ID != "profile-1" {
		t.Fatalf("expected cached profile id to remain profile-1, got %q", got.Profiles[0].ID)
	}
	if got.Profiles[0].Commands[0] != "go test ./..." {
		t.Fatalf("expected cached command to remain unchanged, got %q", got.Profiles[0].Commands[0])
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "test-profiles.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range [][]byte{[]byte("id: profile-1"), []byte("name: go-default"), []byte("- go test ./..."), []byte("- go test ./internal/...")} {
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

func TestProviderByNameReturnsClaude(t *testing.T) {
	t.Parallel()

	svc := NewService(t.TempDir(), DefaultFiles())
	spec, ok := svc.ProviderByName("claude")
	if !ok {
		t.Fatalf("expected claude provider to be registered")
	}
	if spec.Name != "claude" {
		t.Fatalf("expected provider name claude, got %q", spec.Name)
	}
	if len(spec.Models) != 2 {
		t.Fatalf("expected claude provider models, got %#v", spec.Models)
	}
}

func TestServiceUpdateAppClonesImprovementSettings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	svc := NewService(root, DefaultFiles())
	app := DefaultFiles().App
	app.MonitoredRepositories = []MonitoredRepository{{
		Repository:         "owner/repo",
		Branch:             "main",
		WorkDir:            "artifacts/owner-repo/workspace",
		Workers:            2,
		ImprovementEnabled: true,
		ImprovementBranch:  "develop-ai",
		ImprovementDir:     ".improvements-custom",
		ImprovementWorkDir: ".improvement-custom",
	}}

	if err := svc.UpdateApp(app); err != nil {
		t.Fatalf("UpdateApp() error = %v", err)
	}

	app.MonitoredRepositories[0].ImprovementBranch = "changed"
	app.MonitoredRepositories[0].ImprovementDir = "changed"
	app.MonitoredRepositories[0].ImprovementWorkDir = "changed"

	got := svc.App()
	if len(got.MonitoredRepositories) != 1 {
		t.Fatalf("expected one monitored repository, got %d", len(got.MonitoredRepositories))
	}
	repository := got.MonitoredRepositories[0]
	if !repository.ImprovementEnabled {
		t.Fatalf("expected improvement feature enabled in cached config")
	}
	if repository.ImprovementBranch != "develop-ai" || repository.ImprovementDir != ".improvements-custom" || repository.ImprovementWorkDir != ".improvement-custom" {
		t.Fatalf("unexpected cached improvement settings: %#v", repository)
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "app.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range [][]byte{
		[]byte("improvementEnabled: true"),
		[]byte("improvementBranch: develop-ai"),
		[]byte("improvementDir: .improvements-custom"),
		[]byte("improvementWorkDir: .improvement-custom"),
	} {
		if !bytes.Contains(raw, expected) {
			t.Fatalf("expected saved yaml to contain %q, got %s", expected, string(raw))
		}
	}
}
