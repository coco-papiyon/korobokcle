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
	body := buildResultBody("result text", "")
	if body != "### 結果\nresult text" {
		t.Fatalf("body = %q", body)
	}
}

func TestBuildResultBodyIncludeUserComment(t *testing.T) {
	body := buildResultBody("result text", " please fix ")
	want := "### 結果\nresult text\n\n### ユーザコメント\nplease fix"
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
