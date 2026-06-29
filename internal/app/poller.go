package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobSource interface {
	List(context.Context) ([]domain.Job, error)
}

type PollSettingsStore interface {
	Load(context.Context) (domain.WatchSettings, error)
}

type StaticJobSource struct {
	jobs []domain.Job
}

func NewStaticJobSource(jobs []domain.Job) *StaticJobSource {
	return &StaticJobSource{jobs: append([]domain.Job(nil), jobs...)}
}

func (s *StaticJobSource) List(context.Context) ([]domain.Job, error) {
	return append([]domain.Job(nil), s.jobs...), nil
}

type Poller struct {
	cfg      config.Config
	source   JobSource
	store    JobStore
	settings PollSettingsStore
	manager  *WorkerManager

	pollMu sync.Mutex
	mu     sync.Mutex
	seen   map[string]struct{}
}

func NewPoller(cfg config.Config, source JobSource, store JobStore, settings PollSettingsStore, manager *WorkerManager) *Poller {
	return &Poller{
		cfg:      cfg,
		source:   source,
		store:    store,
		settings: settings,
		manager:  manager,
		seen:     make(map[string]struct{}),
	}
}

func (p *Poller) Run(ctx context.Context) error {
	if err := p.PollNow(ctx); err != nil {
		return err
	}

	for {
		interval := p.pollInterval(ctx)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			if err := p.PollNow(ctx); err != nil {
				return err
			}
		}
	}
}

// PollNow runs the same repository scan used by the periodic monitor.
// Calls are serialized so approval-triggered scans cannot overlap timer scans.
func (p *Poller) PollNow(ctx context.Context) error {
	p.pollMu.Lock()
	defer p.pollMu.Unlock()
	return p.poll(ctx)
}

func (p *Poller) pollInterval(ctx context.Context) time.Duration {
	if p.settings != nil {
		if settings, err := p.settings.Load(ctx); err == nil {
			if interval := settings.PollIntervalDuration(); interval > 0 {
				return interval
			}
		}
	}
	if p.cfg.PollInterval > 0 {
		return p.cfg.PollInterval
	}
	return 120 * time.Second
}

func (p *Poller) poll(ctx context.Context) error {
	if p.source == nil || p.manager == nil {
		return nil
	}
	jobs, err := p.source.List(ctx)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	for _, job := range jobs {
		key := p.jobKey(job)
		if p.alreadySeen(key) {
			continue
		}
		if p.store != nil {
			if err := p.store.Upsert(ctx, job); err != nil {
				return fmt.Errorf("persist job %s: %w", job.ID, err)
			}
		}
		if err := p.manager.Submit(job); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) jobKey(job domain.Job) string {
	return string(job.Kind) + ":" + job.ID + ":" + string(job.State)
}

func (p *Poller) alreadySeen(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.seen[key]; ok {
		return true
	}
	p.seen[key] = struct{}{}
	return false
}
