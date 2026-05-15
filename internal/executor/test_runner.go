package executor

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type TestProfile struct {
	Name     string   `json:"name"`
	Commands []string `json:"commands"`
}

type CommandResult struct {
	Command    string `json:"command"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Success    bool   `json:"success"`
}

type TestReport struct {
	Profile    string          `json:"profile"`
	Success    bool            `json:"success"`
	StartedAt  time.Time       `json:"startedAt"`
	FinishedAt time.Time       `json:"finishedAt"`
	Results    []CommandResult `json:"results"`
}

type TestRunner struct{}

func NewTestRunner() *TestRunner {
	return &TestRunner{}
}

func (r *TestRunner) Run(ctx context.Context, profile TestProfile, workDir string) TestReport {
	report := TestReport{
		Profile:   profile.Name,
		Success:   true,
		StartedAt: time.Now().UTC(),
		Results:   make([]CommandResult, 0, len(profile.Commands)),
	}

	for _, command := range profile.Commands {
		started := time.Now()
		result := CommandResult{
			Command: command,
		}

		cmd := shellCommand(ctx, command)
		cmd.Dir = workDir
		stdout, stderr, exitCode, err := runCommand(cmd)
		result.Stdout = stdout
		result.Stderr = stderr
		result.ExitCode = exitCode
		result.DurationMS = time.Since(started).Milliseconds()
		result.Success = err == nil
		report.Results = append(report.Results, result)

		if err != nil {
			report.Success = false
			break
		}
	}

	report.FinishedAt = time.Now().UTC()
	return report
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "pwsh", "-NoProfile", "-Command", command)
	}
	return exec.CommandContext(ctx, "sh", "-lc", command)
}

func runCommand(cmd *exec.Cmd) (stdout string, stderr string, exitCode int, err error) {
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		return stdout, stderr, exitCode, err
	}
	return stdout, stderr, exitCode, nil
}
