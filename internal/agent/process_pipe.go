package agent

import (
	"context"
	"os/exec"
	"strings"
)

func startPipeSessionProcess(ctx context.Context, cfg SessionConfig) (sessionProcess, error) {
	cmd := exec.CommandContext(ctx, strings.TrimSpace(cfg.Command), cfg.Args...)
	cmd.Dir = cfg.WorkDir
	if len(cfg.Env) > 0 {
		cmd.Env = append([]string(nil), cfg.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return sessionProcess{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return sessionProcess{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return sessionProcess{}, err
	}
	if err := cmd.Start(); err != nil {
		return sessionProcess{}, err
	}

	return sessionProcess{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		wait:   cmd.Wait,
		kill: func() error {
			if cmd.Process == nil {
				return nil
			}
			return cmd.Process.Kill()
		},
	}, nil
}
