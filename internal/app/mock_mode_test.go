package app

import (
	"context"
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
