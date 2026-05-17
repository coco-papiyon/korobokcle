package config

import "time"

type App struct {
	HTTPPort        int            `yaml:"httpPort"`
	PollInterval    time.Duration  `yaml:"pollInterval"`
	DataDir         string         `yaml:"dataDir"`
	ArtifactsDir    string         `yaml:"artifactsDir"`
	WorkspaceDir    string         `yaml:"workspaceDir"`
	Provider        string         `yaml:"provider"`
	Model           string         `yaml:"model"`
	PRTitleTemplate string         `yaml:"prTitleTemplate"`
	BranchTemplate  string         `yaml:"branchTemplate"`
	Providers       []ProviderSpec `yaml:"providers"`
	SQLitePath      string         `yaml:"sqlitePath"`
	ShutdownTimeout time.Duration  `yaml:"shutdownTimeout"`
}

type ProviderSpec struct {
	Name   string   `yaml:"name"`
	Models []string `yaml:"models"`
}

type ProjectFieldFilter struct {
	Field  string   `yaml:"field" json:"field"`
	Values []string `yaml:"values" json:"values"`
}

type WatchRule struct {
	ID             string               `yaml:"id"`
	Name           string               `yaml:"name"`
	Repositories   []string             `yaml:"repositories"`
	Target         string               `yaml:"target"`
	Branch         string               `yaml:"branch"`
	ProjectName    string               `yaml:"projectName"`
	Labels         []string             `yaml:"labels"`
	ProjectFilters []ProjectFieldFilter `yaml:"projectFilters"`
	TitlePattern   string               `yaml:"titlePattern"`
	Authors        []string             `yaml:"authors"`
	Assignees      []string             `yaml:"assignees"`
	ExcludeDraftPR bool                 `yaml:"excludeDraftPR"`
	Provider       string               `yaml:"provider"`
	Model          string               `yaml:"model"`
	SkillSet       string               `yaml:"skillSet"`
	TestProfile    string               `yaml:"testProfile"`
	Enabled        bool                 `yaml:"enabled"`
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
