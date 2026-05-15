package config

import "time"

type App struct {
	HTTPPort        int           `yaml:"httpPort"`
	PollInterval    time.Duration `yaml:"pollInterval"`
	DataDir         string        `yaml:"dataDir"`
	ArtifactsDir    string        `yaml:"artifactsDir"`
	WorkspaceDir    string        `yaml:"workspaceDir"`
	Provider        string        `yaml:"provider"`
	SQLitePath      string        `yaml:"sqlitePath"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
}

type WatchRule struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Repositories   []string `yaml:"repositories"`
	Target         string   `yaml:"target"`
	Labels         []string `yaml:"labels"`
	TitlePattern   string   `yaml:"titlePattern"`
	Authors        []string `yaml:"authors"`
	Assignees      []string `yaml:"assignees"`
	ExcludeDraftPR bool     `yaml:"excludeDraftPR"`
	SkillSet       string   `yaml:"skillSet"`
	TestProfile    string   `yaml:"testProfile"`
	Enabled        bool     `yaml:"enabled"`
}

type WatchRulesFile struct {
	Rules []WatchRule `yaml:"rules"`
}

type Notifications struct {
	Channels []NotificationChannel `yaml:"channels"`
}

type NotificationChannel struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Events  []string `yaml:"events"`
	Enabled bool     `yaml:"enabled"`
}

type TestProfiles struct {
	Profiles []TestProfile `yaml:"profiles"`
}

type TestProfile struct {
	Name     string   `yaml:"name"`
	Commands []string `yaml:"commands"`
}

type Files struct {
	App           App
	WatchRules    WatchRulesFile
	Notifications Notifications
	TestProfiles  TestProfiles
}
