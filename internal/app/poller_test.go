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
	stored := store.jobs["1"]
	if stored.FetchedAt.IsZero() {
		t.Fatal("fetchedAt is zero for newly detected job")
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
	created, ok, err := store.Get(ctx, "1")
	if err != nil {
		t.Fatalf("Get() after first poll error = %v", err)
	}
	if !ok {
		t.Fatal("job not found after first poll")
	}
	if created.FetchedAt.IsZero() {
		t.Fatal("fetchedAt is zero after first poll")
	}

	poller.source = NewStaticJobSource([]domain.Job{
		{ID: "1", Kind: domain.JobKindIssueDesign, State: domain.StateDesignRunning},
	})
	if err := poller.poll(ctx); err != nil {
		t.Fatalf("second poll() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	updated, ok, err := store.Get(ctx, "1")
	if err != nil {
		t.Fatalf("Get() after second poll error = %v", err)
	}
	if !ok {
		t.Fatal("job not found after second poll")
	}
	if !updated.FetchedAt.Equal(created.FetchedAt) {
		t.Fatalf("fetchedAt = %s, want %s", updated.FetchedAt, created.FetchedAt)
	}
	if updated.UpdatedAt.IsZero() {
		t.Fatal("updatedAt is zero after state change")
	}
	if updated.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("updatedAt = %s, want it to change from %s", updated.UpdatedAt, created.UpdatedAt)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 2 {
		t.Fatalf("processed jobs = %d, want 2", len(processed))
	}
}

func TestPollerPreservesTimesWhenStateDoesNotChange(t *testing.T) {
	cfg := config.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewWorkerManager(cfg, nil, func(_ context.Context, job domain.Job) error {
		return nil
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	store := newMemoryJobStore()
	createdAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 1, 3, 4, 5, 0, time.UTC)
	existing := domain.Job{
		ID:         "1",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     1,
		Title:      "既存ジョブ",
		FetchedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
	if err := store.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	poller := NewPoller(cfg, NewStaticJobSource([]domain.Job{
		{
			ID:         "1",
			Kind:       domain.JobKindIssueDesign,
			State:      domain.StateDetected,
			Repository: "owner/repo",
			Number:     1,
			Title:      "既存ジョブ",
		},
	}), store, nil, manager)

	if err := poller.poll(ctx); err != nil {
		t.Fatalf("poll() error = %v", err)
	}

	got, ok, err := store.Get(ctx, "1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found")
	}
	if !got.FetchedAt.Equal(createdAt) {
		t.Fatalf("fetchedAt = %s, want %s", got.FetchedAt, createdAt)
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updatedAt = %s, want %s", got.UpdatedAt, updatedAt)
	}
}

func TestPollerCanPersistWithoutAutoSubmit(t *testing.T) {
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
	poller := NewPoller(cfg, NewStaticJobSource([]domain.Job{
		{ID: "1", Kind: domain.JobKindIssueDesign, State: domain.StateDesignRunning},
	}), store, nil, manager)
	poller.SetAutoSubmit(false)

	if err := poller.poll(ctx); err != nil {
		t.Fatalf("poll() error = %v", err)
	}

	stored, ok, err := store.Get(ctx, "1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found")
	}
	if stored.State != domain.StateDesignRunning {
		t.Fatalf("stored state = %s, want %s", stored.State, domain.StateDesignRunning)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 0 {
		t.Fatalf("processed jobs = %d, want 0", len(processed))
	}
}

func TestPollerReprocessesPRReviewAfterFeedbackCycle(t *testing.T) {
	cfg := config.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	processed := make([]domain.Job, 0, 3)
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
		{
			ID:         "pr-1",
			Kind:       domain.JobKindPRReview,
			State:      domain.StateReviewRunning,
			Repository: "owner/repo",
			Number:     1,
			Title:      "reviewed PR",
		},
	}), store, nil, manager)

	if err := poller.poll(ctx); err != nil {
		t.Fatalf("first poll() error = %v", err)
	}

	poller.source = NewStaticJobSource([]domain.Job{
		{
			ID:         "pr-1",
			Kind:       domain.JobKindPRFeedback,
			State:      domain.StatePRReviewComment,
			Repository: "owner/repo",
			Number:     1,
			Title:      "reviewed PR",
		},
	})
	if err := poller.poll(ctx); err != nil {
		t.Fatalf("second poll() error = %v", err)
	}

	poller.source = NewStaticJobSource([]domain.Job{
		{
			ID:         "pr-1",
			Kind:       domain.JobKindPRReview,
			State:      domain.StateReviewRunning,
			Repository: "owner/repo",
			Number:     1,
			Title:      "reviewed PR",
		},
	})
	if err := poller.poll(ctx); err != nil {
		t.Fatalf("third poll() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 3 {
		t.Fatalf("processed jobs = %d, want 3", len(processed))
	}
}

func TestPollerKeepsApprovedPRUntilMissing(t *testing.T) {
	cfg := config.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewWorkerManager(cfg, nil, func(_ context.Context, job domain.Job) error {
		return nil
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	store := newMemoryJobStore()
	approved := domain.Job{
		ID:         "pr-1",
		Kind:       domain.JobKindPRReview,
		State:      domain.StateReviewApproved,
		Repository: "owner/repo",
		Number:     1,
		Title:      "reviewed PR",
	}
	if err := store.Upsert(ctx, approved); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	poller := NewPoller(cfg, NewStaticJobSource([]domain.Job{
		{
			ID:         "pr-1",
			Kind:       domain.JobKindPRReview,
			State:      domain.StateReviewRunning,
			Repository: "owner/repo",
			Number:     1,
			Title:      "reviewed PR",
		},
	}), store, nil, manager)

	if err := poller.poll(ctx); err != nil {
		t.Fatalf("poll() error = %v", err)
	}

	updated, ok, err := store.Get(ctx, "pr-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found")
	}
	if updated.State != domain.StateReviewApproved {
		t.Fatalf("state = %s, want %s", updated.State, domain.StateReviewApproved)
	}

	poller.source = NewStaticJobSource(nil)
	if err := poller.poll(ctx); err != nil {
		t.Fatalf("missing poll() error = %v", err)
	}

	updated, ok, err = store.Get(ctx, "pr-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found after missing poll")
	}
	if updated.State != domain.StateCompleted {
		t.Fatalf("state after missing = %s, want %s", updated.State, domain.StateCompleted)
	}
}

type memoryJobStore struct {
	mu        sync.Mutex
	jobs      map[string]domain.Job
	updatedAt time.Time
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
	s.updatedAt = time.Now().UTC()
	return nil
}

func (s *memoryJobStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	s.updatedAt = time.Now().UTC()
	return nil
}

func (s *memoryJobStore) UpdatedAt(context.Context) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updatedAt, nil
}

var _ JobStore = (*memoryJobStore)(nil)
