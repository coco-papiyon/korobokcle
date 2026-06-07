package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestPublishApprovedImprovementCreatesBranchAndPushesFile(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	if err := runGit(t, root, "init", "--bare", remote); err != nil {
		t.Fatalf("git init --bare error = %v", err)
	}

	seed := filepath.Join(root, "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatalf("MkdirAll(seed) error = %v", err)
	}
	if err := runGit(t, seed, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := runGit(t, seed, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, seed, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if err := runGit(t, seed, "remote", "add", "origin", remote); err != nil {
		t.Fatalf("git remote add error = %v", err)
	}
	if err := runGit(t, seed, "push", "-u", "origin", "main"); err != nil {
		t.Fatalf("git push main error = %v", err)
	}

	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{
			Repository:         remote,
			WorkDir:            "",
			Workers:            1,
			ImprovementEnabled: true,
			ImprovementBranch:  "develop",
			ImprovementDir:     ".improvements",
		},
	}
	cfg := config.NewService(root, files)

	workDir, err := prepareRepositoryWorkspace(context.Background(), cfg, remote, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}
	if err := runGit(t, workDir, "config", "user.name", "test"); err != nil {
		t.Fatalf("git config user.name error = %v", err)
	}
	if err := runGit(t, workDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email error = %v", err)
	}

	const issueNumber = 42
	title := "ボタンを左に配置"
	draft := "# 画面レイアウト方針\n\n- 操作要素は左、説明は右\n"
	if err := publishApprovedImprovement(context.Background(), cfg, remote, issueNumber, title, draft, "fixture-pr-created", nil); err != nil {
		t.Fatalf("publishApprovedImprovement() error = %v", err)
	}

	inspect := filepath.Join(root, "inspect")
	if err := runGit(t, root, "clone", "--branch", "develop", remote, inspect); err != nil {
		t.Fatalf("git clone develop error = %v", err)
	}
	expectedPath := filepath.Join(inspect, ".improvements", artifacts.RepositoryWorkerWorkArtifactFileName(issueNumber, title))
	raw, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", expectedPath, err)
	}
	content := string(raw)
	if !strings.Contains(content, "title: ボタンを左に配置") {
		t.Fatalf("expected front matter title, got %q", content)
	}
	if !strings.Contains(content, "- design") || !strings.Contains(content, "- implementation") {
		t.Fatalf("expected derived phases in front matter, got %q", content)
	}
	if !strings.Contains(content, "jobId: fixture-pr-created") {
		t.Fatalf("expected front matter jobId, got %q", content)
	}
	if !strings.Contains(content, "- 操作要素は左、説明は右") {
		t.Fatalf("expected body content, got %q", content)
	}

	logPath := filepath.Join(artifacts.RepositoryWorkerJobDir(root, cfg.App().ArtifactsDir, remote, issueNumber), "improvement", "git-push.log")
	logRaw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(git-push.log) error = %v", err)
	}
	if !strings.Contains(string(logRaw), "develop") {
		t.Fatalf("expected git-push.log to mention develop, got %q", string(logRaw))
	}

	commitMessage, err := runGitCommand(context.Background(), inspect, "git", "log", "-1", "--pretty=%s")
	if err != nil {
		t.Fatalf("git log error = %v", err)
	}
	if got := strings.TrimSpace(commitMessage); got != "improvement: approve issue #42 ボタンを左に配置" {
		t.Fatalf("unexpected commit message: %q", got)
	}
}
