package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestJobsAPI(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "db", "jobs.json")
	store := newTestJobStore(storePath)
	settingsStore := newTestSettingsStore(domain.WatchSettings{Repository: "owner/repo"})

	cfg := config.Default()
	cfg.ToolDir = dir
	server := NewServer(cfg, store, settingsStore, nil)

	body := map[string]any{
		"kind":       string(domain.JobKindIssueDesign),
		"repository": "owner/repo",
		"number":     42,
		"title":      "design the thing",
	}
	reqBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusCreated)
	}

	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected store file: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	getRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRec.Code, http.StatusOK)
	}

	var resp struct {
		Jobs []domain.Job `json:"jobs"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(resp.Jobs))
	}
	if resp.Jobs[0].Repository != "owner/repo" {
		t.Fatalf("repository = %q, want owner/repo", resp.Jobs[0].Repository)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+resp.Jobs[0].ID, nil)
	detailRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", detailRec.Code, http.StatusOK)
	}

	var detail domain.Job
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("detail json.Unmarshal() error = %v", err)
	}
	if detail.ID != resp.Jobs[0].ID {
		t.Fatalf("detail id = %q, want %q", detail.ID, resp.Jobs[0].ID)
	}

	updateReqBody, err := json.Marshal(map[string]any{
		"state": string(domain.StateDesignRunning),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/jobs/"+detail.ID+"/state", bytes.NewReader(updateReqBody))
	updateRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", updateRec.Code, http.StatusOK)
	}

	var updated domain.Job
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("updated json.Unmarshal() error = %v", err)
	}
	if updated.State != domain.StateDesignRunning {
		t.Fatalf("updated state = %s, want %s", updated.State, domain.StateDesignRunning)
	}
}

func TestJobsAPIRejectsInvalidStateTransition(t *testing.T) {
	dir := t.TempDir()
	store := newTestJobStore(filepath.Join(dir, "db", "jobs.json"))
	settingsStore := newTestSettingsStore(domain.WatchSettings{Repository: "owner/repo"})

	cfg := config.Default()
	cfg.ToolDir = dir
	server := NewServer(cfg, store, settingsStore, nil)

	job := domain.Job{
		ID:         "job-1",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     42,
		Title:      "design the thing",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	updateReqBody, err := json.Marshal(map[string]any{
		"state": string(domain.StatePRCreated),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/jobs/"+job.ID+"/state", bytes.NewReader(updateReqBody))
	updateRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d, want %d", updateRec.Code, http.StatusBadRequest)
	}
}

func TestHealthz(t *testing.T) {
	server := NewServer(config.Default(), nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSkillsAPI(t *testing.T) {
	actions := &testSkillActions{statuses: []domain.SkillStatus{{Purpose: domain.SkillPurposeIssueDesign, Name: "design-from-issue"}}}
	server := NewServer(config.Default(), nil, nil, nil, actions)

	getReq := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	getRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/skills status = %d", getRec.Code)
	}

	body := bytes.NewBufferString(`{"testCommand":"go test ./...","maxFixLoops":3}`)
	postReq := httptest.NewRequest(http.MethodPost, "/api/skills", body)
	postRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK || actions.generateCalls != 1 {
		t.Fatalf("POST /api/skills status=%d calls=%d", postRec.Code, actions.generateCalls)
	}

	objectBody := bytes.NewBufferString(`{"testCommand":"go test ./...","maxFixLoops":3,"forcePurposes":{"purpose":"issue_design"}}`)
	objectReq := httptest.NewRequest(http.MethodPost, "/api/skills", objectBody)
	objectRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(objectRec, objectReq)
	if objectRec.Code != http.StatusOK || actions.generateCalls != 2 {
		t.Fatalf("POST object /api/skills status=%d calls=%d", objectRec.Code, actions.generateCalls)
	}

	mapBody := bytes.NewBufferString(`{"testCommand":"go test ./...","maxFixLoops":3,"forcePurposes":{"issue_design":true}}`)
	mapReq := httptest.NewRequest(http.MethodPost, "/api/skills", mapBody)
	mapRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(mapRec, mapReq)
	if mapRec.Code != http.StatusOK || actions.generateCalls != 3 {
		t.Fatalf("POST map /api/skills status=%d calls=%d", mapRec.Code, actions.generateCalls)
	}
}

type testSkillActions struct {
	statuses      []domain.SkillStatus
	generateCalls int
}

func (a *testSkillActions) SkillStatus(context.Context) ([]domain.SkillStatus, error) {
	return a.statuses, nil
}

func (a *testSkillActions) GenerateSkills(_ context.Context, _ domain.SkillGenerationRequest) (domain.SkillGenerationResult, error) {
	a.generateCalls++
	return domain.SkillGenerationResult{Provider: domain.AIProviderCodex, Skills: a.statuses}, nil
}

func TestSettingsAPI(t *testing.T) {
	store := newTestSettingsStore(domain.WatchSettings{
		Repository: "owner/repo",
		AIProvider: domain.AIProviderGitHubCopilot,
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeDefault},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-4.1"},
		},
	})
	server := NewServer(config.Default(), nil, store, nil)

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRec.Code, http.StatusOK)
	}

	var settings domain.WatchSettings
	if err := json.Unmarshal(getRec.Body.Bytes(), &settings); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if settings.Repository != "owner/repo" {
		t.Fatalf("repository = %q, want owner/repo", settings.Repository)
	}
	if settings.PollIntervalSeconds != 120 {
		t.Fatalf("poll interval = %d, want 120", settings.PollIntervalSeconds)
	}
	if settings.BranchNamePattern != "issue_#<issue番号>" {
		t.Fatalf("branch name pattern = %q, want issue_#<issue番号>", settings.BranchNamePattern)
	}
	if settings.AIProvider != domain.AIProviderGitHubCopilot {
		t.Fatalf("ai provider = %q, want %q", settings.AIProvider, domain.AIProviderGitHubCopilot)
	}
	if settings.Models.GitHubCopilot.Mode != domain.ModelModeCustom || settings.Models.GitHubCopilot.Value != "gpt-4.1" {
		t.Fatalf("github copilot model = %+v, want custom gpt-4.1", settings.Models.GitHubCopilot)
	}

	updateBody, err := json.Marshal(domain.WatchSettings{
		Repository:          "owner/updated",
		AIProvider:          domain.AIProviderCodex,
		PollIntervalSeconds: 240,
		BranchNamePattern:   "feature/<issue番号>",
		Models: domain.AIModels{
			Codex: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "codex-1"},
		},
		Issue: domain.SearchCondition{
			LabelIncludes: []string{"bug"},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(updateBody))
	putRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putRec.Code, http.StatusOK)
	}

	updated, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if updated.Repository != "owner/updated" {
		t.Fatalf("updated repository = %q, want owner/updated", updated.Repository)
	}
	if updated.AIProvider != domain.AIProviderCodex {
		t.Fatalf("updated ai provider = %q, want %q", updated.AIProvider, domain.AIProviderCodex)
	}
	if updated.PollIntervalSeconds != 240 {
		t.Fatalf("updated poll interval = %d, want 240", updated.PollIntervalSeconds)
	}
	if updated.BranchNamePattern != "feature/<issue番号>" {
		t.Fatalf("updated branch name pattern = %q, want feature/<issue番号>", updated.BranchNamePattern)
	}
	if updated.Models.Codex.Mode != domain.ModelModeCustom || updated.Models.Codex.Value != "codex-1" {
		t.Fatalf("updated codex model = %+v, want custom codex-1", updated.Models.Codex)
	}
	if len(updated.Issue.LabelIncludes) != 1 || updated.Issue.LabelIncludes[0] != "bug" {
		t.Fatalf("updated issue labels = %+v, want [bug]", updated.Issue.LabelIncludes)
	}
}

func TestStaticAssetsAndSPAFallback(t *testing.T) {
	dir := t.TempDir()
	distDir := filepath.Join(dir, "frontend", "dist", "assets")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "dist", "index.html"), []byte("<html><body>index</body></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile index.html error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "index-test.js"), []byte("export default 1;"), 0o644); err != nil {
		t.Fatalf("WriteFile js error = %v", err)
	}

	cfg := config.Default()
	cfg.ToolDir = dir
	server := NewServer(cfg, nil, nil, nil)

	assetReq := httptest.NewRequest(http.MethodGet, "/assets/index-test.js", nil)
	assetRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(assetRec, assetReq)
	if assetRec.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d", assetRec.Code, http.StatusOK)
	}
	if ct := assetRec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("asset content-type = %q, want javascript", ct)
	}

	spaReq := httptest.NewRequest(http.MethodGet, "/jobs/123", nil)
	spaRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(spaRec, spaReq)
	if spaRec.Code != http.StatusOK {
		t.Fatalf("spa status = %d, want %d", spaRec.Code, http.StatusOK)
	}
	if body := spaRec.Body.String(); !strings.Contains(body, "index") {
		t.Fatalf("spa body = %q, want index html", body)
	}
}

type testJobStore struct {
	path string

	mu   sync.Mutex
	jobs map[string]domain.Job
}

func newTestJobStore(path string) *testJobStore {
	return &testJobStore{path: path, jobs: map[string]domain.Job{}}
}

func (s *testJobStore) List(context.Context) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, job)
	}
	return out, nil
}

func (s *testJobStore) Get(_ context.Context, id string) (domain.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok, nil
}

func (s *testJobStore) Upsert(_ context.Context, job domain.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	raw, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return err
	}
	return nil
}

var _ JobStore = (*testJobStore)(nil)

type testSettingsStore struct {
	mu       sync.Mutex
	settings domain.WatchSettings
}

func newTestSettingsStore(settings domain.WatchSettings) *testSettingsStore {
	return &testSettingsStore{settings: settings}
}

func (s *testSettingsStore) Load(context.Context) (domain.WatchSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}

func (s *testSettingsStore) Save(_ context.Context, settings domain.WatchSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
	return nil
}

var _ SettingsStore = (*testSettingsStore)(nil)
