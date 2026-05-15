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

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
	"github.com/coco-papiyon/korobokcle/internal/web"
)

func Run(ctx context.Context, root string, options Options) error {
	cfg, err := config.LoadOrInit(root)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	configService := config.NewService(root, cfg)
	infoLogger := log.New(os.Stdout, "", log.LstdFlags)
	debugWriter := io.Discard
	if options.Debug {
		debugWriter = os.Stdout
		infoLogger.Printf("debug mode enabled")
	}
	debugLogger := log.New(debugWriter, "DEBUG ", log.LstdFlags)

	store, err := sqlite.Open(filepath.Join(root, configService.App().SQLitePath))
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

	orch := orchestrator.New(store)
	startWatcher(ctx, configService, orch, infoLogger, debugLogger)
	if err := startDesignWorker(ctx, root, configService, orch, infoLogger); err != nil {
		return fmt.Errorf("start design worker: %w", err)
	}
	if err := startImplementationWorker(ctx, root, configService, orch, infoLogger); err != nil {
		return fmt.Errorf("start implementation worker: %w", err)
	}
	if err := startPRWorker(ctx, root, configService, orch, infoLogger); err != nil {
		return fmt.Errorf("start pr worker: %w", err)
	}

	server, err := web.New(configService, orch)
	if err != nil {
		return fmt.Errorf("build web server: %w", err)
	}

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
