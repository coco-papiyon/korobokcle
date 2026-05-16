package app

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestRunPendingPRCreationsCompletesJob(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, nil)
	cfg := config.NewService(root, config.DefaultFiles())

	job := domain.Job{
		ID:           "job-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 12,
		State:        domain.StatePRCreating,
		Title:        "Implement feature",
		BranchName:   "korobokcle/issue-12",
		CreatedAt:    testNowUTC(),
		UpdatedAt:    testNowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	artifactDir := filepath.Join(root, cfg.App().ArtifactsDir, "changes", job.ID)
	if err := writeTestSummary(artifactDir); err != nil {
		t.Fatalf("writeTestSummary() error = %v", err)
	}

	recorder := &recordingPublisher{}
	if err := runPendingPRCreations(context.Background(), cfg, orch, recorder, recorder, root, testLogger(t)); err != nil {
		t.Fatalf("runPendingPRCreations() error = %v", err)
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateCompleted {
		t.Fatalf("expected completed, got %s", saved.State)
	}
	if strings.Join(recorder.calls, ",") != "push,create" {
		t.Fatalf("expected push then create, got %v", recorder.calls)
	}
	raw, err := os.ReadFile(filepath.Join(artifactDir, "pr-create.json"))
	if err != nil {
		t.Fatalf("ReadFile(pr-create.json) error = %v", err)
	}
	if !strings.Contains(string(raw), `"pushed": true`) {
		t.Fatalf("expected pushed flag in pr-create.json, got %s", string(raw))
	}
}

func TestBuildPRCreateRequestAppendsFixSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.DefaultFiles())
	job := domain.Job{
		ID:           "job-1",
		Repository:   "owner/repo",
		GitHubNumber: 12,
		Title:        "Implement feature",
		BranchName:   "korobokcle/issue-12",
	}

	if err := writeFile(filepath.Join(root, cfg.App().ArtifactsDir, "changes", job.ID, "summary.md"), []byte("original summary")); err != nil {
		t.Fatalf("write summary.md: %v", err)
	}
	if err := writeFile(filepath.Join(root, cfg.App().ArtifactsDir, "fixes", job.ID, "fix-summary.md"), []byte("fix summary")); err != nil {
		t.Fatalf("write fix-summary.md: %v", err)
	}

	req, err := buildPRCreateRequest(cfg, job)
	if err != nil {
		t.Fatalf("buildPRCreateRequest() error = %v", err)
	}
	if !strings.Contains(req.Body, "original summary") {
		t.Fatalf("expected PR body to include original summary, got %q", req.Body)
	}
	if !strings.Contains(req.Body, "## Fix Summary") || !strings.Contains(req.Body, "fix summary") {
		t.Fatalf("expected PR body to append fix summary, got %q", req.Body)
	}
}

func writeTestSummary(artifactDir string) error {
	return writeFile(filepath.Join(artifactDir, "summary.md"), []byte("Implemented the requested changes."))
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func testNowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.New(io.Discard, "", 0)
}

type recordingPublisher struct {
	calls []string
}

func (r *recordingPublisher) Push(_ context.Context, _ PRCreateRequest) error {
	r.calls = append(r.calls, "push")
	return nil
}

func (r *recordingPublisher) Create(_ context.Context, req PRCreateRequest) (string, error) {
	r.calls = append(r.calls, "create")
	return "https://github.com/" + req.Repository + "/pull/123", nil
}
