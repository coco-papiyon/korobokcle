package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	gh "github.com/coco-papiyon/korobokcle/internal/github"
	"github.com/coco-papiyon/korobokcle/internal/notification"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
	"github.com/coco-papiyon/korobokcle/internal/web"
)

func Run(ctx context.Context, repoRoot string, toolRoot string, options Options) error {
	cfg, err := config.LoadOrInit(toolRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if options.HTTPPort > 0 {
		cfg.App.HTTPPort = options.HTTPPort
	}
	configService := config.NewService(toolRoot, cfg)
	infoLogger := log.New(os.Stdout, "", log.LstdFlags)
	debugWriter := io.Discard
	if options.Debug {
		debugWriter = os.Stdout
		infoLogger.Printf("debug mode enabled")
	}
	debugLogger := log.New(debugWriter, "DEBUG ", log.LstdFlags)
	logEnvironment(infoLogger)

	store, err := sqlite.Open(resolvePath(toolRoot, configService.App().SQLitePath))
	if err != nil {
		return fmt.Errorf("open sqlite store: %w", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			infoLogger.Printf("close store: %v", closeErr)
		}
	}()

	if err := store.EnsureSeedData(ctx); err != nil {
		return fmt.Errorf("seed store: %w", err)
	}

	notifier, notifierErr := notification.NewConfiguredNotifier(configService.Notifications())
	if notifierErr != nil {
		infoLogger.Printf("notification setup warning: %v", notifierErr)
	}
	orch := orchestrator.New(store, notifier)
	if recovered, err := orch.RecoverInterruptedJobs(ctx); err != nil {
		return fmt.Errorf("recover interrupted jobs: %w", err)
	} else if recovered > 0 {
		infoLogger.Printf("recovered interrupted jobs: %d", recovered)
	}
	startWatcher(ctx, configService, orch, infoLogger, debugLogger)
	if err := prepareRepositoryWorkspaces(ctx, configService); err != nil {
		return fmt.Errorf("prepare repository workspaces: %w", err)
	}
	if err := startRepositoryWorkers(ctx, configService, orch, infoLogger); err != nil {
		return fmt.Errorf("start repository workers: %w", err)
	}

	issueBodyFetcher := gh.NewClient(gh.NewGHTokenProvider(10*time.Minute), debugLogger).WithInfoLogger(infoLogger)
	server, err := web.New(configService, orch, issueBodyFetcher)
	if err != nil {
		return fmt.Errorf("build web server: %w", err)
	}
	server.SetRepositoryWorkspacePreparer(func(ctx context.Context, appConfig config.App) error {
		snapshot := configService.App()
		snapshot.MonitoredRepositories = append([]config.MonitoredRepository(nil), appConfig.MonitoredRepositories...)
		snapshot.ArtifactsDir = appConfig.ArtifactsDir
		snapshot.WorkspaceDir = appConfig.WorkspaceDir
		return prepareRepositoryWorkspaces(ctx, config.NewService(configService.Root(), config.Files{App: snapshot}))
	})

	errCh := make(chan error, 1)
	go func() {
		infoLogger.Printf("web server listening on http://localhost:%d", configService.App().HTTPPort)
		if serveErr := server.Start(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), configService.App().ShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case serveErr := <-errCh:
		return serveErr
	}
}

func resolvePath(root string, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(root, target)
}

func logEnvironment(logger *log.Logger) {
	for _, key := range []string{
		"KOROBOKCLE_TOOL_ROOT",
		"KOROBOKCLE_CODEX_BIN",
		"KOROBOKCLE_CODEX_ARGS_JSON",
		"KOROBOKCLE_CODEX_DEBUG",
		"KOROBOKCLE_COPILOT_BIN",
		"KOROBOKCLE_COPILOT_ARGS_JSON",
		"KOROBOKCLE_COPILOT_DEBUG",
	} {
		if value, ok := os.LookupEnv(key); ok {
			logger.Printf("env %s=%s", key, value)
		}
	}
}
