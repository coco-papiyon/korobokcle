package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestMockArtifactActionServiceRerunStopsAtRunningState(t *testing.T) {
	store := newMemoryJobStore()
	job := domain.Job{
		ID:         "issue-700",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDesignReady,
		Repository: "owner/repo",
		Number:     700,
		Title:      "mock rerun",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	monitor := &approvalTestMonitor{store: store, jobID: job.ID}
	service := NewMockArtifactActionService(store, nil, nil, t.TempDir(), monitor)

	updated, err := service.RerunArtifact(context.Background(), job.ID, "retry")
	if err != nil {
		t.Fatalf("RerunArtifact() error = %v", err)
	}
	if updated.State != domain.StateDesignRunning {
		t.Fatalf("updated state = %s, want %s", updated.State, domain.StateDesignRunning)
	}

	stored, ok, err := store.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found")
	}
	if stored.State != domain.StateDesignRunning {
		t.Fatalf("stored state = %s, want %s", stored.State, domain.StateDesignRunning)
	}
	if monitor.calls != 1 {
		t.Fatalf("monitor calls = %d, want 1", monitor.calls)
	}
}

func TestMockArtifactActionServiceReturnsMockSourceDiff(t *testing.T) {
	store := newMemoryJobStore()
	job := domain.Job{
		ID:         "issue-701",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateImplementationReady,
		Repository: "owner/repo",
		Number:     701,
		Title:      "mock diff",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	baseDir := t.TempDir()
	artifactPath, err := mockArtifactPath(baseDir, job)
	if err != nil {
		t.Fatalf("mockArtifactPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("# mock artifact\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	diffPath, err := mockSourceDiffPath(baseDir, job)
	if err != nil {
		t.Fatalf("mockSourceDiffPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(diffPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() diff error = %v", err)
	}
	if err := os.WriteFile(diffPath, []byte("diff --git a/mock-source.txt b/mock-source.txt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() diff error = %v", err)
	}

	service := NewMockArtifactActionService(store, nil, nil, baseDir, nil)
	diff, err := service.GetSourceDiff(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetSourceDiff() error = %v", err)
	}
	if diff.BaseRef != "mock" {
		t.Fatalf("baseRef = %q, want mock", diff.BaseRef)
	}
	wantPath := jobSourceDiffTargetPath(job)
	if diff.Path != wantPath {
		t.Fatalf("path = %q, want %q", diff.Path, wantPath)
	}
	if diff.Content == "" || !strings.Contains(diff.Content, "diff --git") {
		t.Fatalf("content = %q, want mock diff", diff.Content)
	}
}

func TestMockArtifactActionServiceUpdatesEditableArtifact(t *testing.T) {
	store := newMemoryJobStore()
	job := domain.Job{
		ID:         "issue-702",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDesignReady,
		Repository: "owner/repo",
		Number:     702,
		Title:      "mock edit",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	baseDir := t.TempDir()
	artifactPath, err := mockArtifactPath(baseDir, job)
	if err != nil {
		t.Fatalf("mockArtifactPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("before"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	service := NewMockArtifactActionService(store, nil, nil, baseDir, nil)
	updated, err := service.UpdateArtifact(context.Background(), job.ID, "after")
	if err != nil {
		t.Fatalf("UpdateArtifact() error = %v", err)
	}
	if updated.Content != "after" {
		t.Fatalf("content = %q, want after", updated.Content)
	}
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(raw) != "after" {
		t.Fatalf("file content = %q, want after", string(raw))
	}
}
