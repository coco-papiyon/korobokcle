package app

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestRunPendingReviewsCompletesJob(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "owner", "repo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "feature/review"); err != nil {
		t.Fatalf("git checkout feature/review error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("WriteFile feature error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add feature error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "feature"); err != nil {
		t.Fatalf("git commit feature error = %v", err)
	}

	skillDir := filepath.Join(root, "skills", "default", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: review\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("PR: {{ .PullNumber }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
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
			Rules: []config.WatchRule{
				{
					ID:          "rule-1",
					SkillSet:    "default",
					TestProfile: "go-default",
				},
			},
		},
	})

	repoRoot, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}

	job := domain.Job{
		ID:           "job-review-1",
		Type:         domain.JobTypePRReview,
		Repository:   source,
		GitHubNumber: 10,
		State:        domain.StateCollectingContext,
		Title:        "review me",
		BranchName:   "feature/review",
		WatchRuleID:  "rule-1",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: string(domain.DomainEventPRMatched),
		StateTo:   string(domain.StateCollectingContext),
		Payload:   `{"body":"review body","author":"alice","labels":["review"],"assignees":["bob"],"url":"https://example.com/pr/1"}`,
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if err := runPendingReviews(context.Background(), repoRoot, cfg, orch, skill.NewRunner(repoRoot, root, "mock", nil), log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("runPendingReviews() error = %v", err)
	}

	saved, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateReviewReady {
		t.Fatalf("expected review_ready, got %s", saved.State)
	}
	if len(events) == 0 {
		t.Fatalf("expected events to be recorded")
	}
}

func TestRunPendingReviewsSyncsLatestBranchBeforeReview(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "owner", "repo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "issue_97"); err != nil {
		t.Fatalf("git checkout issue_97 error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("issue v1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile issue v1 error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add issue v1 error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "issue v1"); err != nil {
		t.Fatalf("git commit issue v1 error = %v", err)
	}

	skillDir := filepath.Join(root, "skills", "default", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: review\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("PR: {{ .PullNumber }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
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
			Rules: []config.WatchRule{
				{
					ID:          "rule-1",
					SkillSet:    "default",
					TestProfile: "go-default",
				},
			},
		},
	})

	repoRoot, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("issue v2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile issue v2 error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add issue v2 error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "issue v2"); err != nil {
		t.Fatalf("git commit issue v2 error = %v", err)
	}

	job := domain.Job{
		ID:           "job-review-sync-1",
		Type:         domain.JobTypePRReview,
		Repository:   source,
		GitHubNumber: 99,
		State:        domain.StateCollectingContext,
		Title:        "review me",
		BranchName:   "issue_97",
		WatchRuleID:  "rule-1",
		CreatedAt:    nowUTC(),
		UpdatedAt:    nowUTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: string(domain.DomainEventPRMatched),
		StateTo:   string(domain.StateCollectingContext),
		Payload:   `{"body":"review body","author":"alice","labels":["review"],"assignees":["bob"],"url":"https://example.com/pr/99"}`,
		CreatedAt: nowUTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if err := runPendingReviews(context.Background(), repoRoot, cfg, orch, skill.NewRunner(repoRoot, root, "mock", nil), log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("runPendingReviews() error = %v", err)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README.md error = %v", err)
	}
	if string(readmeRaw) != "issue v2\n" {
		t.Fatalf("expected latest branch content, got %q", string(readmeRaw))
	}
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

func TestResolveReviewSkillNameFallsBackWhenWatchRuleMissing(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{})

	got, err := resolveReviewSkillName(cfg, "missing-rule")
	if err != nil {
		t.Fatalf("resolveReviewSkillName() error = %v", err)
	}
	if got != "review" {
		t.Fatalf("expected review, got %q", got)
	}
}
