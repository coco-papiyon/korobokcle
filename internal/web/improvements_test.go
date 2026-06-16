package web

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestImprovementWorkspaceFilePath(t *testing.T) {
	t.Parallel()

	improvementDir := filepath.Join("workspace", "owner-repository", ".improvement")
	workDir := filepath.Join("workspace", "owner-repository")

	got, err := improvementWorkspaceFilePath(improvementDir, workDir, ".improvement/design/notes.md")
	if err != nil {
		t.Fatalf("improvementWorkspaceFilePath() error = %v", err)
	}
	if got != filepath.Join(workDir, ".improvement", "design", "notes.md") {
		t.Fatalf("improvementWorkspaceFilePath() = %q", got)
	}

	for _, relativePath := range []string{"", ".", "../escape.md", "notes.txt"} {
		if _, err := improvementWorkspaceFilePath(improvementDir, workDir, relativePath); err == nil {
			t.Fatalf("expected path %q to be rejected", relativePath)
		}
	}
}

func TestIsImprovementRerunSourceEvent(t *testing.T) {
	t.Parallel()

	for _, sourceEventType := range []string{
		"design_rerun_requested",
		"implementation_rerun_requested",
		"pr_rerun_requested",
		"review_rerun_requested",
		"  review_rerun_requested  ",
	} {
		if !isImprovementRerunSourceEvent(sourceEventType) {
			t.Fatalf("expected %q to be recognized as rerun source", sourceEventType)
		}
	}
	for _, sourceEventType := range []string{"design_rejected", "", "review_completed"} {
		if isImprovementRerunSourceEvent(sourceEventType) {
			t.Fatalf("did not expect %q to be recognized as rerun source", sourceEventType)
		}
	}
}

func TestResolveImprovementSourceEventType(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "data", "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	orch := orchestrator.New(store, nil)

	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:            "owner/repository",
				ImprovementEnabled:    true,
				ImprovementBranch:     "improvement",
				ImprovementDir:        ".improvement",
				ImplementationWorkers: 1,
			}},
		},
	})
	server := &Server{config: cfg, orchestrator: orch}
	job := domain.Job{ID: "job-1", Repository: "owner/repository", GitHubNumber: 42}

	t.Run("requested wins", func(t *testing.T) {
		got, err := server.resolveImprovementSourceEventType(job, "manual_source")
		if err != nil {
			t.Fatalf("resolveImprovementSourceEventType() error = %v", err)
		}
		if got != "manual_source" {
			t.Fatalf("resolveImprovementSourceEventType() = %q, want manual_source", got)
		}
	})

	t.Run("artifact context wins", func(t *testing.T) {
		artifactDir := server.repositoryImprovementArtifactDir(job)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(artifactDir) error = %v", err)
		}
		raw, _ := json.Marshal(map[string]any{
			"phases": []string{"design"},
			"source": map[string]any{"eventType": "review_rerun_requested"},
		})
		if err := os.WriteFile(filepath.Join(artifactDir, "context.json"), raw, 0o644); err != nil {
			t.Fatalf("WriteFile(context.json) error = %v", err)
		}

		got, err := server.resolveImprovementSourceEventType(job, "")
		if err != nil {
			t.Fatalf("resolveImprovementSourceEventType() error = %v", err)
		}
		if got != "review_rerun_requested" {
			t.Fatalf("resolveImprovementSourceEventType() = %q, want review_rerun_requested", got)
		}
	})

	t.Run("event history wins when no artifact", func(t *testing.T) {
		job2 := domain.Job{ID: "job-2", Repository: "owner/repository", GitHubNumber: 43}
		if err := store.UpsertJob(context.Background(), job2); err != nil {
			t.Fatalf("UpsertJob() error = %v", err)
		}
		events := []domain.Event{
			{JobID: job2.ID, ID: 1, EventType: "design_rejected", Payload: `{}`, CreatedAt: time.Now().UTC()},
			{JobID: job2.ID, ID: 2, EventType: "pr_comment_analysis_ready", Payload: `{}`, CreatedAt: time.Now().UTC()},
		}
		for _, event := range events {
			if err := store.AppendEvent(context.Background(), event); err != nil {
				t.Fatalf("AppendEvent() error = %v", err)
			}
		}
		got, err := server.resolveImprovementSourceEventType(job2, "")
		if err != nil {
			t.Fatalf("resolveImprovementSourceEventType() error = %v", err)
		}
		if got != "pr_comment_analysis_ready" {
			t.Fatalf("resolveImprovementSourceEventType() = %q, want pr_comment_analysis_ready", got)
		}
	})

	t.Run("missing source returns error", func(t *testing.T) {
		job3 := domain.Job{ID: "job-3", Repository: "owner/repository", GitHubNumber: 44}
		if err := store.UpsertJob(context.Background(), job3); err != nil {
			t.Fatalf("UpsertJob() error = %v", err)
		}
		got, err := server.resolveImprovementSourceEventType(job3, "")
		if err == nil {
			t.Fatalf("expected error, got %q", got)
		}
		if got != "" {
			t.Fatalf("expected empty source event, got %q", got)
		}
	})
}

func TestLoadImprovementSummaryStates(t *testing.T) {
	t.Parallel()

	newServer := func(t *testing.T) (*Server, *sqlite.Store, domain.Job) {
		t.Helper()
		root := t.TempDir()
		store, err := sqlite.Open(filepath.Join(root, "data", "test.db"))
		if err != nil {
			t.Fatalf("sqlite.Open() error = %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		cfg := config.NewService(root, config.Files{
			App: config.App{
				ArtifactsDir: "artifacts",
				MonitoredRepositories: []config.MonitoredRepository{{
					Repository:            "owner/repository",
					ImprovementEnabled:    true,
					ImprovementBranch:     "improvement",
					ImprovementDir:        ".improvement",
					ImplementationWorkers: 1,
				}},
			},
		})
		server := &Server{config: cfg, orchestrator: orchestrator.New(store, nil)}
		job := domain.Job{ID: "job-1", Repository: "owner/repository", GitHubNumber: 42, Title: "Improvement"}
		if err := store.UpsertJob(context.Background(), job); err != nil {
			t.Fatalf("UpsertJob() error = %v", err)
		}
		return server, store, job
	}

	t.Run("missing artifacts", func(t *testing.T) {
		server, _, job := newServer(t)
		_, err := server.loadImprovementSummary(job)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("generating when input exists but no decision", func(t *testing.T) {
		server, _, job := newServer(t)
		artifactDir := server.repositoryImprovementArtifactDir(job)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(artifactDir) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(artifactDir, "input.md"), []byte("input"), 0o644); err != nil {
			t.Fatalf("WriteFile(input.md) error = %v", err)
		}

		summary, err := server.loadImprovementSummary(job)
		if err != nil {
			t.Fatalf("loadImprovementSummary() error = %v", err)
		}
		if summary.Status != "generating" {
			t.Fatalf("expected generating status, got %#v", summary)
		}
	})

	t.Run("approved decision exposes workspace", func(t *testing.T) {
		server, _, job := newServer(t)
		artifactDir := server.repositoryImprovementArtifactDir(job)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(artifactDir) error = %v", err)
		}
		rawContext, _ := json.Marshal(map[string]any{
			"phases": []string{"design", "implementation"},
			"source": map[string]any{"eventType": "pr_comment_analysis_ready"},
		})
		if err := os.WriteFile(filepath.Join(artifactDir, "context.json"), rawContext, 0o644); err != nil {
			t.Fatalf("WriteFile(context.json) error = %v", err)
		}
		rawDecision, _ := json.Marshal(map[string]any{
			"decision":    "approved",
			"reason":      "looks good",
			"updatedAt":   "2026-05-19T00:00:00Z",
			"sourceEvent": "pr_comment_analysis_ready",
		})
		if err := os.WriteFile(filepath.Join(artifactDir, "decision.json"), rawDecision, 0o644); err != nil {
			t.Fatalf("WriteFile(decision.json) error = %v", err)
		}

		summary, err := server.loadImprovementSummary(job)
		if err != nil {
			t.Fatalf("loadImprovementSummary() error = %v", err)
		}
		if summary.Decision != "approved" || summary.Status != "approved" {
			t.Fatalf("unexpected approved summary: %#v", summary)
		}
		if !summary.HasDraft || !summary.ImprovementReady {
			t.Fatalf("expected approved improvement to be ready: %#v", summary)
		}
		if summary.SourceEventType != "pr_comment_analysis_ready" {
			t.Fatalf("unexpected source event type: %#v", summary.SourceEventType)
		}
	})

	t.Run("rerun draft remains generating", func(t *testing.T) {
		server, _, job := newServer(t)
		artifactDir := server.repositoryImprovementArtifactDir(job)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(artifactDir) error = %v", err)
		}
		rawContext, _ := json.Marshal(map[string]any{
			"phases": []string{"design"},
			"source": map[string]any{"eventType": "design_rerun_requested"},
		})
		if err := os.WriteFile(filepath.Join(artifactDir, "context.json"), rawContext, 0o644); err != nil {
			t.Fatalf("WriteFile(context.json) error = %v", err)
		}
		rawDecision, _ := json.Marshal(map[string]any{
			"decision":    "draft_created",
			"reason":      "",
			"updatedAt":   "2026-05-19T00:00:00Z",
			"sourceEvent": "design_rerun_requested",
		})
		if err := os.WriteFile(filepath.Join(artifactDir, "decision.json"), rawDecision, 0o644); err != nil {
			t.Fatalf("WriteFile(decision.json) error = %v", err)
		}

		summary, err := server.loadImprovementSummary(job)
		if err != nil {
			t.Fatalf("loadImprovementSummary() error = %v", err)
		}
		if summary.Status != "generating" || summary.Decision != "draft_created" {
			t.Fatalf("unexpected rerun summary: %#v", summary)
		}
		if !summary.HasDraft || !summary.ImprovementReady {
			t.Fatalf("expected rerun draft to be ready: %#v", summary)
		}
	})
}
