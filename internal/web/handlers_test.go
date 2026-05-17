package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestAvailableActionsForEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		event  domain.Event
		expect []string
	}{
		{
			name: "design ready",
			event: domain.Event{
				EventType: "design_ready",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateDesignReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryDesign},
		},
		{
			name: "implementation ready",
			event: domain.Event{
				EventType: "implementation_ready",
				StateFrom: string(domain.StateImplementationRunning),
				StateTo:   string(domain.StateImplementationReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
		},
		{
			name: "review ready",
			event: domain.Event{
				EventType: "review_ready",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateReviewReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "review completed",
			event: domain.Event{
				EventType: "review_completed",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateCompleted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "pr created",
			event: domain.Event{
				EventType: "pr_created",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StateCompleted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
		{
			name: "review failure",
			event: domain.Event{
				EventType: "review_failed",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "pr failure",
			event: domain.Event{
				EventType: "pr_create_failed",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := availableActionsForEvent(tc.event)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %v, got %v", tc.expect, got)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("expected %v, got %v", tc.expect, got)
				}
			}
		})
	}
}

func TestHandleAppConfigIncludesPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.PollInterval = 45 * time.Second
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app-config", nil)

	server.handleAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got struct {
		Provider        string `json:"provider"`
		Model           string `json:"model"`
		PollInterval    int    `json:"pollInterval"`
		PRTitleTemplate string `json:"prTitleTemplate"`
		BranchTemplate  string `json:"branchTemplate"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.PollInterval != 45 {
		t.Fatalf("expected poll interval 45, got %d", got.PollInterval)
	}
	if got.PRTitleTemplate != "[#{{issue_number}}]{{issue_title}}" {
		t.Fatalf("unexpected pr title template %q", got.PRTitleTemplate)
	}
	if got.BranchTemplate != "issue_{{issue_number}}" {
		t.Fatalf("unexpected branch template %q", got.BranchTemplate)
	}
}

func TestHandleSaveAppConfigUpdatesPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"pollInterval":90,"prTitleTemplate":"[#{{issue_number}}]{{issue_title}}","branchTemplate":"issue_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := svc.App().PollInterval; got != 90*time.Second {
		t.Fatalf("expected saved poll interval 90s, got %s", got)
	}
	if got := svc.App().Provider; got != "mock" {
		t.Fatalf("expected provider to remain mock, got %q", got)
	}

	savedConfigPath := filepath.Join(root, "config", "app.yaml")
	raw, err := os.ReadFile(savedConfigPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !bytes.Contains(raw, []byte("pollInterval: 1m30s")) {
		t.Fatalf("expected saved config to contain updated poll interval, got %s", string(raw))
	}
	if bytes.Contains(raw, []byte("provider:")) && !bytes.Contains(raw, []byte("provider: mock")) {
		t.Fatalf("expected saved config provider to remain unchanged, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigUpdatesProviderAndModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"codex","model":"gpt-4.1","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := svc.App().Provider; got != "codex" {
		t.Fatalf("expected saved provider codex, got %q", got)
	}
	if got := svc.App().Model; got != "gpt-4.1" {
		t.Fatalf("expected saved model gpt-4.1, got %q", got)
	}
	if got := svc.App().PRTitleTemplate; got != "PR {{issue_number}}: {{issue_title}}" {
		t.Fatalf("expected saved pr title template, got %q", got)
	}
	if got := svc.App().BranchTemplate; got != "feature_{{issue_number}}" {
		t.Fatalf("expected saved branch template, got %q", got)
	}

	savedConfigPath := filepath.Join(root, "config", "app.yaml")
	raw, err := os.ReadFile(savedConfigPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !bytes.Contains(raw, []byte("provider: codex")) {
		t.Fatalf("expected saved config to contain updated provider, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("model: gpt-4.1")) {
		t.Fatalf("expected saved config to contain updated model, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("prTitleTemplate: 'PR {{issue_number}}: {{issue_title}}'")) {
		t.Fatalf("expected saved config to contain prTitleTemplate, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("branchTemplate: feature_{{issue_number}}")) {
		t.Fatalf("expected saved config to contain branchTemplate, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigRejectsInvalidPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"pollInterval":0}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleNotificationConfigReturnsChannels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notification-config", nil)

	server.handleNotificationConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got struct {
		Channels []struct {
			Name   string   `json:"name"`
			Events []string `json:"events"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Channels) == 0 {
		t.Fatalf("expected notification channels")
	}
}

func TestHandleSaveNotificationConfigUpdatesEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"channels":[{"name":"windows-toast","type":"windows_toast","enabled":true,"events":["design_ready","waiting_design_approval","pr_created"]}]}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/notification-config", bytes.NewReader(body))

	server.handleSaveNotificationConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	notifications := svc.Notifications()
	if len(notifications.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(notifications.Channels))
	}
	if got := notifications.Channels[0].Events; len(got) != 2 || got[0] != "waiting_design_approval" || got[1] != "pr_created" {
		t.Fatalf("unexpected saved events: %v", got)
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "notifications.yaml"))
	if err != nil {
		t.Fatalf("read saved notification config: %v", err)
	}
	if !bytes.Contains(raw, []byte("- pr_created")) {
		t.Fatalf("expected saved config to include pr_created, got %s", string(raw))
	}
	if bytes.Contains(raw, []byte("design_ready")) {
		t.Fatalf("expected saved config to drop design_ready, got %s", string(raw))
	}
}

func TestHandleJobDetailIncludesFixArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	jobID := "job-1"
	if err := os.MkdirAll(artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerFix), 0o755); err != nil {
		t.Fatalf("MkdirAll(fixes) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerFix), "result.md"), []byte("fix content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	artifact, err := server.loadFixArtifact(jobID)
	if err != nil {
		t.Fatalf("loadFixArtifact() error = %v", err)
	}
	if artifact.Content != "fix content" {
		t.Fatalf("expected fix content, got %q", artifact.Content)
	}
}

func TestLoadDesignArtifactFallsBackToLegacyFileName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	jobID := "job-legacy-design"
	dir := artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerDesign)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(design) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "design.md"), []byte("legacy design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(design.md) error = %v", err)
	}

	artifact, err := server.loadDesignArtifact(jobID)
	if err != nil {
		t.Fatalf("loadDesignArtifact() error = %v", err)
	}
	if artifact.Content != "legacy design content" {
		t.Fatalf("expected legacy design content, got %q", artifact.Content)
	}
}

func TestLoadImplementationArtifactFallsBackToImplementFileName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	jobID := "job-legacy-implementation"
	dir := artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerImplementation)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementation) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "implement.md"), []byte("legacy implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(implement.md) error = %v", err)
	}

	artifact, err := server.loadImplementationArtifact(jobID)
	if err != nil {
		t.Fatalf("loadImplementationArtifact() error = %v", err)
	}
	if artifact.Content != "legacy implementation content" {
		t.Fatalf("expected legacy implementation content, got %q", artifact.Content)
	}
}

func TestLoadTestReportPrefersLatestImplementationReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	jobID := "job-1"
	fixDir := artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerFix)
	implementationDir := artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerImplementation)
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
		"eventId": implementationFailedID,
	})
	if err != nil {
		t.Fatalf("marshal rerun payload error = %v", err)
	}
	events := []domain.Event{
		{ID: implementationFailedID, EventType: "implementation_failed"},
		{ID: 11, EventType: "implementation_rerun_requested", Payload: string(rerunPayload)},
	}

	artifact, err := server.loadTestReport(jobID, events)
	if err != nil {
		t.Fatalf("loadTestReport() error = %v", err)
	}
	if artifact.Content != `{"worker":"implementation"}` {
		t.Fatalf("expected implementation test report, got %q", artifact.Content)
	}
}

func TestLoadTestReportPrefersFixReportForTestFailureRerun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	jobID := "job-1"
	fixDir := artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerFix)
	implementationDir := artifacts.WorkerDir(root, "artifacts", jobID, artifacts.WorkerImplementation)
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

	testFailedID := int64(10)
	rerunPayload, err := json.Marshal(map[string]any{
		"eventId": testFailedID,
	})
	if err != nil {
		t.Fatalf("marshal rerun payload error = %v", err)
	}
	events := []domain.Event{
		{ID: testFailedID, EventType: "test_failed"},
		{ID: 11, EventType: "implementation_rerun_requested", Payload: string(rerunPayload)},
	}

	artifact, err := server.loadTestReport(jobID, events)
	if err != nil {
		t.Fatalf("loadTestReport() error = %v", err)
	}
	if artifact.Content != `{"worker":"fix"}` {
		t.Fatalf("expected fix test report, got %q", artifact.Content)
	}
}

func TestHandleSaveWatchRulesUpdatesBranch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repo"],"target":"issue","branch":"release/1.x","labels":[],"titlePattern":"","authors":[],"assignees":[],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.WatchRules().Rules[0].Branch; got != "release/1.x" {
		t.Fatalf("expected branch release/1.x, got %q", got)
	}
}

func TestHandleSaveWatchRulesUpdatesProjectFilters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repo"],"target":"issue_project","branch":"","projectName":"Roadmap","projectFilters":[{"field":"Status","values":["Ready","In Progress"]}],"labels":[],"titlePattern":"","authors":[],"assignees":[],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	saved := svc.WatchRules().Rules[0]
	if saved.Target != "issue_project" {
		t.Fatalf("expected target issue_project, got %q", saved.Target)
	}
	if saved.ProjectName != "Roadmap" {
		t.Fatalf("expected project name Roadmap, got %q", saved.ProjectName)
	}
	if len(saved.ProjectFilters) != 1 || saved.ProjectFilters[0].Field != "Status" {
		t.Fatalf("unexpected project filters: %+v", saved.ProjectFilters)
	}
}
