package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type DesignFeedbackStore interface {
	Load(context.Context, string) (string, bool, error)
	Save(context.Context, string, string) error
	Delete(context.Context, string) error
}

type FileDesignFeedbackStore struct {
	root string
	mu   sync.Mutex
}

func NewFileDesignFeedbackStore(root string) *FileDesignFeedbackStore {
	return &FileDesignFeedbackStore{root: root}
}

func (s *FileDesignFeedbackStore) Load(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathForID(id)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read design feedback: %w", err)
	}
	return string(raw), true, nil
}

func (s *FileDesignFeedbackStore) Save(_ context.Context, id, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathForID(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create design feedback dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write design feedback: %w", err)
	}
	return nil
}

func (s *FileDesignFeedbackStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathForID(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete design feedback: %w", err)
	}
	return nil
}

func (s *FileDesignFeedbackStore) pathForID(id string) string {
	return filepath.Join(s.root, sanitizePart(id)+".md")
}
