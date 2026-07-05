package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	server := NewServer(cfg, store, settingsStore, nil, testBranchResolver{branch: "issue_#42"}, nil, nil)

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
		UpdatedAt string       `json:"updatedAt"`
		Jobs      []domain.Job `json:"jobs"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.UpdatedAt == "" {
		t.Fatal("updatedAt is empty")
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(resp.Jobs))
	}
	if resp.Jobs[0].Repository != "owner/repo" {
		t.Fatalf("repository = %q, want owner/repo", resp.Jobs[0].Repository)
	}
	if resp.Jobs[0].Branch != "issue_#42" {
		t.Fatalf("branch = %q, want issue_#42", resp.Jobs[0].Branch)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/jobs/"+resp.Jobs[0].ID, nil)
	detailRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", detailRec.Code, http.StatusOK)
	}

	var detail struct {
		UpdatedAt string     `json:"updatedAt"`
		Job       domain.Job `json:"job"`
		Branch    string     `json:"branch"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("detail json.Unmarshal() error = %v", err)
	}
	if detail.UpdatedAt == "" {
		t.Fatal("detail updatedAt is empty")
	}
	if detail.Job.ID != resp.Jobs[0].ID {
		t.Fatalf("detail id = %q, want %q", detail.Job.ID, resp.Jobs[0].ID)
	}
	if detail.Branch != "issue_#42" {
		t.Fatalf("detail branch = %q, want issue_#42", detail.Branch)
	}

	updateReqBody, err := json.Marshal(map[string]any{
		"state": string(domain.StateDesignRunning),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/jobs/"+detail.Job.ID+"/state", bytes.NewReader(updateReqBody))
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
	if updated.UpdatedAt.IsZero() {
		t.Fatal("updatedAt is zero after state patch")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+detail.Job.ID, nil)
	deleteRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", deleteRec.Code, http.StatusNoContent)
	}
	if _, ok, err := store.Get(context.Background(), detail.Job.ID); err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	} else if ok {
		t.Fatal("job still exists after delete")
	}
}

func TestJobsAPIIncludesSubStatus(t *testing.T) {
	dir := t.TempDir()
	store := newTestJobStore(filepath.Join(dir, "db", "jobs.json"))
	settingsStore := newTestSettingsStore(domain.WatchSettings{Repository: "owner/repo"})

	job := domain.Job{
		ID:         "issue-impl",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateImplementationRunning,
		SubStatus:  "検証(1回目)",
		Repository: "owner/repo",
		Number:     42,
		Title:      "implementation target",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	cfg := config.Default()
	cfg.ToolDir = dir
	server := NewServer(cfg, store, settingsStore, nil, testBranchResolver{branch: "issue_#42"}, nil, nil)

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
	if resp.Jobs[0].SubStatus != "検証(1回目)" {
		t.Fatalf("subStatus = %q, want 検証(1回目)", resp.Jobs[0].SubStatus)
	}
}

func TestJobsAPIRejectsInvalidStateTransition(t *testing.T) {
	dir := t.TempDir()
	store := newTestJobStore(filepath.Join(dir, "db", "jobs.json"))
	settingsStore := newTestSettingsStore(domain.WatchSettings{Repository: "owner/repo"})

	cfg := config.Default()
	cfg.ToolDir = dir
	server := NewServer(cfg, store, settingsStore, nil, testBranchResolver{branch: "issue_#42"}, nil, nil)

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
	server := NewServer(config.Default(), nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestArtifactRequestChangesAPI(t *testing.T) {
	actions := &testArtifactActions{
		job: domain.Job{
			ID:         "pr-12",
			Kind:       domain.JobKindPRReview,
			State:      domain.StateCompleted,
			Repository: "owner/repo",
			Number:     12,
			Title:      "review target",
		},
	}
	server := NewServer(config.Default(), newTestJobStore(filepath.Join(t.TempDir(), "jobs.json")), nil, actions, nil, nil, nil)

	body := bytes.NewBufferString(`{"comment":"追加でここも修正"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/pr-12/artifact/request-changes", body)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("request changes status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if actions.requestChangesID != "pr-12" || actions.requestChangesComment != "追加でここも修正" {
		t.Fatalf("request changes id=%q comment=%q", actions.requestChangesID, actions.requestChangesComment)
	}
}

func TestArtifactUpdateAPI(t *testing.T) {
	actions := &testArtifactActions{
		job: domain.Job{
			ID:         "issue-14",
			Kind:       domain.JobKindIssueDesign,
			State:      domain.StateDesignReady,
			Repository: "owner/repo",
			Number:     14,
			Title:      "design target",
		},
	}
	server := NewServer(config.Default(), newTestJobStore(filepath.Join(t.TempDir(), "jobs.json")), nil, actions, nil, nil, nil)

	body := bytes.NewBufferString(`{"content":"edited artifact"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/jobs/issue-14/artifact/content", body)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update artifact status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var artifact DesignArtifact
	if err := json.Unmarshal(rec.Body.Bytes(), &artifact); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if artifact.Content != "edited artifact" {
		t.Fatalf("artifact content = %q, want edited artifact", artifact.Content)
	}
	if actions.updateArtifactID != "issue-14" || actions.updateArtifactContent != "edited artifact" {
		t.Fatalf("update artifact id=%q content=%q", actions.updateArtifactID, actions.updateArtifactContent)
	}
}

func TestJobSourceDiffAPI(t *testing.T) {
	actions := &testArtifactActions{
		job: domain.Job{
			ID:         "issue-102",
			Kind:       domain.JobKindIssueImplementation,
			State:      domain.StateImplementationApproved,
			Repository: "owner/repo",
			Number:     102,
			Title:      "implementation target",
		},
	}
	server := NewServer(config.Default(), newTestJobStore(filepath.Join(t.TempDir(), "jobs.json")), nil, actions, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/issue-102/diff", nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("source diff status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var diff JobSourceDiff
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !strings.Contains(diff.Content, "diff --git") {
		t.Fatalf("content = %q, want git diff", diff.Content)
	}
	if diff.BaseRef != "main" {
		t.Fatalf("baseRef = %q, want main", diff.BaseRef)
	}
}

type testArtifactActions struct {
	job                   domain.Job
	requestChangesID      string
	requestChangesComment string
	updateArtifactID      string
	updateArtifactContent string
}

func (a *testArtifactActions) GetArtifact(context.Context, string) (DesignArtifact, error) {
	return DesignArtifact{Content: "artifact", Path: ".workspace/review/12_review.md"}, nil
}

func (a *testArtifactActions) GetSourceDiff(context.Context, string) (JobSourceDiff, error) {
	return JobSourceDiff{
		Content: "diff --git a/README.md b/README.md\n+after\n",
		Path:    "workspace/mock-owner_mock-repo/issue-102/worktree",
		BaseRef: "main",
	}, nil
}

func (a *testArtifactActions) UpdateArtifact(_ context.Context, id, content string) (DesignArtifact, error) {
	a.updateArtifactID = id
	a.updateArtifactContent = content
	return DesignArtifact{Content: content, Path: ".workspace/design/14_design.md"}, nil
}

func (a *testArtifactActions) ApproveArtifact(context.Context, string, string) (domain.Job, error) {
	return a.job, nil
}

func (a *testArtifactActions) RequestChanges(_ context.Context, id, comment string) (domain.Job, error) {
	a.requestChangesID = id
	a.requestChangesComment = comment
	return a.job, nil
}

func (a *testArtifactActions) RerunArtifact(context.Context, string, string) (domain.Job, error) {
	return a.job, nil
}

func TestSkillsAPI(t *testing.T) {
	actions := &testSkillActions{statuses: []domain.SkillStatus{{Purpose: domain.SkillPurposeIssueDesign, Name: "design-from-issue"}}}
	server := NewServer(config.Default(), nil, nil, nil, nil, nil, actions)

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

type testBranchResolver struct {
	branch string
	err    error
}

func (r testBranchResolver) ResolveJobBranch(context.Context, domain.Job) (string, error) {
	return r.branch, r.err
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
		Repository:        "owner/repo",
		BaseBranch:        "develop",
		AIProvider:        domain.AIProviderGitHubCopilot,
		VerificationAIProvider: domain.AIProviderCodex,
		VerificationAIModel:    domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.4-mini"},
		ReviewerAIProvider:     domain.AIProviderGitHubCopilot,
		ReviewerAIModel:        domain.ModelSelection{Mode: domain.ModelModeDefault},
		AIAllowedCommands: []string{"npm ci"},
		Models: domain.AIModels{
			Codex:         domain.ModelSelection{Mode: domain.ModelModeDefault},
			GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-4.1"},
		},
	})
	server := NewServer(config.Default(), nil, store, nil, nil, nil)

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
	if settings.JobConcurrency != 4 {
		t.Fatalf("job concurrency = %d, want 4", settings.JobConcurrency)
	}
	if settings.BaseBranch != "develop" {
		t.Fatalf("base branch = %q, want develop", settings.BaseBranch)
	}
	if settings.BranchNamePattern != "issue_#<issue番号>" {
		t.Fatalf("branch name pattern = %q, want issue_#<issue番号>", settings.BranchNamePattern)
	}
	if len(settings.AIAllowedCommands) != 1 || settings.AIAllowedCommands[0] != "npm ci" {
		t.Fatalf("ai allowed commands = %+v, want [npm ci]", settings.AIAllowedCommands)
	}
	if settings.AIProvider != domain.AIProviderGitHubCopilot {
		t.Fatalf("ai provider = %q, want %q", settings.AIProvider, domain.AIProviderGitHubCopilot)
	}
	if settings.Models.GitHubCopilot.Mode != domain.ModelModeCustom || settings.Models.GitHubCopilot.Value != "gpt-4.1" {
		t.Fatalf("github copilot model = %+v, want custom gpt-4.1", settings.Models.GitHubCopilot)
	}
	if settings.VerificationAIProvider != domain.AIProviderCodex {
		t.Fatalf("verification ai provider = %q, want %q", settings.VerificationAIProvider, domain.AIProviderCodex)
	}
	if settings.VerificationAIModel.Mode != domain.ModelModeCustom || settings.VerificationAIModel.Value != "gpt-5.4-mini" {
		t.Fatalf("verification ai model = %+v, want custom gpt-5.4-mini", settings.VerificationAIModel)
	}
	if settings.ReviewerAIProvider != domain.AIProviderGitHubCopilot {
		t.Fatalf("reviewer ai provider = %q, want %q", settings.ReviewerAIProvider, domain.AIProviderGitHubCopilot)
	}
	if settings.ReviewerAIModel.Mode != domain.ModelModeDefault {
		t.Fatalf("reviewer ai model = %+v, want default", settings.ReviewerAIModel)
	}

	updateBody, err := json.Marshal(domain.WatchSettings{
		Repository:          "owner/updated",
		AIProvider:          domain.AIProviderCodex,
		PollIntervalSeconds: 240,
		JobConcurrency:      6,
		BaseBranch:          "release",
		BranchNamePattern:   "feature/<issue番号>",
		AIAllowedCommands:   []string{"npm ci", "go test ./..."},
		Models: domain.AIModels{
			Codex: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "codex-1"},
		},
		VerificationAIProvider: domain.AIProviderGitHubCopilot,
		VerificationAIModel:    domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "claude-opus-4.6"},
		ReviewerAIProvider:     domain.AIProviderCodex,
		ReviewerAIModel:        domain.ModelSelection{Mode: domain.ModelModeDefault},
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
	if updated.JobConcurrency != 6 {
		t.Fatalf("updated job concurrency = %d, want 6", updated.JobConcurrency)
	}
	if updated.BaseBranch != "release" {
		t.Fatalf("updated base branch = %q, want release", updated.BaseBranch)
	}
	if updated.BranchNamePattern != "feature/<issue番号>" {
		t.Fatalf("updated branch name pattern = %q, want feature/<issue番号>", updated.BranchNamePattern)
	}
	if len(updated.AIAllowedCommands) != 2 || updated.AIAllowedCommands[0] != "npm ci" || updated.AIAllowedCommands[1] != "go test ./..." {
		t.Fatalf("updated ai allowed commands = %+v, want [npm ci go test ./...]", updated.AIAllowedCommands)
	}
	if updated.Models.Codex.Mode != domain.ModelModeCustom || updated.Models.Codex.Value != "codex-1" {
		t.Fatalf("updated codex model = %+v, want custom codex-1", updated.Models.Codex)
	}
	if updated.VerificationAIProvider != domain.AIProviderGitHubCopilot {
		t.Fatalf("updated verification ai provider = %q, want %q", updated.VerificationAIProvider, domain.AIProviderGitHubCopilot)
	}
	if updated.VerificationAIModel.Mode != domain.ModelModeCustom || updated.VerificationAIModel.Value != "claude-opus-4.6" {
		t.Fatalf("updated verification ai model = %+v, want custom claude-opus-4.6", updated.VerificationAIModel)
	}
	if updated.ReviewerAIProvider != domain.AIProviderCodex {
		t.Fatalf("updated reviewer ai provider = %q, want %q", updated.ReviewerAIProvider, domain.AIProviderCodex)
	}
	if updated.ReviewerAIModel.Mode != domain.ModelModeDefault {
		t.Fatalf("updated reviewer ai model = %+v, want default", updated.ReviewerAIModel)
	}
	if len(updated.Issue.LabelIncludes) != 1 || updated.Issue.LabelIncludes[0] != "bug" {
		t.Fatalf("updated issue labels = %+v, want [bug]", updated.Issue.LabelIncludes)
	}
}

func TestJobDetailAPIIncludesBranch(t *testing.T) {
	store := newTestJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	job := domain.Job{
		ID:         "pr-7",
		Kind:       domain.JobKindPRReview,
		State:      domain.StateReviewRunning,
		Repository: "owner/repo",
		Number:     7,
		Title:      "review target",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	server := NewServer(config.Default(), store, nil, nil, testBranchResolver{branch: "feature/pr-7"}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", rec.Code, http.StatusOK)
	}

	var detail struct {
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if detail.Branch != "feature/pr-7" {
		t.Fatalf("branch = %q, want feature/pr-7", detail.Branch)
	}
}

func TestJobDetailAPIIncludesIssueContext(t *testing.T) {
	store := newTestJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	job := domain.Job{
		ID:         "issue-1",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     1,
		Title:      "design target",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	server := NewServer(
		config.Default(),
		store,
		nil,
		nil,
		testBranchResolver{branch: "issue_#1"},
		testContextLoader{content: "#1 design target\n\nDetailed description"},
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", rec.Code, http.StatusOK)
	}

	var detail struct {
		IssueContext string `json:"issueContext"`
		Job          struct {
			IssueContext string `json:"issueContext"`
		} `json:"job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !strings.Contains(detail.Job.IssueContext, "#1 design target") {
		t.Fatalf("issueContext = %q, want issue text", detail.Job.IssueContext)
	}
}

func TestJobDetailAPIIncludesLogs(t *testing.T) {
	dir := t.TempDir()
	store := newTestJobStore(filepath.Join(dir, "jobs.json"))
	job := domain.Job{
		ID:         "job-log",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateCompleted,
		Repository: "owner/repo",
		Number:     2,
		Title:      "log target",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	logDir := jobWorkspaceLogDir(dir, job)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	files := map[string]string{
		"implementation_attempt-1_agent.log":           "agent request\nagent response",
		"implementation_attempt-1_agent_stdout.log":    "stdout 1",
		"implementation_attempt-1_agent_stderr.log":    "stderr 1",
		"implementation_attempt-1_verifier.log":        "verifier summary",
		"implementation_attempt-1_verifier_stdout.log": "verifier stdout",
		"implementation_attempt-1_verifier_stderr.log": "verifier stderr",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(logDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	cfg := config.Default()
	cfg.ToolDir = dir
	cfg.WorkDir = dir
	server := NewServer(cfg, store, nil, nil, testBranchResolver{branch: "issue_#2"}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", rec.Code, http.StatusOK)
	}

	var detail struct {
		Logs []JobLogGroup `json:"logs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(detail.Logs) != 2 {
		t.Fatalf("logs = %d, want 2", len(detail.Logs))
	}
	if detail.Logs[0].RoleLabel != "実装者" || detail.Logs[0].Attempt != 1 {
		t.Fatalf("first log group = %+v, want 実装者 attempt 1", detail.Logs[0])
	}
	if len(detail.Logs[0].Files) != 3 {
		t.Fatalf("first log files = %d, want 3", len(detail.Logs[0].Files))
	}
	if detail.Logs[1].RoleLabel != "検証者" || detail.Logs[1].Attempt != 1 {
		t.Fatalf("second log group = %+v, want 検証者 attempt 1", detail.Logs[1])
	}
}

func TestJobDetailAPIEmptyBranchOnResolverError(t *testing.T) {
	store := newTestJobStore(filepath.Join(t.TempDir(), "jobs.json"))
	job := domain.Job{
		ID:         "issue-1",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     1,
		Title:      "design target",
	}
	if err := store.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	server := NewServer(config.Default(), store, nil, nil, testBranchResolver{err: errors.New("boom")}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+job.ID, nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d", rec.Code, http.StatusOK)
	}

	var detail struct {
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if detail.Branch != "" {
		t.Fatalf("branch = %q, want empty", detail.Branch)
	}
}

type testContextLoader struct {
	content string
	err     error
	calls   int
}

func (l testContextLoader) Load(context.Context, domain.Job) (string, error) {
	if l.err != nil {
		return "", l.err
	}
	return l.content, nil
}

func TestStaticAssetsAndSPAFallback(t *testing.T) {
	dir := t.TempDir()
	distDir := filepath.Join(dir, "static", "assets")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "static", "index.html"), []byte("<html><body>index</body></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile index.html error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "index-test.js"), []byte("export default 1;"), 0o644); err != nil {
		t.Fatalf("WriteFile js error = %v", err)
	}

	cfg := config.Default()
	cfg.ToolDir = dir
	server := NewServer(cfg, nil, nil, nil, nil, nil)

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

	mu        sync.Mutex
	jobs      map[string]domain.Job
	updatedAt time.Time
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
	s.updatedAt = time.Now().UTC()
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

func (s *testJobStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	s.updatedAt = time.Now().UTC()
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

func (s *testJobStore) UpdatedAt(context.Context) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updatedAt, nil
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
