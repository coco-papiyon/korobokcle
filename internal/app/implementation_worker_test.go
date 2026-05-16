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
	}

	got, err := buildImplementationContext(svc, job, events)
	if err != nil {
		t.Fatalf("buildImplementationContext() error = %v", err)
	}
	if got.PreviousFailure != "tests failed" {
		t.Fatalf("expected previous failure to be captured, got %q", got.PreviousFailure)
	}
	if got.PreviousTestReport == "" {
		t.Fatalf("expected previous test report to be captured")
	}
}
