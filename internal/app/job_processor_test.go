package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestWorkflowProcessorPersistsFailureStateAndMessage(t *testing.T) {
	baseDir := t.TempDir()
	store := newMemoryJobStore()
	processor := newWorkflowProcessor(
		store,
		&workflowTestSettingsStore{settings: domain.NormalizeWatchSettings(domain.WatchSettings{
			Repository: "owner/repo",
			AIProvider: domain.AIProviderCodex,
		})},
		nil,
		baseDir,
		t.TempDir(),
		nil,
		fakeAIRunner{err: errors.New("test command failed")},
		fakeJobContextLoader{content: "Issue context"},
	)
	job := domain.Job{
		ID: "issue-500", Kind: domain.JobKindIssueDesign, State: domain.StateDetected,
		Repository: "owner/repo", Number: 500, Title: "失敗確認",
	}
	if err := processor.Process(context.Background(), job); err == nil {
		t.Fatal("Process() error = nil, want error")
	}
	updated, ok, err := store.Get(context.Background(), job.ID)
	if err != nil || !ok {
		t.Fatalf("Get() = (%+v, %v, %v), want stored job", updated, ok, err)
	}
	if updated.State != domain.StateFailed {
		t.Fatalf("state = %s, want %s", updated.State, domain.StateFailed)
	}
	if !strings.Contains(updated.ErrorMessage, "test command failed") {
		t.Fatalf("errorMessage = %q, want runner error", updated.ErrorMessage)
	}
}

func TestWorkflowProcessorProcessesDesignJob(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()

	store := newMemoryJobStore()
	settingsStore := &workflowTestSettingsStore{
		settings: domain.NormalizeWatchSettings(domain.WatchSettings{
			Repository:          "owner/repo",
			AIProvider:          domain.AIProviderCodex,
			PollIntervalSeconds: 120,
			Models: domain.AIModels{
				Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.5"},
				GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeDefault},
			},
		}),
	}

	processor := NewWorkflowProcessorWithDeps(
		store,
		settingsStore,
		NewFileDesignFeedbackStore(filepath.Join(toolDir, "workspace", "design_feedback")),
		baseDir,
		toolDir,
		nil,
		fakeAIRunner{response: AIResponse{ArtifactMarkdown: "## Output\n設計結果"}},
		fakeJobContextLoader{content: "Issue context"},
	)
	job := domain.Job{
		ID:         "issue-114",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     114,
		Title:      "画面構成変更",
		FetchedAt:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC),
	}

	if err := processor(context.Background(), job); err != nil {
		t.Fatalf("processor() error = %v", err)
	}

	updated, ok, err := store.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found in store")
	}
	if updated.State != domain.StateDesignReady {
		t.Fatalf("updated state = %s, want %s", updated.State, domain.StateDesignReady)
	}
	if !updated.FetchedAt.Equal(job.FetchedAt) {
		t.Fatalf("fetchedAt = %s, want %s", updated.FetchedAt, job.FetchedAt)
	}
	if updated.UpdatedAt.Equal(job.UpdatedAt) || updated.UpdatedAt.IsZero() {
		t.Fatalf("updatedAt = %s, want a new timestamp", updated.UpdatedAt)
	}

	artifactPath := filepath.Join(baseDir, ".workspace", "design", "114_画面構成変更.md")
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(raw)
	want := "# 画面構成変更\n\n## Output\n設計結果"
	if content != want {
		t.Fatalf("artifact content = %q, want %q", content, want)
	}

	stdoutRaw, err := os.ReadFile(filepath.Join(jobLogDir(toolDir, job), "design_attempt-1_agent_stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile stdout log error = %v", err)
	}
	if !strings.Contains(string(stdoutRaw), "fake stdout") {
		t.Fatalf("stdout log = %q, want fake stdout", string(stdoutRaw))
	}
	stderrRaw, err := os.ReadFile(filepath.Join(jobLogDir(toolDir, job), "design_attempt-1_agent_stderr.log"))
	if err != nil {
		t.Fatalf("ReadFile stderr log error = %v", err)
	}
	if !strings.Contains(string(stderrRaw), "fake stderr") {
		t.Fatalf("stderr log = %q, want fake stderr", string(stderrRaw))
	}
}

func TestWorkflowProcessorProcessesImplementationJob(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	store := newMemoryJobStore()
	settingsStore := &workflowTestSettingsStore{
		settings: domain.NormalizeWatchSettings(domain.WatchSettings{
			Repository:        "owner/repo",
			AIProvider:        domain.AIProviderCodex,
			BranchNamePattern: "issue_#<issue番号>",
			Models: domain.AIModels{
				Codex: domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.4-mini"},
			},
		}),
	}

	runGitTestCommand(t, baseDir, "init", "-b", "main")
	runGitTestCommand(t, baseDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, baseDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(baseDir, "README.md"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGitTestCommand(t, baseDir, "add", "README.md")
	runGitTestCommand(t, baseDir, "commit", "-m", "initial")

	if err := os.MkdirAll(filepath.Join(baseDir, ".workspace", "design"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, ".workspace", "design", "114_画面構成変更.md"), []byte("# design"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	diff := strings.Join([]string{
		"diff --git a/README.md b/README.md",
		"index df967b9..9f30ee3 100644",
		"--- a/README.md",
		"+++ b/README.md",
		"@@ -1 +1,2 @@",
		" before",
		"+after",
		"",
	}, "\n")
	processor := NewWorkflowProcessorWithDeps(
		store,
		settingsStore,
		NewFileDesignFeedbackStore(filepath.Join(toolDir, "workspace", "design_feedback")),
		baseDir,
		toolDir,
		nil,
		fakeAIRunner{response: AIResponse{ArtifactMarkdown: "## Output\n実装結果", GitDiff: diff}},
		fakeJobContextLoader{content: "Issue context"},
	)

	job := domain.Job{
		ID:         "issue-114",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateDesignApproved,
		Repository: "owner/repo",
		Number:     114,
		Title:      "画面構成変更",
		FetchedAt:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC),
	}
	if err := processor(context.Background(), job); err != nil {
		t.Fatalf("processor() error = %v", err)
	}

	updated, ok, err := store.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found in store")
	}
	if updated.State != domain.StateImplementationReady {
		t.Fatalf("updated state = %s, want %s", updated.State, domain.StateImplementationReady)
	}
	if !updated.FetchedAt.Equal(job.FetchedAt) {
		t.Fatalf("fetchedAt = %s, want %s", updated.FetchedAt, job.FetchedAt)
	}
	if updated.UpdatedAt.Equal(job.UpdatedAt) || updated.UpdatedAt.IsZero() {
		t.Fatalf("updatedAt = %s, want a new timestamp", updated.UpdatedAt)
	}

	worktreePath := implementationWorktreePath(toolDir, job)
	raw, err := os.ReadFile(filepath.Join(worktreePath, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "after") {
		t.Fatalf("worktree file missing applied diff: %s", string(raw))
	}
}

func TestImplementationLoopPassesVerifierFeedbackToNextAttempt(t *testing.T) {
	implementer := &recordingAIRunner{responses: []AIResponse{
		{ArtifactMarkdown: "## 概要\n初回実装"},
		{ArtifactMarkdown: "## 概要\n修正実装"},
	}}
	verifier := &recordingAIRunner{responses: []AIResponse{
		{RawOutput: `{"status":"changes_requested","feedback":"境界値テストを追加してください","summary":"テスト不足"}`},
		{RawOutput: `{"status":"passed","feedback":"","summary":"必要なテストを確認しました"}`},
	}}
	processor := &WorkflowProcessor{
		baseDir: t.TempDir(), toolDir: t.TempDir(), runner: implementer, verifier: verifier,
	}
	job := domain.Job{
		ID: "issue-600", Kind: domain.JobKindIssueImplementation, State: domain.StateImplementationRunning,
		Repository: "owner/repo", Number: 600, Title: "ループ実装",
	}
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		AIProvider: domain.AIProviderCodex, ImplementationLoopCount: 2,
	})

	artifact, err := processor.runImplementationLoop(
		context.Background(), job, settings, "", "Issue context", processor.baseDir, "issue_#600",
		domain.StateImplementationRunning, domain.StateImplementationReady,
	)
	if err != nil {
		t.Fatalf("runImplementationLoop() error = %v", err)
	}
	if len(implementer.requests) != 2 || len(verifier.requests) != 2 {
		t.Fatalf("request counts = implementer:%d verifier:%d, want 2 each", len(implementer.requests), len(verifier.requests))
	}
	if !strings.Contains(implementer.requests[1].Prompt, "境界値テストを追加してください") {
		t.Fatalf("second implementer prompt does not contain verifier feedback:\n%s", implementer.requests[1].Prompt)
	}
	if !strings.Contains(artifact, "必要なテストを確認しました") {
		t.Fatalf("artifact does not contain verification summary: %s", artifact)
	}
	if !strings.Contains(verifier.requests[0].Prompt, "Do not edit, create, delete") {
		t.Fatalf("verifier prompt does not prohibit repository edits:\n%s", verifier.requests[0].Prompt)
	}
}

func TestImplementationLoopFailsAtConfiguredLimit(t *testing.T) {
	implementer := &recordingAIRunner{responses: []AIResponse{{ArtifactMarkdown: "実装1"}, {ArtifactMarkdown: "実装2"}}}
	verifier := &recordingAIRunner{responses: []AIResponse{
		{RawOutput: `{"status":"changes_requested","feedback":"失敗1","summary":"不合格1"}`},
		{RawOutput: `{"status":"changes_requested","feedback":"失敗2","summary":"不合格2"}`},
	}}
	processor := &WorkflowProcessor{baseDir: t.TempDir(), toolDir: t.TempDir(), runner: implementer, verifier: verifier}
	job := domain.Job{ID: "issue-601", Kind: domain.JobKindIssueImplementation, Number: 601, Title: "上限確認"}
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{AIProvider: domain.AIProviderCodex, ImplementationLoopCount: 2})

	_, err := processor.runImplementationLoop(context.Background(), job, settings, "", "", processor.baseDir, "issue_#601", domain.StateImplementationRunning, domain.StateImplementationReady)
	if err == nil || !strings.Contains(err.Error(), "after 2 attempts") {
		t.Fatalf("runImplementationLoop() error = %v, want configured-limit error", err)
	}
}

func TestParseImplementationVerificationRejectsUnknownStatus(t *testing.T) {
	_, err := parseImplementationVerification(`{"status":"maybe","summary":"判定不能"}`)
	if err == nil || !strings.Contains(err.Error(), "invalid verifier status") {
		t.Fatalf("parseImplementationVerification() error = %v, want invalid status", err)
	}
}

func TestBuildPromptIncludesMandatoryImplementationSkill(t *testing.T) {
	workDir := t.TempDir()
	skillDir := filepath.Join(workDir, ".agents", "skills", "implement-from-design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "## 必須出力形式\n## 概要\n## 変更内容\n## テスト結果\n## 残課題"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	processor := &WorkflowProcessor{baseDir: t.TempDir()}
	job := domain.Job{ID: "issue-146", Kind: domain.JobKindIssueImplementation, Number: 146, Title: "表示修正"}
	prompt := processor.buildPrompt(
		job,
		domain.WatchSettings{AIProvider: domain.AIProviderGitHubCopilot},
		"",
		"Issue context",
		workDir,
		"issue_#146",
		domain.StateImplementationRunning,
		domain.StateImplementationReady,
	)
	for _, want := range []string{
		"Mandatory Agent Skill instructions (implement-from-design):",
		skill,
		"Do not return progress updates as the final response.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "base_dir:") {
		t.Fatalf("implementation prompt exposes base_dir:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Do not access the original repository root") {
		t.Fatalf("implementation prompt does not restrict access to working_dir:\n%s", prompt)
	}
}

func TestWorkDirForJobPrunesMissingRegisteredWorktree(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		BranchNamePattern: "issue_#<issue番号>",
	})
	job := domain.Job{
		ID:         "issue-114",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateDesignApproved,
		Repository: "owner/repo",
		Number:     114,
		Title:      "画面構成変更",
	}

	runGitTestCommand(t, baseDir, "init", "-b", "main")
	runGitTestCommand(t, baseDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, baseDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(baseDir, "README.md"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGitTestCommand(t, baseDir, "add", "README.md")
	runGitTestCommand(t, baseDir, "commit", "-m", "initial")

	processor := &WorkflowProcessor{baseDir: baseDir, toolDir: toolDir}
	worktreePath := implementationWorktreePath(toolDir, job)
	if _, _, err := processor.workDirForJob(context.Background(), job, settings); err != nil {
		t.Fatalf("initial workDirForJob() error = %v", err)
	}
	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	workDir, branch, err := processor.workDirForJob(context.Background(), job, settings)
	if err != nil {
		t.Fatalf("workDirForJob() error = %v", err)
	}
	if workDir != worktreePath {
		t.Fatalf("workDir = %q, want %q", workDir, worktreePath)
	}
	if branch != "issue_#114" {
		t.Fatalf("branch = %q, want issue_#114", branch)
	}
	if _, err := os.Stat(filepath.Join(worktreePath, ".git")); err != nil {
		t.Fatalf("worktree was not recreated: %v", err)
	}
}

func TestWorkDirForJobRebasesRemoteBranchBeforeImplementation(t *testing.T) {
	baseDir := t.TempDir()
	remoteDir := filepath.Join(baseDir, "remote.git")
	repoDir := filepath.Join(baseDir, "repo")
	otherRepoDir := filepath.Join(baseDir, "other")
	toolDir := t.TempDir()
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		BranchNamePattern: "issue_#<issue番号>",
	})
	job := domain.Job{
		ID:         "issue-114",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateDesignApproved,
		Repository: "owner/repo",
		Number:     114,
		Title:      "画面構成変更",
	}

	runGitTestCommand(t, baseDir, "init", "--bare", remoteDir)
	runGitTestCommand(t, baseDir, "clone", remoteDir, repoDir)
	runGitTestCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, repoDir, "config", "user.name", "Test User")
	runGitTestCommand(t, repoDir, "checkout", "-b", "main")
	writeTestFile(t, repoDir, "README.md", "initial\n")
	runGitTestCommand(t, repoDir, "add", "README.md")
	runGitTestCommand(t, repoDir, "commit", "-m", "initial")
	runGitTestCommand(t, repoDir, "push", "-u", "origin", "main")
	runGitTestCommand(t, repoDir, "checkout", "-b", "issue_#114")
	writeTestFile(t, repoDir, "branch.txt", "local branch\n")
	runGitTestCommand(t, repoDir, "add", "branch.txt")
	runGitTestCommand(t, repoDir, "commit", "-m", "branch start")
	runGitTestCommand(t, repoDir, "push", "-u", "origin", "issue_#114")
	runGitTestCommand(t, repoDir, "checkout", "main")

	runGitTestCommand(t, baseDir, "clone", remoteDir, otherRepoDir)
	runGitTestCommand(t, otherRepoDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, otherRepoDir, "config", "user.name", "Test User")
	runGitTestCommand(t, otherRepoDir, "checkout", "issue_#114")
	writeTestFile(t, otherRepoDir, "remote.txt", "remote change\n")
	runGitTestCommand(t, otherRepoDir, "add", "remote.txt")
	runGitTestCommand(t, otherRepoDir, "commit", "-m", "remote implementation base")
	runGitTestCommand(t, otherRepoDir, "push", "origin", "issue_#114")

	processor := &WorkflowProcessor{baseDir: repoDir, toolDir: toolDir}
	workDir, branch, err := processor.workDirForJob(context.Background(), job, settings)
	if err != nil {
		t.Fatalf("workDirForJob() error = %v", err)
	}
	if branch != "issue_#114" {
		t.Fatalf("branch = %q, want issue_#114", branch)
	}
	if _, err := os.Stat(filepath.Join(workDir, "remote.txt")); err != nil {
		t.Fatalf("worktree was not rebased from remote branch: %v", err)
	}
}

func TestWorkDirForJobKeepsDirtyWorktree(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	settings := domain.NormalizeWatchSettings(domain.WatchSettings{
		BranchNamePattern: "issue_#<issue番号>",
	})
	job := domain.Job{
		ID:         "issue-127",
		Kind:       domain.JobKindIssueImplementation,
		State:      domain.StateDesignApproved,
		Repository: "owner/repo",
		Number:     127,
		Title:      "dirty worktree",
	}

	runGitTestCommand(t, baseDir, "init", "-b", "main")
	runGitTestCommand(t, baseDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, baseDir, "config", "user.name", "Test User")
	writeTestFile(t, baseDir, "README.md", "initial\n")
	runGitTestCommand(t, baseDir, "add", "README.md")
	runGitTestCommand(t, baseDir, "commit", "-m", "initial")

	worktreePath := implementationWorktreePath(toolDir, job)
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	runGitTestCommand(t, baseDir, "worktree", "add", "-B", "issue_#127", worktreePath, "HEAD")
	writeTestFile(t, worktreePath, "README.md", "dirty change\n")

	processor := &WorkflowProcessor{baseDir: baseDir, toolDir: toolDir}
	workDir, branch, err := processor.workDirForJob(context.Background(), job, settings)
	if err != nil {
		t.Fatalf("workDirForJob() error = %v", err)
	}
	if workDir != worktreePath {
		t.Fatalf("workDir = %q, want %q", workDir, worktreePath)
	}
	if branch != "issue_#127" {
		t.Fatalf("branch = %q, want issue_#127", branch)
	}
	raw, err := os.ReadFile(filepath.Join(worktreePath, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(raw) != "dirty change\n" {
		t.Fatalf("dirty worktree content changed unexpectedly: %q", string(raw))
	}
}

func TestAppendIssueAILog(t *testing.T) {
	toolDir := t.TempDir()
	processor := &WorkflowProcessor{toolDir: toolDir}
	job := domain.Job{ID: "issue-114", Kind: domain.JobKindIssueImplementation, State: domain.StateImplementationRunning, Number: 114}

	processor.appendIssueAILog(job, 1, "agent", "request", "hello")

	raw, err := os.ReadFile(filepath.Join(jobLogDir(toolDir, job), "implementation_attempt-1_agent.log"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "request") || !strings.Contains(string(raw), "hello") {
		t.Fatalf("log content = %q", string(raw))
	}
}

func TestImplementationJobExcludesReviewFixed(t *testing.T) {
	job := domain.Job{Kind: domain.JobKindPRFeedback, State: domain.StateReviewFixed}
	if implementationJob(job) {
		t.Fatal("expected review_fixed to be treated as review workflow, not implementation workflow")
	}
}

func TestStripLeadingH1(t *testing.T) {
	artifact := "# 画面構成変更 設計\n\n## 概要\n設計結果"
	want := "## 概要\n設計結果"
	if got := stripLeadingH1(artifact); got != want {
		t.Fatalf("stripLeadingH1() = %q, want %q", got, want)
	}
}

type workflowTestSettingsStore struct {
	mu       sync.Mutex
	settings domain.WatchSettings
}

func (s *workflowTestSettingsStore) Load(context.Context) (domain.WatchSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}

func (s *workflowTestSettingsStore) Save(_ context.Context, settings domain.WatchSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
	return nil
}

var _ SettingsStore = (*workflowTestSettingsStore)(nil)

type fakeAIRunner struct {
	response AIResponse
	err      error
}

type recordingAIRunner struct {
	requests  []AIRequest
	responses []AIResponse
	err       error
}

func (r *recordingAIRunner) Run(_ context.Context, req AIRequest) (AIResponse, error) {
	r.requests = append(r.requests, req)
	if r.err != nil {
		return AIResponse{}, r.err
	}
	if len(r.responses) == 0 {
		return AIResponse{}, errors.New("no fake AI response")
	}
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response, nil
}

func (r fakeAIRunner) Run(_ context.Context, req AIRequest) (AIResponse, error) {
	if req.Stdout != nil {
		_, _ = req.Stdout.Write([]byte("fake stdout\n"))
	}
	if req.Stderr != nil {
		_, _ = req.Stderr.Write([]byte("fake stderr\n"))
	}
	return r.response, r.err
}

type fakeJobContextLoader struct {
	content string
	err     error
}

func (l fakeJobContextLoader) Load(context.Context, domain.Job) (string, error) {
	return l.content, l.err
}
