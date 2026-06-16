package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

type fakePRCommentFetcher struct {
	artifact PRCommentsArtifact
}

func (f fakePRCommentFetcher) Fetch(_ context.Context, req PRCommentFetchRequest) (PRCommentsArtifact, error) {
	f.artifact.PullNumber = req.PullNumber
	return f.artifact, nil
}

func TestProcessPRCommentAnalysisWithDepsWritesAnalysisArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "review_fix")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: review_fix\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("Title: {{ .Title }}\nComments: {{ len .ReviewComments }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt.md.tmpl) error = %v", err)
	}

	store, err := sqlite.Open(filepath.Join(root, "data", "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	orch := orchestrator.New(store, nil)
	cfg := config.NewService(root, config.Files{
		App: config.App{
			WorkspaceDir: root,
			ArtifactsDir: "artifacts",
			Provider:     "mock",
		},
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{{
				ID:       "rule-1",
				Name:     "rule-1",
				Provider: "mock",
			}},
		},
	})

	job := domain.Job{
		ID:           "job-pr-comments-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateCompleted,
		Title:        "PR コメント確認",
		BranchName:   "feature/pr-comments",
		WatchRuleID:  "rule-1",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: string(domain.DomainEventIssueMatched),
		StateTo:   string(domain.StateDetected),
		Payload:   `{"body":"issue body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(issue) error = %v", err)
	}
	prDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(prDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.json"), []byte(`{"pullNumber":123,"url":"https://github.com/owner/repository/pull/123"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	implementationDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(implementationDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementationDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(implementationDir, "result.md"), []byte("previous implementation result"), 0o644); err != nil {
		t.Fatalf("WriteFile(implementation result.md) error = %v", err)
	}

	runnerFactory := func(workDir string) *skill.Runner {
		return skill.NewRunner(workDir, cfg.Root(), "", nil)
	}

	if err := processPRCommentAnalysisWithDeps(context.Background(), cfg, orch, job.ID, PRComment{Author: "carol", Body: "please simplify"}, nil, runnerFactory); err != nil {
		t.Fatalf("processPRCommentAnalysisWithDeps() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(prDir, "result.md"))
	if err != nil {
		t.Fatalf("ReadFile(result.md) error = %v", err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		t.Fatalf("expected analysis result to be written")
	}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerReview, job.GitHubNumber, job.Title)
	if _, err := os.Stat(workingPath); err != nil {
		t.Fatalf("expected workdir copy at %s: %v", workingPath, err)
	}
}

func TestBuildPRCommentAnalysisContextLoadsSourceDataAndImplementationArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:            "owner/repository",
				WorkDir:               "workspace/owner-repository",
				ImplementationWorkers: 1,
			}},
		},
	})

	job := domain.Job{
		ID:           "job-1",
		Repository:   "owner/repository",
		GitHubNumber: 42,
		Title:        "PR comment analysis",
		WatchRuleID:  "rule-1",
		BranchName:   "feature/analysis",
	}
	events := []domain.Event{
		{
			EventType: string(domain.DomainEventIssueMatched),
			Payload:   `{"body":"issue body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
		},
		{
			EventType: "pr_created",
			Payload:   `{"url":"https://github.com/owner/repository/pull/42"}`,
		},
	}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	artifactPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerImplementation, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(artifactPath dir) error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(artifactPath) error = %v", err)
	}

	got, err := buildPRCommentAnalysisContext(cfg, workDir, job, events, PRComment{
		Author:    "reviewer",
		Body:      "please fix the edge case",
		URL:       "https://github.com/owner/repository/pull/42#discussion_r1",
		CreatedAt: "2026-05-19T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("buildPRCommentAnalysisContext() error = %v", err)
	}
	if got.JobID != job.ID || got.Repository != job.Repository || got.IssueNumber != job.GitHubNumber {
		t.Fatalf("unexpected context job fields: %#v", got)
	}
	if got.Body != "issue body" || got.Author != "alice" {
		t.Fatalf("unexpected issue fields: %#v", got)
	}
	if got.SourceURL != "https://github.com/owner/repository/pull/42" {
		t.Fatalf("unexpected source url: %#v", got.SourceURL)
	}
	if got.ImplementationArtifact != "implementation content" {
		t.Fatalf("expected implementation artifact to be loaded, got %#v", got.ImplementationArtifact)
	}
	if len(got.ReviewComments) != 1 || got.ReviewComments[0].Author != "reviewer" {
		t.Fatalf("unexpected review comments: %#v", got.ReviewComments)
	}
}

func TestResolveRepositoryWorkerPullNumberUsesJobEventsAndArtifactFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:            "owner/repository",
				WorkDir:               "workspace/owner-repository",
				ImplementationWorkers: 1,
			}},
		},
	})

	issueJob := domain.Job{ID: "job-issue", Repository: "owner/repository", GitHubNumber: 7, Title: "issue", WatchRuleID: "rule-1"}
	events := []domain.Event{
		{
			EventType: "pr_created",
			Payload:   `{"pullNumber":42}`,
		},
	}
	got, err := resolveRepositoryWorkerPullNumber(cfg, issueJob, events)
	if err != nil {
		t.Fatalf("resolveRepositoryWorkerPullNumber() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("resolveRepositoryWorkerPullNumber() = %d, want 42", got)
	}

	prFeedbackJob := domain.Job{ID: "job-pr", Type: domain.JobTypePRFeedback, Repository: "owner/repository", GitHubNumber: 33}
	got, err = resolveRepositoryWorkerPullNumber(cfg, prFeedbackJob, nil)
	if err != nil {
		t.Fatalf("resolveRepositoryWorkerPullNumber(PR feedback) error = %v", err)
	}
	if got != 33 {
		t.Fatalf("resolveRepositoryWorkerPullNumber(PR feedback) = %d, want 33", got)
	}

	artifactDir := repositoryWorkerArtifactDir(cfg, issueJob.Repository, issueJob.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(artifactDir) error = %v", err)
	}
	raw, _ := json.Marshal(map[string]any{"pullNumber": 55})
	if err := os.WriteFile(filepath.Join(artifactDir, "result.json"), raw, 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	got, err = resolveRepositoryWorkerPullNumber(cfg, issueJob, nil)
	if err != nil {
		t.Fatalf("resolveRepositoryWorkerPullNumber(artifact fallback) error = %v", err)
	}
	if got != 55 {
		t.Fatalf("resolveRepositoryWorkerPullNumber(artifact fallback) = %d, want 55", got)
	}
}
