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

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
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
		return fmt.Errorf("prepare repository workdirs: %w", err)
	}
	if err := startRepositoryWorkers(ctx, configService, orch, infoLogger); err != nil {
		return fmt.Errorf("start repository workers: %w", err)
	}

	issueBodyFetcher := gh.NewClient(gh.NewGHTokenProvider(10*time.Minute), debugLogger).WithInfoLogger(infoLogger)
	server, err := web.New(configService, orch, issueBodyFetcher)
	if err != nil {
		return fmt.Errorf("build web server: %w", err)
	}
	server.SetPRCommentsFetcher(func(ctx context.Context, req web.PRCommentsFetchRequest) (web.PRCommentsArtifact, error) {
		fetcher := gh.NewClient(gh.NewGHTokenProvider(10*time.Minute), debugLogger).WithInfoLogger(infoLogger)
		artifact, err := fetcher.FetchPullRequestComments(ctx, req.Repository, req.PullNumber, req.ArtifactDir)
		if err != nil {
			return web.PRCommentsArtifact{}, err
		}
		comments := make([]web.PRCommentData, 0, len(artifact.Comments))
		for _, comment := range artifact.Comments {
			comments = append(comments, web.PRCommentData{
				Author:    comment.Author,
				Body:      comment.Body,
				URL:       comment.URL,
				CreatedAt: comment.CreatedAt,
			})
		}
		return web.PRCommentsArtifact{PullNumber: artifact.PullNumber, Comments: comments}, nil
	})
	server.SetPRCommentSubmitter(func(ctx context.Context, req web.PRCommentSubmitRequest) error {
		submitter := GHPRCommentSubmitter{}
		return submitter.Submit(ctx, PRCommentSubmitRequest{
			Repository:  req.Repository,
			PullNumber:  req.PullNumber,
			Body:        req.Body,
			ArtifactDir: req.ArtifactDir,
		})
	})
	server.SetPRCommentAnalyzer(func(ctx context.Context, jobID string, comment web.PRCommentData) error {
		if err := orch.UpdateJobState(ctx, jobID, domain.StateDesignRunning, "pr_comment_analysis_requested", map[string]any{
			"comment": comment,
		}); err != nil {
			return err
		}
		go func() {
			if err := processPRCommentAnalysis(context.Background(), configService, orch, jobID, PRComment{Author: comment.Author, Body: comment.Body, URL: comment.URL, CreatedAt: comment.CreatedAt}, infoLogger); err != nil {
				infoLogger.Printf("pr comment analysis failed job_id=%s error=%v", jobID, err)
			}
		}()
		return nil
	})
	server.SetImprovementGenerator(func(ctx context.Context, jobID string, sourceEventType string) error {
		_, err := generateImprovementDraft(ctx, configService, orch, jobID, sourceEventType, infoLogger)
		return err
	})
	server.SetImprovementApprover(func(ctx context.Context, jobID string, req web.ImprovementApprovalRequest) error {
		return applyImprovementApproval(ctx, configService, orch, jobID, improvementApprovalRequest{
			Status:     req.Status,
			Comment:    req.Comment,
			ResultBody: req.ResultBody,
		}, infoLogger)
	})
	server.SetImprovementPusher(func(ctx context.Context, jobID string) error {
		job, _, err := orch.JobDetail(ctx, jobID)
		if err != nil {
			return err
		}
		repoConfig, ok := resolveMonitoredRepository(configService, job.Repository)
		if !ok || !repoConfig.ImprovementEnabled {
			return fmt.Errorf("improvement feature is disabled for repository %q", job.Repository)
		}
		workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(configService.Root(), configService.App().ArtifactsDir, job.Repository, config.ResolveImprovementBranch(repoConfig))
		artifactDir := artifacts.RepositoryWorkerImprovementArtifactDir(configService.Root(), configService.App().ArtifactsDir, job.Repository, job.GitHubNumber)
		return pushImprovementBranch(ctx, workDir, config.ResolveImprovementBranch(repoConfig), artifactDir)
	})
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
