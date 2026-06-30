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

	mu          sync.Mutex
	queue       chan domain.Job
	started     bool
	ctx         context.Context
	nextWorker  int
	workerStops map[int]context.CancelFunc
	wg          sync.WaitGroup
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
		cfg:         cfg,
		logger:      logger,
		factory:     factory,
		queue:       make(chan domain.Job, 128),
		workerStops: make(map[int]context.CancelFunc),
	}
}

func (m *WorkerManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("worker manager already started")
	}
	m.started = true
	m.ctx = ctx
	m.setConcurrencyLocked(m.cfg.JobWorkers)
	return nil
}

func (m *WorkerManager) SetConcurrency(limit int) {
	if limit < 1 {
		limit = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.JobWorkers = limit
	if m.started {
		m.setConcurrencyLocked(limit)
	}
}

func (m *WorkerManager) Concurrency() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.workerStops)
}

func (m *WorkerManager) setConcurrencyLocked(limit int) {
	if limit < 1 {
		limit = 1
	}
	for len(m.workerStops) < limit {
		m.nextWorker++
		id := m.nextWorker
		workerCtx, stop := context.WithCancel(m.ctx)
		m.workerStops[id] = stop
		m.startWorker(workerCtx, id)
	}
	for len(m.workerStops) > limit {
		for id, stop := range m.workerStops {
			stop()
			delete(m.workerStops, id)
			break
		}
	}
}

func (m *WorkerManager) Submit(job domain.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return errors.New("worker manager not started")
	}
	switch job.Kind {
	case domain.JobKindIssueDesign, domain.JobKindIssueImplementation, domain.JobKindPRReview, domain.JobKindPRFeedback, domain.JobKindPRConflict:
	default:
		return fmt.Errorf("unsupported job kind: %s", job.Kind)
	}
	select {
	case m.queue <- job:
		return nil
	default:
		return errors.New("job queue full")
	}
}

func (m *WorkerManager) Wait() { m.wg.Wait() }

func (m *WorkerManager) startWorker(workerCtx context.Context, workerID int) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		processor := m.factory()
		if err := processor.Start(m.ctx); err != nil {
			if m.logger != nil {
				m.logger.Printf("worker startup failed worker=%d error=%v", workerID, err)
			}
			return
		}
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := processor.Stop(stopCtx); err != nil && m.logger != nil {
				m.logger.Printf("worker shutdown failed worker=%d error=%v", workerID, err)
			}
		}()
		for {
			select {
			case <-workerCtx.Done():
				return
			default:
			}
			select {
			case <-workerCtx.Done():
				return
			case job := <-m.queue:
				if m.logger != nil {
					m.logger.Printf("worker started kind=%s worker=%d job=%s state=%s", job.Kind, workerID, job.ID, job.State)
				}
				if err := processor.Process(m.ctx, job); err != nil && m.logger != nil {
					m.logger.Printf("worker failed kind=%s worker=%d job=%s error=%v", job.Kind, workerID, job.ID, err)
				}
			}
		}
	}()
}
