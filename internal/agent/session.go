package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const defaultIdleTimeout = 200 * time.Millisecond

type SessionConfig struct {
	Command           string
	Args              []string
	WorkDir           string
	Env               []string
	RequestTerminator string
	EndMarker         string
	IdleTimeout       time.Duration
	UsePTY            bool
}

type Response struct {
	Stdout string
	Stderr string
}

type JSONLEvent struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Stream    string `json:"stream,omitempty"`
	Data      string `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
}

type JSONLResponse struct {
	Events []JSONLEvent
	Stderr string
}

type Session struct {
	stdin       io.WriteCloser
	stdoutCh    chan streamChunk
	stderrCh    chan streamChunk
	doneCh      chan error
	idleTimeout time.Duration
	terminator  string
	endMarker   string
	kill        func() error
	cleanup     func() error

	mu     sync.Mutex
	closed bool
}

type streamChunk struct {
	text string
	err  error
}

func StartSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	proc, err := startSessionProcess(ctx, cfg)
	if err != nil {
		return nil, err
	}

	session := &Session{
		stdin:       proc.stdin,
		stdoutCh:    make(chan streamChunk, 32),
		stderrCh:    make(chan streamChunk, 32),
		doneCh:      make(chan error, 1),
		idleTimeout: cfg.IdleTimeout,
		terminator:  cfg.RequestTerminator,
		endMarker:   cfg.EndMarker,
		kill:        proc.kill,
		cleanup:     proc.cleanup,
	}
	if session.idleTimeout <= 0 {
		session.idleTimeout = defaultIdleTimeout
	}
	if session.terminator == "" {
		session.terminator = "\n"
	}

	if proc.stdout != nil {
		go session.readStream(proc.stdout, session.stdoutCh)
	} else {
		close(session.stdoutCh)
	}
	if proc.stderr != nil {
		go session.readStream(proc.stderr, session.stderrCh)
	} else {
		close(session.stderrCh)
	}
	go func() {
		session.doneCh <- proc.wait()
	}()

	return session, nil
}

func (s *Session) Send(ctx context.Context, input string) (Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.writeInput(input); err != nil {
		return Response{}, err
	}

	if s.endMarker != "" {
		return s.collectUntilMarker(ctx, input)
	}
	return s.collectUntilIdle(ctx)
}

func (s *Session) SendJSONL(ctx context.Context, input string) (JSONLResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.writeInput(input); err != nil {
		return JSONLResponse{}, err
	}

	var stderr strings.Builder
	events := make([]JSONLEvent, 0, 8)

	for {
		select {
		case chunk, ok := <-s.stdoutCh:
			if !ok {
				return JSONLResponse{Events: events, Stderr: stderr.String()}, nil
			}
			if chunk.err != nil {
				return JSONLResponse{Events: events, Stderr: stderr.String()}, chunk.err
			}
			for _, line := range splitNonEmptyLines(chunk.text) {
				var event JSONLEvent
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					return JSONLResponse{Events: events, Stderr: stderr.String()}, fmt.Errorf("decode jsonl event: %w", err)
				}
				events = append(events, event)
				if event.Type == "end" {
					s.drainAvailableStderr(&stderr)
					return JSONLResponse{Events: events, Stderr: stderr.String()}, nil
				}
			}
		case chunk, ok := <-s.stderrCh:
			if !ok {
				return JSONLResponse{Events: events, Stderr: stderr.String()}, nil
			}
			if chunk.err != nil {
				return JSONLResponse{Events: events, Stderr: stderr.String()}, chunk.err
			}
			stderr.WriteString(chunk.text)
		case err := <-s.doneCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return JSONLResponse{Events: events, Stderr: stderr.String()}, err
			}
			s.closed = true
			return JSONLResponse{Events: events, Stderr: stderr.String()}, nil
		case <-ctx.Done():
			return JSONLResponse{Events: events, Stderr: stderr.String()}, ctx.Err()
		}
	}
}

func (s *Session) writeInput(input string) error {
	if s.closed {
		return fmt.Errorf("session is closed")
	}

	if _, err := io.WriteString(s.stdin, input+s.terminator); err != nil {
		return err
	}
	return nil
}

func (s *Session) collectUntilIdle(ctx context.Context) (Response, error) {
	timer := time.NewTimer(s.idleTimeout)
	defer timer.Stop()

	var stdout strings.Builder
	var stderr strings.Builder

	for {
		select {
		case chunk, ok := <-s.stdoutCh:
			if !ok {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
			}
			if chunk.err != nil {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, chunk.err
			}
			stdout.WriteString(chunk.text)
			resetTimer(timer, s.idleTimeout)
		case chunk, ok := <-s.stderrCh:
			if !ok {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
			}
			if chunk.err != nil {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, chunk.err
			}
			stderr.WriteString(chunk.text)
			resetTimer(timer, s.idleTimeout)
		case err := <-s.doneCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, err
			}
			s.closed = true
			return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
		case <-timer.C:
			return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
		case <-ctx.Done():
			return Response{Stdout: stdout.String(), Stderr: stderr.String()}, ctx.Err()
		}
	}
}

func (s *Session) collectUntilMarker(ctx context.Context, input string) (Response, error) {
	var stdout strings.Builder
	var stderr strings.Builder
	timer := time.NewTimer(s.idleTimeout)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	var markerTimer <-chan time.Time

	for {
		select {
		case chunk, ok := <-s.stdoutCh:
			if !ok {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
			}
			if chunk.err != nil {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, chunk.err
			}
			stdout.WriteString(chunk.text)
			visibleStdout := stripLeadingEcho(stdout.String(), input, s.terminator)
			if strings.Contains(visibleStdout, s.endMarker) {
				resetTimer(timer, s.idleTimeout)
				markerTimer = timer.C
			}
		case chunk, ok := <-s.stderrCh:
			if !ok {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
			}
			if chunk.err != nil {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, chunk.err
			}
			stderr.WriteString(chunk.text)
			if markerTimer != nil {
				resetTimer(timer, s.idleTimeout)
			}
		case err := <-s.doneCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, err
			}
			s.closed = true
			return finalizeMarkedResponse(stdout.String(), stderr.String(), input, s.terminator, s.endMarker), nil
		case <-markerTimer:
			s.drainAvailableStderr(&stderr)
			return finalizeMarkedResponse(stdout.String(), stderr.String(), input, s.terminator, s.endMarker), nil
		case <-ctx.Done():
			return Response{Stdout: stdout.String(), Stderr: stderr.String()}, ctx.Err()
		}
	}
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cleanup != nil {
		defer s.cleanup()
	}
	if s.kill == nil {
		return nil
	}
	if err := s.kill(); err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	return nil
}

func (s *Session) closeInput() error {
	if s.stdin == nil {
		return nil
	}
	err := s.stdin.Close()
	s.stdin = nil
	return err
}

func (s *Session) readStream(reader io.Reader, out chan<- streamChunk) {
	defer close(out)

	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			out <- streamChunk{text: string(buffer[:n])}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			out <- streamChunk{err: err}
			return
		}
	}
}

func (s *Session) collectUntilDone(ctx context.Context) (Response, error) {
	var stdout strings.Builder
	var stderr strings.Builder
	stdoutCh := s.stdoutCh
	stderrCh := s.stderrCh
	doneSeen := false

	for stdoutCh != nil || stderrCh != nil || !doneSeen {
		select {
		case chunk, ok := <-stdoutCh:
			if !ok {
				stdoutCh = nil
				continue
			}
			if chunk.err != nil {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, chunk.err
			}
			stdout.WriteString(chunk.text)
		case chunk, ok := <-stderrCh:
			if !ok {
				stderrCh = nil
				continue
			}
			if chunk.err != nil {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, chunk.err
			}
			stderr.WriteString(chunk.text)
		case err := <-s.doneCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return Response{Stdout: stdout.String(), Stderr: stderr.String()}, err
			}
			s.closed = true
			doneSeen = true
		case <-ctx.Done():
			return Response{Stdout: stdout.String(), Stderr: stderr.String()}, ctx.Err()
		}
	}

	return Response{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

func splitNonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func (s *Session) drainAvailableStderr(stderr *strings.Builder) {
	timer := time.NewTimer(s.idleTimeout)
	defer timer.Stop()

	for {
		select {
		case chunk, ok := <-s.stderrCh:
			if !ok {
				return
			}
			if chunk.err != nil {
				return
			}
			stderr.WriteString(chunk.text)
			resetTimer(timer, s.idleTimeout)
		case <-timer.C:
			return
		}
	}
}

func finalizeMarkedResponse(stdout, stderr, input, terminator, endMarker string) Response {
	visibleStdout := stripLeadingEcho(stdout, input, terminator)
	if idx := strings.LastIndex(visibleStdout, endMarker); idx >= 0 {
		visibleStdout = visibleStdout[:idx]
	}
	return Response{
		Stdout: visibleStdout,
		Stderr: stderr,
	}
}

func stripLeadingEcho(output, input, terminator string) string {
	candidates := []string{input + terminator}
	if terminator == "" {
		candidates = append(candidates, input)
	}
	for _, candidate := range candidates {
		for _, variant := range newlineVariants(candidate) {
			if strings.HasPrefix(output, variant) {
				return output[len(variant):]
			}
		}
	}
	return output
}

func newlineVariants(value string) []string {
	variants := []string{value}

	crlf := strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\n", "\r\n")
	if crlf != value {
		variants = append(variants, crlf)
	}

	lf := strings.ReplaceAll(value, "\r\n", "\n")
	if lf != value && lf != crlf {
		variants = append(variants, lf)
	}

	return variants
}
