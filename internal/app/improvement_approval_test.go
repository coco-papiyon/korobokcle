package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func TestApplyImprovementApprovalPreparesBranchAndPushesLater(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "default", "improvement_implementation")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: improvement_implementation\ntitle: Improvement Implementation\nrole: test role\npromptTemplates:\n  - prompt.md.tmpl\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte("{{ .TargetPath }}"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt.md.tmpl) error = %v", err)
	}
	remote := filepath.Join(root, "remote.git")
	if err := runGit(t, root, "init", "--bare", remote); err != nil {
		t.Fatalf("git init bare error = %v", err)
	}

	worktree := filepath.Join(root, "seed")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("MkdirAll(seed) error = %v", err)
	}
	if err := runGit(t, worktree, "init", "--initial-branch=main"); err != nil {
		t.Fatalf("git init error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("seed"), 0o644); err != nil {
		t.Fatalf("WriteFile(README) error = %v", err)
	}
	if err := runGit(t, worktree, "add", "README.md"); err != nil {
		t.Fatalf("git add error = %v", err)
	}
	if err := runGit(t, worktree, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "seed"); err != nil {
		t.Fatalf("git commit error = %v", err)
	}
	if err := runGit(t, worktree, "remote", "add", "origin", remote); err != nil {
		t.Fatalf("git remote add error = %v", err)
	}
	if err := runGit(t, worktree, "push", "-u", "origin", "main"); err != nil {
		t.Fatalf("git push main error = %v", err)
	}

	store, err := sqlite.Open(filepath.Join(root, "data", "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	orch := orchestrator.New(store, nil)

	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:            "owner/repository",
				WorkDir:               worktree,
				ImplementationWorkers: 1,
				ImprovementEnabled:    true,
				ImprovementBranch:     "develop",
				ImprovementDir:        ".improvement",
			}},
		},
	})

	job := domain.Job{
		ID:           "job-approve",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 42,
		State:        domain.StateWaitingDesignApproval,
		Title:        "改善承認",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, job.Repository, "develop")
	if err := runGit(t, root, "clone", remote, workDir); err != nil {
		t.Fatalf("git clone improvement workspace error = %v", err)
	}
	workFiles := repositoryImprovementWorkFiles(workDir, ".improvement", job.ID, job.Title)
	artifactFiles := repositoryImprovementArtifactFiles(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber)

	contextData := improvementContextData{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
		Source: improvementSourceInput{
			EventType: improvementSourceDesignRejected,
			Comment:   "Please keep this generic.",
		},
		Phases: []string{"design"},
	}
	contextRaw, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(context) error = %v", err)
	}
	if err := writeImprovementFile(artifactFiles.ContextPath, contextRaw); err != nil {
		t.Fatalf("write context error = %v", err)
	}
	draft := "# 改善方針案\n\n## タイトル\n\n設計書の API 境界方針\n\n## 汎化した方針案\n\n- API 境界を先に明示する。\n"
	if err := writeImprovementFile(workFiles.DraftPath, []byte(draft)); err != nil {
		t.Fatalf("write draft error = %v", err)
	}

	if err := applyImprovementApproval(context.Background(), cfg, orch, job.ID, improvementApprovalRequest{
		Status: improvementApprovalApproved,
	}, nil); err != nil {
		t.Fatalf("applyImprovementApproval() error = %v", err)
	}

	if _, err := os.Stat(artifactFiles.ApprovalPath); err != nil {
		t.Fatalf("expected approval.json: %v", err)
	}
	if _, err := os.Stat(artifactFiles.ResultPath); err != nil {
		t.Fatalf("expected result.md: %v", err)
	}
	if _, err := os.Stat(artifactFiles.ImplementationPromptPath); err != nil {
		t.Fatalf("expected implementation-prompt.md: %v", err)
	}
	phaseRaw, err := os.ReadFile(filepath.Join(workDir, ".improvement", "design.md"))
	if err != nil {
		t.Fatalf("ReadFile(design.md) error = %v", err)
	}
	if !strings.Contains(string(phaseRaw), "# 改善実装結果") {
		t.Fatalf("expected implementation summary, got %s", string(phaseRaw))
	}
	if !strings.Contains(string(phaseRaw), "src/example.go") {
		t.Fatalf("expected modified source summary, got %s", string(phaseRaw))
	}
	if _, err := os.Stat(filepath.Join(workDir, ".improvement", "設計書の-api-境界方針.md")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy improvement markdown to be removed, got err=%v", err)
	}
	if out, err := exec.Command("git", "ls-remote", "--exit-code", "--heads", remote, "develop").CombinedOutput(); err == nil {
		t.Fatalf("expected remote branch to be absent before push, output=%s", string(out))
	}

	editedPhase := strings.TrimSpace(string(phaseRaw)) + "\n\n<!-- reviewed before push -->\n"
	if err := os.WriteFile(filepath.Join(workDir, ".improvement", "design.md"), []byte(editedPhase), 0o644); err != nil {
		t.Fatalf("WriteFile(edited design.md) error = %v", err)
	}

	if err := pushImprovementBranch(context.Background(), workDir, "develop", artifactFiles.Dir); err != nil {
		t.Fatalf("pushImprovementBranch() error = %v", err)
	}

	promptRaw, err := os.ReadFile(artifactFiles.ImplementationPromptPath)
	if err != nil {
		t.Fatalf("ReadFile(implementation-prompt.md) error = %v", err)
	}
	if !strings.Contains(string(promptRaw), ".improvement/design.md") {
		t.Fatalf("expected implementation prompt to mention target path, got %s", string(promptRaw))
	}

	verifyDir := filepath.Join(root, "verify")
	if err := runGit(t, root, "clone", remote, verifyDir); err != nil {
		t.Fatalf("git clone verify error = %v", err)
	}
	if err := runGit(t, verifyDir, "checkout", "develop"); err != nil {
		t.Fatalf("git checkout develop error = %v", err)
	}
	verifiedPhaseRaw, err := os.ReadFile(filepath.Join(verifyDir, ".improvement", "design.md"))
	if err != nil {
		t.Fatalf("ReadFile(verified design.md) error = %v", err)
	}
	if !strings.Contains(string(verifiedPhaseRaw), "# 改善実装結果") {
		t.Fatalf("expected pushed implementation summary, got %s", string(verifiedPhaseRaw))
	}
	if !strings.Contains(string(verifiedPhaseRaw), "reviewed before push") {
		t.Fatalf("expected pushed edited content, got %s", string(verifiedPhaseRaw))
	}
	if _, err := os.Stat(filepath.Join(verifyDir, ".improvement", "設計書の-api-境界方針.md")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy improvement markdown to be removed from branch, got err=%v", err)
	}
}

func TestApplyImprovementApprovalRejectsWithoutGitUpdate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "data", "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	orch := orchestrator.New(store, nil)

	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository:            "owner/repository",
				ImplementationWorkers: 1,
				ImprovementEnabled:    true,
			}},
		},
	})

	job := domain.Job{
		ID:           "job-reject",
		Type:         domain.JobTypeIssue,
		Repository:   "owner/repository",
		GitHubNumber: 43,
		State:        domain.StateWaitingDesignApproval,
		Title:        "改善却下",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertJob(context.Background(), job); err != nil {
		t.Fatalf("UpsertJob() error = %v", err)
	}

	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(root, cfg.App().ArtifactsDir, job.Repository, "")
	workFiles := repositoryImprovementWorkFiles(workDir, "", job.ID, job.Title)
	contextRaw := []byte(`{"jobId":"job-reject","repository":"owner/repository","issueNumber":43,"title":"改善却下","source":{"eventType":"final_rejected","comment":"not now"},"phases":["implementation"]}`)
	artifactFiles := repositoryImprovementArtifactFiles(root, cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber)
	if err := writeImprovementFile(artifactFiles.ContextPath, contextRaw); err != nil {
		t.Fatalf("write context error = %v", err)
	}
	if err := writeImprovementFile(workFiles.DraftPath, []byte("draft body")); err != nil {
		t.Fatalf("write draft error = %v", err)
	}

	if err := applyImprovementApproval(context.Background(), cfg, orch, job.ID, improvementApprovalRequest{
		Status:  improvementApprovalRejected,
		Comment: "keep this one-off",
	}, nil); err != nil {
		t.Fatalf("applyImprovementApproval() error = %v", err)
	}

	approvalRaw, err := os.ReadFile(artifactFiles.ApprovalPath)
	if err != nil {
		t.Fatalf("ReadFile(approval.json) error = %v", err)
	}
	if !strings.Contains(string(approvalRaw), improvementApprovalRejected) {
		t.Fatalf("expected rejected approval, got %s", string(approvalRaw))
	}
	rejectedPhasePath := artifacts.RepositoryWorkerImprovementPhaseFile(workDir, cfg.App().MonitoredRepositories[0].ImprovementDir, "implementation")
	if _, err := os.Stat(rejectedPhasePath); !os.IsNotExist(err) {
		t.Fatalf("expected no phase file update on rejection, err=%v", err)
	}
}
