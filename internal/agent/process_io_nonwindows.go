//go:build !windows && !linux

package agent

import "context"

func startSessionProcess(ctx context.Context, cfg SessionConfig) (sessionProcess, error) {
	return startPipeSessionProcess(ctx, cfg)
}
