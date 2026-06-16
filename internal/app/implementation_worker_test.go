package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func TestBuildImplementationContextIncludesPreviousFailureAndTestReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)

	job := domain.Job{
		ID:           "job-1",
		Repository:   "coco-papiyon/korobokcle",
		GitHubNumber: 1,
		State:        domain.StateImplementationRunning,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
	}

	designDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(designDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "result.md"), []byte("design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}
	implementationDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(implementationDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementationDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(implementationDir, "result.md"), []byte("implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(implementation result.md) error = %v", err)
	}

	reportRaw, err := json.Marshal(map[string]any{
		"profile": "default",
		"success": false,
		"results": []map[string]any{
			{
				"command":    "go test ./...",
				"success":    false,
				"stderr":     "FAIL",
				"stdout":     "",
				"exitCode":   1,
				"durationMs": 123,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal test report error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(implementationDir, "test-report.json"), reportRaw, 0o644); err != nil {
		t.Fatalf("WriteFile(test-report.json) error = %v", err)
	}

	testReportPath := filepath.Join(implementationDir, "test-report.json")
	testFailedPayload, err := json.Marshal(map[string]any{
		"error":      "tests failed",
		"reportPath": testReportPath,
	})
	if err != nil {
		t.Fatalf("marshal test failed payload error = %v", err)
	}
	rerunPayload, err := json.Marshal(map[string]any{
		"comment": "git apply failed: exit status 128: error: corrupt patch at line 381",
	})
	if err != nil {
		t.Fatalf("marshal rerun payload error = %v", err)
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
			EventType: "design_approved",
			Payload:   `{"comment":"keep the implementation small and avoid new dependencies"}`,
			CreatedAt: time.Now(),
		},
		{
			EventType: "test_failed",
			Payload:   string(testFailedPayload),
			CreatedAt: time.Now(),
		},
		{
			EventType: "implementation_rerun_requested",
			Payload:   string(rerunPayload),
			CreatedAt: time.Now(),
		},
	}

	runSpec, err := resolveImplementationRunSpec(svc, job, events)
	if err != nil {
		t.Fatalf("resolveImplementationRunSpec() error = %v", err)
	}

	got, err := buildImplementationContext(svc, root, job, events, runSpec)
	if err != nil {
		t.Fatalf("buildImplementationContext() error = %v", err)
	}
	if got.PreviousFailure != "tests failed" {
		t.Fatalf("expected previous failure to be captured, got %q", got.PreviousFailure)
	}
	if got.RerunComment != "git apply failed: exit status 128: error: corrupt patch at line 381" {
		t.Fatalf("expected rerun comment to be captured, got %q", got.RerunComment)
	}
	if got.DesignApprovalComment != "keep the implementation small and avoid new dependencies" {
		t.Fatalf("expected design approval comment to be captured, got %q", got.DesignApprovalComment)
	}
	if got.PreviousTestReport == "" {
		t.Fatalf("expected previous test report to be captured")
	}
	if got.Body != "latest issue body" {
		t.Fatalf("expected latest issue body to be used, got %q", got.Body)
	}
	if got.Author != "alice" || len(got.Labels) != 1 || got.Labels[0] != "bug" || len(got.Assignees) != 1 || got.Assignees[0] != "bob" {
		t.Fatalf("expected issue metadata from issue matched, got %+v", got)
	}
	if got.ImplementationArtifact != "implementation content" {
		t.Fatalf("expected implementation artifact to be captured, got %q", got.ImplementationArtifact)
	}
	if got.ArtifactDir != artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerImplementation) {
		t.Fatalf("expected changes artifact dir, got %q", got.ArtifactDir)
	}
}

func TestBuildImplementationContextSkipsRerunArtifactsWhenCommentMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)

	job := domain.Job{
		ID:           "job-2",
		Repository:   "coco-papiyon/korobokcle",
		GitHubNumber: 2,
		State:        domain.StateImplementationRunning,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
	}

	designDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(designDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "result.md"), []byte("design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}
	implementationDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(implementationDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementationDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(implementationDir, "result.md"), []byte("implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(implementation result.md) error = %v", err)
	}

	runSpec := implementationRunSpec{
		SkillName:   "implement",
		ArtifactDir: artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerImplementation),
	}

	got, err := buildImplementationContext(svc, root, job, []domain.Event{
		{
			EventType: "issue_matched",
			Payload:   `{"body":"issue body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
			CreatedAt: time.Now(),
		},
	}, runSpec)
	if err != nil {
		t.Fatalf("buildImplementationContext() error = %v", err)
	}
	if got.RerunComment != "" {
		t.Fatalf("expected empty rerun comment, got %q", got.RerunComment)
	}
	if got.ImplementationArtifact != "" {
		t.Fatalf("expected implementation artifact to be omitted, got %q", got.ImplementationArtifact)
	}
	if got.PreviousFailure != "" || got.PreviousTestReport != "" {
		t.Fatalf("expected retry context to be omitted, got %#v", got)
	}
}

func TestImplementationPromptIncludesExistingImplementationAndPreviousTestReport(t *testing.T) {
	t.Parallel()

	ctx := skill.ImplementationContext{
		Repository:             "coco-papiyon/korobokcle",
		IssueNumber:            42,
		Title:                  "Issue",
		Body:                   "issue body",
		Author:                 "alice",
		Labels:                 []string{"bug"},
		Assignees:              []string{"bob"},
		DesignArtifact:         "design content",
		DesignApprovalComment:  "keep the implementation small and avoid new dependencies",
		ImplementationArtifact: "implementation content",
		RerunComment:           "please keep the API stable",
		PreviousFailure:        "tests failed",
		PreviousTestReport:     "{\"success\":false}",
		WatchRuleID:            "rule-1",
		BranchName:             "issue-42",
		ArtifactDir:            t.TempDir(),
		TestProfile: skill.TestProfileContext{
			Commands: []string{"go test ./...", "go test ./internal/app"},
		},
	}

	prompt, err := skill.RenderSkillPrompt(filepath.Join("..", ".."), "implement", ctx)
	if err != nil {
		t.Fatalf("RenderSkillPrompt() error = %v", err)
	}
	for _, expected := range []string{"## Design Approval Comment", ctx.DesignApprovalComment, "## Test Profile", "go test ./...", "go test ./internal/app", "テスト時はこの commands を参考にして、同じ確認手順を優先してください。", "## Existing Implementation", ctx.ImplementationArtifact, "## Previous Test Report", ctx.PreviousTestReport} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q, got %q", expected, prompt)
		}
	}
}

func TestImplementFixPromptIncludesExistingImplementation(t *testing.T) {
	t.Parallel()

	ctx := skill.ImplementationContext{
		Repository:             "coco-papiyon/korobokcle",
		IssueNumber:            42,
		Title:                  "Issue",
		Body:                   "issue body",
		Author:                 "alice",
		Labels:                 []string{"bug"},
		Assignees:              []string{"bob"},
		DesignArtifact:         "design content",
		DesignApprovalComment:  "keep the implementation small and avoid new dependencies",
		ImplementationArtifact: "implementation content",
		PreviousFailure:        "tests failed",
		PreviousTestReport:     "{\"success\":false}",
		WatchRuleID:            "rule-1",
		BranchName:             "issue-42",
		ArtifactDir:            t.TempDir(),
		TestProfile: skill.TestProfileContext{
			Commands: []string{"go test ./..."},
		},
	}

	prompt, err := skill.RenderSkillPrompt(filepath.Join("..", ".."), "implement_fix", ctx)
	if err != nil {
		t.Fatalf("RenderSkillPrompt() error = %v", err)
	}
	for _, expected := range []string{"## Design Approval Comment", ctx.DesignApprovalComment, "## Test Profile", "go test ./...", "テスト時はこの commands を参考にして、同じ確認手順を優先してください。", "## Existing Implementation", ctx.ImplementationArtifact, "## Previous Test Report", ctx.PreviousTestReport} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q, got %q", expected, prompt)
		}
	}
}

func TestResolveImplementationRunSpecUsesFixSkillAfterTestFailureRerun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)

	job := domain.Job{
		ID:           "job-1",
		Repository:   "coco-papiyon/korobokcle",
		GitHubNumber: 1,
		WatchRuleID:  "rule-1",
	}
	testFailedID := int64(10)
	rerunPayload, err := json.Marshal(map[string]any{
		"eventId": testFailedID,
	})
	if err != nil {
		t.Fatalf("marshal rerun payload: %v", err)
	}

	events := []domain.Event{
		{ID: testFailedID, EventType: "test_failed", CreatedAt: time.Now()},
		{ID: 11, EventType: "implementation_rerun_requested", Payload: string(rerunPayload), CreatedAt: time.Now()},
	}

	got, err := resolveImplementationRunSpec(svc, job, events)
	if err != nil {
		t.Fatalf("resolveImplementationRunSpec() error = %v", err)
	}
	if got.SkillName != implementFixSkillName {
		t.Fatalf("expected skill %q, got %q", implementFixSkillName, got.SkillName)
	}
	wantDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	if got.ArtifactDir != wantDir {
		t.Fatalf("expected artifact dir %q, got %q", wantDir, got.ArtifactDir)
	}
}

func TestBuildImplementationContextFallsBackToLegacyDesignFileName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)

	job := domain.Job{
		ID:           "job-legacy-design",
		Repository:   "coco-papiyon/korobokcle",
		GitHubNumber: 1,
		State:        domain.StateImplementationRunning,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
	}

	designDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(designDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "design.md"), []byte("legacy design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(design.md) error = %v", err)
	}

	runSpec, err := resolveImplementationRunSpec(svc, job, nil)
	if err != nil {
		t.Fatalf("resolveImplementationRunSpec() error = %v", err)
	}

	got, err := buildImplementationContext(svc, root, job, nil, runSpec)
	if err != nil {
		t.Fatalf("buildImplementationContext() error = %v", err)
	}
	if got.DesignArtifact != "legacy design content" {
		t.Fatalf("expected legacy design artifact, got %q", got.DesignArtifact)
	}
}

func TestLoadImplementationRetryContextPrefersLatestImplementationReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	svc := config.NewService(root, files)

	job := domain.Job{ID: "job-1"}
	job.Repository = "coco-papiyon/korobokcle"
	job.GitHubNumber = 1
	fixDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	implementationDir := artifacts.RepositoryWorkerJobPhaseDir(root, "artifacts", job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(fixDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fixDir) error = %v", err)
	}
	if err := os.MkdirAll(implementationDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementationDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixDir, "test-report.json"), []byte(`{"worker":"fix"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(fix test-report.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(implementationDir, "test-report.json"), []byte(`{"worker":"implementation"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(implementation test-report.json) error = %v", err)
	}

	implementationFailedID := int64(10)
	rerunPayload, err := json.Marshal(map[string]any{
		"comment": "retry with narrower scope",
		"eventId": implementationFailedID,
	})
	if err != nil {
		t.Fatalf("marshal rerun payload error = %v", err)
	}

	events := []domain.Event{
		{ID: implementationFailedID, EventType: "implementation_failed", Payload: `{"error":"apply failed"}`, CreatedAt: time.Now()},
		{ID: 11, EventType: "implementation_rerun_requested", Payload: string(rerunPayload), CreatedAt: time.Now()},
	}

	_, _, previousTestReport, err := loadImplementationRetryContext(svc, job, events)
	if err != nil {
		t.Fatalf("loadImplementationRetryContext() error = %v", err)
	}
	if previousTestReport != `{"worker":"implementation"}` {
		t.Fatalf("expected implementation test report, got %q", previousTestReport)
	}
}

func TestResolveImplementationSkillNameDefault(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "default"},
			},
		},
	})

	got, err := resolveImplementationSkillName(cfg, domain.Job{WatchRuleID: "rule-1", Type: domain.JobTypeIssue}, false)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "implement" {
		t.Fatalf("expected implement, got %q", got)
	}
}

func TestResolveImplementationSkillNameFallsBackWhenWatchRuleMissing(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{})

	got, err := resolveImplementationSkillName(cfg, domain.Job{WatchRuleID: "missing-rule", Type: domain.JobTypeIssue}, false)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "implement" {
		t.Fatalf("expected implement, got %q", got)
	}
}

func TestResolveImplementationSkillNameFromSkillSet(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "team-a"},
			},
		},
	})

	got, err := resolveImplementationSkillName(cfg, domain.Job{WatchRuleID: "rule-1", Type: domain.JobTypeIssue}, false)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "team-a/implement" {
		t.Fatalf("expected team-a/implement, got %q", got)
	}
}

func TestResolveImplementationSkillNameImplementFixFromSkillSet(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "team-a"},
			},
		},
	})

	got, err := resolveImplementationSkillName(cfg, domain.Job{WatchRuleID: "rule-1", Type: domain.JobTypeIssue}, true)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "team-a/implement_fix" {
		t.Fatalf("expected team-a/implement_fix, got %q", got)
	}
}

func TestResolveImplementationSkillNameUsesReviewFixForPRFeedback(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "team-a"},
			},
		},
	})

	got, err := resolveImplementationSkillName(cfg, domain.Job{WatchRuleID: "rule-1", Type: domain.JobTypePRFeedback}, false)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "team-a/review_fix" {
		t.Fatalf("expected team-a/review_fix, got %q", got)
	}
}

func TestResolveJobTestProfileSkipsWhenProfileNameIsEmpty(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", TestProfile: "   "},
			},
		},
	})

	profile, shouldRun, err := resolveJobTestProfile(cfg, domain.Job{WatchRuleID: "rule-1"})
	if err != nil {
		t.Fatalf("resolveJobTestProfile() error = %v", err)
	}
	if shouldRun {
		t.Fatal("expected tests to be skipped")
	}
	if profile.Name != "" || len(profile.Commands) != 0 {
		t.Fatalf("expected empty profile when skipped, got %+v", profile)
	}
}

func TestResolveJobTestProfileReturnsConfiguredProfile(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", TestProfile: "go-default"},
			},
		},
		TestProfiles: config.TestProfiles{
			Profiles: []config.TestProfile{
				{Name: "go-default", Commands: []string{"go test ./..."}},
			},
		},
	})

	profile, shouldRun, err := resolveJobTestProfile(cfg, domain.Job{WatchRuleID: "rule-1"})
	if err != nil {
		t.Fatalf("resolveJobTestProfile() error = %v", err)
	}
	if !shouldRun {
		t.Fatal("expected tests to run")
	}
	if profile.Name != "go-default" {
		t.Fatalf("expected profile go-default, got %q", profile.Name)
	}
	if len(profile.Commands) != 1 || profile.Commands[0] != "go test ./..." {
		t.Fatalf("unexpected commands: %+v", profile.Commands)
	}
}
