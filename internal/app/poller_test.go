package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestPollerDeduplicatesJobs(t *testing.T) {
	cfg := config.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	processed := make([]domain.Job, 0, 1)
	manager := NewWorkerManager(cfg, nil, func(_ context.Context, job domain.Job) error {
		mu.Lock()
		processed = append(processed, job)
		mu.Unlock()
		return nil
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	store := newMemoryJobStore()
	jobs := []domain.Job{
		{ID: "1", Kind: domain.JobKindIssueDesign, State: domain.StateDetected},
	}
	poller := NewPoller(cfg, NewStaticJobSource(jobs), store, nil, manager)

	if err := poller.poll(ctx); err != nil {
		t.Fatalf("poll() error = %v", err)
	}
	if err := poller.poll(ctx); err != nil {
		t.Fatalf("second poll() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 1 {
		t.Fatalf("processed jobs = %d, want 1", len(processed))
	}
	if len(store.jobs) != 1 {
		t.Fatalf("stored jobs = %d, want 1", len(store.jobs))
	}
}

func TestPollerAllowsNewStateForSameJob(t *testing.T) {
	cfg := config.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	processed := make([]domain.Job, 0, 2)
	manager := NewWorkerManager(cfg, nil, func(_ context.Context, job domain.Job) error {
		mu.Lock()
		processed = append(processed, job)
		mu.Unlock()
		return nil
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	store := newMemoryJobStore()
	poller := NewPoller(cfg, NewStaticJobSource([]domain.Job{
		{ID: "1", Kind: domain.JobKindIssueDesign, State: domain.StateDetected},
	}), store, nil, manager)

	if err := poller.poll(ctx); err != nil {
		t.Fatalf("poll() error = %v", err)
	}

	poller.mu.Lock()
	poller.seen = make(map[string]struct{})
	poller.mu.Unlock()

	poller.source = NewStaticJobSource([]domain.Job{
		{ID: "1", Kind: domain.JobKindIssueDesign, State: domain.StateDesignRunning},
	})
	if err := poller.poll(ctx); err != nil {
		t.Fatalf("second poll() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 2 {
		t.Fatalf("processed jobs = %d, want 2", len(processed))
	}
}

type memoryJobStore struct {
	mu   sync.Mutex
	jobs map[string]domain.Job
}

func newMemoryJobStore() *memoryJobStore {
	return &memoryJobStore{jobs: make(map[string]domain.Job)}
}

func (s *memoryJobStore) List(context.Context) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, job)
	}
	return out, nil
}

func (s *memoryJobStore) Get(_ context.Context, id string) (domain.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok, nil
}

func (s *memoryJobStore) Upsert(_ context.Context, job domain.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

var _ JobStore = (*memoryJobStore)(nil)
