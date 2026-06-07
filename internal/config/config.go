package config

import "time"

type App struct {
	HTTPPort              int                   `yaml:"httpPort"`
	PollInterval          time.Duration         `yaml:"pollInterval"`
	ScreenRefreshInterval time.Duration         `yaml:"screenRefreshInterval"`
	DataDir               string                `yaml:"dataDir"`
	ArtifactsDir          string                `yaml:"artifactsDir"`
	WorkspaceDir          string                `yaml:"workspaceDir"`
	MonitoredRepositories []MonitoredRepository `yaml:"monitoredRepositories"`
	Provider              string                `yaml:"provider"`
	Model                 string                `yaml:"model"`
	CopilotAllowTools     []string              `yaml:"copilotAllowTools"`
	PRTitleTemplate       string                `yaml:"prTitleTemplate"`
	BranchTemplate        string                `yaml:"branchTemplate"`
	SQLitePath            string                `yaml:"sqlitePath"`
	ShutdownTimeout       time.Duration         `yaml:"shutdownTimeout"`
}

type MonitoredRepository struct {
	Repository          string `yaml:"repository"`
	Branch              string `yaml:"branch"`
	WorkDir             string `yaml:"workDir"`
	Workers             int    `yaml:"workers"`
	ImprovementEnabled  bool   `yaml:"improvementEnabled"`
	ImprovementBranch   string `yaml:"improvementBranch"`
	ImprovementDir      string `yaml:"improvementDir"`
	ImprovementWorkDir  string `yaml:"improvementWorkDir"`
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
	ProjectName    string               `yaml:"projectName"`
	Labels         []string             `yaml:"labels"`
	ProjectFilters []ProjectFieldFilter `yaml:"projectFilters"`
	TitlePattern   string               `yaml:"titlePattern"`
	Authors        []string             `yaml:"authors"`
	Assignees      []string             `yaml:"assignees"`
	Reviewers      []string             `yaml:"reviewers"`
	ExcludeDraftPR bool                 `yaml:"excludeDraftPR"`
	Provider       string               `yaml:"provider"`
	Model          string               `yaml:"model"`
	SkillSet       string               `yaml:"skillSet"`
	TestProfile    string               `yaml:"testProfile"`
	ToolCommand    string               `yaml:"toolCommand"`
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
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Commands []string `yaml:"commands"`
}

type ToolCommands struct {
	Commands []ToolCommand `yaml:"commands"`
}

type ToolCommand struct {
	Name     string `yaml:"name"`
	Command  string `yaml:"command"`
	Resident bool   `yaml:"resident"`
}

type Files struct {
	App           App
	WatchRules    WatchRulesFile
	Notifications Notifications
	TestProfiles  TestProfiles
	ToolCommands  ToolCommands
}
