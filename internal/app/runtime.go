package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/coco-papiyon/korobokcle/internal/domain"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type RuntimeActions interface {
	Status(context.Context, string) (domain.RuntimeStatus, error)
	Start(context.Context, string) (domain.RuntimeStatus, error)
	Stop(context.Context, string) (domain.RuntimeStatus, error)
	Logs(context.Context, string) (domain.RuntimeLogResponse, error)
}

type runtimeProcess struct {
	cmd      *exec.Cmd
	logFile  *os.File
	logPath  string
	status   domain.RuntimeStatus
	stopping bool
	waitDone chan struct{}
}

type RuntimeController struct {
	baseDir  string
	toolDir  string
	store    JobStore
	settings SettingsStore
	logger   workflowLogger

	mu        sync.Mutex
	processes map[string]*runtimeProcess
}

func NewRuntimeController(baseDir, toolDir string, store JobStore, settings SettingsStore, logger workflowLogger) *RuntimeController {
	return &RuntimeController{
		baseDir:   baseDir,
		toolDir:   toolDir,
		store:     store,
		settings:  settings,
		logger:    logger,
		processes: map[string]*runtimeProcess{},
	}
}

func (r *RuntimeController) Status(ctx context.Context, jobID string) (domain.RuntimeStatus, error) {
	job, settings, err := r.loadRuntimeContext(ctx, jobID)
	if err != nil {
		return domain.RuntimeStatus{}, err
	}
	baseStatus := r.baseStatus(job, settings)

	r.mu.Lock()
	process, ok := r.processes[job.ID]
	if ok {
		status := process.status
		if strings.TrimSpace(status.Command) == "" {
			status.Command = baseStatus.Command
		}
		if strings.TrimSpace(status.WorkingDir) == "" {
			status.WorkingDir = baseStatus.WorkingDir
		}
		if strings.TrimSpace(status.LogPath) == "" {
			status.LogPath = baseStatus.LogPath
		}
		status.StartupMode = baseStatus.StartupMode
		status.ResidentMode = baseStatus.ResidentMode
		status.HasStopCommand = baseStatus.HasStopCommand
		r.mu.Unlock()
		return status, nil
	}
	r.mu.Unlock()
	return baseStatus, nil
}

func (r *RuntimeController) Start(ctx context.Context, jobID string) (domain.RuntimeStatus, error) {
	job, settings, err := r.loadRuntimeContext(ctx, jobID)
	if err != nil {
		return domain.RuntimeStatus{}, err
	}
	command := strings.TrimSpace(settings.StartupCommand)
	if command == "" {
		return domain.RuntimeStatus{}, fmt.Errorf("startup command is required")
	}
	startupMode := settings.StartupMode
	if !startupMode.IsValid() {
		startupMode = domain.StartupModeOneShot
	}

	workDir, err := r.runtimeWorkDirForJob(ctx, job, settings)
	if err != nil {
		return domain.RuntimeStatus{}, err
	}
	logPath := runtimeLogPath(r.toolDir, job)
	displayLogPath := runtimeLogDisplayPath(job)

	r.mu.Lock()
	process, ok := r.processes[job.ID]
	if ok && process.status.Running {
		status := process.status
		r.mu.Unlock()
		return status, fmt.Errorf("runtime process already running")
	}
	r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return domain.RuntimeStatus{}, fmt.Errorf("create runtime log dir: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return domain.RuntimeStatus{}, fmt.Errorf("open runtime log: %w", err)
	}

	cmd, err := buildRuntimeCommand(command, workDir)
	if err != nil {
		_ = logFile.Close()
		return domain.RuntimeStatus{}, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return domain.RuntimeStatus{}, fmt.Errorf("start runtime command: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := domain.RuntimeStatus{
		Running:        true,
		PID:            cmd.Process.Pid,
		Command:        command,
		StartupMode:    startupMode,
		ResidentMode:   startupMode == domain.StartupModeResident,
		HasStopCommand: strings.TrimSpace(settings.StopCommand) != "",
		WorkingDir:     runtimeWorkingDirDisplayPath(job),
		StartedAt:      now,
		LogPath:        displayLogPath,
	}

	r.mu.Lock()
	r.processes[job.ID] = &runtimeProcess{
		cmd:      cmd,
		logFile:  logFile,
		logPath:  logPath,
		status:   status,
		waitDone: make(chan struct{}),
	}
	r.mu.Unlock()

	go r.waitForExit(job.ID, cmd, startupMode)
	return status, nil
}

func (r *RuntimeController) Stop(ctx context.Context, jobID string) (domain.RuntimeStatus, error) {
	job, settings, err := r.loadRuntimeContext(ctx, jobID)
	if err != nil {
		return domain.RuntimeStatus{}, err
	}
	workDir, err := r.runtimeWorkDirForJob(ctx, job, settings)
	if err != nil {
		return domain.RuntimeStatus{}, err
	}
	startupMode := settings.StartupMode
	if !startupMode.IsValid() {
		startupMode = domain.StartupModeOneShot
	}
	stopCommand := strings.TrimSpace(settings.StopCommand)

	r.mu.Lock()
	process, ok := r.processes[job.ID]
	status := r.baseStatus(job, settings)
	if ok {
		status = process.status
		if strings.TrimSpace(status.Command) == "" {
			status.Command = strings.TrimSpace(settings.StartupCommand)
		}
		if strings.TrimSpace(status.LogPath) == "" {
			status.LogPath = runtimeLogDisplayPath(job)
		}
		status.StartupMode = startupMode
		status.ResidentMode = startupMode == domain.StartupModeResident
		status.HasStopCommand = stopCommand != ""
	}
	if ok && process != nil && process.cmd != nil && process.cmd.Process != nil {
		process.stopping = true
	}
	r.mu.Unlock()

	if stopCommand != "" {
		if err := executeRuntimeCommand(stopCommand, workDir, runtimeLogPath(r.toolDir, job)); err != nil {
			return status, err
		}
		r.mu.Lock()
		process = r.processes[job.ID]
		if process != nil {
			if process.cmd != nil && process.cmd.Process != nil {
				waitDone := process.waitDone
				r.mu.Unlock()
				if waitDone != nil {
					select {
					case <-waitDone:
					case <-time.After(5 * time.Second):
					case <-ctx.Done():
					}
				}
			} else {
				delete(r.processes, job.ID)
				r.mu.Unlock()
			}
		} else {
			r.mu.Unlock()
		}
		return r.Status(ctx, jobID)
	}

	if startupMode != domain.StartupModeResident {
		r.mu.Lock()
		if current := r.processes[job.ID]; current != nil {
			status = current.status
			status.StartupMode = startupMode
			status.ResidentMode = false
			status.HasStopCommand = false
		}
		r.mu.Unlock()
		return status, fmt.Errorf("stop command is required for %s startup mode", startupMode)
	}

	if !ok || process == nil || process.cmd == nil || process.cmd.Process == nil {
		return status, fmt.Errorf("runtime process is not running")
	}

	if err := terminateProcessTree(process.cmd.Process.Pid); err != nil {
		r.mu.Lock()
		if current := r.processes[job.ID]; current != nil {
			current.status.Error = err.Error()
		}
		r.mu.Unlock()
	}

	if process.waitDone != nil {
		select {
		case <-process.waitDone:
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
		}
	}
	return r.Status(ctx, jobID)
}

func (r *RuntimeController) Logs(ctx context.Context, jobID string) (domain.RuntimeLogResponse, error) {
	job, _, err := r.loadRuntimeContext(ctx, jobID)
	if err != nil {
		return domain.RuntimeLogResponse{}, err
	}
	logPath := runtimeLogPath(r.toolDir, job)
	raw, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.RuntimeLogResponse{Path: runtimeLogDisplayPath(job)}, nil
		}
		return domain.RuntimeLogResponse{}, fmt.Errorf("read runtime log: %w", err)
	}
	info, statErr := os.Stat(logPath)
	updatedAt := ""
	if statErr == nil {
		updatedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
	}
	return domain.RuntimeLogResponse{
		Content:   normalizeRuntimeLogContent(raw),
		Path:      runtimeLogDisplayPath(job),
		UpdatedAt: updatedAt,
	}, nil
}

func (r *RuntimeController) waitForExit(jobID string, cmd *exec.Cmd, startupMode domain.StartupMode) {
	err := cmd.Wait()
	exitCode := exitCodeFromError(err)

	r.mu.Lock()
	process := r.processes[jobID]
	if process == nil {
		r.mu.Unlock()
		return
	}
	if process.logFile != nil {
		_ = process.logFile.Sync()
	}
	process.status.ExitCode = exitCode
	keepRunning := startupMode == domain.StartupModeBackground && err == nil && exitCode != nil && *exitCode == 0 && !process.stopping
	if keepRunning {
		process.status.Running = true
		process.status.PID = 0
		process.status.StoppedAt = ""
		if process.stopping {
			process.status.Error = ""
		} else if err != nil && !isStopError(err) {
			process.status.Error = err.Error()
		}
		waitDone := process.waitDone
		process.waitDone = nil
		process.stopping = false
		process.cmd = nil
		if process.logFile != nil {
			_ = process.logFile.Close()
			process.logFile = nil
		}
		r.mu.Unlock()

		if waitDone != nil {
			close(waitDone)
		}
		return
	}

	process.status.Running = false
	process.status.PID = 0
	process.status.StoppedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if process.stopping {
		process.status.Error = ""
	}
	if err != nil && !isStopError(err) {
		process.status.Error = err.Error()
	}
	waitDone := process.waitDone
	process.waitDone = nil
	process.stopping = false
	process.cmd = nil
	if process.logFile != nil {
		_ = process.logFile.Close()
		process.logFile = nil
	}
	delete(r.processes, jobID)
	r.mu.Unlock()

	if waitDone != nil {
		close(waitDone)
	}
}

func (r *RuntimeController) loadRuntimeContext(ctx context.Context, jobID string) (domain.Job, domain.WatchSettings, error) {
	if r.store == nil {
		return domain.Job{}, domain.WatchSettings{}, fmt.Errorf("job store not configured")
	}
	job, ok, err := r.store.Get(ctx, jobID)
	if err != nil {
		return domain.Job{}, domain.WatchSettings{}, err
	}
	if !ok {
		return domain.Job{}, domain.WatchSettings{}, fmt.Errorf("job not found")
	}
	if !supportsRuntimeJob(job) {
		return domain.Job{}, domain.WatchSettings{}, fmt.Errorf("runtime is not supported for this job")
	}
	if r.settings == nil {
		return job, domain.WatchSettings{}, fmt.Errorf("settings store not configured")
	}
	settings, err := r.settings.Load(ctx)
	if err != nil {
		return domain.Job{}, domain.WatchSettings{}, fmt.Errorf("load settings: %w", err)
	}
	return job, domain.NormalizeWatchSettings(settings), nil
}

func (r *RuntimeController) baseStatus(job domain.Job, settings domain.WatchSettings) domain.RuntimeStatus {
	startupMode := settings.StartupMode
	if !startupMode.IsValid() {
		startupMode = domain.StartupModeOneShot
	}
	status := domain.RuntimeStatus{
		Command:        normalizeRuntimeCommand(settings.StartupCommand),
		StartupMode:    startupMode,
		ResidentMode:   startupMode == domain.StartupModeResident,
		HasStopCommand: strings.TrimSpace(settings.StopCommand) != "",
		WorkingDir:     runtimeWorkingDirDisplayPath(job),
		LogPath:        runtimeLogDisplayPath(job),
	}
	return status
}

func (r *RuntimeController) runtimeWorkDirForJob(ctx context.Context, job domain.Job, settings domain.WatchSettings) (string, error) {
	switch {
	case job.Kind == domain.JobKindIssueImplementation || job.Kind == domain.JobKindPRConflict || isPRFeedbackImplementationJob(job):
		branch := renderBranchName(settings.BranchNamePattern, job.Number)
		baseBranch := ""
		prepareMerge := false
		if job.Kind == domain.JobKindPRConflict {
			var err error
			branch, baseBranch, err = pullRequestBranches(ctx, job)
			if err != nil {
				return "", err
			}
			prepareMerge = true
		}
		workDir, _, err := ensureJobWorktree(ctx, r.baseDir, r.toolDir, r.logger, job, branch, baseBranch, prepareMerge)
		return workDir, err
	case job.Kind == domain.JobKindPRReview || job.Kind == domain.JobKindPRAcceptance:
		headBranch, _, err := pullRequestBranches(ctx, job)
		if err != nil {
			return "", err
		}
		workDir, _, err := ensureJobWorktree(ctx, r.baseDir, r.toolDir, r.logger, job, headBranch, "", false)
		return workDir, err
	default:
		return "", fmt.Errorf("runtime is not supported for this job")
	}
}

func supportsRuntimeJob(job domain.Job) bool {
	switch job.Kind {
	case domain.JobKindIssueImplementation:
		switch job.State {
		case domain.StateImplementationReady, domain.StateImplementationApproved, domain.StatePRCreated, domain.StateCompleted:
			return true
		}
	case domain.JobKindPRReview:
		switch job.State {
		case domain.StateReviewReady, domain.StateReviewApproved, domain.StateCompleted:
			return true
		}
	case domain.JobKindPRAcceptance:
		switch job.State {
		case domain.StateAcceptanceTestReady, domain.StateAcceptanceTestApproved, domain.StateCompleted:
			return true
		}
	case domain.JobKindPRFeedback:
		switch job.State {
		case domain.StateReviewFixImplementationReady, domain.StateReviewFixImplementationApproved, domain.StateReviewFixed, domain.StateCompleted:
			return true
		}
	case domain.JobKindPRConflict:
		switch job.State {
		case domain.StatePRConflictReady, domain.StatePRConflictResolved, domain.StateCompleted:
			return true
		}
	}
	return false
}

func runtimeLogPath(toolDir string, job domain.Job) string {
	return filepath.Join(jobLogDir(toolDir, job), "startup.log")
}

func runtimeLogDisplayPath(job domain.Job) string {
	repoDir := sanitizePart(strings.ReplaceAll(job.Repository, "/", "_"))
	return filepath.ToSlash(filepath.Join("workspace", repoDir, job.ID, "logs", "startup.log"))
}

func runtimeWorkingDirDisplayPath(job domain.Job) string {
	return jobSourceDiffTargetPath(job)
}

func normalizeRuntimeLogContent(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	if decoded, ok := decodeUTF16Log(raw); ok {
		return decoded
	}
	if decoded, _, err := transform.String(japanese.ShiftJIS.NewDecoder(), string(raw)); err == nil {
		trimmed := strings.TrimSpace(decoded)
		if trimmed != "" {
			return decoded
		}
	}
	return string(raw)
}

func decodeUTF16Log(raw []byte) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	switch {
	case raw[0] == 0xFF && raw[1] == 0xFE:
		decoded := decodeUTF16Bytes(raw[2:], binaryLittleEndian)
		return decoded, decoded != ""
	case raw[0] == 0xFE && raw[1] == 0xFF:
		decoded := decodeUTF16Bytes(raw[2:], binaryBigEndian)
		return decoded, decoded != ""
	default:
		return "", false
	}
}

type binaryEndian int

const (
	binaryLittleEndian binaryEndian = iota
	binaryBigEndian
)

func decodeUTF16Bytes(raw []byte, endian binaryEndian) string {
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	if len(raw) == 0 {
		return ""
	}
	codes := make([]uint16, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		var code uint16
		if endian == binaryLittleEndian {
			code = uint16(raw[i]) | uint16(raw[i+1])<<8
		} else {
			code = uint16(raw[i])<<8 | uint16(raw[i+1])
		}
		codes = append(codes, code)
	}
	return string(utf16.Decode(codes))
}

func buildRuntimeCommand(command string, workDir string) (*exec.Cmd, error) {
	command = normalizeRuntimeCommand(command)
	if runtime.GOOS == "windows" {
		// `call` keeps batch execution inside the current cmd session and returns control here.
		cmd := exec.Command("cmd", "/C", "call", command)
		cmd.Dir = workDir
		return cmd, nil
	}
	escapedDir := strings.ReplaceAll(workDir, `'`, `'\''`)
	return exec.Command("sh", "-lc", fmt.Sprintf("cd '%s' && %s", escapedDir, command)), nil
}

func executeRuntimeCommand(command string, workDir string, logPath string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("create runtime log dir: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open runtime log: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()
	cmd, err := buildRuntimeCommand(command, workDir)
	if err != nil {
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run runtime command: %w", err)
	}
	return nil
}

func normalizeRuntimeCommand(command string) string {
	normalized := strings.TrimSpace(command)
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if runtime.GOOS == "windows" {
		normalized = strings.ReplaceAll(normalized, `..\\`, `..\`)
		normalized = strings.ReplaceAll(normalized, `.\\`, `.\`)
	}
	return normalized
}

func terminateProcessTree(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process id")
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("terminate runtime process: %w", err)
		}
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find runtime process: %w", err)
	}
	if err := process.Kill(); err != nil {
		return fmt.Errorf("signal runtime process: %w", err)
	}
	return nil
}

func exitCodeFromError(err error) *int {
	if err == nil {
		code := 0
		return &code
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return nil
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		code := status.ExitStatus()
		return &code
	}
	return nil
}

func isStopError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "process killed") || strings.Contains(message, "terminated")
}
