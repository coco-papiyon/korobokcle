package config

import "time"

const DefaultPollInterval = 2 * time.Minute

func DefaultFiles() Files {
	return Files{
		App: App{
			HTTPPort:     8080,
			PollInterval: DefaultPollInterval,
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
					Branch:         "",
					ProjectName:    "",
					Labels:         []string{"ai:design"},
					ProjectFilters: nil,
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
					Branch:         "",
					ProjectName:    "",
					Labels:         []string{"ai:review"},
					ProjectFilters: nil,
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
					Events:  []string{"waiting_design_approval", "waiting_final_approval", "review_completed", "pr_created", "failed"},
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
