package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

type recordingReviewSubmitter struct {
	req ReviewSubmitRequest
}

func (r *recordingReviewSubmitter) Submit(_ context.Context, req ReviewSubmitRequest) error {
	r.req = req
	return nil
}

type recordingPRCommentSubmitter struct {
	req PRCommentSubmitRequest
}

func (r *recordingPRCommentSubmitter) Submit(_ context.Context, req PRCommentSubmitRequest) error {
	r.req = req
	return nil
}

type recordingIssueBodyFetcher struct {
	repository  string
	issueNumber int
	body        string
	err         error
}

func (f *recordingIssueBodyFetcher) FetchIssueBody(_ context.Context, repository string, issueNumber int) (string, error) {
	f.repository = repository
	f.issueNumber = issueNumber
	return f.body, f.err
}

func toolStartLongRunningCommand() string {
	if runtime.GOOS == "windows" {
		return "Start-Sleep -Seconds 30"
	}
	return "sleep 30"
}

func setupToolCommandJobServer(t *testing.T) (*Server, *domain.Job) {
	t.Helper()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repository", Branch: "", Workers: 1},
	}
	files.WatchRules.Rules = []config.WatchRule{
		{
			ID:          "rule-1",
			Name:        "rule-1",
			ToolCommand: "default-tool",
			Enabled:     true,
		},
	}
	files.ToolCommands.Commands = []config.ToolCommand{
		{Name: "default-tool", Command: toolStartLongRunningCommand(), Resident: false},
		{Name: "alt-tool", Command: toolStartLongRunningCommand(), Resident: false},
	}
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	server := &Server{
		config:       svc,
		orchestrator: orchestrator.New(store, nil),
		tools:        newToolRuntimeManager(),
	}
	job := &domain.Job{
		ID:           "job-tool-start",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 1,
		State:        domain.StateDetected,
		Title:        "tool job",
		WatchRuleID:  "rule-1",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), *job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workerDir := artifacts.RepositoryWorkerSourceDir(root, svc.App().ArtifactsDir, job.Repository, 0)
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	return server, job
}

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
			name: "test failed",
			event: domain.Event{
				EventType: "test_failed",
				StateFrom: string(domain.StateTestRunning),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
		},
		{
			name: "design failed",
			event: domain.Event{
				EventType: "design_failed",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryDesign},
		},
		{
			name: "design interrupted",
			event: domain.Event{
				EventType: "design_interrupted",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateInterrupted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryDesign},
		},
		{
			name: "implementation interrupted",
			event: domain.Event{
				EventType: "implementation_interrupted",
				StateFrom: string(domain.StateImplementationRunning),
				StateTo:   string(domain.StateInterrupted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
		},
		{
			name: "test interrupted",
			event: domain.Event{
				EventType: "test_interrupted",
				StateFrom: string(domain.StateTestRunning),
				StateTo:   string(domain.StateInterrupted),
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
		{
			name: "pr interrupted",
			event: domain.Event{
				EventType: "pr_interrupted",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StateInterrupted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
		{
			name: "review interrupted",
			event: domain.Event{
				EventType: "review_interrupted",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateInterrupted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "design started has no actions",
			event: domain.Event{
				EventType: "design_started",
				StateFrom: string(domain.StateDetected),
				StateTo:   string(domain.StateDesignRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{},
		},
		{
			name: "implementation started has no actions",
			event: domain.Event{
				EventType: "implementation_started",
				StateFrom: string(domain.StateImplementationRunning),
				StateTo:   string(domain.StateImplementationRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{},
		},
		{
			name: "review started has no actions",
			event: domain.Event{
				EventType: "review_started",
				StateFrom: string(domain.StateCollectingContext),
				StateTo:   string(domain.StateReviewRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{},
		},
		{
			name: "pr creating started has no actions",
			event: domain.Event{
				EventType: "pr_creating_started",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StatePRCreating),
				CreatedAt: time.Now(),
			},
			expect: []string{},
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
	files.App.Model = "default"
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
		Provider              string `json:"provider"`
		Model                 string `json:"model"`
		PollInterval          int    `json:"pollInterval"`
		ScreenRefreshInterval int    `json:"screenRefreshInterval"`
		ShutdownTimeout       int    `json:"shutdownTimeout"`
		PRTitleTemplate       string `json:"prTitleTemplate"`
		BranchTemplate        string `json:"branchTemplate"`
		Providers             []struct {
			Name   string   `json:"name"`
			Models []string `json:"models"`
		} `json:"providers"`
		MonitoredRepositories []struct {
			Repository string `json:"repository"`
			Workers    int    `json:"workers"`
		} `json:"monitoredRepositories"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.PollInterval != 45 {
		t.Fatalf("expected poll interval 45, got %d", got.PollInterval)
	}
	if got.ScreenRefreshInterval != 5 {
		t.Fatalf("expected screen refresh interval 5, got %d", got.ScreenRefreshInterval)
	}
	if got.ShutdownTimeout != 10 {
		t.Fatalf("expected shutdown timeout 10, got %d", got.ShutdownTimeout)
	}
	if got.PRTitleTemplate != "[#{{issue_number}}]{{issue_title}}" {
		t.Fatalf("unexpected pr title template %q", got.PRTitleTemplate)
	}
	if got.BranchTemplate != "issue_{{issue_number}}" {
		t.Fatalf("unexpected branch template %q", got.BranchTemplate)
	}
	if got.Model != "" {
		t.Fatalf("expected default model to be normalized away, got %q", got.Model)
	}
	if len(got.Providers) != 4 || got.Providers[0].Name != "copilot" || len(got.Providers[0].Models) != 5 {
		t.Fatalf("unexpected provider catalog: %#v", got.Providers)
	}
	wantModels := []string{"claude-sonnet-4.6", "claude-opus-4.6", "gpt-5.4", "gpt-5-mini", "gpt-4.1"}
	for i, want := range wantModels {
		if got.Providers[0].Models[i] != want {
			t.Fatalf("unexpected provider catalog: %#v", got.Providers)
		}
	}
	if got.Providers[1].Name != "claude" || len(got.Providers[1].Models) != 2 {
		t.Fatalf("unexpected claude provider catalog: %#v", got.Providers)
	}
	if len(got.MonitoredRepositories) != 1 || got.MonitoredRepositories[0].Repository != "owner/repository" || got.MonitoredRepositories[0].Workers != 1 {
		t.Fatalf("unexpected monitored repositories: %#v", got.MonitoredRepositories)
	}
}

func TestHandleJobIssueBodyRefreshSuccess(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	fetcher := &recordingIssueBodyFetcher{body: "latest issue body"}
	server := &Server{
		config:           svc,
		orchestrator:     orchestrator.New(store, nil),
		issueBodyFetcher: fetcher,
	}
	job := domain.Job{
		ID:           "job-issue-body-refresh",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateDetected,
		Title:        "issue job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID+"/issue-body", nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	recorder := httptest.NewRecorder()

	server.handleJobIssueBody(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if fetcher.repository != job.Repository {
		t.Fatalf("expected repository %q, got %q", job.Repository, fetcher.repository)
	}
	if fetcher.issueNumber != job.GitHubNumber {
		t.Fatalf("expected issue number %d, got %d", job.GitHubNumber, fetcher.issueNumber)
	}

	var got issueBodyResponse
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
	if got.IssueBody != "latest issue body" {
		t.Fatalf("expected latest issue body, got %q", got.IssueBody)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	detailReq = mux.SetURLVars(detailReq, map[string]string{"id": job.ID})
	detailRecorder := httptest.NewRecorder()

	server.handleJobDetail(detailRecorder, detailReq)

	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected job detail status %d, got %d body=%s", http.StatusOK, detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail struct {
		IssueBody string `json:"issueBody"`
	}
	if err := json.NewDecoder(bytes.NewReader(detailRecorder.Body.Bytes())).Decode(&detail); err != nil {
		t.Fatalf("Decode(detail) error = %v", err)
	}
	if detail.IssueBody != "latest issue body" {
		t.Fatalf("expected job detail to return latest issue body, got %q", detail.IssueBody)
	}
}

func TestHandleJobIssueBodyRefreshFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{
		config:           svc,
		orchestrator:     orchestrator.New(store, nil),
		issueBodyFetcher: &recordingIssueBodyFetcher{err: fmt.Errorf("github unavailable")},
	}
	job := domain.Job{
		ID:           "job-issue-body-refresh-failure",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateDetected,
		Title:        "issue job",
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
		Payload:   `{"body":"existing issue body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID+"/issue-body", nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	recorder := httptest.NewRecorder()

	server.handleJobIssueBody(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "github unavailable") {
		t.Fatalf("expected error body to mention upstream failure, got %s", recorder.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	detailReq = mux.SetURLVars(detailReq, map[string]string{"id": job.ID})
	detailRecorder := httptest.NewRecorder()

	server.handleJobDetail(detailRecorder, detailReq)

	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected job detail status %d, got %d body=%s", http.StatusOK, detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail struct {
		IssueBody string `json:"issueBody"`
	}
	if err := json.NewDecoder(bytes.NewReader(detailRecorder.Body.Bytes())).Decode(&detail); err != nil {
		t.Fatalf("Decode(detail) error = %v", err)
	}
	if detail.IssueBody != "existing issue body" {
		t.Fatalf("expected existing issue body to remain, got %q", detail.IssueBody)
	}
}

func TestDisplayPathUnderToolRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "tool-root")
	svc := config.NewService(root, config.DefaultFiles())
	server := &Server{config: svc}

	got := server.displayPath(filepath.Join(root, "artifacts", "job-1", "design", "result.md"))
	want := "artifacts/job-1/design/result.md"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDisplayPathOutsideToolRootFallsBackToCleanPath(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "tool-root")
	outside := filepath.Join(t.TempDir(), "other", "result.md")
	svc := config.NewService(root, config.DefaultFiles())
	server := &Server{config: svc}

	got := server.displayPath(outside)
	want := filepath.ToSlash(filepath.Clean(outside))
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLoadArtifactResolvesRelativePathAgainstToolRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "tool-root")
	svc := config.NewService(root, config.DefaultFiles())
	server := &Server{config: svc}

	artifactPath := filepath.Join(root, "artifacts", "job-1", "design", "result.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("artifact content"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	artifact, err := server.loadArtifact(filepath.ToSlash(filepath.Join("artifacts", "job-1", "design", "result.md")))
	if err != nil {
		t.Fatalf("loadArtifact() error = %v", err)
	}
	if artifact.Path != "artifacts/job-1/design/result.md" {
		t.Fatalf("expected artifact path %q, got %q", "artifacts/job-1/design/result.md", artifact.Path)
	}
	if artifact.Content != "artifact content" {
		t.Fatalf("expected artifact content, got %q", artifact.Content)
	}
}

func TestHandleSPAReturnsHelpfulErrorWhenStaticDistIsMissing(t *testing.T) {
	t.Parallel()

	server := &Server{staticDir: filepath.Join(t.TempDir(), "frontend", "dist")}
	var logBuf bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(previousWriter)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	server.handleSPA(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Fatalf("expected text content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if body != "Service Unavailable\n" {
		t.Fatalf("expected generic service unavailable body, got %q", body)
	}
	if !bytes.Contains(logBuf.Bytes(), []byte("frontend dist is missing: expected "+server.staticDir)) {
		t.Fatalf("expected missing dist log, got %q", logBuf.String())
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
	if !bytes.Contains(raw, []byte("pollInterval: 90")) {
		t.Fatalf("expected saved config to contain updated poll interval, got %s", string(raw))
	}
	if bytes.Contains(raw, []byte("provider:")) && !bytes.Contains(raw, []byte("provider: mock")) {
		t.Fatalf("expected saved config provider to remain unchanged, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigUpdatesScreenRefreshInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.PollInterval = 45 * time.Second
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"screenRefreshInterval":0,"prTitleTemplate":"[#{{issue_number}}]{{issue_title}}","branchTemplate":"issue_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := svc.App().PollInterval; got != 45*time.Second {
		t.Fatalf("expected watcher poll interval to remain 45s, got %s", got)
	}
	if got := svc.App().ScreenRefreshInterval; got != 0 {
		t.Fatalf("expected screen refresh interval to be disabled, got %s", got)
	}

	savedConfigPath := filepath.Join(root, "config", "app.yaml")
	raw, err := os.ReadFile(savedConfigPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !bytes.Contains(raw, []byte("screenRefreshInterval: 0")) {
		t.Fatalf("expected saved config to contain screen refresh interval, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("shutdownTimeout: 10")) {
		t.Fatalf("expected saved config to keep shutdown timeout, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("pollInterval: 45")) {
		t.Fatalf("expected saved config to keep watcher poll interval, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigAllowsDisablingPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"pollInterval":0,"screenRefreshInterval":0,"shutdownTimeout":0,"prTitleTemplate":"[#{{issue_number}}]{{issue_title}}","branchTemplate":"issue_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().PollInterval; got != 0 {
		t.Fatalf("expected poll interval to be disabled, got %s", got)
	}
	if got := svc.App().ScreenRefreshInterval; got != 0 {
		t.Fatalf("expected screen refresh interval to be disabled, got %s", got)
	}
	if got := svc.App().ShutdownTimeout; got != 0 {
		t.Fatalf("expected shutdown timeout to be zero, got %s", got)
	}

	var response struct {
		PollInterval          int `json:"pollInterval"`
		ScreenRefreshInterval int `json:"screenRefreshInterval"`
		ShutdownTimeout       int `json:"shutdownTimeout"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.PollInterval != 0 || response.ScreenRefreshInterval != 0 || response.ShutdownTimeout != 0 {
		t.Fatalf("expected zeroed timing fields in response, got %#v", response)
	}

	raw, err := os.ReadFile(filepath.Join(root, "config", "app.yaml"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	for _, expected := range [][]byte{[]byte("pollInterval: 0"), []byte("screenRefreshInterval: 0"), []byte("shutdownTimeout: 0")} {
		if !bytes.Contains(raw, expected) {
			t.Fatalf("expected saved config to contain %q, got %s", expected, string(raw))
		}
	}
}

func TestHandleSaveAppConfigUpdatesProviderAndModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"codex","model":"gpt-5.4-mini","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := svc.App().Provider; got != "codex" {
		t.Fatalf("expected saved provider codex, got %q", got)
	}
	if got := svc.App().Model; got != "gpt-5.4-mini" {
		t.Fatalf("expected saved model gpt-5.4-mini, got %q", got)
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
	if !bytes.Contains(raw, []byte("model: gpt-5.4-mini")) {
		t.Fatalf("expected saved config to contain updated model, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("prTitleTemplate: 'PR {{issue_number}}: {{issue_title}}'")) {
		t.Fatalf("expected saved config to contain prTitleTemplate, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte("branchTemplate: feature_{{issue_number}}")) {
		t.Fatalf("expected saved config to contain branchTemplate, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigAcceptsNewCopilotModels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "copilot"
	files.App.Model = "gpt-4.1"
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"copilot","model":"gpt-5-mini","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().Provider; got != "copilot" {
		t.Fatalf("expected saved provider copilot, got %q", got)
	}
	if got := svc.App().Model; got != "gpt-5-mini" {
		t.Fatalf("expected saved model gpt-5-mini, got %q", got)
	}
}

func TestHandleSaveAppConfigAcceptsClaudeProviderModels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "mock"
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"claude","model":"claude-opus-4.6","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().Provider; got != "claude" {
		t.Fatalf("expected saved provider claude, got %q", got)
	}
	if got := svc.App().Model; got != "claude-opus-4.6" {
		t.Fatalf("expected saved model claude-opus-4.6, got %q", got)
	}
}

func TestHandleSaveAppConfigRejectsInvalidModelForProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"copilot","model":"gpt-4.5"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveAppConfigRejectsInvalidClaudeModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"claude","model":"gpt-5.4"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveAppConfigUpdatesCopilotAllowTools(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"copilot","model":"","copilotAllowTools":["write","shell(go:*)","shell(git:*)"],"pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().CopilotAllowTools; len(got) != 3 || got[0] != "write" || got[1] != "shell(go:*)" || got[2] != "shell(git:*)" {
		t.Fatalf("unexpected copilot allow tools: %#v", got)
	}

	savedConfigPath := filepath.Join(root, "config", "app.yaml")
	raw, err := os.ReadFile(savedConfigPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !bytes.Contains(raw, []byte("copilotAllowTools:")) || !bytes.Contains(raw, []byte("- shell(go:*)")) {
		t.Fatalf("expected saved config to contain copilotAllowTools, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigUpdatesMonitoredRepositories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"mock","model":"","copilotAllowTools":[],"monitoredRepositories":[{"repository":"owner/repository","branch":"main","workDir":"artifacts/custom/repository-0","workers":1},{"repository":"owner/other","branch":"release/1.x","workDir":"/tmp/korobokcle-worker","workers":3}],"pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().MonitoredRepositories; len(got) != 2 || got[0].Repository != "owner/repository" || got[0].Branch != "main" || got[0].Workers != 1 || got[0].WorkDir != "artifacts/custom/repository-0" || got[1].Repository != "owner/other" || got[1].Branch != "release/1.x" || got[1].Workers != 3 || got[1].WorkDir != "/tmp/korobokcle-worker" {
		t.Fatalf("unexpected monitored repositories: %#v", got)
	}

	savedConfigPath := filepath.Join(root, "config", "app.yaml")
	raw, err := os.ReadFile(savedConfigPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !bytes.Contains(raw, []byte("monitoredRepositories:")) || !bytes.Contains(raw, []byte("repository: owner/other")) || !bytes.Contains(raw, []byte("branch: release/1.x")) || !bytes.Contains(raw, []byte("workers: 3")) || !bytes.Contains(raw, []byte("workDir: /tmp/korobokcle-worker")) {
		t.Fatalf("expected saved config to contain monitoredRepositories, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigRejectsInvalidMonitoredRepositoryWorkers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"mock","model":"","copilotAllowTools":[],"monitoredRepositories":[{"repository":"owner/repository","workers":0}],"pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveAppConfigClearsModelWhenProviderChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "codex"
	files.App.Model = "gpt-4.5-mini"
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"copilot","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().Provider; got != "copilot" {
		t.Fatalf("expected saved provider copilot, got %q", got)
	}
	if got := svc.App().Model; got != "" {
		t.Fatalf("expected model to be cleared, got %q", got)
	}
}

func TestHandleSaveAppConfigKeepsValidModelWhenProviderChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "mock"
	files.App.Model = ""
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"codex","model":"gpt-5.4","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().Provider; got != "codex" {
		t.Fatalf("expected saved provider codex, got %q", got)
	}
	if got := svc.App().Model; got != "gpt-5.4" {
		t.Fatalf("expected model to stay gpt-5.4, got %q", got)
	}
}

func TestHandleSaveAppConfigClearsModelWhenDefaultSelected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "codex"
	files.App.Model = "gpt-4.5-mini"
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"codex","model":"default","pollInterval":90,"prTitleTemplate":"PR {{issue_number}}: {{issue_title}}","branchTemplate":"feature_{{issue_number}}"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.App().Model; got != "" {
		t.Fatalf("expected model to be cleared, got %q", got)
	}
}

func TestHandleSaveAppConfigRejectsInvalidScreenRefreshInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"screenRefreshInterval":-1}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveAppConfigRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"shutdownTimeout":-1}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveWatchRulesRejectsUnregisteredRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/not-registered"],"target":"issue","branch":"release/1.x","labels":[],"titlePattern":"","authors":[],"assignees":[],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveWatchRulesRejectsMultipleRepositories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository","owner/other"],"target":"issue","branch":"release/1.x","labels":[],"titlePattern":"","authors":[],"assignees":[],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
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
			Type   string   `json:"type"`
			Events []string `json:"events"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Channels) == 0 {
		t.Fatalf("expected notification channels")
	}
	if got.Channels[0].Name != "Windowsデスクトップ通知" {
		t.Fatalf("expected channel name Windowsデスクトップ通知, got %q", got.Channels[0].Name)
	}
	if got.Channels[0].Type != "windows_toast" {
		t.Fatalf("expected channel type windows_toast, got %q", got.Channels[0].Type)
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
	if got := notifications.Channels[0].Name; got != "Windowsデスクトップ通知" {
		t.Fatalf("expected saved channel name Windowsデスクトップ通知, got %q", got)
	}
	if got := notifications.Channels[0].Type; got != "windows_toast" {
		t.Fatalf("expected saved channel type windows_toast, got %q", got)
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
	if !bytes.Contains(raw, []byte("Windowsデスクトップ通知")) {
		t.Fatalf("expected saved config to normalize channel name, got %s", string(raw))
	}
}

func TestHandleSaveNotificationConfigRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"channels":[{"name":"Custom","type":"mail","enabled":true,"events":["waiting_design_approval"]}]}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/notification-config", bytes.NewReader(body))

	server.handleSaveNotificationConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestHandleJobDetailIncludesFixArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 1}
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fixes) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "result.md"), []byte("fix content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	artifact, err := server.loadFixArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadFixArtifact() error = %v", err)
	}
	if artifact.Content != "fix content" {
		t.Fatalf("expected fix content, got %q", artifact.Content)
	}
}

func TestLoadDesignArtifactPrefersWorkingCopy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 2, Title: "設計済み"}
	workDir := artifacts.RepositoryWorkerWorkDir(root, svc.App().ArtifactsDir, job.Repository, "")
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerDesign, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(workingPath dir) error = %v", err)
	}
	if err := os.WriteFile(workingPath, []byte("working design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(workingPath) error = %v", err)
	}
	fallbackDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fallbackDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fallbackDir, "result.md"), []byte("fallback design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	artifact, err := server.loadDesignArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadDesignArtifact() error = %v", err)
	}
	if artifact.Content != "working design content" {
		t.Fatalf("expected working design content, got %q", artifact.Content)
	}
}

func TestLoadDesignArtifactFallsBackToLegacyFileName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 2}
	dir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(design) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "design.md"), []byte("legacy design content"), 0o644); err != nil {
		t.Fatalf("WriteFile(design.md) error = %v", err)
	}

	artifact, err := server.loadDesignArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadDesignArtifact() error = %v", err)
	}
	if artifact.Content != "legacy design content" {
		t.Fatalf("expected legacy design content, got %q", artifact.Content)
	}
}

func TestLoadImplementationArtifactPrefersWorkingCopy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 3, Title: "実装済み"}
	workDir := artifacts.RepositoryWorkerWorkDir(root, svc.App().ArtifactsDir, job.Repository, "")
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerImplementation, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(workingPath dir) error = %v", err)
	}
	if err := os.WriteFile(workingPath, []byte("working implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(workingPath) error = %v", err)
	}
	fallbackDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fallbackDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fallbackDir, "result.md"), []byte("fallback implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	artifact, err := server.loadImplementationArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadImplementationArtifact() error = %v", err)
	}
	if artifact.Content != "working implementation content" {
		t.Fatalf("expected working implementation content, got %q", artifact.Content)
	}
}

func TestLoadImplementationArtifactFallsBackToImplementFileName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 3}
	dir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementation) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "implement.md"), []byte("legacy implementation content"), 0o644); err != nil {
		t.Fatalf("WriteFile(implement.md) error = %v", err)
	}

	artifact, err := server.loadImplementationArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadImplementationArtifact() error = %v", err)
	}
	if artifact.Content != "legacy implementation content" {
		t.Fatalf("expected legacy implementation content, got %q", artifact.Content)
	}
}

func TestLoadReviewArtifactPrefersWorkingCopy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 4, Title: "レビュー済み"}
	workDir := artifacts.RepositoryWorkerWorkDir(root, svc.App().ArtifactsDir, job.Repository, "")
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerReview, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(workingPath dir) error = %v", err)
	}
	if err := os.WriteFile(workingPath, []byte("working review content"), 0o644); err != nil {
		t.Fatalf("WriteFile(workingPath) error = %v", err)
	}
	fallbackDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerReview)
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fallbackDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fallbackDir, "result.md"), []byte("fallback review content"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	artifact, err := server.loadReviewArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadReviewArtifact() error = %v", err)
	}
	if artifact.Content != "working review content" {
		t.Fatalf("expected working review content, got %q", artifact.Content)
	}
}

func TestLoadImplementationArtifactFallsBackToReviewFixFileName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 4}
	dir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementation) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review_fix.md"), []byte("review fix content"), 0o644); err != nil {
		t.Fatalf("WriteFile(review_fix.md) error = %v", err)
	}

	artifact, err := server.loadImplementationArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadImplementationArtifact() error = %v", err)
	}
	if artifact.Content != "review fix content" {
		t.Fatalf("expected review fix content, got %q", artifact.Content)
	}
}

func TestLoadImplementationArtifactFallsBackToStdoutLog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 5}
	dir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implementation) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte("partial implementation output"), 0o644); err != nil {
		t.Fatalf("WriteFile(stdout.log) error = %v", err)
	}

	artifact, err := server.loadImplementationArtifact(job, nil)
	if err != nil {
		t.Fatalf("loadImplementationArtifact() error = %v", err)
	}
	if artifact.Content != "partial implementation output" {
		t.Fatalf("expected stdout log fallback, got %q", artifact.Content)
	}
}

func TestLoadTestReportPrefersLatestImplementationReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 6}
	fixDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	implementationDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
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
		{ID: implementationFailedID, EventType: "implementation_failed", Payload: `{"error":"apply failed","reportPath":"` + filepath.ToSlash(filepath.Join(implementationDir, "test-report.json")) + `"}`},
		{ID: 11, EventType: "implementation_rerun_requested", Payload: string(rerunPayload)},
	}

	artifact, err := server.loadTestReport(job, events)
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

	job := domain.Job{Repository: "owner/repository", GitHubNumber: 7}
	fixDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	implementationDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
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
		{ID: testFailedID, EventType: "test_failed", Payload: `{"reportPath":"` + filepath.ToSlash(filepath.Join(fixDir, "test-report.json")) + `"}`},
		{ID: 11, EventType: "implementation_rerun_requested", Payload: string(rerunPayload)},
	}

	artifact, err := server.loadTestReport(job, events)
	if err != nil {
		t.Fatalf("loadTestReport() error = %v", err)
	}
	if artifact.Content != `{"worker":"fix"}` {
		t.Fatalf("expected fix test report, got %q", artifact.Content)
	}
}

func TestHandleSaveWatchRulesSavesReviewers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"issue","labels":[],"titlePattern":"","authors":[],"assignees":[],"reviewers":["reviewer1"],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := svc.WatchRules().Rules[0].Reviewers; len(got) != 1 || got[0] != "reviewer1" {
		t.Fatalf("expected reviewers to be saved, got %+v", got)
	}
	var response []struct {
		Reviewers []string `json:"reviewers"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode watch rules response: %v", err)
	}
	if len(response) != 1 || len(response[0].Reviewers) != 1 || response[0].Reviewers[0] != "reviewer1" {
		t.Fatalf("expected reviewers in response, got %+v", response)
	}
}

func TestHandleSaveWatchRulesUpdatesProjectFilters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"issue_project","projectName":"Roadmap","projectFilters":[{"field":"Status","values":["Ready","In Progress"]}],"labels":[],"titlePattern":"","authors":[],"assignees":[],"reviewers":[],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
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

func TestHandleSaveWatchRulesAcceptsNewCopilotModels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"issue","labels":[],"titlePattern":"","authors":[],"assignees":[],"reviewers":[],"excludeDraftPR":true,"provider":"copilot","model":"claude-opus-4.6","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := svc.WatchRules().Rules[0].Model; got != "claude-opus-4.6" {
		t.Fatalf("expected saved model claude-opus-4.6, got %q", got)
	}
}

func TestHandleSaveWatchRulesAcceptsClaudeProviderModels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"issue","labels":[],"titlePattern":"","authors":[],"assignees":[],"reviewers":[],"excludeDraftPR":true,"provider":"claude","model":"claude-sonnet-4.6","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := svc.WatchRules().Rules[0].Provider; got != "claude" {
		t.Fatalf("expected saved provider claude, got %q", got)
	}
	if got := svc.WatchRules().Rules[0].Model; got != "claude-sonnet-4.6" {
		t.Fatalf("expected saved model claude-sonnet-4.6, got %q", got)
	}
}

func TestHandleSaveWatchRulesRejectsInvalidModelForProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"issue","labels":[],"titlePattern":"","authors":[],"assignees":[],"reviewers":[],"excludeDraftPR":true,"provider":"copilot","model":"gpt-4.5","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveWatchRulesRejectsInvalidClaudeModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"issue","labels":[],"titlePattern":"","authors":[],"assignees":[],"reviewers":[],"excludeDraftPR":true,"provider":"claude","model":"gpt-5.4","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestHandleSaveWatchRulesAcceptsPullRequestReviewCommentTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"id":"rule-1","name":"Rule 1","repositories":["owner/repository"],"target":"pull_request_review","labels":["ai:fix"],"titlePattern":"","authors":[],"assignees":[],"reviewers":[],"excludeDraftPR":true,"provider":"","model":"","skillSet":"default","testProfile":"go-default","enabled":true}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/watch-rules", bytes.NewReader(body))

	server.handleSaveWatchRules(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := svc.WatchRules().Rules[0].Target; got != "pull_request_review" {
		t.Fatalf("expected target pull_request_review, got %q", got)
	}
}

func TestHandleTestProfilesReturnsProfiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.TestProfiles = config.TestProfiles{
		Profiles: []config.TestProfile{
			{ID: "profile-1", Name: "go-default", Commands: []string{"go test ./...", "go test ./internal/..."}},
		},
	}
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/test-profiles", nil)

	server.handleTestProfiles(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got []struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Commands []string `json:"commands"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0].ID != "profile-1" || got[0].Name != "go-default" || len(got[0].Commands) != 2 {
		t.Fatalf("unexpected test profiles response: %+v", got)
	}
}

func TestHandleSaveTestProfilesNormalizesCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"name":"go-default","commands":["  go test ./...  ","","go test ./internal/..."]}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/test-profiles", bytes.NewReader(body))

	server.handleSaveTestProfiles(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := svc.TestProfiles().Profiles; len(got) != 1 || got[0].ID != "profile-1" || len(got[0].Commands) != 2 || got[0].Commands[0] != "go test ./..." || got[0].Commands[1] != "go test ./internal/..." {
		t.Fatalf("unexpected saved test profiles: %#v", got)
	}
}

func TestHandleSaveTestProfilesRejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"name":"go-default","commands":["go test ./..."]},{"name":"go-default","commands":["go test ./internal/..."]}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/test-profiles", bytes.NewReader(body))

	server.handleSaveTestProfiles(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestHandleSaveTestProfilesRejectsEmptyCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`[{"name":"go-default","commands":["   ",""]}]`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/test-profiles", bytes.NewReader(body))

	server.handleSaveTestProfiles(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestHandleSubmitReviewCommentUsesReviewArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, nil)
	submitter := &recordingReviewSubmitter{}
	server := &Server{config: svc, orchestrator: orch, reviewer: submitter}

	job := domain.Job{
		ID:           "job-review-1",
		Type:         domain.JobTypePRReview,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateCompleted,
		Title:        "review job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	reviewDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerReview)
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(reviewDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "result.md"), []byte("review summary"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	body := []byte(`{"comment":"review from ui"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/reviews/submit", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	recorder := httptest.NewRecorder()

	server.handleSubmitReviewComment(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if submitter.req.Repository != "owner/repository" {
		t.Fatalf("expected repository owner/repository, got %q", submitter.req.Repository)
	}
	if submitter.req.PullNumber != 42 {
		t.Fatalf("expected pull number 42, got %d", submitter.req.PullNumber)
	}
	if submitter.req.Body != "review from ui" {
		t.Fatalf("expected review body from request, got %q", submitter.req.Body)
	}
	if submitter.req.ArtifactDir != reviewDir {
		t.Fatalf("expected artifact dir %q, got %q", reviewDir, submitter.req.ArtifactDir)
	}
}

func TestHandleReviewApprovalCompletesReviewJob(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, nil)
	server := &Server{config: svc, orchestrator: orch}

	job := domain.Job{
		ID:           "job-review-approval-1",
		Type:         domain.JobTypePRReview,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateReviewReady,
		Title:        "review job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/approvals/review", nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	recorder := httptest.NewRecorder()

	server.handleReviewApproval(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	saved, _, err := orch.JobDetail(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("JobDetail() error = %v", err)
	}
	if saved.State != domain.StateCompleted {
		t.Fatalf("expected completed, got %s", saved.State)
	}
}

func TestHandleJobDetailForPRFeedbackIncludesReviewCommentsAndSanitizedPayload(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-feedback-1",
		Type:         domain.JobTypePRFeedback,
		Repository:   "owner/repository",
		GitHubNumber: 46,
		State:        domain.StateWaitingFinalApproval,
		Title:        "Fix PR feedback",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"body": "full pr body",
		"reviewComments": []map[string]any{
			{
				"author": "reviewer",
				"body":   "please rename this",
				"path":   "internal/app/example.go",
				"line":   12,
				"url":    "https://github.com/example/comment/1",
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal(payload) error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: string(domain.DomainEventPRReviewMatched),
		StateTo:   string(domain.StateImplementationRunning),
		Payload:   string(payload),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	recorder := httptest.NewRecorder()

	server.handleJobDetail(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var got struct {
		ReviewComments []struct {
			Author string `json:"author"`
			Body   string `json:"body"`
			Path   string `json:"path"`
			Line   int    `json:"line"`
		} `json:"reviewComments"`
		Events []struct {
			Payload string `json:"payload"`
		} `json:"events"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
	if len(got.ReviewComments) != 1 || got.ReviewComments[0].Body != "please rename this" {
		t.Fatalf("unexpected review comments: %+v", got.ReviewComments)
	}
	if len(got.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got.Events))
	}
	var eventPayload map[string]any
	if err := json.Unmarshal([]byte(got.Events[0].Payload), &eventPayload); err != nil {
		t.Fatalf("Unmarshal(event payload) error = %v", err)
	}
	if _, ok := eventPayload["body"]; ok {
		t.Fatalf("expected top-level payload body to be omitted, got %s", got.Events[0].Payload)
	}
	reviewCommentsRaw, ok := eventPayload["reviewComments"].([]any)
	if !ok || len(reviewCommentsRaw) != 1 {
		t.Fatalf("unexpected reviewComments payload: %#v", eventPayload["reviewComments"])
	}
	reviewComment, ok := reviewCommentsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected review comment payload type: %#v", reviewCommentsRaw[0])
	}
	if _, ok := reviewComment["body"]; ok {
		t.Fatalf("expected nested review comment body to be omitted, got %s", got.Events[0].Payload)
	}
}

func TestHandleJobDetailIncludesPRCommentsAndPullNumber(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-pr-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 12,
		State:        domain.StateCompleted,
		Title:        "PR job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	prDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(prDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.json"), []byte(`{"url":"https://github.com/owner/repository/pull/123","pullNumber":123,"repository":"owner/repository","branchName":"feature/pr-12","title":"PR job","pushed":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "gh-pr-comments.json"), []byte(`{"pullNumber":123,"comments":[{"author":"alice","body":"looks good","url":"https://github.com/owner/repository/pull/123#issuecomment-1","createdAt":"2026-06-05T00:00:00Z"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(gh-pr-comments.json) error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "pr_created",
		StateTo:   string(domain.StateCompleted),
		Payload:   `{"artifactDir":"` + prDir + `","url":"https://github.com/owner/repository/pull/123","pullNumber":123,"title":"PR job","head":"feature/pr-12"}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	recorder := httptest.NewRecorder()

	server.handleJobDetail(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var got struct {
		PRComments []struct {
			Author    string `json:"author"`
			Body      string `json:"body"`
			URL       string `json:"url"`
			CreatedAt string `json:"createdAt"`
		} `json:"prComments"`
		PRCreateArtifact struct {
			Content string `json:"content"`
		} `json:"prCreateArtifact"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
	if len(got.PRComments) != 1 || got.PRComments[0].Body != "looks good" || got.PRComments[0].Author != "alice" {
		t.Fatalf("unexpected pr comments: %+v", got.PRComments)
	}
	if !strings.Contains(got.PRCreateArtifact.Content, `"pullNumber":123`) {
		t.Fatalf("expected pr create artifact to include pull number, got %s", got.PRCreateArtifact.Content)
	}
}

func TestHandlePRCommentsFetchesAndShowsAnalysis(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-pr-2",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 33,
		State:        domain.StateCompleted,
		Title:        "PR comments",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}
	prDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(prDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.json"), []byte(`{"url":"https://github.com/owner/repository/pull/444","pullNumber":444,"repository":"owner/repository","branchName":"feature/pr","title":"PR comments","pushed":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	server.SetPRCommentsFetcher(func(_ context.Context, req PRCommentsFetchRequest) (PRCommentsArtifact, error) {
		if req.PullNumber != 444 {
			t.Fatalf("unexpected pullNumber %d", req.PullNumber)
		}
		if err := os.WriteFile(filepath.Join(prDir, "gh-pr-comments.json"), []byte(`{"pullNumber":444,"comments":[{"author":"alice","body":"please rename"}]}`), 0o644); err != nil {
			t.Fatalf("WriteFile(gh-pr-comments.json) error = %v", err)
		}
		return PRCommentsArtifact{
			PullNumber: 444,
			Comments: []PRCommentData{
				{Author: "alice", Body: "please rename"},
			},
		}, nil
	})
	server.SetPRCommentAnalyzer(func(_ context.Context, jobID string, comment PRCommentData) error {
		if jobID != job.ID {
			t.Fatalf("unexpected jobID %q", jobID)
		}
		if comment.Body != "please rename" {
			t.Fatalf("unexpected comment body %q", comment.Body)
		}
		if err := os.WriteFile(filepath.Join(prDir, "result.md"), []byte("analysis result"), 0o644); err != nil {
			return err
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID+"/pr-comments", nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	rec := httptest.NewRecorder()

	server.handlePRComments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var got struct {
		PullNumber int `json:"pullNumber"`
		Comments   []struct {
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"comments"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
	if got.PullNumber != 444 {
		t.Fatalf("unexpected pull number: %+v", got.PullNumber)
	}
	if len(got.Comments) != 1 || got.Comments[0].Body != "please rename" {
		t.Fatalf("unexpected pr comments: %+v", got.Comments)
	}

	analysisReqBody := bytes.NewBufferString(`{"comment":{"author":"alice","body":"please rename"}}`)
	analysisReq := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/pr-comments/analyze", analysisReqBody)
	analysisReq = mux.SetURLVars(analysisReq, map[string]string{"id": job.ID})
	analysisRec := httptest.NewRecorder()

	server.handleAnalyzePRComment(analysisRec, analysisReq)
	if analysisRec.Code != http.StatusOK {
		t.Fatalf("expected analysis status %d, got %d body=%s", http.StatusOK, analysisRec.Code, analysisRec.Body.String())
	}
	var analyzed struct {
		PRCommentAnalysisArtifact struct {
			Content string `json:"content"`
		} `json:"prCommentAnalysisArtifact"`
	}
	if err := json.NewDecoder(bytes.NewReader(analysisRec.Body.Bytes())).Decode(&analyzed); err != nil {
		t.Fatalf("Decode(analysis response) error = %v", err)
	}
	if analyzed.PRCommentAnalysisArtifact.Content != "analysis result" {
		t.Fatalf("unexpected analyzed content: %+v", analyzed.PRCommentAnalysisArtifact)
	}
}

func TestHandlePRCommentsUsesLocalArtifactWithoutFetcher(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-pr-local-comments",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 34,
		State:        domain.StateWaitingDesignApproval,
		Title:        "PR local comments",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	prDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(prDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.json"), []byte(`{"url":"https://github.com/owner/repository/pull/555","pullNumber":555,"repository":"owner/repository","branchName":"feature/pr-local","title":"PR local comments","pushed":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "gh-pr-comments.json"), []byte(`{"pullNumber":555,"comments":[{"author":"alice","body":"local fixture comment","url":"https://github.com/owner/repository/pull/555#issuecomment-1","createdAt":"2026-06-06T00:00:00Z"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(gh-pr-comments.json) error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "pr_created",
		StateTo:   string(domain.StateCompleted),
		Payload:   `{"artifactDir":"` + prDir + `","url":"https://github.com/owner/repository/pull/555","pullNumber":555,"title":"PR local comments","head":"feature/pr-local"}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID+"/pr-comments", nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	rec := httptest.NewRecorder()

	server.handlePRComments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var got struct {
		PullNumber int `json:"pullNumber"`
		Comments   []struct {
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"comments"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
	if got.PullNumber != 555 {
		t.Fatalf("unexpected pull number: %+v", got.PullNumber)
	}
	if len(got.Comments) != 1 || got.Comments[0].Author != "alice" || got.Comments[0].Body != "local fixture comment" {
		t.Fatalf("unexpected pr comments: %+v", got.Comments)
	}
}

func TestHandlePRCommentsFiltersAnalysisComments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-pr-filtered-comments",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 35,
		State:        domain.StateCompleted,
		Title:        "PR filtered comments",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	prDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(prDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.json"), []byte(`{"url":"https://github.com/owner/repository/pull/556","pullNumber":556,"repository":"owner/repository","branchName":"feature/pr-filtered","title":"PR filtered comments","pushed":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "gh-pr-comments.json"), []byte(`{"pullNumber":556,"comments":[{"author":"alice","body":"local fixture comment","url":"https://github.com/owner/repository/pull/556#issuecomment-1","createdAt":"2026-06-06T00:00:00Z"},{"author":"korobokcle","body":"<!-- korobokcle:pr-comment-analysis -->\n\nhidden analysis comment","url":"https://github.com/owner/repository/pull/556#issuecomment-2","createdAt":"2026-06-06T00:01:00Z"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(gh-pr-comments.json) error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "pr_created",
		StateTo:   string(domain.StateCompleted),
		Payload:   `{"artifactDir":"` + prDir + `","url":"https://github.com/owner/repository/pull/556","pullNumber":556,"title":"PR filtered comments","head":"feature/pr-filtered"}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID+"/pr-comments", nil)
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	rec := httptest.NewRecorder()

	server.handlePRComments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var got struct {
		Comments []struct {
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"comments"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected only one visible comment, got %+v", got.Comments)
	}
	if got.Comments[0].Author != "alice" || got.Comments[0].Body != "local fixture comment" {
		t.Fatalf("unexpected visible comment: %+v", got.Comments[0])
	}
}

func TestHandleDesignApprovalPostsPRCommentAnalysis(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	submitter := &recordingPRCommentSubmitter{}
	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	server.SetPRCommentSubmitter(func(_ context.Context, req PRCommentSubmitRequest) error {
		submitter.req = req
		return nil
	})

	job := domain.Job{
		ID:           "job-pr-comment-analysis-approve",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 36,
		State:        domain.StateWaitingDesignApproval,
		Title:        "PR comment analysis",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	prDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)
	if err := os.MkdirAll(prDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.json"), []byte(`{"url":"https://github.com/owner/repository/pull/557","pullNumber":557,"repository":"owner/repository","branchName":"feature/pr-analysis","title":"PR comment analysis","pushed":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(prDir, "result.md"), []byte("analysis result body"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "pr_created",
		StateTo:   string(domain.StateCompleted),
		Payload:   `{"artifactDir":"` + prDir + `","url":"https://github.com/owner/repository/pull/557","pullNumber":557,"title":"PR comment analysis","head":"feature/pr-analysis"}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(pr_created) error = %v", err)
	}
	if err := store.AppendEvent(context.Background(), domain.Event{
		JobID:     job.ID,
		EventType: "pr_comment_analysis_ready",
		StateFrom: string(domain.StateDesignRunning),
		StateTo:   string(domain.StateWaitingDesignApproval),
		Payload:   `{"artifactDir":"` + prDir + `","pullNumber":557,"comment":{"author":"alice","body":"Please split this logic into a helper.","url":"https://github.com/owner/repository/pull/557#issuecomment-1","createdAt":"2026-06-06T00:00:00Z"}}`,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(pr_comment_analysis_ready) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/approvals/design", bytes.NewBufferString(`{"status":"approved","comment":"analysis result body"}`))
	req = mux.SetURLVars(req, map[string]string{"id": job.ID})
	rec := httptest.NewRecorder()

	server.handleDesignApproval(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if submitter.req.Repository != job.Repository {
		t.Fatalf("unexpected repository: %+v", submitter.req)
	}
	if submitter.req.PullNumber != 557 {
		t.Fatalf("unexpected pull number: %+v", submitter.req)
	}
	if !strings.HasPrefix(submitter.req.Body, prCommentAnalysisPostedMarker) {
		t.Fatalf("expected posted comment to include marker, got %q", submitter.req.Body)
	}
	if !strings.Contains(submitter.req.Body, "analysis result body") {
		t.Fatalf("expected posted comment to include analysis body, got %q", submitter.req.Body)
	}
}

func TestHandleJobsExcludesDeletedByDefaultAndCanShowDeletedOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	activeJob := domain.Job{
		ID:           "job-active",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 1,
		State:        domain.StateDetected,
		Title:        "active",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	deletedAt := time.Now().UTC()
	deletedJob := domain.Job{
		ID:           "job-deleted",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 2,
		State:        domain.StateCompleted,
		Title:        "deleted",
		DeletedAt:    &deletedAt,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), activeJob); err != nil {
		t.Fatalf("UpsertJob(active) error = %v", err)
	}
	if err := store.UpsertJob(context.Background(), deletedJob); err != nil {
		t.Fatalf("UpsertJob(deleted) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	server.handleJobs(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var active []jobResponse
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&active); err != nil {
		t.Fatalf("Decode(active response) error = %v", err)
	}
	if len(active) != 1 || active[0].ID != activeJob.ID {
		t.Fatalf("expected only active job, got %+v", active)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/jobs?deleted=only", nil)
	server.handleJobs(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var deleted []jobResponse
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&deleted); err != nil {
		t.Fatalf("Decode(deleted response) error = %v", err)
	}
	if len(deleted) != 1 || deleted[0].ID != deletedJob.ID || deleted[0].DeletedAt == "" {
		t.Fatalf("expected only deleted job, got %+v", deleted)
	}
}

func TestHandleDeleteAndRestoreJob(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-delete-restore",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 7,
		State:        domain.StateCompleted,
		Title:        "job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/delete", nil)
	request = mux.SetURLVars(request, map[string]string{"id": job.ID})
	server.handleDeleteJob(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var deleted jobDetailResponse
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&deleted); err != nil {
		t.Fatalf("Decode(delete response) error = %v", err)
	}
	if deleted.Job.DeletedAt == "" {
		t.Fatalf("expected deletedAt to be set, got %+v", deleted.Job)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/restore", nil)
	request = mux.SetURLVars(request, map[string]string{"id": job.ID})
	server.handleRestoreJob(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var restored jobDetailResponse
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&restored); err != nil {
		t.Fatalf("Decode(restore response) error = %v", err)
	}
	if restored.Job.DeletedAt != "" {
		t.Fatalf("expected deletedAt to be cleared, got %+v", restored.Job)
	}
}

func TestHandleStartToolCommandUsesRequestedToolCommandAndStop(t *testing.T) {
	t.Parallel()

	server, job := setupToolCommandJobServer(t)

	startBody := []byte(`{"toolCommand":"alt-tool"}`)
	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/start", bytes.NewReader(startBody))
	startRequest = mux.SetURLVars(startRequest, map[string]string{"id": job.ID})

	server.handleStartToolCommand(startRecorder, startRequest)

	if startRecorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, startRecorder.Code, startRecorder.Body.String())
	}

	var started jobDetailResponse
	if err := json.NewDecoder(bytes.NewReader(startRecorder.Body.Bytes())).Decode(&started); err != nil {
		t.Fatalf("Decode(start response) error = %v", err)
	}
	t.Cleanup(func() {
		stopRecorder := httptest.NewRecorder()
		stopRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/stop", nil)
		stopRequest = mux.SetURLVars(stopRequest, map[string]string{"id": job.ID})
		server.handleStopToolCommand(stopRecorder, stopRequest)
	})
	if started.ToolCommand == nil || started.ToolCommand.Name != "default-tool" {
		t.Fatalf("expected watch rule default tool command, got %+v", started.ToolCommand)
	}
	if started.ToolExecution == nil || started.ToolExecution.Name != "alt-tool" || !started.ToolExecution.Running {
		t.Fatalf("expected requested tool command to be running, got %+v", started.ToolExecution)
	}

	stopRecorder := httptest.NewRecorder()
	stopRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/stop", nil)
	stopRequest = mux.SetURLVars(stopRequest, map[string]string{"id": job.ID})

	server.handleStopToolCommand(stopRecorder, stopRequest)

	if stopRecorder.Code != http.StatusOK {
		t.Fatalf("expected stop status %d, got %d body=%s", http.StatusOK, stopRecorder.Code, stopRecorder.Body.String())
	}

	var stopped jobDetailResponse
	if err := json.NewDecoder(bytes.NewReader(stopRecorder.Body.Bytes())).Decode(&stopped); err != nil {
		t.Fatalf("Decode(stop response) error = %v", err)
	}
	if stopped.ToolExecution == nil || stopped.ToolExecution.Name != "alt-tool" || stopped.ToolExecution.Running {
		t.Fatalf("expected stopped tool execution to keep the requested name, got %+v", stopped.ToolExecution)
	}
}

func TestHandleStartToolCommandFallsBackToWatchRuleAndRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	server, job := setupToolCommandJobServer(t)

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/start", bytes.NewReader([]byte(`{}`)))
	startRequest = mux.SetURLVars(startRequest, map[string]string{"id": job.ID})

	server.handleStartToolCommand(startRecorder, startRequest)

	if startRecorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, startRecorder.Code, startRecorder.Body.String())
	}

	var started jobDetailResponse
	if err := json.NewDecoder(bytes.NewReader(startRecorder.Body.Bytes())).Decode(&started); err != nil {
		t.Fatalf("Decode(start response) error = %v", err)
	}
	t.Cleanup(func() {
		stopRecorder := httptest.NewRecorder()
		stopRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/stop", nil)
		stopRequest = mux.SetURLVars(stopRequest, map[string]string{"id": job.ID})
		server.handleStopToolCommand(stopRecorder, stopRequest)
	})
	if started.ToolExecution == nil || started.ToolExecution.Name != "default-tool" {
		t.Fatalf("expected watch rule default tool command, got %+v", started.ToolExecution)
	}

	stopRecorder := httptest.NewRecorder()
	stopRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/stop", nil)
	stopRequest = mux.SetURLVars(stopRequest, map[string]string{"id": job.ID})
	server.handleStopToolCommand(stopRecorder, stopRequest)
	if stopRecorder.Code != http.StatusOK {
		t.Fatalf("expected stop status %d, got %d body=%s", http.StatusOK, stopRecorder.Code, stopRecorder.Body.String())
	}

	unknownRecorder := httptest.NewRecorder()
	unknownRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/tool/start", bytes.NewReader([]byte(`{"toolCommand":"missing-tool"}`)))
	unknownRequest = mux.SetURLVars(unknownRequest, map[string]string{"id": job.ID})

	server.handleStartToolCommand(unknownRecorder, unknownRequest)

	if unknownRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unknown command to be rejected, got %d body=%s", unknownRecorder.Code, unknownRecorder.Body.String())
	}
}

func TestHandlePurgeJobRequiresDeletedJobAndKeepsArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	job := domain.Job{
		ID:           "job-purge-http",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 8,
		State:        domain.StateCompleted,
		Title:        "job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/purge", nil)
	request = mux.SetURLVars(request, map[string]string{"id": job.ID})
	server.handlePurgeJob(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected active job purge to be rejected, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/delete", nil)
	deleteRequest = mux.SetURLVars(deleteRequest, map[string]string{"id": job.ID})
	server.handleDeleteJob(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete to succeed, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(root, svc.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(artifactDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "result.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.txt) error = %v", err)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/jobs/"+job.ID+"/purge", nil)
	request = mux.SetURLVars(request, map[string]string{"id": job.ID})
	server.handlePurgeJob(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	if _, err := store.GetJob(context.Background(), job.ID); err == nil {
		t.Fatalf("expected job to be removed from DB")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "result.txt")); err != nil {
		t.Fatalf("expected artifact to remain, stat error = %v", err)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	request = mux.SetURLVars(request, map[string]string{"id": job.ID})
	server.handleJobDetail(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected purged job detail to return 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleJobDetailDoesNotReusePurgedArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	store, err := sqlite.Open(filepath.Join(root, "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	server := &Server{config: svc, orchestrator: orchestrator.New(store, nil)}
	appConfig := svc.App()
	rule := config.WatchRule{ID: "rule-issue", Name: "Issue"}
	event := domain.DomainEvent{
		Type: domain.DomainEventIssueMatched,
		Item: domain.RepositoryItem{
			Repository: "owner/repository",
			Number:     9,
			Title:      "issue",
			Target:     domain.TargetIssue,
		},
	}

	if err := server.orchestrator.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("first ProcessMatch() error = %v", err)
	}

	jobs, err := server.orchestrator.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	oldJobID := jobs[0].ID

	if err := server.orchestrator.DeleteJob(context.Background(), oldJobID); err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}
	if err := server.orchestrator.PurgeJob(context.Background(), oldJobID); err != nil {
		t.Fatalf("PurgeJob() error = %v", err)
	}

	oldArtifactDir := artifacts.WorkerDir(root, svc.App().ArtifactsDir, oldJobID, artifacts.WorkerDesign)
	if err := os.MkdirAll(oldArtifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(oldArtifactDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldArtifactDir, "result.md"), []byte("stale artifact"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	if err := server.orchestrator.ProcessMatch(context.Background(), appConfig, rule, event); err != nil {
		t.Fatalf("second ProcessMatch() error = %v", err)
	}

	jobs, err = server.orchestrator.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() after purge error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 recreated job, got %d", len(jobs))
	}
	newJobID := jobs[0].ID
	if newJobID == oldJobID {
		t.Fatalf("expected fresh job ID after purge, reused %q", oldJobID)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/jobs/"+newJobID, nil)
	request = mux.SetURLVars(request, map[string]string{"id": newJobID})
	server.handleJobDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected job detail to succeed, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var detail jobDetailResponse
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&detail); err != nil {
		t.Fatalf("Decode(job detail) error = %v", err)
	}
	if detail.DesignArtifact != nil {
		t.Fatalf("expected recreated job to ignore stale artifact, got %+v", detail.DesignArtifact)
	}
}
