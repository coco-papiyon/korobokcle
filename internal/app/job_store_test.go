package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestFileJobStoreUpsertAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")

	store, err := NewFileJobStore(path)
	if err != nil {
		t.Fatalf("NewFileJobStore() error = %v", err)
	}

	job := domain.Job{
		ID:         "job-1",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     42,
		Title:      "design me",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected store file to exist: %v", err)
	}

	reloaded, err := NewFileJobStore(path)
	if err != nil {
		t.Fatalf("reload NewFileJobStore() error = %v", err)
	}
	jobs, err := reloaded.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	if jobs[0].ID != job.ID || jobs[0].Title != job.Title {
		t.Fatalf("loaded job = %+v, want %+v", jobs[0], job)
	}
}
