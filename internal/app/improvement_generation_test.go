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
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestGenerateImprovementDraftCreatesDraftAndContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "improvement_consideration")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: improvement_consideration\ntitle: Improvement Generalization\nrole: test role\npromptTemplates:\n  - prompt.md.tmpl\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("{{ .Source.Comment }}"), 0o644); err != nil {
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
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:         "owner/repository",
				Workers:            1,
				ImprovementEnabled: true,
				ImprovementDir:     ".repo-improvement",
			}},
		},
	})

	job := domain.Job{
		ID:           "job-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateDesignRejected,
		Title:        "設計差し戻し",
		WatchRuleID:  "rule-1",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "design_rejected",
		StateTo:   string(domain.StateDesignRejected),
		Payload:   `{"comment":"Please keep the design document focused on API boundaries."}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(design_rejected) error = %v", err)
	}

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	existingDocument := ImprovementDocument{
		FrontMatter: ImprovementFrontMatter{
			ID:        "existing-rule",
			Title:     "Existing Rule",
			Scope:     "repository",
			Phases:    []string{"design"},
			Status:    "active",
			UpdatedAt: time.Now().UTC(),
		},
		Body: "- Keep API boundaries explicit.",
	}
	rawExisting, err := existingDocument.MarshalMarkdown()
	if err != nil {
		t.Fatalf("MarshalMarkdown() error = %v", err)
	}
	if err := os.MkdirAll(artifacts.RepositoryWorkerImprovementsDir(workDir, ".repo-improvement"), 0o755); err != nil {
		t.Fatalf("MkdirAll(improvementsDir) error = %v", err)
	}
	if err := os.WriteFile(artifacts.RepositoryWorkerImprovementsFile(workDir, ".repo-improvement", "existing-rule.md"), rawExisting, 0o644); err != nil {
		t.Fatalf("WriteFile(existing-rule.md) error = %v", err)
	}

	result, err := generateImprovementDraft(context.Background(), cfg, orch, job.ID, improvementSourceDesignRejected, nil)
	if err != nil {
		t.Fatalf("generateImprovementDraft() error = %v", err)
	}
	if result.Decision != improvementDecisionDraftCreated {
		t.Fatalf("expected draft_created, got %#v", result)
	}

	workFiles := repositoryImprovementWorkFiles(workDir, ".repo-improvement", job.ID, job.Title)
	artifactFiles := repositoryImprovementArtifactFiles(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber)

	draftRaw, err := os.ReadFile(workFiles.DraftPath)
	if err != nil {
		t.Fatalf("ReadFile(draft) error = %v", err)
	}
	if !strings.Contains(string(draftRaw), "設計差し戻しから抽出した改善方針") {
		t.Fatalf("expected generated draft title, got %s", string(draftRaw))
	}
	if !strings.Contains(string(draftRaw), "## 改善項目") {
		t.Fatalf("expected generated draft section, got %s", string(draftRaw))
	}
	if !strings.Contains(string(draftRaw), "- 設計方針") {
		t.Fatalf("expected generated draft category, got %s", string(draftRaw))
	}
	if !strings.Contains(string(draftRaw), "Please keep the design document focused on API boundaries.") {
		t.Fatalf("expected original comment in draft, got %s", string(draftRaw))
	}

	contextRaw, err := os.ReadFile(artifactFiles.ContextPath)
	if err != nil {
		t.Fatalf("ReadFile(context.json) error = %v", err)
	}
	var contextData improvementContextData
	if err := json.Unmarshal(contextRaw, &contextData); err != nil {
		t.Fatalf("json.Unmarshal(context.json) error = %v", err)
	}
	if len(contextData.Phases) != 1 || contextData.Phases[0] != "design" {
		t.Fatalf("unexpected phases: %#v", contextData.Phases)
	}
	if len(contextData.ExistingImprovements) != 1 || contextData.ExistingImprovements[0].ID != "existing-rule" {
		t.Fatalf("unexpected existing improvements: %#v", contextData.ExistingImprovements)
	}

	decisionRaw, err := os.ReadFile(artifactFiles.DecisionPath)
	if err != nil {
		t.Fatalf("ReadFile(decision.json) error = %v", err)
	}
	if !strings.Contains(string(decisionRaw), improvementDecisionDraftCreated) {
		t.Fatalf("expected decision draft_created, got %s", string(decisionRaw))
	}
}

func TestGenerateImprovementDraftWritesNoImprovementDecisionForEmptyComment(t *testing.T) {
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
				Repository:         "owner/repository",
				Workers:            1,
				ImprovementEnabled: true,
			}},
		},
	})

	job := domain.Job{
		ID:           "job-2",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 43,
		State:        domain.StateFinalRejected,
		Title:        "最終差し戻し",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "final_rejected",
		StateTo:   string(domain.StateFinalRejected),
		Payload:   `{"comment":"   "}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(final_rejected) error = %v", err)
	}

	result, err := generateImprovementDraft(context.Background(), cfg, orch, job.ID, improvementSourceFinalRejected, nil)
	if err != nil {
		t.Fatalf("generateImprovementDraft() error = %v", err)
	}
	if result.Decision != improvementDecisionNoImprovementNeeded {
		t.Fatalf("expected no_improvement_needed, got %#v", result)
	}

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	workFiles := repositoryImprovementWorkFiles(workDir, "", job.ID, job.Title)
	artifactFiles := repositoryImprovementArtifactFiles(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber)

	if _, err := os.Stat(workFiles.DraftPath); !os.IsNotExist(err) {
		t.Fatalf("expected no draft to be created, err=%v", err)
	}
	decisionRaw, err := os.ReadFile(artifactFiles.DecisionPath)
	if err != nil {
		t.Fatalf("ReadFile(decision.json) error = %v", err)
	}
	if !strings.Contains(string(decisionRaw), improvementDecisionNoImprovementNeeded) {
		t.Fatalf("expected no_improvement_needed decision, got %s", string(decisionRaw))
	}
}

func TestBuildImprovementDraftUsesLayoutCategoryForScreenLayoutComments(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		ID:           "job-layout",
		Repository:   "owner/repository",
		GitHubNumber: 99,
		Title:        "XX画面",
	}
	draft := buildImprovementDraft(job, improvementSourceInput{
		EventType: improvementSourceDesignRerun,
		Comment:   "XX画面で、ボタンを左に、説明を右に配置する",
	}, []string{"design"})

	if !strings.Contains(draft, "- 画面レイアウト方針") {
		t.Fatalf("expected layout category, got %s", draft)
	}
	if !strings.Contains(draft, "XX画面で、ボタンを左に、説明を右に配置する") {
		t.Fatalf("expected original comment in draft, got %s", draft)
	}
}

func TestGenerateImprovementDraftUsesAIProviderWhenExecutionConfigured(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "improvement_consideration")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: improvement_consideration\ntitle: Improvement Generalization\nrole: test role\npromptTemplates:\n  - prompt.md.tmpl\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("{{ .Source.Comment }}"), 0o644); err != nil {
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
			ArtifactsDir: "artifacts",
			Provider:     "mock",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:         "owner/repository",
				Workers:            1,
				ImprovementEnabled: true,
			}},
		},
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{{
				ID:           "rule-1",
				Name:         "default",
				Repositories: []string{"owner/repository"},
				Target:       "issue",
				Provider:     "mock",
				Enabled:      true,
			}},
		},
	})

	job := domain.Job{
		ID:           "job-ai",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 44,
		State:        domain.StateDesignRejected,
		Title:        "設計差し戻し",
		WatchRuleID:  "rule-1",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "design_rerun_requested",
		StateTo:   string(domain.StateDetected),
		Payload:   `{"comment":"XX画面で、ボタンを左に、説明を右に配置する"}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(design_rerun_requested) error = %v", err)
	}

	result, err := generateImprovementDraft(context.Background(), cfg, orch, job.ID, improvementSourceDesignRerun, nil)
	if err != nil {
		t.Fatalf("generateImprovementDraft() error = %v", err)
	}
	if result.Decision != improvementDecisionDraftCreated {
		t.Fatalf("expected draft_created, got %#v", result)
	}

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	workFiles := repositoryImprovementWorkFiles(workDir, "", job.ID, job.Title)
	draftRaw, err := os.ReadFile(workFiles.DraftPath)
	if err != nil {
		t.Fatalf("ReadFile(draft) error = %v", err)
	}
	notesRaw, err := os.ReadFile(repositoryImprovementArtifactFiles(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber).NotesPath)
	if err != nil {
		t.Fatalf("ReadFile(notes) error = %v", err)
	}
	if !strings.Contains(string(draftRaw), "Mock provider が汎化した改善方針") {
		t.Fatalf("expected AI-generated draft, got draft=%s notes=%s", string(draftRaw), string(notesRaw))
	}
	if !strings.Contains(string(notesRaw), "- mode: ai") {
		t.Fatalf("expected ai notes, got %s", string(notesRaw))
	}
}

func TestGenerateImprovementDraftSupportsRerunRequestSources(t *testing.T) {
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
				Repository:         "owner/repository",
				Workers:            1,
				ImprovementEnabled: true,
			}},
		},
	})

	tests := []struct {
		name       string
		jobID      string
		jobState   domain.JobState
		eventType  string
		comment    string
		wantPhases []string
		sourceType string
	}{
		{
			name:       "design rerun",
			jobID:      "job-rerun-design",
			jobState:   domain.StateDetected,
			eventType:  improvementSourceDesignRerun,
			comment:    "API 境界に集中する",
			wantPhases: []string{"design"},
			sourceType: improvementSourceDesignRerun,
		},
		{
			name:       "review rerun",
			jobID:      "job-rerun-review",
			jobState:   domain.StateCollectingContext,
			eventType:  improvementSourceReviewRerun,
			comment:    "レビュー観点を整理する",
			wantPhases: []string{"review"},
			sourceType: improvementSourceReviewRerun,
		},
	}

	for index, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			job := domain.Job{
				ID:           tt.jobID,
				Type:         domain.JobTypeIssue,
				Repository:   "owner/repository",
				GitHubNumber: 50 + index,
				State:        tt.jobState,
				Title:        "再実行コメント",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}
			if err := store.UpsertJob(context.Background(), job); err != nil {
				t.Fatalf("UpsertJob() error = %v", err)
			}
			if err := store.AppendEvent(context.Background(), domain.Event{
				JobID:     job.ID,
				EventType: tt.eventType,
				StateTo:   string(tt.jobState),
				Payload:   `{"comment":"` + tt.comment + `"}`,
				CreatedAt: time.Now().UTC(),
			}); err != nil {
				t.Fatalf("AppendEvent(%s) error = %v", tt.eventType, err)
			}

			result, err := generateImprovementDraft(context.Background(), cfg, orch, job.ID, tt.sourceType, nil)
			if err != nil {
				t.Fatalf("generateImprovementDraft() error = %v", err)
			}
			if result.Decision != improvementDecisionDraftCreated {
				t.Fatalf("expected draft_created, got %#v", result)
			}

			contextRaw, err := os.ReadFile(repositoryImprovementArtifactFiles(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber).ContextPath)
			if err != nil {
				t.Fatalf("ReadFile(context.json) error = %v", err)
			}
			var contextData improvementContextData
			if err := json.Unmarshal(contextRaw, &contextData); err != nil {
				t.Fatalf("json.Unmarshal(context.json) error = %v", err)
			}
			if strings.Join(contextData.Phases, ",") != strings.Join(tt.wantPhases, ",") {
				t.Fatalf("unexpected phases: got %#v want %#v", contextData.Phases, tt.wantPhases)
			}
			if contextData.Source.EventType != tt.sourceType {
				t.Fatalf("unexpected source event type: got %q want %q", contextData.Source.EventType, tt.sourceType)
			}
		})
	}
}

func TestLoadCurrentImprovementResultUsesExistingDraftForRerunComments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workFiles := improvementWorkFiles{
		DraftPath:       filepath.Join(root, ".improvement", "draft", "job-1_改善案.md"),
		LegacyDraftPath: filepath.Join(root, ".improvement", "draft", "draft.md"),
	}
	if err := os.MkdirAll(filepath.Dir(workFiles.DraftPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(work draft) error = %v", err)
	}
	if err := os.WriteFile(workFiles.DraftPath, []byte("current draft body\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(work draft) error = %v", err)
	}

	got, err := loadCurrentImprovementResult("  revise the layout  ", workFiles)
	if err != nil {
		t.Fatalf("loadCurrentImprovementResult() error = %v", err)
	}
	if got != "current draft body" {
		t.Fatalf("expected current draft body, got %q", got)
	}
}

func TestLoadCurrentImprovementResultSkipsWhenCommentEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workFiles := improvementWorkFiles{
		DraftPath:       filepath.Join(root, ".improvement", "draft", "job-1_改善案.md"),
		LegacyDraftPath: filepath.Join(root, ".improvement", "draft", "draft.md"),
	}
	if err := os.MkdirAll(filepath.Dir(workFiles.DraftPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(work draft) error = %v", err)
	}
	if err := os.WriteFile(workFiles.DraftPath, []byte("current draft body\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(work draft) error = %v", err)
	}

	got, err := loadCurrentImprovementResult("   ", workFiles)
	if err != nil {
		t.Fatalf("loadCurrentImprovementResult() error = %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty current result, got %q", got)
	}
}
