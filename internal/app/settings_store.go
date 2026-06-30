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

type SettingsStore interface {
	Load(context.Context) (domain.WatchSettings, error)
	Save(context.Context, domain.WatchSettings) error
}

type FileSettingsStore struct {
	path     string
	defaults domain.WatchSettings

	mu       sync.Mutex
	settings domain.WatchSettings
	onSave   func(domain.WatchSettings)
}

func NewFileSettingsStore(path string, defaults domain.WatchSettings) (*FileSettingsStore, error) {
	defaults = domain.NormalizeWatchSettings(defaults)
	store := &FileSettingsStore{
		path:     path,
		defaults: defaults,
		settings: defaults,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	if err := store.saveLocked(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileSettingsStore) Load(context.Context) (domain.WatchSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = domain.NormalizeWatchSettings(s.settings)
	return s.settings, nil
}

func (s *FileSettingsStore) Save(_ context.Context, settings domain.WatchSettings) error {
	s.mu.Lock()
	s.settings = domain.NormalizeWatchSettings(settings)
	err := s.saveLocked()
	onSave := s.onSave
	saved := s.settings
	s.mu.Unlock()
	if err == nil && onSave != nil {
		onSave(saved)
	}
	return err
}

func (s *FileSettingsStore) SetOnSave(onSave func(domain.WatchSettings)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSave = onSave
}

func (s *FileSettingsStore) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read settings store: %w", err)
	}

	if err := json.Unmarshal(raw, &s.settings); err != nil {
		return fmt.Errorf("decode settings store: %w", err)
	}
	s.settings = domain.NormalizeWatchSettings(s.settings)
	return nil
}

func (s *FileSettingsStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create settings store dir: %w", err)
	}
	raw, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings store: %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write settings store: %w", err)
	}
	return nil
}
