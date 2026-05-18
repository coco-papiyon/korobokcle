package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type appYAML struct {
	HTTPPort              int                   `yaml:"httpPort"`
	PollInterval          int                   `yaml:"pollInterval"`
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

type pollIntervalYAML struct {
	value time.Duration
	set   bool
}

func (a App) MarshalYAML() (any, error) {
	seconds := 0
	if a.PollInterval > 0 {
		seconds = int(a.PollInterval / time.Second)
	}
	return appYAML{
		HTTPPort:              a.HTTPPort,
		PollInterval:          seconds,
		ScreenRefreshInterval: a.ScreenRefreshInterval,
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
		ShutdownTimeout:       a.ShutdownTimeout,
	}, nil
}

func (a *App) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		HTTPPort              *int                   `yaml:"httpPort"`
		PollInterval          pollIntervalYAML       `yaml:"pollInterval"`
		ScreenRefreshInterval *time.Duration         `yaml:"screenRefreshInterval"`
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
		ShutdownTimeout       *time.Duration         `yaml:"shutdownTimeout"`
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
	if raw.ScreenRefreshInterval != nil {
		a.ScreenRefreshInterval = *raw.ScreenRefreshInterval
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
	if raw.ShutdownTimeout != nil {
		a.ShutdownTimeout = *raw.ShutdownTimeout
	}

	return nil
}

func (p *pollIntervalYAML) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(value.Value)
	if trimmed == "" && value.Tag == "!!null" {
		return nil
	}

	if seconds, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		p.value = time.Duration(seconds) * time.Second
		p.set = true
		return nil
	}

	if duration, err := time.ParseDuration(trimmed); err == nil {
		p.value = duration
		p.set = true
		return nil
	}

	return fmt.Errorf("pollInterval must be a number of seconds or a duration string, got %q", value.Value)
}
