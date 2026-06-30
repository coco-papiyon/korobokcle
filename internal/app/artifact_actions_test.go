package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestBuildResultBodyOmitEmptyUserComment(t *testing.T) {
	body := buildResultBody(domain.Job{Kind: domain.JobKindIssueDesign}, "# ジョブ一覧のフィルタ\n\n## 概要\nresult text", "")
	want := "# 設計結果\n\n## 概要\nresult text"
	if body != want {
		t.Fatalf("body = %q", body)
	}
}

func TestBuildResultBodyIncludeUserComment(t *testing.T) {
	body := buildResultBody(domain.Job{Kind: domain.JobKindIssueImplementation}, "## 概要\nresult text", " please fix ")
	want := "# 実装結果\n\n## 概要\nresult text\n\n### ユーザコメント\nplease fix"
	if body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

func TestBuildResultBodyReviewFeedbackTitle(t *testing.T) {
	body := buildResultBody(domain.Job{Kind: domain.JobKindPRFeedback, State: domain.StateReviewFixImplementationReady}, "## 概要\nfixed", "")
	want := "# レビュー指摘修正結果\n\n## 概要\nfixed"
	if body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

func TestCompleteApprovalPersistsBeforeRepositoryRefresh(t *testing.T) {
	store := newMemoryJobStore()
	job := domain.Job{ID: "issue-114", State: domain.StateCompleted}
	monitor := &approvalTestMonitor{store: store, jobID: job.ID}
	service := &ArtifactActionService{store: store, monitor: monitor}

	if err := service.completeApproval(context.Background(), job); err != nil {
		t.Fatalf("completeApproval() error = %v", err)
	}
	if monitor.calls != 1 {
		t.Fatalf("monitor calls = %d, want 1", monitor.calls)
	}
	if monitor.stateAtPoll != domain.StateCompleted {
		t.Fatalf("state at poll = %s, want %s", monitor.stateAtPoll, domain.StateCompleted)
	}
}

func TestStateLabelsExceptKeepsOnlyLatestState(t *testing.T) {
	remove := stateLabelsExcept(domain.MustLabel(domain.StatePRCreated))
	joined := strings.Join(remove, ",")
	if strings.Contains(joined, domain.MustLabel(domain.StatePRCreated)) {
		t.Fatalf("remove labels = %v, should not include latest state", remove)
	}
	for _, label := range []string{
		domain.MustLabel(domain.StateDesignApproved),
		domain.MustLabel(domain.StateImplementationApproved),
	} {
		if !strings.Contains(joined, label) {
			t.Fatalf("remove labels = %v, want %s", remove, label)
		}
	}
}

func TestExistingLabelsOnly(t *testing.T) {
	got := existingLabelsOnly(
		[]string{"state:detected", "state:review_ready", "state:pr_created", "STATE:REVIEW_READY"},
		[]string{"state:review_ready", "state:pr_created"},
		[]string{"state:pr_created"},
	)
	if len(got) != 1 || got[0] != "state:review_ready" {
		t.Fatalf("existingLabelsOnly() = %v, want [state:review_ready]", got)
	}
}

type approvalTestMonitor struct {
	mu          sync.Mutex
	store       JobStore
	jobID       string
	calls       int
	stateAtPoll domain.JobState
}

func (m *approvalTestMonitor) PollNow(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	job, _, err := m.store.Get(ctx, m.jobID)
	if err == nil {
		m.stateAtPoll = job.State
	}
	return err
}

func TestRenderBranchName(t *testing.T) {
	if got := renderBranchName("issue_#<issue番号>", 114); got != "issue_#114" {
		t.Fatalf("renderBranchName() = %q, want issue_#114", got)
	}
	if got := renderBranchName("feature/<issueNumber>", 7); got != "feature/7" {
		t.Fatalf("renderBranchName() = %q, want feature/7", got)
	}
}

func TestImplementationWorktreeBranchName(t *testing.T) {
	job := domain.Job{ID: "issue-114"}
	if got := implementationWorktreeBranchName("issue_#114", job); got != "issue_#114__issue-114" {
		t.Fatalf("implementationWorktreeBranchName() = %q, want issue_#114__issue-114", got)
	}
}

func TestCheckoutOrCreateBranch(t *testing.T) {
	dir := t.TempDir()
	runGitTestCommand(t, dir, "init", "-b", "main")
	runGitTestCommand(t, dir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGitTestCommand(t, dir, "add", "README.md")
	runGitTestCommand(t, dir, "commit", "-m", "initial")

	if err := checkoutOrCreateBranch(context.Background(), dir, "issue_#114"); err != nil {
		t.Fatalf("checkoutOrCreateBranch() error = %v", err)
	}

	current := strings.TrimSpace(runGitTestOutput(t, dir, "branch", "--show-current"))
	if current != "issue_#114" {
		t.Fatalf("current branch = %q, want issue_#114", current)
	}
}

func TestEnsureBranchHasCommitAddsEmptyCommit(t *testing.T) {
	dir := t.TempDir()
	runGitTestCommand(t, dir, "init", "-b", "main")
	runGitTestCommand(t, dir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGitTestCommand(t, dir, "add", "README.md")
	runGitTestCommand(t, dir, "commit", "-m", "initial")
	runGitTestCommand(t, dir, "checkout", "-b", "issue_#114")

	if err := ensureBranchHasCommit(context.Background(), dir, "issue_#114"); err != nil {
		t.Fatalf("ensureBranchHasCommit() error = %v", err)
	}

	count := strings.TrimSpace(runGitTestOutput(t, dir, "rev-list", "--count", "main..HEAD"))
	if count != "1" {
		t.Fatalf("commit count = %q, want 1", count)
	}
}

func TestPublishBranchPushesToOrigin(t *testing.T) {
	baseDir := t.TempDir()
	remoteDir := filepath.Join(baseDir, "remote.git")
	repoDir := filepath.Join(baseDir, "repo")

	runGitTestCommand(t, baseDir, "init", "--bare", remoteDir)
	runGitTestCommand(t, baseDir, "clone", remoteDir, repoDir)
	runGitTestCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGitTestCommand(t, repoDir, "add", "README.md")
	runGitTestCommand(t, repoDir, "commit", "-m", "initial")
	runGitTestCommand(t, repoDir, "checkout", "-b", "issue_#114")

	if err := publishBranch(context.Background(), repoDir, "issue_#114"); err != nil {
		t.Fatalf("publishBranch() error = %v", err)
	}

	remoteBranch := strings.TrimSpace(runGitTestOutput(t, remoteDir, "rev-parse", "--verify", "refs/heads/issue_#114"))
	if remoteBranch == "" {
		t.Fatal("remote branch was not created")
	}
}

func TestPublishBranchRebasesRemoteBranchBeforePush(t *testing.T) {
	baseDir := t.TempDir()
	remoteDir := filepath.Join(baseDir, "remote.git")
	repoDir := filepath.Join(baseDir, "repo")
	otherRepoDir := filepath.Join(baseDir, "other")

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
	writeTestFile(t, repoDir, "local.txt", "first local\n")
	runGitTestCommand(t, repoDir, "add", "local.txt")
	runGitTestCommand(t, repoDir, "commit", "-m", "first local")
	if err := publishBranch(context.Background(), repoDir, "issue_#114"); err != nil {
		t.Fatalf("publishBranch() initial error = %v", err)
	}

	runGitTestCommand(t, baseDir, "clone", remoteDir, otherRepoDir)
	runGitTestCommand(t, otherRepoDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, otherRepoDir, "config", "user.name", "Test User")
	runGitTestCommand(t, otherRepoDir, "checkout", "issue_#114")
	writeTestFile(t, otherRepoDir, "remote.txt", "remote change\n")
	runGitTestCommand(t, otherRepoDir, "add", "remote.txt")
	runGitTestCommand(t, otherRepoDir, "commit", "-m", "remote change")
	runGitTestCommand(t, otherRepoDir, "push", "origin", "issue_#114")

	writeTestFile(t, repoDir, "local2.txt", "second local\n")
	runGitTestCommand(t, repoDir, "add", "local2.txt")
	runGitTestCommand(t, repoDir, "commit", "-m", "second local")
	if err := publishBranch(context.Background(), repoDir, "issue_#114"); err != nil {
		t.Fatalf("publishBranch() rebase error = %v", err)
	}

	remoteLog := runGitTestOutput(t, remoteDir, "log", "--format=%s", "refs/heads/issue_#114")
	for _, want := range []string{"second local", "remote change", "first local"} {
		if !strings.Contains(remoteLog, want) {
			t.Fatalf("remote log = %q, want commit %q", remoteLog, want)
		}
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func runGitTestOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}
