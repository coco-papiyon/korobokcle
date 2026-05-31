package config

import "time"

const (
	DefaultPollInterval          = 2 * time.Minute
	DefaultScreenRefreshInterval = 5 * time.Second
)

var DefaultCopilotAllowTools = []string{}

func DefaultFiles() Files {
	return Files{
		App: App{
			HTTPPort:              8080,
			PollInterval:          DefaultPollInterval,
			ScreenRefreshInterval: DefaultScreenRefreshInterval,
			DataDir:               "data",
			ArtifactsDir:          "artifacts",
			WorkspaceDir:          ".workspace",
			MonitoredRepositories: []MonitoredRepository{{Repository: "owner/repository", Branch: "", Workers: 1}},
			Provider:              "mock",
			Model:                 "",
			CopilotAllowTools:     append([]string(nil), DefaultCopilotAllowTools...),
			PRTitleTemplate:       "[#{{issue_number}}]{{issue_title}}",
			BranchTemplate:        "issue_{{issue_number}}",
			SQLitePath:            "data/korobokcle.db",
			ShutdownTimeout:       10 * time.Second,
		},
		WatchRules: WatchRulesFile{
			Rules: []WatchRule{
				{
					ID:             "default-issues",
					Name:           "Default Issue Rule",
					Repositories:   []string{"owner/repository"},
					Target:         "issue",
					ProjectName:    "",
					Labels:         []string{"ai:design"},
					ProjectFilters: nil,
					ExcludeDraftPR: true,
					Provider:       "",
					Model:          "",
					SkillSet:       "default",
					TestProfile:    "go-default",
					ToolCommand:    "",
					Enabled:        false,
				},
				{
					ID:             "default-prs",
					Name:           "Default PR Rule",
					Repositories:   []string{"owner/repository"},
					Target:         "pull_request",
					ProjectName:    "",
					Labels:         []string{"ai:review"},
					ProjectFilters: nil,
					ExcludeDraftPR: true,
					Provider:       "",
					Model:          "",
					SkillSet:       "default",
					TestProfile:    "go-default",
					ToolCommand:    "",
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
					ID:       "profile-1",
					Name:     "go-default",
					Commands: []string{"go test ./..."},
				},
			},
		},
		ToolCommands: ToolCommands{
			Commands: []ToolCommand{},
		},
	}
}
