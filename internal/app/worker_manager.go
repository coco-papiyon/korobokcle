package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobProcessor func(context.Context, domain.Job) error

type WorkerManager struct {
	cfg       config.Config
	logger    *log.Logger
	processor JobProcessor

	mu      sync.Mutex
	queues  map[domain.JobKind]chan domain.Job
	started bool
	wg      sync.WaitGroup
}

func NewWorkerManager(cfg config.Config, logger *log.Logger, processor JobProcessor) *WorkerManager {
	if processor == nil {
		processor = func(context.Context, domain.Job) error { return nil }
	}
	return &WorkerManager{
		cfg:       cfg,
		logger:    logger,
		processor: processor,
		queues: map[domain.JobKind]chan domain.Job{
			domain.JobKindIssueDesign:         make(chan domain.Job, 32),
			domain.JobKindIssueImplementation: make(chan domain.Job, 32),
			domain.JobKindPRReview:            make(chan domain.Job, 32),
			domain.JobKindPRFeedback:          make(chan domain.Job, 32),
		},
	}
}

func (m *WorkerManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("worker manager already started")
	}
	m.started = true

	m.startPool(ctx, domain.JobKindIssueDesign, m.cfg.DesignWorkers)
	m.startPool(ctx, domain.JobKindIssueImplementation, m.cfg.ImplementationWorkers)
	m.startPool(ctx, domain.JobKindPRFeedback, m.cfg.ImplementationWorkers)
	m.startPool(ctx, domain.JobKindPRReview, m.cfg.ReviewWorkers)
	return nil
}

func (m *WorkerManager) Submit(job domain.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return errors.New("worker manager not started")
	}
	queue, ok := m.queues[job.Kind]
	if !ok {
		return fmt.Errorf("unsupported job kind: %s", job.Kind)
	}
	select {
	case queue <- job:
		return nil
	default:
		return fmt.Errorf("job queue full for kind %s", job.Kind)
	}
}

func (m *WorkerManager) Wait() {
	m.wg.Wait()
}

func (m *WorkerManager) startPool(ctx context.Context, kind domain.JobKind, limit int) {
	if limit < 1 {
		limit = 1
	}
	queue := m.queues[kind]
	for i := 0; i < limit; i++ {
		m.wg.Add(1)
		go func(workerIndex int) {
			defer m.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-queue:
					if job.Kind == "" {
						continue
					}
					if m.logger != nil {
						m.logger.Printf("worker started kind=%s worker=%d job=%s state=%s", job.Kind, workerIndex+1, job.ID, job.State)
					}
					if err := m.processor(ctx, job); err != nil && m.logger != nil {
						m.logger.Printf("worker failed kind=%s worker=%d job=%s error=%v", job.Kind, workerIndex+1, job.ID, err)
					}
				}
			}
		}(i)
	}
}
