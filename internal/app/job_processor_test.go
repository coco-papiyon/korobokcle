package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

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

	stdoutRaw, err := os.ReadFile(filepath.Join(toolDir, "logs", "114", "design_stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile stdout log error = %v", err)
	}
	if !strings.Contains(string(stdoutRaw), "fake stdout") {
		t.Fatalf("stdout log = %q, want fake stdout", string(stdoutRaw))
	}
	stderrRaw, err := os.ReadFile(filepath.Join(toolDir, "logs", "114", "design_stderr.log"))
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

	worktreePath := implementationWorktreePath(toolDir, job)
	raw, err := os.ReadFile(filepath.Join(worktreePath, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "after") {
		t.Fatalf("worktree file missing applied diff: %s", string(raw))
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

func TestAppendIssueAILog(t *testing.T) {
	toolDir := t.TempDir()
	processor := &WorkflowProcessor{toolDir: toolDir}
	job := domain.Job{ID: "issue-114", Kind: domain.JobKindIssueImplementation, State: domain.StateImplementationRunning, Number: 114}

	processor.appendIssueAILog(job, "request", "hello")

	raw, err := os.ReadFile(filepath.Join(toolDir, "logs", "114", "implementation.log"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "request") || !strings.Contains(string(raw), "hello") {
		t.Fatalf("log content = %q", string(raw))
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
