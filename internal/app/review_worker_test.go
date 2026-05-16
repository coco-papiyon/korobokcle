package app

import (
	"context"
	"io"
	"log"
	"os"
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

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: review\nprovider: mock\nartifacts:\n  output_file: review.md\n"), 0o644); err != nil {
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
			Providers: []config.ProviderSpec{
				{
					Name:   "mock",
					Models: []string{},
				},
			},
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

	job := domain.Job{
		ID:           "job-review-1",
		Type:         domain.JobTypePRReview,
		Repository:   "owner/repo",
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

	if err := runPendingReviews(context.Background(), cfg, orch, skill.NewRunner(root, "mock"), log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("runPendingReviews() error = %v", err)
	}

	saved, events, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateCompleted {
		t.Fatalf("expected completed, got %s", saved.State)
	}
	if len(events) == 0 {
		t.Fatalf("expected events to be recorded")
	}
}

func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
