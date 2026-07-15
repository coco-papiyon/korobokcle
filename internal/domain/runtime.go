package domain

type RuntimeStatus struct {
	Running        bool        `json:"running"`
	PID            int         `json:"pid,omitempty"`
	Command        string      `json:"command,omitempty"`
	StartupMode    StartupMode `json:"startupMode,omitempty"`
	ResidentMode   bool        `json:"residentMode,omitempty"`
	HasStopCommand bool        `json:"hasStopCommand,omitempty"`
	WorkingDir     string      `json:"workingDir,omitempty"`
	StartedAt      string      `json:"startedAt,omitempty"`
	StoppedAt      string      `json:"stoppedAt,omitempty"`
	ExitCode       *int        `json:"exitCode,omitempty"`
	Error          string      `json:"error,omitempty"`
	LogPath        string      `json:"logPath"`
}

type RuntimeLogResponse struct {
	Content   string `json:"content"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}
