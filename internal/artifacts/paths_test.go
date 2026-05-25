package artifacts

import (
	"path/filepath"
	"testing"
)

func TestJobDirResolvesRelativeArtifactsDirAgainstRoot(t *testing.T) {
	t.Parallel()

	got := JobDir(filepath.Join("workspace", "tool"), "artifacts", "job-1")
	want := filepath.Join("workspace", "tool", "artifacts", "jobs", "job-1")
	if got != want {
		t.Fatalf("JobDir() = %q, want %q", got, want)
	}
}

func TestJobDirPreservesAbsoluteArtifactsDir(t *testing.T) {
	t.Parallel()

	absoluteArtifactsDir := filepath.Join(t.TempDir(), "artifacts")

	got := JobDir(filepath.Join("workspace", "tool"), absoluteArtifactsDir, "job-1")
	want := filepath.Join(absoluteArtifactsDir, "jobs", "job-1")
	if got != want {
		t.Fatalf("JobDir() = %q, want %q", got, want)
	}
}
