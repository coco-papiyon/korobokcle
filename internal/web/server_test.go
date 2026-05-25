package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveStaticDirUsesExecutableDir(t *testing.T) {
	t.Parallel()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}

	got, err := resolveStaticDir()
	if err != nil {
		t.Fatalf("resolveStaticDir() error = %v", err)
	}

	want := filepath.Join(filepath.Dir(exe), "frontend", "dist")
	if got != want {
		t.Fatalf("resolveStaticDir() = %q, want %q", got, want)
	}
}
