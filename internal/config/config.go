package config

import (
	"os"
	"time"
)

type Config struct {
	BaseDir               string
	ToolDir               string
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
	return Config{
		BaseDir:               wd,
		ToolDir:               wd,
		Repository:            "",
		Addr:                  ":8080",
		PollInterval:          120 * time.Second,
		DesignWorkers:         1,
		ImplementationWorkers: 1,
		ReviewWorkers:         1,
	}
}
