package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositorySessionPathUsesRepositoryJobsDir(t *testing.T) {
	t.Parallel()

	root := filepath.Join("workspace", "tool")
	got := repositorySessionPath(root, "artifacts", "https://github.com/coco-papiyon/korobokcle.git")
	want := filepath.Join(root, "artifacts", "coco-papiyon-korobokcle", "jobs", "session.json")
	if got != want {
		t.Fatalf("repositorySessionPath() = %q, want %q", got, want)
	}
}

func TestSaveAndLoadJobSessionIDUseRepositorySessionFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifactsDir := "artifacts"
	repository := "https://github.com/coco-papiyon/korobokcle.git"

	if err := saveJobSessionID(root, artifactsDir, repository, "session-123"); err != nil {
		t.Fatalf("saveJobSessionID() error = %v", err)
	}

	path := repositorySessionPath(root, artifactsDir, repository)
	wantPath := filepath.Join(root, artifactsDir, "coco-papiyon-korobokcle", "jobs", "session.json")
	if path != wantPath {
		t.Fatalf("repositorySessionPath() = %q, want %q", path, wantPath)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected session file to exist: %v", err)
	}

	got := loadJobSessionID(root, artifactsDir, repository)
	if got != "session-123" {
		t.Fatalf("loadJobSessionID() = %q, want %q", got, "session-123")
	}

	if err := saveJobSessionID(root, artifactsDir, repository, ""); err != nil {
		t.Fatalf("saveJobSessionID(clear) error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected session file to be removed, got err=%v", err)
	}
	if got := loadJobSessionID(root, artifactsDir, repository); got != "" {
		t.Fatalf("loadJobSessionID() after clear = %q, want empty", got)
	}
}
