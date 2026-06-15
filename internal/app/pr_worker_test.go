package app

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestRunPendingPRCreationsCompletesJob(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := runGit(t, root, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := runGit(t, root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
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

	implementationDir := artifacts.WorkerDir(root, cfg.App().ArtifactsDir, job.ID, artifacts.WorkerImplementation)
	if err := writeTestSummary(implementationDir); err != nil {
		t.Fatalf("writeTestSummary() error = %v", err)
	}
	artifactDir := artifacts.WorkerDir(root, cfg.App().ArtifactsDir, job.ID, artifacts.WorkerPR)

	recorder := &recordingPublisher{}
	if err := runPendingPRCreations(context.Background(), cfg, orch, recorder, recorder, MockPRCommentFetcher{}, root, testLogger(t)); err != nil {
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
	raw, err := os.ReadFile(filepath.Join(artifactDir, "result.json"))
	if err != nil {
		t.Fatalf("ReadFile(result.json) error = %v", err)
	}
	if !strings.Contains(string(raw), `"pushed": true`) {
		t.Fatalf("expected pushed flag in result.json, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"pullNumber": 123`) {
		t.Fatalf("expected pull number in result.json, got %s", string(raw))
	}
}

func TestBuildPRCreateRequestAppendsFixSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{{Repository: "owner/repo", Branch: "release/1.x", ImplementationWorkers: 1}}
	cfg := config.NewService(root, files)
	job := domain.Job{
		ID:           "job-1",
		Repository:   "owner/repo",
		GitHubNumber: 12,
		Title:        "Implement feature",
		BranchName:   "korobokcle/issue-12",
	}

	if err := writeFile(filepath.Join(artifacts.WorkerDir(root, cfg.App().ArtifactsDir, job.ID, artifacts.WorkerImplementation), "result.md"), []byte("original summary")); err != nil {
		t.Fatalf("write result.md: %v", err)
	}
	if err := writeFile(filepath.Join(artifacts.WorkerDir(root, cfg.App().ArtifactsDir, job.ID, artifacts.WorkerFix), "result.md"), []byte("fix summary")); err != nil {
		t.Fatalf("write result.md: %v", err)
	}

	req, err := buildPRCreateRequest(context.Background(), cfg, job, root)
	if err != nil {
		t.Fatalf("buildPRCreateRequest() error = %v", err)
	}
	if !strings.Contains(req.Body, "original summary") {
		t.Fatalf("expected PR body to include original summary, got %q", req.Body)
	}
	if req.Title != "[#12]Implement feature" {
		t.Fatalf("expected default PR title, got %q", req.Title)
	}
	if !strings.Contains(req.Body, "## Fix Summary") || !strings.Contains(req.Body, "fix summary") {
		t.Fatalf("expected PR body to append fix summary, got %q", req.Body)
	}
	if !strings.Contains(req.Body, "Closes owner/repo#12") {
		t.Fatalf("expected PR body to include issue auto-close directive, got %q", req.Body)
	}
	if req.BaseBranch != "release/1.x" {
		t.Fatalf("expected base branch release/1.x, got %q", req.BaseBranch)
	}
}

func TestBuildPRCreateRequestUsesDefaultBranchWhenMonitoringBranchIsEmpty(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if err := runGit(t, source, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main error = %v", err)
	}

	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{{Repository: "owner/repo", Branch: "", ImplementationWorkers: 1}}
	cfg := config.NewService(root, files)
	workDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	job := domain.Job{
		ID:           "job-1",
		Repository:   "owner/repo",
		GitHubNumber: 12,
		Title:        "Implement feature",
		BranchName:   "korobokcle/issue-12",
	}
	if err := writeFile(filepath.Join(artifacts.WorkerDir(root, cfg.App().ArtifactsDir, job.ID, artifacts.WorkerImplementation), "result.md"), []byte("summary")); err != nil {
		t.Fatalf("write result.md: %v", err)
	}

	req, err := buildPRCreateRequest(context.Background(), cfg, job, workDir)
	if err != nil {
		t.Fatalf("buildPRCreateRequest() error = %v", err)
	}
	if req.BaseBranch != "main" {
		t.Fatalf("expected default branch main, got %q", req.BaseBranch)
	}
}

func writeTestSummary(artifactDir string) error {
	return writeFile(filepath.Join(artifactDir, "result.md"), []byte("Implemented the requested changes."))
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

func (r *recordingPublisher) Create(_ context.Context, req PRCreateRequest) (PRCreateResult, error) {
	r.calls = append(r.calls, "create")
	return PRCreateResult{
		URL:        "https://github.com/" + req.Repository + "/pull/123",
		PullNumber: 123,
	}, nil
}
