package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	appPath           = "config/app.yaml"
	watchRulesPath    = "config/watch-rules.yaml"
	notificationsPath = "config/notifications.yaml"
	testProfilesPath  = "config/test-profiles.yaml"
)

func LoadOrInit(root string) (Files, error) {
	files := DefaultFiles()
	if err := ensureDefaults(root, files); err != nil {
		return Files{}, err
	}

	if err := loadYAML(filepath.Join(root, appPath), &files.App); err != nil {
		return Files{}, fmt.Errorf("load app config: %w", err)
	}
	if err := loadYAML(filepath.Join(root, watchRulesPath), &files.WatchRules); err != nil {
		return Files{}, fmt.Errorf("load watch rules: %w", err)
	}
	if err := loadYAML(filepath.Join(root, notificationsPath), &files.Notifications); err != nil {
		return Files{}, fmt.Errorf("load notifications: %w", err)
	}
	if err := loadYAML(filepath.Join(root, testProfilesPath), &files.TestProfiles); err != nil {
		return Files{}, fmt.Errorf("load test profiles: %w", err)
	}
	return files, nil
}

func ensureDefaults(root string, files Files) error {
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		return err
	}

	targets := []struct {
		path string
		data any
	}{
		{appPath, files.App},
		{watchRulesPath, files.WatchRules},
		{notificationsPath, files.Notifications},
		{testProfilesPath, files.TestProfiles},
	}

	for _, target := range targets {
		fullPath := filepath.Join(root, target.path)
		if _, err := os.Stat(fullPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}

		raw, err := yaml.Marshal(target.data)
		if err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadYAML(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(raw, out)
}

func saveYAML(path string, value any) error {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
