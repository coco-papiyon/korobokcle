package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobProcessor func(context.Context, domain.Job) error

type WorkerProcessor interface {
	Start(context.Context) error
	Process(context.Context, domain.Job) error
	Stop(context.Context) error
}

type WorkerProcessorFactory func() WorkerProcessor

type functionWorkerProcessor struct{ process JobProcessor }

func (p *functionWorkerProcessor) Start(context.Context) error { return nil }
func (p *functionWorkerProcessor) Process(ctx context.Context, job domain.Job) error {
	return p.process(ctx, job)
}
func (p *functionWorkerProcessor) Stop(context.Context) error { return nil }

type WorkerManager struct {
	cfg     config.Config
	logger  *log.Logger
	factory WorkerProcessorFactory

	mu      sync.Mutex
	queues  map[domain.JobKind]chan domain.Job
	started bool
	wg      sync.WaitGroup
}

func NewWorkerManager(cfg config.Config, logger *log.Logger, processor JobProcessor) *WorkerManager {
	if processor == nil {
		processor = func(context.Context, domain.Job) error { return nil }
	}
	return NewWorkerManagerWithFactory(cfg, logger, func() WorkerProcessor {
		return &functionWorkerProcessor{process: processor}
	})
}

func NewWorkerManagerWithFactory(cfg config.Config, logger *log.Logger, factory WorkerProcessorFactory) *WorkerManager {
	return &WorkerManager{
		cfg:     cfg,
		logger:  logger,
		factory: factory,
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
			processor := m.factory()
			if err := processor.Start(ctx); err != nil {
				if m.logger != nil {
					m.logger.Printf("worker startup failed kind=%s worker=%d error=%v", kind, workerIndex+1, err)
				}
				return
			}
			defer func() {
				stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := processor.Stop(stopCtx); err != nil && m.logger != nil {
					m.logger.Printf("worker shutdown failed kind=%s worker=%d error=%v", kind, workerIndex+1, err)
				}
			}()
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
					if err := processor.Process(ctx, job); err != nil && m.logger != nil {
						m.logger.Printf("worker failed kind=%s worker=%d job=%s error=%v", job.Kind, workerIndex+1, job.ID, err)
					}
				}
			}
		}(i)
	}
}
