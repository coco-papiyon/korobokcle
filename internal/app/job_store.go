package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobStore interface {
	List(context.Context) ([]domain.Job, error)
	Get(context.Context, string) (domain.Job, bool, error)
	Upsert(context.Context, domain.Job) error
}

type FileJobStore struct {
	path string

	mu   sync.Mutex
	jobs map[string]domain.Job
}

func NewFileJobStore(path string) (*FileJobStore, error) {
	store := &FileJobStore{
		path: path,
		jobs: make(map[string]domain.Job),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileJobStore) List(context.Context) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, job)
	}
	return out, nil
}

func (s *FileJobStore) Get(_ context.Context, id string) (domain.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	return job, ok, nil
}

func (s *FileJobStore) Upsert(_ context.Context, job domain.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("job id is required")
	}
	s.jobs[job.ID] = job
	return s.saveLocked()
}

func (s *FileJobStore) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read jobs store: %w", err)
	}

	var stored []domain.Job
	if err := json.Unmarshal(raw, &stored); err != nil {
		return fmt.Errorf("decode jobs store: %w", err)
	}
	for _, job := range stored {
		s.jobs[job.ID] = job
	}
	return nil
}

func (s *FileJobStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create job store dir: %w", err)
	}

	stored := make([]domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		stored = append(stored, job)
	}

	raw, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode jobs store: %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write jobs store: %w", err)
	}
	return nil
}
