package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/notification"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestCloneRepositoryWorkspaceClonesLocalRepository(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("clone test"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	workerDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	baseDir := artifacts.RepositoryWorkerBaseDir(root, cfg.App().ArtifactsDir, source, 0)
	if _, err := os.Stat(filepath.Join(baseDir, ".git")); err != nil {
		t.Fatalf("expected base git repository: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerDir, ".git")); err != nil {
		t.Fatalf("expected cloned git repository: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "README.md")); err != nil {
		t.Fatalf("expected cloned base file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerDir, "README.md")); err != nil {
		t.Fatalf("expected cloned file: %v", err)
	}
	if workerDir != artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, source, "main") {
		t.Fatalf("unexpected worker dir: %s", workerDir)
	}
	if _, err := os.Stat(filepath.Join(workerDir, ".workspace")); !os.IsNotExist(err) {
		t.Fatalf("expected no workspace directory, got err=%v", err)
	}
}

func TestCloneRepositoryWorkspaceReplacesExistingNonGitDirectory(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("preserve test"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	baseDir := artifacts.RepositoryWorkerBaseDir(root, cfg.App().ArtifactsDir, source, 0)
	if err := os.MkdirAll(filepath.Join(baseDir, ".workspace", "issue_42", "design"), 0o755); err != nil {
		t.Fatalf("MkdirAll(workspaceDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, ".workspace", "issue_42", "design", "result.md"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	workerDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	if workerDir != artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, source, "main") {
		t.Fatalf("unexpected worker dir: %s", workerDir)
	}
	if _, err := os.Stat(filepath.Join(workerDir, ".workspace")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace directory to be removed, got err=%v", err)
	}
}

func TestCloneRepositoryWorkspaceRemovesStaleWorkspaceDirectory(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("preserve test"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, source, "main")
	legacyWorkspaceDir := filepath.Join(workerDir, ".workspace", "issue_42", "design")
	if err := os.MkdirAll(legacyWorkspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyWorkspaceDir, "result.md"), []byte("keep me"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	clonedDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	if clonedDir != workerDir {
		t.Fatalf("unexpected worker dir: %s", clonedDir)
	}

	if _, err := os.Stat(filepath.Join(workerDir, ".git")); err != nil {
		t.Fatalf("expected cloned git repository: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerDir, ".workspace")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace directory to be cleared, got err=%v", err)
	}
}

func TestCloneRepositoryWorkspaceUsesRepositoryRemoteInsteadOfSharedImprovementCheckout(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "develop"); err != nil {
		t.Fatalf("git checkout develop error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("develop\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README develop error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add develop error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "develop"); err != nil {
		t.Fatalf("git commit develop error = %v", err)
	}
	if err := runGit(t, source, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	workDir, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}
	repositoryConfig := config.MonitoredRepository{
		Repository:         source,
		ImprovementEnabled: true,
		ImprovementBranch:  "develop",
	}
	improvementDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, source, config.ResolveImprovementBranch(repositoryConfig))
	if _, err := prepareRepositoryImprovementWorkspace(context.Background(), cfg, repositoryConfig); err != nil {
		t.Fatalf("prepareRepositoryImprovementWorkspace() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(improvementDir, ".improvement", "draft"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.improvement/draft) error = %v", err)
	}
	if err := syncRepositoryImprovementWorkspace(context.Background(), cfg, repositoryConfig, improvementDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryImprovementWorkspace() error = %v", err)
	}

	workerDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0, workDir)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), workerDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "main" {
		t.Fatalf("expected worker clone to stay on main, got %q", currentBranch)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(workerDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README.md error = %v", err)
	}
	if string(readmeRaw) != "main\n" {
		t.Fatalf("expected worker clone from repository remote, got %q", string(readmeRaw))
	}
}

func TestBuildRepositoryDesignContextIgnoresLegacyArtifactDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)

	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, svc.App().ArtifactsDir, "owner/repository", "main")
	legacyDesignDir := filepath.Join(workerDir, ".workspace", "design", "job-42")
	if err := os.MkdirAll(legacyDesignDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(legacyDesignDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDesignDir, "result.md"), []byte("legacy design"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	job := domain.Job{
		ID:           "job-42",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
	}

	ctx, err := buildRepositoryDesignContext(svc, workerDir, "", job, nil)
	if err != nil {
		t.Fatalf("buildRepositoryDesignContext() error = %v", err)
	}
	if ctx.ExistingDesign != "" {
		t.Fatalf("expected legacy artifact directory to be ignored, got %q", ctx.ExistingDesign)
	}
}

func TestBuildRepositoryContextsUseLatestIssueBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.ArtifactsDir = "artifacts"
	files.WatchRules.Rules = []config.WatchRule{{ID: "rule-1", SkillSet: "default"}}
	svc := config.NewService(root, files)

	job := domain.Job{
		ID:           "job-42",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
		BranchName:   "issue-42",
	}

	designDir := repositoryWorkerArtifactDir(svc, job.Repository, job.GitHubNumber, artifacts.WorkerDesign)
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(designDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "result.md"), []byte("design"), 0o644); err != nil {
		t.Fatalf("WriteFile(design result.md) error = %v", err)
	}

	events := []domain.Event{
		{
			EventType: string(domain.DomainEventIssueMatched),
			Payload:   `{"body":"original body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
			CreatedAt: time.Now(),
		},
		{
			EventType: "issue_body_refreshed",
			Payload:   `{"body":"latest body"}`,
			CreatedAt: time.Now(),
		},
	}

	designCtx, err := buildRepositoryDesignContext(svc, root, "", job, events)
	if err != nil {
		t.Fatalf("buildRepositoryDesignContext() error = %v", err)
	}
	if designCtx.Body != "latest body" {
		t.Fatalf("expected latest body in design context, got %q", designCtx.Body)
	}
	if designCtx.Author != "alice" || len(designCtx.Labels) != 1 || designCtx.Labels[0] != "bug" || len(designCtx.Assignees) != 1 || designCtx.Assignees[0] != "bob" {
		t.Fatalf("expected issue metadata in design context, got %+v", designCtx)
	}

	runSpec := implementationRunSpec{
		SkillName:   "implement",
		ArtifactDir: repositoryWorkerArtifactDir(svc, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation),
	}
	implDir := repositoryWorkerArtifactDir(svc, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(implDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(implDir, "result.md"), []byte("implementation"), 0o644); err != nil {
		t.Fatalf("WriteFile(implementation result.md) error = %v", err)
	}

	implCtx, err := buildRepositoryImplementationContext(svc, root, "", job, events, runSpec)
	if err != nil {
		t.Fatalf("buildRepositoryImplementationContext() error = %v", err)
	}
	if implCtx.Body != "latest body" {
		t.Fatalf("expected latest body in implementation context, got %q", implCtx.Body)
	}
	if implCtx.Author != "alice" || len(implCtx.Labels) != 1 || implCtx.Labels[0] != "bug" || len(implCtx.Assignees) != 1 || implCtx.Assignees[0] != "bob" {
		t.Fatalf("expected issue metadata in implementation context, got %+v", implCtx)
	}
}

func TestSyncRepositoryWorkspaceResetsToBaseBranchAndPulls(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "base"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	workerDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}

	if err := runGit(t, workerDir, "checkout", "-b", "feature/test"); err != nil {
		t.Fatalf("git checkout feature error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workerDir, "README.md"), []byte("worker change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile worker README error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workerDir, "TEMP.txt"), []byte("temp\n"), 0o644); err != nil {
		t.Fatalf("WriteFile TEMP.txt error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("remote update\n"), 0o644); err != nil {
		t.Fatalf("WriteFile source README error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add updated README error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "update"); err != nil {
		t.Fatalf("git commit update error = %v", err)
	}

	job := domain.Job{ID: "job-1"}
	if err := syncRepositoryWorkspace(context.Background(), cfg, job, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), workerDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "main" {
		t.Fatalf("expected main branch, got %q", currentBranch)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(workerDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README.md error = %v", err)
	}
	if string(readmeRaw) != "remote update\n" {
		t.Fatalf("expected synced README, got %q", string(readmeRaw))
	}
	if _, err := os.Stat(filepath.Join(workerDir, "TEMP.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected TEMP.txt removed, stat err = %v", err)
	}
}

func TestSyncRepositoryWorkspaceUsesPullRequestBranchForPRFeedback(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "feature/review-42"); err != nil {
		t.Fatalf("git checkout feature error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("feature remote\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README feature error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add feature error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "feature"); err != nil {
		t.Fatalf("git commit feature error = %v", err)
	}
	if err := runGit(t, source, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	workerDir, err := cloneRepositoryWorkspace(context.Background(), cfg, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	if err := runGit(t, workerDir, "checkout", "main"); err != nil {
		t.Fatalf("git checkout worker main error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workerDir, "LOCAL.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatalf("WriteFile LOCAL error = %v", err)
	}

	job := domain.Job{
		ID:         "job-feedback",
		Type:       domain.JobTypePRFeedback,
		BranchName: "feature/review-42",
	}
	if err := syncRepositoryWorkspace(context.Background(), cfg, job, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), workerDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "feature/review-42" {
		t.Fatalf("expected feature/review-42 branch, got %q", currentBranch)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(workerDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README error = %v", err)
	}
	if string(readmeRaw) != "feature remote\n" {
		t.Fatalf("expected feature branch content, got %q", string(readmeRaw))
	}
	if _, err := os.Stat(filepath.Join(workerDir, "LOCAL.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected LOCAL.txt removed, stat err = %v", err)
	}
}

func TestCloneRepositoryWorkspaceForBranchIsIdempotentWhenWorktreeExists(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	baseDir, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}

	worktreeDir, err := cloneRepositoryWorkspaceForBranch(context.Background(), cfg, source, "main", baseDir)
	if err != nil {
		t.Fatalf("first cloneRepositoryWorkspaceForBranch() error = %v", err)
	}

	again, err := cloneRepositoryWorkspaceForBranch(context.Background(), cfg, source, "main", baseDir)
	if err != nil {
		t.Fatalf("second cloneRepositoryWorkspaceForBranch() error = %v", err)
	}
	if again != worktreeDir {
		t.Fatalf("expected reused worktree dir %q, got %q", worktreeDir, again)
	}
}

func TestCloneRepositoryWorkspaceForBranchRepairsMissingWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	baseDir, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}

	worktreeDir, err := cloneRepositoryWorkspaceForBranch(context.Background(), cfg, source, "main", baseDir)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspaceForBranch() error = %v", err)
	}
	if err := os.RemoveAll(worktreeDir); err != nil {
		t.Fatalf("RemoveAll(worktreeDir) error = %v", err)
	}

	repairedDir, err := cloneRepositoryWorkspaceForBranch(context.Background(), cfg, source, "main", baseDir)
	if err != nil {
		t.Fatalf("repair cloneRepositoryWorkspaceForBranch() error = %v", err)
	}
	if repairedDir != worktreeDir {
		t.Fatalf("expected repaired worktree dir %q, got %q", worktreeDir, repairedDir)
	}
	if _, err := os.Stat(repairedDir); err != nil {
		t.Fatalf("expected repaired worktree to exist: %v", err)
	}
}

func TestSameRepositoryWorkerPathNormalizesSeparators(t *testing.T) {
	t.Parallel()

	left := "C:/data/dev/go/src/github.com/coco-papiyon/korobokcle/tests/base/source/coco-papiyon-korobokcle-issue_101"
	right := `C:\data\dev\go\src\github.com\coco-papiyon\korobokcle\tests\base\source\coco-papiyon-korobokcle-issue_101`
	if !sameRepositoryWorkerPath(left, right) {
		t.Fatalf("expected paths to match: %q vs %q", left, right)
	}
}

func TestSyncRepositoryWorkspaceUsesConfiguredMonitoredRepositoryBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "release/1.x"); err != nil {
		t.Fatalf("git checkout release error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("release\n"), 0o644); err != nil {
		t.Fatalf("WriteFile release error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add release error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "release"); err != nil {
		t.Fatalf("git commit release error = %v", err)
	}
	if err := runGit(t, source, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main error = %v", err)
	}

	cfg := config.DefaultFiles()
	cfg.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repo", Branch: "release/1.x", ImplementationWorkers: 1},
	}
	svc := config.NewService(root, cfg)
	workerDir, err := cloneRepositoryWorkspace(context.Background(), svc, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}

	job := domain.Job{
		ID:         "job-branch",
		Type:       domain.JobTypeIssue,
		Repository: "owner/repo",
		State:      domain.StateDetected,
	}
	if err := syncRepositoryWorkspace(context.Background(), svc, job, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), workerDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "release/1.x" {
		t.Fatalf("expected release/1.x branch, got %q", currentBranch)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(workerDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README.md error = %v", err)
	}
	if string(readmeRaw) != "release\n" {
		t.Fatalf("expected synced release README, got %q", string(readmeRaw))
	}
}

func TestSyncRepositoryWorkspaceUsesDefaultBranchWhenMonitoringBranchIsEmpty(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "release/1.x"); err != nil {
		t.Fatalf("git checkout release error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("release\n"), 0o644); err != nil {
		t.Fatalf("WriteFile release error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add release error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "release"); err != nil {
		t.Fatalf("git commit release error = %v", err)
	}

	cfg := config.DefaultFiles()
	cfg.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repo", Branch: "", ImplementationWorkers: 1},
	}
	svc := config.NewService(root, cfg)
	workerDir, err := cloneRepositoryWorkspace(context.Background(), svc, source, 0)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspace() error = %v", err)
	}
	if err := runGit(t, workerDir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("git symbolic-ref origin/HEAD error = %v", err)
	}

	job := domain.Job{
		ID:         "job-default-branch",
		Type:       domain.JobTypeIssue,
		Repository: "owner/repo",
		State:      domain.StateDetected,
	}
	if err := syncRepositoryWorkspace(context.Background(), svc, job, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), workerDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "main" {
		t.Fatalf("expected main branch, got %q", currentBranch)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(workerDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README.md error = %v", err)
	}
	if string(readmeRaw) != "main\n" {
		t.Fatalf("expected synced main README, got %q", string(readmeRaw))
	}
}

func TestSyncRepositoryWorkspaceUsesIssueBranchNameAndFallsBackToBaseBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}

	cfg := config.DefaultFiles()
	cfg.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repo", Branch: "", ImplementationWorkers: 1},
	}
	svc := config.NewService(root, cfg)
	workerDir, err := cloneRepositoryWorkspaceForBranch(context.Background(), svc, source, "issue_97")
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspaceForBranch() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("updated main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile updated README error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add updated README error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "update"); err != nil {
		t.Fatalf("git commit update error = %v", err)
	}

	job := domain.Job{
		ID:         "job-issue-branch",
		Type:       domain.JobTypeIssue,
		Repository: "owner/repo",
		BranchName: "issue_97",
		State:      domain.StateDetected,
	}
	if err := syncRepositoryWorkspace(context.Background(), svc, job, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), workerDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "issue_97" {
		t.Fatalf("expected issue_97 branch, got %q", currentBranch)
	}

	readmeRaw, err := os.ReadFile(filepath.Join(workerDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile README.md error = %v", err)
	}
	if string(readmeRaw) != "updated main\n" {
		t.Fatalf("expected synced README from base branch, got %q", string(readmeRaw))
	}
}

func TestSyncRepositoryImprovementWorkspaceChecksOutImprovementBranchAndPreservesDraftDir(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README main error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add main error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit main error = %v", err)
	}
	if err := runGit(t, source, "checkout", "-b", "develop"); err != nil {
		t.Fatalf("git checkout develop error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, ".improvement"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.improvement) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, ".improvement", "policy.md"), []byte("---\nid: policy\ntitle: Policy\nscope: repository\nphases:\n  - design\nstatus: active\nupdatedAt: 2026-06-08T00:00:00Z\nsource:\n  repository: owner/repository\n\n---\n\nAlways keep tests green.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(policy.md) error = %v", err)
	}
	if err := runGit(t, source, "add", ".improvement/policy.md"); err != nil {
		t.Fatalf("git add policy error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "develop"); err != nil {
		t.Fatalf("git commit develop error = %v", err)
	}
	if err := runGit(t, source, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	_, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}
	repositoryConfig := config.MonitoredRepository{
		Repository:         source,
		ImprovementEnabled: true,
		ImprovementBranch:  "develop",
		ImprovementDir:     ".improvement",
	}
	improvementDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, source, config.ResolveImprovementBranch(repositoryConfig))
	if _, err := prepareRepositoryImprovementWorkspace(context.Background(), cfg, repositoryConfig); err != nil {
		t.Fatalf("prepareRepositoryImprovementWorkspace() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(improvementDir, ".improvement", "draft"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.improvement/draft) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(improvementDir, ".improvement", "draft", "source-repo_keep-me-safe.md"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(draft) error = %v", err)
	}

	if err := syncRepositoryImprovementWorkspace(context.Background(), cfg, repositoryConfig, improvementDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryImprovementWorkspace() error = %v", err)
	}

	currentBranch, err := runGitCommand(context.Background(), improvementDir, "git", "branch", "--show-current")
	if err != nil {
		t.Fatalf("git branch --show-current error = %v", err)
	}
	if strings.TrimSpace(currentBranch) != "develop" {
		t.Fatalf("expected develop branch, got %q", currentBranch)
	}

	policyRaw, err := os.ReadFile(filepath.Join(improvementDir, ".improvement", "policy.md"))
	if err != nil {
		t.Fatalf("ReadFile(policy.md) error = %v", err)
	}
	if !strings.Contains(string(policyRaw), "Always keep tests green.") {
		t.Fatalf("expected improvement policy from develop branch, got %q", string(policyRaw))
	}

	draftRaw, err := os.ReadFile(filepath.Join(improvementDir, ".improvement", "draft", "source-repo_keep-me-safe.md"))
	if err != nil {
		t.Fatalf("ReadFile(draft) error = %v", err)
	}
	if string(draftRaw) != "keep me\n" {
		t.Fatalf("expected draft dir to be preserved, got %q", string(draftRaw))
	}
}

func TestSyncRepositoryImprovementWorkspaceNoopWhenDisabled(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	_, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}
	improvementDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, source, "")
	if err := os.MkdirAll(improvementDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(improvementDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(improvementDir, "LOCAL.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(LOCAL.txt) error = %v", err)
	}

	repositoryConfig := config.MonitoredRepository{Repository: source, ImprovementEnabled: false}
	if err := syncRepositoryImprovementWorkspace(context.Background(), cfg, repositoryConfig, improvementDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("syncRepositoryImprovementWorkspace() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(improvementDir, "LOCAL.txt")); err != nil {
		t.Fatalf("expected disabled sync to keep local file: %v", err)
	}
}

func TestBuildRepositoryDesignContextLoadsInstructionsFromImprovementWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{
			Repository:            "owner/repository",
			ImplementationWorkers: 1,
			ImprovementEnabled:    true,
			ImprovementDir:        ".improvement",
		},
	}
	svc := config.NewService(root, files)

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, svc.App().ArtifactsDir, "owner/repository", "")
	if err := os.MkdirAll(filepath.Join(workDir, ".improvement"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.improvement) error = %v", err)
	}
	raw := []byte("---\nid: policy\ntitle: Policy\nscope: repository\nphases:\n  - design\nstatus: active\nupdatedAt: 2026-06-08T00:00:00Z\nsource:\n  repository: owner/repository\n\n---\n\nAlways include rollback steps.\n")
	if err := os.WriteFile(filepath.Join(workDir, ".improvement", "policy.md"), raw, 0o644); err != nil {
		t.Fatalf("WriteFile(policy.md) error = %v", err)
	}

	job := domain.Job{
		ID:           "job-42",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		Title:        "Issue",
		WatchRuleID:  "rule-1",
	}
	ctx, err := buildRepositoryDesignContext(svc, workDir, workDir, job, nil)
	if err != nil {
		t.Fatalf("buildRepositoryDesignContext() error = %v", err)
	}
	if len(ctx.ManagedInstructions) != 1 {
		t.Fatalf("managed instruction count = %d, want 1", len(ctx.ManagedInstructions))
	}
	if ctx.ManagedInstructions[0].Title != "Policy" {
		t.Fatalf("managed instruction title = %q, want %q", ctx.ManagedInstructions[0].Title, "Policy")
	}
}

func TestRepositoryWorkerDirUsesOwnerRepoName(t *testing.T) {
	t.Parallel()

	got := artifacts.RepositoryWorkerDir("C:\\repo", "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 2)
	want := filepath.Join("C:\\repo", "artifacts", "coco-papiyon-korobokcle", "workers", "worker-2")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRepositoryWorkerSourceAndLogPaths(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 19, 14, 52, 0, 0, time.Local)
	sourceDir := artifacts.RepositoryWorkerBranchWorkDir("C:\\repo", "artifacts", "https://github.com/coco-papiyon/korobokcle.git", "main")
	wantSourceDir := filepath.Join("C:\\repo", "source", "coco-papiyon-korobokcle-main")
	if sourceDir != wantSourceDir {
		t.Fatalf("expected source dir %q, got %q", wantSourceDir, sourceDir)
	}

	logPath := artifacts.RepositoryWorkerLogPath("C:\\repo", "artifacts", "https://github.com/coco-papiyon/korobokcle.git", 2, startedAt)
	wantLogPath := filepath.Join("C:\\repo", "artifacts", "coco-papiyon-korobokcle", "logs", "2026-05-19", "worker-2-2026-05-19_14-52-00.log")
	if logPath != wantLogPath {
		t.Fatalf("expected log path %q, got %q", wantLogPath, logPath)
	}
}

func TestNewRepositoryWorkerLoggerMirrorsErrorLinesToFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.DefaultFiles())
	var fallback bytes.Buffer
	fallbackLogger := log.New(&fallback, "", 0)
	startedAt := time.Date(2026, 5, 19, 14, 52, 0, 0, time.Local)

	repository := "https://github.com/coco-papiyon/korobokcle.git"
	logger, cleanup, err := newRepositoryWorkerLogger(cfg, fallbackLogger, repository, 2, startedAt)
	if err != nil {
		t.Fatalf("newRepositoryWorkerLogger() error = %v", err)
	}
	defer cleanup()

	logger.Printf("worker only log")

	if fallback.Len() != 0 {
		t.Fatalf("expected no fallback log output, got %q", fallback.String())
	}

	logger.Printf("repository worker cycle failed repository=owner/repo worker=2 kind=review error=boom")

	if !strings.Contains(fallback.String(), "repository worker cycle failed repository=owner/repo worker=2 kind=review error=boom") {
		t.Fatalf("expected fallback logger to receive error line, got %q", fallback.String())
	}

	logPath := artifacts.RepositoryWorkerLogPath(root, cfg.App().ArtifactsDir, repository, 2, startedAt)
	if _, err := os.Stat(filepath.Dir(logPath)); err != nil {
		t.Fatalf("expected log directory to exist: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(worker log) error = %v", err)
	}
	if !bytes.Contains(data, []byte("worker only log")) {
		t.Fatalf("expected worker log file to contain message, got %q", string(data))
	}
	if !bytes.Contains(data, []byte("repository worker cycle failed repository=owner/repo worker=2 kind=review error=boom")) {
		t.Fatalf("expected worker log file to contain error message, got %q", string(data))
	}
}

func TestMarkRepositoryWorkerJobInterruptedUpdatesStateAndEvent(t *testing.T) {
	t.Parallel()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "korobokcle.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, notification.NewNopNotifier())
	job := domain.Job{
		ID:           "job-1",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repo",
		GitHubNumber: 1,
		State:        domain.StateImplementationRunning,
		Title:        "test job",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	cause := errors.New("git worktree add failed: fatal: 'issue_1' is already used")
	if err := markRepositoryWorkerJobInterrupted(context.Background(), orch, job, cause); err != nil {
		t.Fatalf("markRepositoryWorkerJobInterrupted() error = %v", err)
	}

	gotJob, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if gotJob.State != domain.StateInterrupted {
		t.Fatalf("expected job state %q, got %q", domain.StateInterrupted, gotJob.State)
	}

	events, err := store.ListEvents(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected interrupted event")
	}
	gotEvent := events[len(events)-1]
	if gotEvent.EventType != "implementation_interrupted" {
		t.Fatalf("expected event type %q, got %q", "implementation_interrupted", gotEvent.EventType)
	}
	if !strings.Contains(gotEvent.Payload, "already used") {
		t.Fatalf("expected payload to contain error, got %q", gotEvent.Payload)
	}
}

func TestRepositoryWorkerSourceDirUsesConfiguredWorkerDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repository", Branch: "main", WorkDir: "custom/workers", ImplementationWorkers: 2},
	}
	cfg := config.NewService(root, files)

	got0 := repositoryWorkerSourceDir(cfg, "owner/repository", 0)
	want0 := filepath.Join(root, "source", "owner-repository-main")
	if got0 != want0 {
		t.Fatalf("expected worker dir %q, got %q", want0, got0)
	}
	got1 := repositoryWorkerSourceDir(cfg, "owner/repository", 1)
	want1 := filepath.Join(root, "source", "owner-repository-main")
	if got1 != want1 {
		t.Fatalf("expected worker dir %q, got %q", want1, got1)
	}
}

func TestRepositoryMatchesNormalizesRepositoryFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		jobRepo    string
		configRepo string
		want       bool
	}{
		{
			name:       "https and owner repo",
			jobRepo:    "coco-papiyon/korobokcle",
			configRepo: "https://github.com/coco-papiyon/korobokcle",
			want:       true,
		},
		{
			name:       "git and owner repo",
			jobRepo:    "coco-papiyon/korobokcle",
			configRepo: "git@github.com:coco-papiyon/korobokcle.git",
			want:       true,
		},
		{
			name:       "different repository",
			jobRepo:    "coco-papiyon/korobokcle",
			configRepo: "coco-papiyon/another",
			want:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := repositoryMatches(tt.jobRepo, tt.configRepo); got != tt.want {
				t.Fatalf("repositoryMatches(%q, %q) = %v, want %v", tt.jobRepo, tt.configRepo, got, tt.want)
			}
		})
	}
}

func TestJobAssignedToWorkerUsesCanonicalRepositoryID(t *testing.T) {
	t.Parallel()

	job := domain.Job{ID: "issue-owner-repository-1"}
	if got, want := jobAssignedToWorker(job, "https://github.com/owner/repository", 0, 2), jobAssignedToWorker(job, "owner/repository", 0, 2); got != want {
		t.Fatalf("expected canonical repository hashing, got %v and %v", got, want)
	}
}

func TestJobsForRepositoryWorkerKeepsDetectedJobsWhenReservedJobsAreNotRunnable(t *testing.T) {
	t.Parallel()

	jobs := []domain.Job{
		{
			ID:         "issue-owner-repository-2",
			Type:       domain.JobTypeIssue,
			Repository: "owner/repository",
			State:      domain.StateWaitingFinalApproval,
		},
		{
			ID:         "issue-owner-repository-1",
			Type:       domain.JobTypeIssue,
			Repository: "owner/repository",
			State:      domain.StateDetected,
		},
	}

	selected := jobsForRepositoryWorker(jobs, "https://github.com/owner/repository", 0, 1)
	if len(selected) != 2 {
		t.Fatalf("expected both jobs to stay visible, got %d", len(selected))
	}
	if selected[0].ID != "issue-owner-repository-2" {
		t.Fatalf("expected reserved job first, got %q", selected[0].ID)
	}
	if selected[1].ID != "issue-owner-repository-1" {
		t.Fatalf("expected detected job second, got %q", selected[1].ID)
	}
}

func TestJobsForRepositoryWorkerKeepsNonReservedQueueWhenUnlocked(t *testing.T) {
	t.Parallel()

	jobs := []domain.Job{
		{
			ID:         "issue-owner-repository-1",
			Type:       domain.JobTypeIssue,
			Repository: "owner/repository",
			State:      domain.StateDetected,
		},
		{
			ID:         "pull_request-owner-repository-2",
			Type:       domain.JobTypePRReview,
			Repository: "owner/repository",
			State:      domain.StateCollectingContext,
		},
	}

	selected := jobsForRepositoryWorker(jobs, "owner/repository", 0, 1)
	if len(selected) != 2 {
		t.Fatalf("expected unlocked worker to see both jobs, got %d", len(selected))
	}
}

func TestJobAssignedToWorkerDeterministic(t *testing.T) {
	t.Parallel()

	job := domain.Job{ID: "issue-owner-repository-1"}
	first := jobAssignedToWorker(job, "owner/repository", 0, 2)
	second := jobAssignedToWorker(job, "owner/repository", 0, 2)
	if first != second {
		t.Fatalf("expected deterministic worker assignment, got %v and %v", first, second)
	}
	other := jobAssignedToWorker(job, "owner/repository", 1, 2)
	if first == other {
		t.Fatalf("expected job to map to a single worker index, got duplicate assignment")
	}
}

func TestProcessPRJobForPRFeedbackPushesAndCommentsWithoutCreatingPR(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.DefaultFiles())
	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, notification.NewNopNotifier())
	job := domain.Job{
		ID:           "job-pr-feedback",
		Type:         domain.JobTypePRFeedback,
		Repository:   "owner/repository",
		GitHubNumber: 46,
		State:        domain.StatePRCreating,
		Title:        "Address review",
		BranchName:   "feature/review-46",
		WatchRuleID:  "review",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(ctx, job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "main")
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerFix)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	wantBody := "# Fix Summary\n\n- update behavior\n"
	if err := os.WriteFile(filepath.Join(artifactDir, "review_fix.md"), []byte(wantBody), 0o644); err != nil {
		t.Fatalf("WriteFile(review_fix.md) error = %v", err)
	}

	pusher := &recordingBranchPusher{}
	creator := &recordingPRCreator{}
	commenter := &recordingPRCommentSubmitter{}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	if _, err := buildRepositoryPRFeedbackPushRequest(ctx, cfg, job, workDir); err != nil {
		t.Fatalf("buildRepositoryPRFeedbackPushRequest() error = %v", err)
	}
	if err := processPRJob(ctx, cfg, orch, pusher, creator, commenter, MockPRCommentFetcher{}, job, workDir, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("processPRJob() error = %v", err)
	}

	if !pusher.called {
		t.Fatalf("expected branch push to be called")
	}
	if creator.called {
		t.Fatalf("expected PR creator not to be called for pr_feedback")
	}
	if !commenter.called {
		t.Fatalf("expected review comment submitter to be called")
	}
	if commenter.req.Repository != job.Repository {
		t.Fatalf("comment repository = %q, want %q", commenter.req.Repository, job.Repository)
	}
	if commenter.req.PullNumber != job.GitHubNumber {
		t.Fatalf("comment pull number = %d, want %d", commenter.req.PullNumber, job.GitHubNumber)
	}
	if commenter.req.Body != strings.TrimSpace(wantBody) {
		t.Fatalf("comment body = %q, want %q", commenter.req.Body, strings.TrimSpace(wantBody))
	}

	updatedJob, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if updatedJob.State != domain.StateCompleted {
		t.Fatalf("job state = %s, want %s", updatedJob.State, domain.StateCompleted)
	}

	events, err := store.ListEvents(ctx, job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].EventType != "pr_updated" {
		t.Fatalf("event type = %q, want pr_updated", events[0].EventType)
	}
}

func TestProcessPRJobForIssueFetchesPRCommentsAfterCreate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.DefaultFiles())
	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, notification.NewNopNotifier())
	job := domain.Job{
		ID:           "job-pr-create",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StatePRCreating,
		Title:        "Implement feature",
		BranchName:   "feature/pr-42",
		WatchRuleID:  "rule",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(ctx, job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "main")
	summaryDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(summaryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(summaryDir, "result.md"), []byte("summary"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	pusher := &recordingBranchPusher{}
	creator := &recordingPRCreator{}
	commentSubmitter := &recordingPRCommentSubmitter{}
	commentFetcher := &recordingPRCommentFetcher{}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workDir) error = %v", err)
	}
	if err := runGit(t, workDir, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("pr create test"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := runGit(t, workDir, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, workDir, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if _, err := buildRepositoryPRCreateRequest(ctx, cfg, job, workDir); err != nil {
		t.Fatalf("buildRepositoryPRCreateRequest() error = %v", err)
	}
	if err := processPRJob(ctx, cfg, orch, pusher, creator, commentSubmitter, commentFetcher, job, workDir, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("processPRJob() error = %v", err)
	}

	if !commentFetcher.called {
		t.Fatalf("expected PR comment fetcher to be called")
	}
	if commentFetcher.req.PullNumber != 1 {
		t.Fatalf("expected fetcher pull number 1, got %d", commentFetcher.req.PullNumber)
	}

	updatedJob, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if updatedJob.State != domain.StateCompleted {
		t.Fatalf("job state = %s, want %s", updatedJob.State, domain.StateCompleted)
	}

	events, err := store.ListEvents(ctx, job.ID)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].EventType != "pr_created" {
		t.Fatalf("event type = %q, want pr_created", events[0].EventType)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(events[0].Payload), &payload); err != nil {
		t.Fatalf("Unmarshal(payload) error = %v", err)
	}
	if got := payload["pullNumber"]; got != float64(1) {
		t.Fatalf("expected pullNumber 1, got %#v", got)
	}
}

func TestProcessPRJobSkipsExternalOperationsForDummyRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.DefaultFiles())
	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, notification.NewNopNotifier())
	job := domain.Job{
		ID:           "job-pr-create-dummy",
		Type:         domain.JobTypeIssue,
		Repository:   "coco-papiyon/dummy",
		GitHubNumber: 42,
		State:        domain.StatePRCreating,
		Title:        "Implement feature",
		BranchName:   "feature/pr-42",
		WatchRuleID:  "rule",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(ctx, job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "main")
	summaryDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(summaryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(summaryDir, "result.md"), []byte("summary"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerPR)

	pusher := &recordingBranchPusher{}
	creator := &recordingPRCreator{}
	commentSubmitter := &recordingPRCommentSubmitter{}
	commentFetcher := &recordingPRCommentFetcher{}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workDir) error = %v", err)
	}
	if err := runGit(t, workDir, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("dummy pr create test"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := runGit(t, workDir, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, workDir, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if _, err := buildRepositoryPRCreateRequest(ctx, cfg, job, workDir); err != nil {
		t.Fatalf("buildRepositoryPRCreateRequest() error = %v", err)
	}
	if err := processPRJob(ctx, cfg, orch, pusher, creator, commentSubmitter, commentFetcher, job, workDir, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("processPRJob() error = %v", err)
	}

	if pusher.called {
		t.Fatalf("expected branch push to be skipped")
	}
	if creator.called {
		t.Fatalf("expected PR creator to be skipped")
	}
	if commentSubmitter.called {
		t.Fatalf("expected comment submitter to be skipped")
	}
	if commentFetcher.called {
		t.Fatalf("expected comment fetcher to be skipped")
	}

	resultRaw, err := os.ReadFile(filepath.Join(artifactDir, "result.json"))
	if err != nil {
		t.Fatalf("ReadFile(result.json) error = %v", err)
	}
	var resultPayload map[string]any
	if err := json.Unmarshal(resultRaw, &resultPayload); err != nil {
		t.Fatalf("Unmarshal(result.json) error = %v", err)
	}
	if got := resultPayload["pushed"]; got != false {
		t.Fatalf("expected pushed=false, got %#v", got)
	}

	updatedJob, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if updatedJob.State != domain.StateCompleted {
		t.Fatalf("job state = %s, want %s", updatedJob.State, domain.StateCompleted)
	}
}

func TestProcessPRJobForIssueBootstrapsPRReviewJobOnlyForMockProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "mock"
	files.WatchRules.Rules = []config.WatchRule{{
		ID:           "pr-review",
		Name:         "PR Review",
		Repositories: []string{"owner/repository"},
		Target:       string(domain.TargetPullRequest),
		Enabled:      true,
	}}
	cfg := config.NewService(root, files)
	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, notification.NewNopNotifier())
	job := domain.Job{
		ID:           "job-pr-create",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StatePRCreating,
		Title:        "Implement feature",
		BranchName:   "feature/pr-42",
		WatchRuleID:  "rule",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(ctx, job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "main")
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "result.md"), []byte("summary"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	pusher := &recordingBranchPusher{}
	creator := &recordingPRCreator{}
	commentSubmitter := &recordingPRCommentSubmitter{}
	commentFetcher := &recordingPRCommentFetcher{}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workDir) error = %v", err)
	}
	if err := runGit(t, workDir, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("pr create test"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := runGit(t, workDir, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, workDir, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if _, err := buildRepositoryPRCreateRequest(ctx, cfg, job, workDir); err != nil {
		t.Fatalf("buildRepositoryPRCreateRequest() error = %v", err)
	}
	if err := processPRJob(ctx, cfg, orch, pusher, creator, commentSubmitter, commentFetcher, job, workDir, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("processPRJob() error = %v", err)
	}

	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if got := countJobsOfType(jobs, domain.JobTypePRReview); got != 1 {
		t.Fatalf("review job count = %d, want 1", got)
	}
}

func TestProcessPRJobForIssueDoesNotBootstrapPRReviewJobForNonMockProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.Provider = "codex"
	files.WatchRules.Rules = []config.WatchRule{{
		ID:           "pr-review",
		Name:         "PR Review",
		Repositories: []string{"owner/repository"},
		Target:       string(domain.TargetPullRequest),
		Enabled:      true,
	}}
	cfg := config.NewService(root, files)
	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	orch := orchestrator.New(store, notification.NewNopNotifier())
	job := domain.Job{
		ID:           "job-pr-create",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StatePRCreating,
		Title:        "Implement feature",
		BranchName:   "feature/pr-42",
		WatchRuleID:  "rule",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(ctx, job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "main")
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber, artifacts.WorkerImplementation)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "result.md"), []byte("summary"), 0o644); err != nil {
		t.Fatalf("WriteFile(result.md) error = %v", err)
	}

	pusher := &recordingBranchPusher{}
	creator := &recordingPRCreator{}
	commentSubmitter := &recordingPRCommentSubmitter{}
	commentFetcher := &recordingPRCommentFetcher{}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workDir) error = %v", err)
	}
	if err := runGit(t, workDir, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("pr create test"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	if err := runGit(t, workDir, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, workDir, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if err := processPRJob(ctx, cfg, orch, pusher, creator, commentSubmitter, commentFetcher, job, workDir, workerDir, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("processPRJob() error = %v", err)
	}

	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if got := countJobsOfType(jobs, domain.JobTypePRReview); got != 0 {
		t.Fatalf("review job count = %d, want 0", got)
	}
}

func TestRepositoryCloneSourceUsesFixtureLocalSourceWhenAvailable(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source", "owner-repository")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sourceDir) error = %v", err)
	}
	t.Setenv("KOROBOKCLE_TOOL_ROOT", root)

	got := repositoryCloneSource("owner/repository")
	if !filepath.IsAbs(got) {
		t.Fatalf("repositoryCloneSource() = %q, want absolute path", got)
	}
	if got != sourceDir {
		t.Fatalf("repositoryCloneSource() = %q, want %q", got, sourceDir)
	}
}

func TestRepositoryWorkersFromJobsSkipsEmptyRepositories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	jobs := []domain.Job{
		{ID: "job-1", Type: domain.JobTypeIssue, Repository: "owner/repository", GitHubNumber: 1, State: domain.StateDetected, CreatedAt: now, UpdatedAt: now},
		{ID: "job-2", Type: domain.JobTypeIssue, Repository: "", GitHubNumber: 2, State: domain.StateDetected, CreatedAt: now, UpdatedAt: now},
	}
	for _, job := range jobs {
		if err := store.UpsertJob(context.Background(), job); err != nil {
			t.Fatalf("UpsertJob() error = %v", err)
		}
	}

	repositories := repositoryWorkersFromJobs(context.Background(), orchestrator.New(store, notification.NewNopNotifier()))
	if len(repositories) != 1 {
		t.Fatalf("repository count = %d, want 1", len(repositories))
	}
	if repositories[0].Repository != "owner/repository" {
		t.Fatalf("repository = %q, want owner/repository", repositories[0].Repository)
	}
	if repositories[0].ImplementationWorkers != 1 {
		t.Fatalf("workers = %d, want 1", repositories[0].ImplementationWorkers)
	}
}

type recordingBranchPusher struct {
	called bool
	req    PRCreateRequest
}

func (r *recordingBranchPusher) Push(_ context.Context, req PRCreateRequest) error {
	r.called = true
	r.req = req
	return nil
}

type recordingPRCreator struct {
	called bool
}

func (r *recordingPRCreator) Create(_ context.Context, _ PRCreateRequest) (PRCreateResult, error) {
	r.called = true
	return PRCreateResult{URL: "https://example.invalid/pull/1", PullNumber: 1}, nil
}

type recordingPRCommentSubmitter struct {
	called bool
	req    PRCommentSubmitRequest
}

func (r *recordingPRCommentSubmitter) Submit(_ context.Context, req PRCommentSubmitRequest) error {
	r.called = true
	r.req = req
	return nil
}

type recordingPRCommentFetcher struct {
	called bool
	req    PRCommentFetchRequest
}

func (r *recordingPRCommentFetcher) Fetch(_ context.Context, req PRCommentFetchRequest) (PRCommentsArtifact, error) {
	r.called = true
	r.req = req
	return PRCommentsArtifact{PullNumber: req.PullNumber, Comments: []PRComment{{Author: "alice", Body: "Looks good", URL: "https://example.invalid/comment/1"}}}, nil
}

func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
	return nil
}

func countJobsOfType(jobs []domain.Job, jobType domain.JobType) int {
	count := 0
	for _, job := range jobs {
		if job.Type == jobType {
			count++
		}
	}
	return count
}

func TestWithRepositoryWorkspaceLockSerializesSameRepository(t *testing.T) {
	t.Parallel()

	repository := "owner/repo"
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- withRepositoryWorkspaceLock(repository, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first lock holder did not start")
	}

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- withRepositoryWorkspaceLock(repository, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second lock holder entered before the first released the lock")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first lock holder returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first lock holder did not finish")
	}

	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second lock holder did not start after release")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second lock holder returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second lock holder did not finish")
	}
}

func TestRepositoryWorkerWorktreeExistsDetectsRegisteredWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := runGit(t, source, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := runGit(t, source, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, source, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "main"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}

	cfg := config.NewService(root, config.DefaultFiles())
	baseDir, err := prepareRepositoryWorkspace(context.Background(), cfg, source, "")
	if err != nil {
		t.Fatalf("prepareRepositoryWorkspace() error = %v", err)
	}

	worktreeDir, err := cloneRepositoryWorkspaceForBranch(context.Background(), cfg, source, "main", baseDir)
	if err != nil {
		t.Fatalf("cloneRepositoryWorkspaceForBranch() error = %v", err)
	}

	exists, err := repositoryWorkerWorktreeExists(context.Background(), baseDir, worktreeDir)
	if err != nil {
		t.Fatalf("repositoryWorkerWorktreeExists() error = %v", err)
	}
	if !exists {
		t.Fatal("expected worktree to be reported as existing")
	}
}
