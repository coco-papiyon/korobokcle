package app

import (
	"context"
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
