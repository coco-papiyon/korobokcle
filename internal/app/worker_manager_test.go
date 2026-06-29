package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestWorkerManagerSubmitAndProcess(t *testing.T) {
	cfg := config.Default()
	cfg.DesignWorkers = 1
	cfg.ImplementationWorkers = 1
	cfg.ReviewWorkers = 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	processed := make([]domain.Job, 0, 1)
	manager := NewWorkerManager(cfg, nil, func(_ context.Context, job domain.Job) error {
		mu.Lock()
		processed = append(processed, job)
		mu.Unlock()
		cancel()
		return nil
	})

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	job := domain.Job{ID: "job-1", Kind: domain.JobKindIssueDesign, State: domain.StateDetected}
	if err := manager.Submit(job); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		manager.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manager.Wait() timed out")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 1 {
		t.Fatalf("processed jobs = %d, want 1", len(processed))
	}
	if processed[0].ID != job.ID || processed[0].Kind != job.Kind {
		t.Fatalf("processed job = %+v, want %+v", processed[0], job)
	}
}

func TestWorkerManagerRejectsUnsupportedKind(t *testing.T) {
	cfg := config.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewWorkerManager(cfg, nil, nil)
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	err := manager.Submit(domain.Job{ID: "job-x", Kind: "unknown"})
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestWorkerManagerRequiresStart(t *testing.T) {
	manager := NewWorkerManager(config.Default(), nil, nil)
	err := manager.Submit(domain.Job{ID: "job-x", Kind: domain.JobKindIssueDesign})
	if err == nil {
		t.Fatal("expected error before start")
	}
}

func TestWorkerManagerOwnsProcessorLifecycle(t *testing.T) {
	cfg := config.Default()
	cfg.DesignWorkers = 1
	cfg.ImplementationWorkers = 1
	cfg.ReviewWorkers = 1

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	starts, stops := 0, 0
	manager := NewWorkerManagerWithFactory(cfg, nil, func() WorkerProcessor {
		return &lifecycleTestProcessor{mu: &mu, starts: &starts, stops: &stops}
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	cancel()
	manager.Wait()

	mu.Lock()
	defer mu.Unlock()
	if starts != 4 || stops != 4 {
		t.Fatalf("processor lifecycle starts=%d stops=%d, want 4/4", starts, stops)
	}
}

type lifecycleTestProcessor struct {
	mu            *sync.Mutex
	starts, stops *int
}

func (p *lifecycleTestProcessor) Start(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	*p.starts++
	return nil
}

func (p *lifecycleTestProcessor) Process(context.Context, domain.Job) error { return nil }

func (p *lifecycleTestProcessor) Stop(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	*p.stops++
	return nil
}
