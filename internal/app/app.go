package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/web"
)

type Options struct {
	BaseDir  string
	ToolDir  string
	WorkDir  string
	MockMode bool
	Addr     string
}

func Run(ctx context.Context, opts Options) error {
	cfg := config.Default()
	if opts.BaseDir != "" {
		cfg.BaseDir = opts.BaseDir
	}
	if opts.ToolDir != "" {
		cfg.ToolDir = opts.ToolDir
		if opts.WorkDir == "" {
			cfg.WorkDir = opts.ToolDir
		}
	}
	if opts.WorkDir != "" {
		cfg.WorkDir = opts.WorkDir
	}
	if opts.Addr != "" {
		cfg.Addr = opts.Addr
	}
	if cfg.Repository == "" {
		if repo, err := inferRepository(cfg.BaseDir); err == nil {
			cfg.Repository = repo
		}
	}

	if err := ensureDirs(cfg); err != nil {
		return err
	}

	infoLogger := log.New(os.Stdout, "INFO ", log.LstdFlags|log.Lmicroseconds)
	debugLogger, logFile, err := newDebugLogger(filepath.Join(cfg.WorkDir, "logs", "korobokcle.log"))
	if err != nil {
		return err
	}
	defer logFile.Close()
	logger := &appLogger{
		info:  infoLogger,
		debug: debugLogger,
	}

	store, err := NewFileJobStore(filepath.Join(cfg.WorkDir, "db", "jobs.json"))
	if err != nil {
		return err
	}

	settingsStore, err := NewFileSettingsStore(filepath.Join(cfg.WorkDir, "config", "settings.json"), domain.WatchSettings{
		Repository:        cfg.Repository,
		AIProvider:        domain.AIProviderCodex,
		BaseBranch:        "main",
		BranchNamePattern: "issue_#<issue番号>",
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeDefault},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeDefault},
		},
	})
	if err != nil {
		return err
	}
	if settings, err := settingsStore.Load(ctx); err == nil {
		if strings.TrimSpace(settings.Repository) != "" {
			cfg.Repository = settings.Repository
		}
		cfg.PollInterval = settings.PollIntervalDuration()
		cfg.JobWorkers = settings.JobConcurrency
	}

	feedbackStore := NewFileDesignFeedbackStore(filepath.Join(cfg.WorkDir, "workspace", "design_feedback"))
	var processorFactory WorkerProcessorFactory
	if opts.MockMode {
		processorFactory = NewMockWorkflowProcessorFactory(store, feedbackStore, cfg.BaseDir, logger)
	} else {
		processorFactory = NewWorkflowProcessorFactory(store, settingsStore, feedbackStore, cfg.BaseDir, cfg.WorkDir, logger)
	}
	manager := NewWorkerManagerWithFactory(cfg, infoLogger, processorFactory)
	settingsStore.SetOnSave(func(settings domain.WatchSettings) {
		manager.SetConcurrency(settings.JobConcurrency)
	})
	if err := manager.Start(ctx); err != nil {
		return err
	}

	var source JobSource = NewGitHubSource(settingsStore, cfg.Repository, logger)
	if opts.MockMode {
		source = NewFileMockJobSource(filepath.Join(cfg.WorkDir, "db", "mock_jobs.json"), logger)
	}
	poller := NewPoller(cfg, source, store, settingsStore, manager)
	var artifactActions ArtifactActions = NewArtifactActionService(store, settingsStore, manager, feedbackStore, cfg.BaseDir, cfg.WorkDir, logger, poller)
	if opts.MockMode {
		artifactActions = NewMockArtifactActionService(store, manager, feedbackStore, cfg.BaseDir, poller)
	}
	var skillGenerator web.SkillActions = NewSkillGenerator(cfg.BaseDir, cfg.ToolDir, cfg.WorkDir, settingsStore, logger)
	if opts.MockMode {
		skillGenerator = NewMockSkillGenerator(cfg.BaseDir)
	}
	go func() {
		if err := poller.Run(ctx); err != nil && ctx.Err() == nil {
			infoLogger.Printf("poller error: %v", err)
		}
	}()

	srv := web.NewServer(cfg, store, settingsStore, artifactActions, skillGenerator)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		manager.Wait()
		return shutdownErr
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

type appLogger struct {
	info  *log.Logger
	debug *log.Logger
}

func (l *appLogger) Infof(format string, args ...any) {
	if l == nil || l.info == nil {
		return
	}
	l.info.Printf(format, args...)
}

func (l *appLogger) Debugf(format string, args ...any) {
	if l == nil || l.debug == nil {
		return
	}
	l.debug.Printf(format, args...)
}

func newDebugLogger(path string) (*log.Logger, *os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create debug log dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open debug log: %w", err)
	}
	return log.New(file, "DEBUG ", log.LstdFlags|log.Lmicroseconds), file, nil
}

func inferRepository(baseDir string) (string, error) {
	cmd := exec.Command("git", "-C", baseDir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(out))
	switch {
	case strings.HasPrefix(url, "git@github.com:"):
		return strings.TrimSuffix(strings.TrimPrefix(url, "git@github.com:"), ".git"), nil
	case strings.HasPrefix(url, "https://github.com/"):
		return strings.TrimSuffix(strings.TrimPrefix(url, "https://github.com/"), ".git"), nil
	default:
		return "", fmt.Errorf("unsupported repository url: %s", url)
	}
}

func ensureDirs(cfg config.Config) error {
	dirs := []string{
		cfg.BaseDir,
		cfg.ToolDir,
		cfg.WorkDir,
		filepath.Join(cfg.BaseDir, ".workspace"),
		filepath.Join(cfg.ToolDir, "prompt"),
		filepath.Join(cfg.ToolDir, "static"),
		filepath.Join(cfg.WorkDir, "config"),
		filepath.Join(cfg.WorkDir, "db"),
		filepath.Join(cfg.WorkDir, "workspace"),
		filepath.Join(cfg.WorkDir, "state"),
		filepath.Join(cfg.WorkDir, "logs"),
		filepath.Join(cfg.WorkDir, "logs", "skill"),
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %q: %w", dir, err)
		}
	}
	return nil
}
