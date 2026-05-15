package github

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

type GHTokenProvider struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
	ttl       time.Duration
}

func NewGHTokenProvider(ttl time.Duration) *GHTokenProvider {
	return &GHTokenProvider{ttl: ttl}
}

func (p *GHTokenProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	if p.token != "" && time.Now().Before(p.expiresAt) {
		defer p.mu.Unlock()
		return p.token, nil
	}
	p.mu.Unlock()

	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh auth token: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	token := strings.TrimSpace(stdout.String())
	if token == "" {
		return "", fmt.Errorf("gh auth token returned an empty token")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = token
	p.expiresAt = time.Now().Add(p.ttl)
	return token, nil
}
