package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
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

	designDir := filepath.Join(root, "artifacts", "designs", job.ID)
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(designDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "design.md"), []byte("design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(design.md) error = %v", err)
	}

	changesDir := filepath.Join(root, "artifacts", "changes", job.ID)
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(changesDir) error = %v", err)
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
	if err := os.WriteFile(filepath.Join(changesDir, "test-report.json"), reportRaw, 0o644); err != nil {
		t.Fatalf("WriteFile(test-report.json) error = %v", err)
	}

	testReportPath := filepath.Join(changesDir, "test-report.json")
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

	got, err := buildImplementationContext(svc, job, events, runSpec)
	if err != nil {
		t.Fatalf("buildImplementationContext() error = %v", err)
	}
	if got.PreviousFailure != "tests failed" {
		t.Fatalf("expected previous failure to be captured, got %q", got.PreviousFailure)
	}
	if got.RerunComment != "git apply failed: exit status 128: error: corrupt patch at line 381" {
		t.Fatalf("expected rerun comment to be captured, got %q", got.RerunComment)
	}
	if got.PreviousTestReport == "" {
		t.Fatalf("expected previous test report to be captured")
	}
	if got.ArtifactDir != filepath.Join(root, "artifacts", "changes", job.ID) {
		t.Fatalf("expected changes artifact dir, got %q", got.ArtifactDir)
	}
}

func TestResolveImplementationRunSpecUsesFixSkillAfterTestFailureRerun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)

	job := domain.Job{ID: "job-1", WatchRuleID: "rule-1"}
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
	if got.SkillName != fixSkillName {
		t.Fatalf("expected skill %q, got %q", fixSkillName, got.SkillName)
	}
	wantDir := filepath.Join(root, "artifacts", "fixes", job.ID)
	if got.ArtifactDir != wantDir {
		t.Fatalf("expected artifact dir %q, got %q", wantDir, got.ArtifactDir)
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

	got, err := resolveImplementationSkillName(cfg, "rule-1", false)
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

	got, err := resolveImplementationSkillName(cfg, "rule-1", false)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "team-a/implement" {
		t.Fatalf("expected team-a/implement, got %q", got)
	}
}

func TestResolveImplementationSkillNameFixFromSkillSet(t *testing.T) {
	t.Parallel()

	cfg := config.NewService(t.TempDir(), config.Files{
		WatchRules: config.WatchRulesFile{
			Rules: []config.WatchRule{
				{ID: "rule-1", SkillSet: "team-a"},
			},
		},
	})

	got, err := resolveImplementationSkillName(cfg, "rule-1", true)
	if err != nil {
		t.Fatalf("resolveImplementationSkillName() error = %v", err)
	}
	if got != "team-a/fix" {
		t.Fatalf("expected team-a/fix, got %q", got)
	}
}
