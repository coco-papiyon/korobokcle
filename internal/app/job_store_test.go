package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		FetchedAt:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 7, 1, 3, 4, 5, 0, time.UTC),
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
	if !jobs[0].FetchedAt.Equal(job.FetchedAt) {
		t.Fatalf("loaded fetchedAt = %s, want %s", jobs[0].FetchedAt, job.FetchedAt)
	}
	if !jobs[0].UpdatedAt.Equal(job.UpdatedAt) {
		t.Fatalf("loaded updatedAt = %s, want %s", jobs[0].UpdatedAt, job.UpdatedAt)
	}
	if got, err := store.UpdatedAt(context.Background()); err != nil {
		t.Fatalf("UpdatedAt() error = %v", err)
	} else if got.IsZero() {
		t.Fatal("updatedAt is zero after upsert")
	}
}

func TestFileJobStoreDelete(t *testing.T) {
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
	if err := store.Delete(context.Background(), job.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	reloaded, err := NewFileJobStore(path)
	if err != nil {
		t.Fatalf("reload NewFileJobStore() error = %v", err)
	}
	jobs, err := reloaded.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs = %d, want 0", len(jobs))
	}
	if got, err := reloaded.UpdatedAt(context.Background()); err != nil {
		t.Fatalf("UpdatedAt() error = %v", err)
	} else if got.IsZero() {
		t.Fatal("updatedAt is zero after delete")
	}
}

func TestFileJobStoreLoadsLegacyArrayFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	raw, err := json.MarshalIndent([]domain.Job{{
		ID:         "job-1",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     42,
		Title:      "design me",
	}}, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := NewFileJobStore(path)
	if err != nil {
		t.Fatalf("NewFileJobStore() error = %v", err)
	}
	jobs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	if got, err := store.UpdatedAt(context.Background()); err != nil {
		t.Fatalf("UpdatedAt() error = %v", err)
	} else if got.IsZero() {
		t.Fatal("updatedAt is zero for legacy format")
	}
}
