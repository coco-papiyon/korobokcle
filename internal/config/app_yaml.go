package config

import (
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type appYAML struct {
	HTTPPort              int                   `yaml:"httpPort"`
	PollInterval          int                   `yaml:"pollInterval"`
	ScreenRefreshInterval int                   `yaml:"screenRefreshInterval"`
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
	ShutdownTimeout       int                   `yaml:"shutdownTimeout"`
}

type integerSecondsYAML struct {
	value time.Duration
	set   bool
}

func (a App) MarshalYAML() (any, error) {
	return appYAML{
		HTTPPort:              a.HTTPPort,
		PollInterval:          durationToSeconds(a.PollInterval),
		ScreenRefreshInterval: durationToSeconds(a.ScreenRefreshInterval),
		DataDir:               a.DataDir,
		ArtifactsDir:          a.ArtifactsDir,
		WorkspaceDir:          a.WorkspaceDir,
		MonitoredRepositories: a.MonitoredRepositories,
		Provider:              a.Provider,
		Model:                 a.Model,
		CopilotAllowTools:     a.CopilotAllowTools,
		PRTitleTemplate:       a.PRTitleTemplate,
		BranchTemplate:        a.BranchTemplate,
		SQLitePath:            a.SQLitePath,
		ShutdownTimeout:       durationToSeconds(a.ShutdownTimeout),
	}, nil
}

func (a *App) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		HTTPPort              *int                   `yaml:"httpPort"`
		PollInterval          integerSecondsYAML     `yaml:"pollInterval"`
		ScreenRefreshInterval integerSecondsYAML     `yaml:"screenRefreshInterval"`
		DataDir               *string                `yaml:"dataDir"`
		ArtifactsDir          *string                `yaml:"artifactsDir"`
		WorkspaceDir          *string                `yaml:"workspaceDir"`
		MonitoredRepositories *[]MonitoredRepository `yaml:"monitoredRepositories"`
		Provider              *string                `yaml:"provider"`
		Model                 *string                `yaml:"model"`
		CopilotAllowTools     *[]string              `yaml:"copilotAllowTools"`
		PRTitleTemplate       *string                `yaml:"prTitleTemplate"`
		BranchTemplate        *string                `yaml:"branchTemplate"`
		SQLitePath            *string                `yaml:"sqlitePath"`
		ShutdownTimeout       integerSecondsYAML     `yaml:"shutdownTimeout"`
	}

	if err := value.Decode(&raw); err != nil {
		return err
	}

	if raw.HTTPPort != nil {
		a.HTTPPort = *raw.HTTPPort
	}
	if raw.PollInterval.set {
		a.PollInterval = raw.PollInterval.value
	}
	if raw.ScreenRefreshInterval.set {
		a.ScreenRefreshInterval = raw.ScreenRefreshInterval.value
	}
	if raw.DataDir != nil {
		a.DataDir = *raw.DataDir
	}
	if raw.ArtifactsDir != nil {
		a.ArtifactsDir = *raw.ArtifactsDir
	}
	if raw.WorkspaceDir != nil {
		a.WorkspaceDir = *raw.WorkspaceDir
	}
	if raw.MonitoredRepositories != nil {
		a.MonitoredRepositories = append([]MonitoredRepository(nil), (*raw.MonitoredRepositories)...)
	}
	if raw.Provider != nil {
		a.Provider = *raw.Provider
	}
	if raw.Model != nil {
		a.Model = *raw.Model
	}
	if raw.CopilotAllowTools != nil {
		a.CopilotAllowTools = append([]string(nil), (*raw.CopilotAllowTools)...)
	}
	if raw.PRTitleTemplate != nil {
		a.PRTitleTemplate = *raw.PRTitleTemplate
	}
	if raw.BranchTemplate != nil {
		a.BranchTemplate = *raw.BranchTemplate
	}
	if raw.SQLitePath != nil {
		a.SQLitePath = *raw.SQLitePath
	}
	if raw.ShutdownTimeout.set {
		a.ShutdownTimeout = raw.ShutdownTimeout.value
	}

	return nil
}

func (p *integerSecondsYAML) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Tag == "!!null" {
		return nil
	}

	seconds, err := strconv.ParseInt(value.Value, 10, 64)
	if err != nil || seconds < 0 {
		return nil
	}

	p.value = time.Duration(seconds) * time.Second
	p.set = true
	return nil
}

func durationToSeconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Second)
}
