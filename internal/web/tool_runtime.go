package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

const (
	toolLogFileName  = "tool.log"
	toolMetaFileName = "tool-run.json"
)

type toolRuntimeManager struct {
	mu   sync.Mutex
	runs map[string]*toolRunState
}

type toolRunState struct {
	jobID     string
	toolName  string
	command   string
	resident  bool
	artifact  string
	workDir   string
	logPath   string
	metaPath  string
	startedAt time.Time
	finished  time.Time
	running   bool
	exitCode  *int
	cmd       *exec.Cmd
	done      chan struct{}
}

type toolRunMetadata struct {
	ToolName   string `json:"toolName"`
	Command    string `json:"command"`
	Resident   bool   `json:"resident"`
	Running    bool   `json:"running"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	ExitCode   *int   `json:"exitCode,omitempty"`
	LogPath    string `json:"logPath,omitempty"`
}

func newToolRuntimeManager() *toolRuntimeManager {
	return &toolRuntimeManager{
		runs: make(map[string]*toolRunState),
	}
}

func (m *toolRuntimeManager) start(ctx context.Context, cfg *config.Service, job domain.Job, events []domain.Event, tool config.ToolCommand) error {
	artifactDir := resolveTestReportArtifactDir(cfg, job.ID, events)
	workDir, err := resolveJobToolWorkDir(cfg, job)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}

	logPath := filepath.Join(artifactDir, toolLogFileName)
	metaPath := filepath.Join(artifactDir, toolMetaFileName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	cmd := shellExecCommand(ctx, tool.Command)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	m.mu.Lock()
	if running := m.runs[job.ID]; running != nil && running.running {
		m.mu.Unlock()
		_ = logFile.Close()
		return fmt.Errorf("tool command %q is already running", running.toolName)
	}

	state := &toolRunState{
		jobID:     job.ID,
		toolName:  tool.Name,
		command:   tool.Command,
		resident:  tool.Resident,
		artifact:  artifactDir,
		workDir:   workDir,
		logPath:   logPath,
		metaPath:  metaPath,
		startedAt: time.Now().UTC(),
		running:   true,
		cmd:       cmd,
		done:      make(chan struct{}),
	}
	m.runs[job.ID] = state
	m.mu.Unlock()

	if err := writeToolRunMetadata(metaPath, state); err != nil {
		_ = logFile.Close()
		m.clear(job.ID, state)
		return err
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		m.finishWithError(job.ID, state, -1)
		return fmt.Errorf("start tool command: %w", err)
	}

	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		m.finish(job.ID, state, exitCode)
	}()

	return nil
}

func (m *toolRuntimeManager) stop(jobID string) error {
	m.mu.Lock()
	state := m.runs[jobID]
	if state == nil || !state.running || state.cmd == nil || state.cmd.Process == nil {
		m.mu.Unlock()
		return fmt.Errorf("tool command is not running")
	}
	done := state.done
	process := state.cmd.Process
	m.mu.Unlock()

	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop tool command: %w", err)
	}

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return nil
	}
}

func (m *toolRuntimeManager) snapshot(cfg *config.Service, job domain.Job, events []domain.Event, tool config.ToolCommand) (*toolExecutionResponse, error) {
	artifactDir := resolveTestReportArtifactDir(cfg, job.ID, events)
	metaPath := filepath.Join(artifactDir, toolMetaFileName)
	logPath := filepath.Join(artifactDir, toolLogFileName)

	metadata, err := readToolRunMetadata(metaPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	m.mu.Lock()
	live := m.runs[job.ID]
	m.mu.Unlock()

	if metadata == nil && live == nil {
		return nil, nil
	}
	if metadata != nil && metadata.ToolName != "" && metadata.ToolName != tool.Name {
		metadata = nil
	}

	out := &toolExecutionResponse{
		Name:     tool.Name,
		Resident: tool.Resident,
	}
	if metadata != nil {
		out.Running = metadata.Running
		out.StartedAt = metadata.StartedAt
		out.FinishedAt = metadata.FinishedAt
		out.ExitCode = metadata.ExitCode
	}
	if live != nil && live.toolName == tool.Name {
		out.Running = live.running
		out.StartedAt = live.startedAt.Format(timeFormat)
		if !live.finished.IsZero() {
			out.FinishedAt = live.finished.Format(timeFormat)
		}
		out.ExitCode = live.exitCode
	}

	rawLog, err := os.ReadFile(logPath)
	if err == nil {
		out.Log = &artifactResponse{
			Path:    displayPathAgainstRoot(cfg.Root(), logPath),
			Content: string(rawLog),
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return out, nil
}

func (m *toolRuntimeManager) clear(jobID string, state *toolRunState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.runs[jobID]; current == state {
		delete(m.runs, jobID)
	}
}

func (m *toolRuntimeManager) finishWithError(jobID string, state *toolRunState, exitCode int) {
	m.mu.Lock()
	state.running = false
	state.finished = time.Now().UTC()
	state.exitCode = &exitCode
	m.mu.Unlock()
	_ = writeToolRunMetadata(state.metaPath, state)
	close(state.done)
}

func (m *toolRuntimeManager) finish(jobID string, state *toolRunState, exitCode int) {
	m.mu.Lock()
	state.running = false
	state.finished = time.Now().UTC()
	state.exitCode = &exitCode
	state.cmd = nil
	m.mu.Unlock()
	_ = writeToolRunMetadata(state.metaPath, state)
	close(state.done)
}

func writeToolRunMetadata(path string, state *toolRunState) error {
	payload := toolRunMetadata{
		ToolName: state.toolName,
		Command:  state.command,
		Resident: state.resident,
		Running:  state.running,
		LogPath:  state.logPath,
		ExitCode: state.exitCode,
	}
	if !state.startedAt.IsZero() {
		payload.StartedAt = state.startedAt.Format(timeFormat)
	}
	if !state.finished.IsZero() {
		payload.FinishedAt = state.finished.Format(timeFormat)
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func readToolRunMetadata(path string) (*toolRunMetadata, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload toolRunMetadata
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func shellExecCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "pwsh", "-NoProfile", "-Command", command)
	}
	return exec.CommandContext(ctx, "sh", "-lc", command)
}

func resolveJobToolWorkDir(cfg *config.Service, job domain.Job) (string, error) {
	workers := 1
	found := false
	for _, repository := range cfg.App().MonitoredRepositories {
		if canonicalRepositoryID(repository.Repository) != canonicalRepositoryID(job.Repository) {
			continue
		}
		found = true
		if repository.Workers > 0 {
			workers = repository.Workers
		}
		break
	}
	if !found {
		return "", fmt.Errorf("repository %q is not registered", job.Repository)
	}
	workerIndex := assignedWorkerIndex(job, job.Repository, workers)
	repoDir := artifacts.RepositoryWorkerSourceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, workerIndex)
	info, err := os.Stat(repoDir)
	if err != nil {
		return "", fmt.Errorf("tool workspace is not available: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("tool workspace is not a directory: %s", repoDir)
	}
	return repoDir, nil
}

func assignedWorkerIndex(job domain.Job, repository string, workerCount int) int {
	if workerCount <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = io.WriteString(h, canonicalRepositoryID(repository))
	_, _ = h.Write([]byte{':'})
	_, _ = io.WriteString(h, job.ID)
	return int(h.Sum32() % uint32(workerCount))
}

func canonicalRepositoryID(repository string) string {
	trimmed := strings.TrimSpace(repository)
	if trimmed == "" {
		return ""
	}

	candidate := strings.TrimSuffix(trimmed, ".git")
	if strings.HasPrefix(candidate, "git@") {
		if idx := strings.LastIndex(candidate, ":"); idx >= 0 && idx+1 < len(candidate) {
			candidate = candidate[idx+1:]
		}
	}
	if strings.Contains(candidate, "://") {
		if parsed, err := url.Parse(candidate); err == nil {
			candidate = strings.Trim(parsed.Path, "/")
		}
	}

	candidate = strings.Trim(path.Clean(strings.ReplaceAll(candidate, "\\", "/")), "/")
	parts := strings.Split(candidate, "/")
	if len(parts) >= 2 {
		candidate = strings.Join(parts[len(parts)-2:], "/")
	}
	return strings.ToLower(candidate)
}

func displayPathAgainstRoot(root string, value string) string {
	cleanRoot := filepath.Clean(root)
	cleanValue := filepath.Clean(value)
	rel, err := filepath.Rel(cleanRoot, cleanValue)
	if err == nil && rel == "." {
		return "."
	}
	if err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(cleanValue)
}
