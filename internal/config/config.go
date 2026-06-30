package config

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	BaseDir               string
	ToolDir               string
	WorkDir               string
	Repository            string
	Addr                  string
	PollInterval          time.Duration
	DesignWorkers         int
	ImplementationWorkers int
	ReviewWorkers         int
}

func Default() Config {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	toolDir := wd
	if exe, err := os.Executable(); err == nil {
		toolDir = filepath.Dir(exe)
	}
	return Config{
		BaseDir:               wd,
		ToolDir:               toolDir,
		WorkDir:               toolDir,
		Repository:            "",
		Addr:                  ":8080",
		PollInterval:          120 * time.Second,
		DesignWorkers:         1,
		ImplementationWorkers: 1,
		ReviewWorkers:         1,
	}
}
