package config

import (
	"path/filepath"
	"sync"
)

type Service struct {
	root  string
	mu    sync.RWMutex
	files Files
}

func NewService(root string, files Files) *Service {
	return &Service{
		root:  root,
		files: cloneFiles(files),
	}
}

func (s *Service) App() App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneApp(s.files.App)
}

func (s *Service) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

func (s *Service) WatchRules() WatchRulesFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWatchRulesFile(s.files.WatchRules)
}

func (s *Service) TestProfiles() TestProfiles {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneTestProfiles(s.files.TestProfiles)
}

func (s *Service) Notifications() Notifications {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneNotifications(s.files.Notifications)
}

func (s *Service) Providers() []ProviderSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneProviderSpecs(s.files.App.Providers)
}

func (s *Service) ProviderByName(name string) (ProviderSpec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, provider := range s.files.App.Providers {
		if provider.Name == name {
			return cloneProviderSpec(provider), true
		}
	}
	return ProviderSpec{}, false
}

func (s *Service) WatchRuleByID(id string) (WatchRule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rule := range s.files.WatchRules.Rules {
		if rule.ID == id {
			return cloneWatchRule(rule), true
		}
	}
	return WatchRule{}, false
}

func (s *Service) UpdateWatchRules(file WatchRulesFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := saveYAML(filepath.Join(s.root, watchRulesPath), file); err != nil {
		return err
	}
	s.files.WatchRules = cloneWatchRulesFile(file)
	return nil
}

func (s *Service) UpdateApp(app App) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := saveYAML(filepath.Join(s.root, appPath), app); err != nil {
		return err
	}
	s.files.App = cloneApp(app)
	return nil
}

func (s *Service) UpdateNotifications(file Notifications) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := saveYAML(filepath.Join(s.root, notificationsPath), file); err != nil {
		return err
	}
	s.files.Notifications = cloneNotifications(file)
	return nil
}

func cloneFiles(files Files) Files {
	files.App = cloneApp(files.App)
	files.WatchRules = cloneWatchRulesFile(files.WatchRules)
	files.Notifications = cloneNotifications(files.Notifications)
	files.TestProfiles = cloneTestProfiles(files.TestProfiles)
	return files
}

func cloneApp(app App) App {
	cloned := app
	cloned.Providers = cloneProviderSpecs(app.Providers)
	return cloned
}

func cloneWatchRulesFile(file WatchRulesFile) WatchRulesFile {
	cloned := WatchRulesFile{
		Rules: make([]WatchRule, 0, len(file.Rules)),
	}
	for _, rule := range file.Rules {
		cloned.Rules = append(cloned.Rules, cloneWatchRule(rule))
	}
	return cloned
}

func cloneWatchRule(rule WatchRule) WatchRule {
	cloned := rule
	cloned.Repositories = append([]string(nil), rule.Repositories...)
	cloned.Labels = append([]string(nil), rule.Labels...)
	cloned.Authors = append([]string(nil), rule.Authors...)
	cloned.Assignees = append([]string(nil), rule.Assignees...)
	return cloned
}

func cloneProviderSpecs(values []ProviderSpec) []ProviderSpec {
	cloned := make([]ProviderSpec, 0, len(values))
	for _, provider := range values {
		cloned = append(cloned, cloneProviderSpec(provider))
	}
	return cloned
}

func cloneProviderSpec(provider ProviderSpec) ProviderSpec {
	cloned := provider
	cloned.Models = append([]string(nil), provider.Models...)
	return cloned
}

func cloneNotifications(file Notifications) Notifications {
	cloned := Notifications{
		Channels: make([]NotificationChannel, 0, len(file.Channels)),
	}
	for _, channel := range file.Channels {
		cloned.Channels = append(cloned.Channels, NotificationChannel{
			Name:    channel.Name,
			Type:    channel.Type,
			Events:  append([]string(nil), channel.Events...),
			Enabled: channel.Enabled,
		})
	}
	return cloned
}

func cloneTestProfiles(file TestProfiles) TestProfiles {
	cloned := TestProfiles{
		Profiles: make([]TestProfile, 0, len(file.Profiles)),
	}
	for _, profile := range file.Profiles {
		cloned.Profiles = append(cloned.Profiles, TestProfile{
			Name:     profile.Name,
			Commands: append([]string(nil), profile.Commands...),
		})
	}
	return cloned
}
