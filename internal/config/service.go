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

func (s *Service) ToolCommands() ToolCommands {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneToolCommands(s.files.ToolCommands)
}

func (s *Service) Notifications() Notifications {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneNotifications(s.files.Notifications)
}

func (s *Service) Providers() []ProviderSpec {
	return ProviderCatalog()
}

func (s *Service) ProviderByName(name string) (ProviderSpec, bool) {
	return ProviderSpecByName(name)
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

func (s *Service) UpdateTestProfiles(file TestProfiles) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := saveYAML(filepath.Join(s.root, testProfilesPath), file); err != nil {
		return err
	}
	s.files.TestProfiles = cloneTestProfiles(file)
	return nil
}

func (s *Service) UpdateToolCommands(file ToolCommands) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := saveYAML(filepath.Join(s.root, toolCommandsPath), file); err != nil {
		return err
	}
	s.files.ToolCommands = cloneToolCommands(file)
	return nil
}

func cloneFiles(files Files) Files {
	files.App = cloneApp(files.App)
	files.WatchRules = cloneWatchRulesFile(files.WatchRules)
	files.Notifications = cloneNotifications(files.Notifications)
	files.TestProfiles = cloneTestProfiles(files.TestProfiles)
	files.ToolCommands = cloneToolCommands(files.ToolCommands)
	return files
}

func cloneApp(app App) App {
	cloned := app
	cloned.MonitoredRepositories = cloneMonitoredRepositories(app.MonitoredRepositories)
	cloned.CopilotAllowTools = append([]string(nil), app.CopilotAllowTools...)
	return cloned
}

func cloneMonitoredRepositories(values []MonitoredRepository) []MonitoredRepository {
	cloned := make([]MonitoredRepository, 0, len(values))
	for _, repository := range values {
		cloned = append(cloned, MonitoredRepository{
			Repository:         repository.Repository,
			Branch:             repository.Branch,
			WorkDir:            repository.WorkDir,
			Workers:            repository.Workers,
			ImprovementEnabled: repository.ImprovementEnabled,
			ImprovementBranch:  repository.ImprovementBranch,
			ImprovementDir:     repository.ImprovementDir,
			ImprovementWorkDir: repository.ImprovementWorkDir,
		})
	}
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
	cloned.ProjectFilters = make([]ProjectFieldFilter, 0, len(rule.ProjectFilters))
	for _, filter := range rule.ProjectFilters {
		cloned.ProjectFilters = append(cloned.ProjectFilters, ProjectFieldFilter{
			Field:  filter.Field,
			Values: append([]string(nil), filter.Values...),
		})
	}
	cloned.Authors = append([]string(nil), rule.Authors...)
	cloned.Assignees = append([]string(nil), rule.Assignees...)
	cloned.Reviewers = append([]string(nil), rule.Reviewers...)
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
	cloned.Models = make([]string, 0, len(provider.Models))
	cloned.Models = append(cloned.Models, provider.Models...)
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
			ID:       profile.ID,
			Name:     profile.Name,
			Commands: append([]string(nil), profile.Commands...),
		})
	}
	return cloned
}

func cloneToolCommands(file ToolCommands) ToolCommands {
	cloned := ToolCommands{
		Commands: make([]ToolCommand, 0, len(file.Commands)),
	}
	for _, command := range file.Commands {
		cloned.Commands = append(cloned.Commands, ToolCommand{
			Name:     command.Name,
			Command:  command.Command,
			Resident: command.Resident,
		})
	}
	return cloned
}
