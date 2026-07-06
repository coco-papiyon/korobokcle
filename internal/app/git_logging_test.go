package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

type bufferLogger struct {
	lines []string
}

func (l *bufferLogger) Infof(string, ...any) {}

func (l *bufferLogger) Debugf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func TestRunGitLoggedEmitsDebugLog(t *testing.T) {
	logger := &bufferLogger{}
	repoDir := t.TempDir()
	runGitTestCommand(t, repoDir, "init", "-b", "main")
	if err := runGitLogged(context.Background(), logger, repoDir, "", "status", "--porcelain"); err != nil {
		t.Fatalf("runGitLogged() error = %v", err)
	}
	if len(logger.lines) == 0 {
		t.Fatal("expected debug log for git operation")
	}
	if !strings.Contains(logger.lines[0], "git -C") || !strings.Contains(logger.lines[0], "status --porcelain") {
		t.Fatalf("debug log = %q, want git command", logger.lines[0])
	}
}

func TestRunGitLoggedEmitsWorktreeNote(t *testing.T) {
	logger := &bufferLogger{}
	repoDir := t.TempDir()
	runGitTestCommand(t, repoDir, "init", "-b", "main")
	if err := runGitLogged(context.Background(), logger, repoDir, "worktree=workspace/sample/worktree", "status", "--porcelain"); err != nil {
		t.Fatalf("runGitLogged() error = %v", err)
	}
	if len(logger.lines) == 0 {
		t.Fatal("expected debug log for git operation")
	}
	if !strings.Contains(logger.lines[0], "worktree=workspace/sample/worktree") {
		t.Fatalf("debug log = %q, want worktree note", logger.lines[0])
	}
}

func TestPublishBranchLoggedPushesLocalBranchToRemoteBranch(t *testing.T) {
	baseDir := t.TempDir()
	remoteDir := filepath.Join(baseDir, "remote.git")
	repoDir := filepath.Join(baseDir, "repo")

	runGitTestCommand(t, baseDir, "init", "--bare", remoteDir)
	runGitTestCommand(t, baseDir, "clone", remoteDir, repoDir)
	runGitTestCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, repoDir, "config", "user.name", "Test User")
	runGitTestCommand(t, repoDir, "checkout", "-b", "main")
	writeTestFile(t, repoDir, "README.md", "initial\n")
	runGitTestCommand(t, repoDir, "add", "README.md")
	runGitTestCommand(t, repoDir, "commit", "-m", "initial")
	runGitTestCommand(t, repoDir, "push", "-u", "origin", "main")

	runGitTestCommand(t, repoDir, "checkout", "-b", "issue_#159__pr-conflict-160")
	writeTestFile(t, repoDir, "conflict.txt", "resolved\n")
	runGitTestCommand(t, repoDir, "add", "conflict.txt")
	runGitTestCommand(t, repoDir, "commit", "-m", "resolve conflict")

	if err := publishBranchLogged(context.Background(), nil, repoDir, "", "issue_#159__pr-conflict-160", "issue_#159"); err != nil {
		t.Fatalf("publishBranchLogged() error = %v", err)
	}

	remoteLog := runGitTestOutput(t, remoteDir, "log", "--format=%s", "refs/heads/issue_#159")
	if !strings.Contains(remoteLog, "resolve conflict") {
		t.Fatalf("remote log = %q, want resolved commit on issue_#159", remoteLog)
	}
}
