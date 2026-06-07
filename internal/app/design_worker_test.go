package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func TestResolveDesignSkillNameDefault(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "default"},
			},
		},
	})

	got, err := resolveDesignSkillName(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveDesignSkillName() error = %v", err)
	}
	if got != "design" {
		t.Fatalf("expected design, got %q", got)
	}
}

func TestResolveDesignSkillNameFromSkillSet(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "team-a"},
			},
		},
	})

	got, err := resolveDesignSkillName(cfg, "rule-1")
	if err != nil {
		t.Fatalf("resolveDesignSkillName() error = %v", err)
	}
	if got != "team-a/design" {
		t.Fatalf("expected team-a/design, got %q", got)
	}
}

func TestBuildDesignContextIncludesRerunComment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)
	designDir := filepath.Join(root, "artifacts", "workers", "coco-papiyon-korobokcle", "jobs", "issue_42", "design")
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "result.md"), []byte("# existing design\n\n- keep the API stable\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	job := domain.Job{
		ID:           "job-1",
		Repository:   "coco-papiyon/korobokcle",
		GitHubNumber: 42,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
	}

	events := []domain.Event{
		{
			EventType: "issue_matched",
			Payload:   `{"body":"issue body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
			CreatedAt: time.Now(),
		},
		{
			EventType: "issue_body_refreshed",
			Payload:   `{"body":"latest issue body"}`,
			CreatedAt: time.Now(),
		},
		{
			EventType: "design_rerun_requested",
			Payload:   `{"comment":"  prioritize architecture and keep the API stable  "}`,
			CreatedAt: time.Now(),
		},
	}

	got, err := buildDesignContext(svc, root, job, events)
	if err != nil {
		t.Fatalf("buildDesignContext() error = %v", err)
	}
	if got.RerunComment != "prioritize architecture and keep the API stable" {
		t.Fatalf("expected rerun comment to be captured, got %q", got.RerunComment)
	}
	if got.Body != "latest issue body" {
		t.Fatalf("expected latest issue body to be used, got %q", got.Body)
	}
	if got.Author != "alice" || len(got.Labels) != 1 || got.Labels[0] != "bug" || len(got.Assignees) != 1 || got.Assignees[0] != "bob" {
		t.Fatalf("expected issue metadata from issue matched, got %+v", got)
	}
	if got.ExistingDesign != "# existing design\n\n- keep the API stable\n" {
		t.Fatalf("expected existing design to be loaded, got %q", got.ExistingDesign)
	}
}

func TestDesignPromptIncludesRerunCommentSection(t *testing.T) {
	t.Parallel()

	ctx := skill.DesignContext{
		Repository:     "coco-papiyon/korobokcle",
		IssueNumber:    42,
		Title:          "Issue",
		Body:           "issue body",
		Author:         "alice",
		Labels:         []string{"bug"},
		Assignees:      []string{"bob"},
		RerunComment:   "prioritize architecture and keep the API stable",
		ExistingDesign: "# existing design\n\n- keep the API stable\n",
		WatchRuleID:    "rule-1",
		BranchName:     "issue-42",
		ArtifactDir:    t.TempDir(),
	}

	prompt, err := skill.RenderSkillPrompt(filepath.Join("..", ".."), "design", ctx)
	if err != nil {
		t.Fatalf("RenderSkillPrompt() error = %v", err)
	}
	if !strings.Contains(prompt, "# Issue Design Request") {
		t.Fatalf("expected title in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Rerun Comment") {
		t.Fatalf("expected rerun comment section in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, ctx.RerunComment) {
		t.Fatalf("expected rerun comment text in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Existing Design") {
		t.Fatalf("expected existing design section in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, ctx.ExistingDesign) {
		t.Fatalf("expected existing design text in prompt, got %q", prompt)
	}
}

func TestDesignPromptOmitsRerunCommentSectionWhenEmpty(t *testing.T) {
	t.Parallel()

	ctx := skill.DesignContext{
		Repository:  "coco-papiyon/korobokcle",
		IssueNumber: 42,
		Title:       "Issue",
		Body:        "issue body",
		Author:      "alice",
		Labels:      []string{"bug"},
		Assignees:   []string{"bob"},
		WatchRuleID: "rule-1",
		BranchName:  "issue-42",
		ArtifactDir: t.TempDir(),
	}

	prompt, err := skill.RenderSkillPrompt(filepath.Join("..", ".."), "design", ctx)
	if err != nil {
		t.Fatalf("RenderSkillPrompt() error = %v", err)
	}
	if strings.Contains(prompt, "## Rerun Comment") {
		t.Fatalf("expected rerun comment section to be omitted, got %q", prompt)
	}
}

func TestDesignPromptOmitsExistingDesignSectionWhenEmpty(t *testing.T) {
	t.Parallel()

	ctx := skill.DesignContext{
		Repository:  "coco-papiyon/korobokcle",
		IssueNumber: 42,
		Title:       "Issue",
		Body:        "issue body",
		Author:      "alice",
		Labels:      []string{"bug"},
		Assignees:   []string{"bob"},
		WatchRuleID: "rule-1",
		BranchName:  "issue-42",
		ArtifactDir: t.TempDir(),
	}

	prompt, err := skill.RenderSkillPrompt(filepath.Join("..", ".."), "design", ctx)
	if err != nil {
		t.Fatalf("RenderSkillPrompt() error = %v", err)
	}
	if strings.Contains(prompt, "## Existing Design") {
		t.Fatalf("expected existing design section to be omitted, got %q", prompt)
	}
}
