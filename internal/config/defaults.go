package config

import "time"

func DefaultFiles() Files {
	return Files{
		App: App{
			HTTPPort:     8080,
			PollInterval: 2 * time.Minute,
			DataDir:      "data",
			ArtifactsDir: "artifacts",
			WorkspaceDir: ".",
			Provider:     "mock",
			Model:        "",
			Providers: []ProviderSpec{
				{
					Name:   "mock",
					Models: []string{},
				},
				{
					Name:   "copilot",
					Models: []string{"gpt-4.1", "gpt-4o", "o4-mini"},
				},
				{
					Name:   "codex",
					Models: []string{"gpt-4.1", "gpt-4o", "o4-mini"},
				},
			},
			SQLitePath:      "data/korobokcle.db",
			ShutdownTimeout: 10 * time.Second,
		},
		WatchRules: WatchRulesFile{
			Rules: []WatchRule{
				{
					ID:             "default-issues",
					Name:           "Default Issue Rule",
					Repositories:   []string{"owner/repository"},
					Target:         "issue",
					Labels:         []string{"ai:design"},
					ExcludeDraftPR: true,
					Provider:       "",
					Model:          "",
					SkillSet:       "default",
					TestProfile:    "go-default",
					Enabled:        false,
				},
				{
					ID:             "default-prs",
					Name:           "Default PR Rule",
					Repositories:   []string{"owner/repository"},
					Target:         "pull_request",
					Labels:         []string{"ai:review"},
					ExcludeDraftPR: true,
					Provider:       "",
					Model:          "",
					SkillSet:       "default",
					TestProfile:    "go-default",
					Enabled:        false,
				},
			},
		},
		Notifications: Notifications{
			Channels: []NotificationChannel{
				{
					Name:    "windows-toast",
					Type:    "windows_toast",
					Events:  []string{"design_ready", "waiting_design_approval", "implementation_ready", "review_ready", "review_completed", "failed"},
					Enabled: true,
				},
			},
		},
		TestProfiles: TestProfiles{
			Profiles: []TestProfile{
				{
					Name:     "go-default",
					Commands: []string{"go test ./..."},
				},
			},
		},
	}
}
