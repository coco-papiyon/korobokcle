package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
)

type PRCreator interface {
	Create(ctx context.Context, req PRCreateRequest) (string, error)
}

type BranchPusher interface {
	Push(ctx context.Context, req PRCreateRequest) error
}

type PRCreateRequest struct {
	Repository  string
	BranchName  string
	Title       string
	Body        string
	ArtifactDir string
	WorkDir     string
}

type MockBranchPusher struct{}

func (p *MockBranchPusher) Push(_ context.Context, _ PRCreateRequest) error {
	return nil
}

type MockPRCreator struct{}

func (c *MockPRCreator) Create(_ context.Context, req PRCreateRequest) (string, error) {
	return fmt.Sprintf("https://github.com/%s/pull/%s", req.Repository, strings.ReplaceAll(req.BranchName, "/", "-")), nil
}

type GHPRCreator struct{}

type GitBranchPusher struct {
	Remote string
}

func (p *GitBranchPusher) Push(ctx context.Context, req PRCreateRequest) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git command is not available: %w", err)
	}

	remote := p.Remote
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}

	if err := preparePRBranch(ctx, req); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "push", remote, fmt.Sprintf("HEAD:refs/heads/%s", req.BranchName))
	cmd.Dir = req.WorkDir

	raw, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(raw))
	if err != nil {
		return fmt.Errorf("git push failed: %w: %s", err, output)
	}
	return writeCommandLog(req.ArtifactDir, "git-push.log", output)
}

func preparePRBranch(ctx context.Context, req PRCreateRequest) error {
	branchCmd := exec.CommandContext(ctx, "git", "checkout", "-B", req.BranchName)
	branchCmd.Dir = req.WorkDir
	branchOut, err := branchCmd.CombinedOutput()
	branchOutput := strings.TrimSpace(string(branchOut))
	if err != nil {
		return fmt.Errorf("git checkout failed: %w: %s", err, branchOutput)
	}
	if err := writeCommandLog(req.ArtifactDir, "git-checkout.log", branchOutput); err != nil {
		return err
	}

	addCmd := exec.CommandContext(ctx, "git", "add", "-A")
	addCmd.Dir = req.WorkDir
	addOut, err := addCmd.CombinedOutput()
	addOutput := strings.TrimSpace(string(addOut))
	if err != nil {
		return fmt.Errorf("git add failed: %w: %s", err, addOutput)
	}
	if err := writeCommandLog(req.ArtifactDir, "git-add.log", addOutput); err != nil {
		return err
	}

	commitCmd := exec.CommandContext(ctx, "git", "commit", "--allow-empty", "-m", req.Title)
	commitCmd.Dir = req.WorkDir
	commitOut, err := commitCmd.CombinedOutput()
	commitOutput := strings.TrimSpace(string(commitOut))
	if err != nil {
		return fmt.Errorf("git commit failed: %w: %s", err, commitOutput)
	}
	return writeCommandLog(req.ArtifactDir, "git-commit.log", commitOutput)
}

func (c *GHPRCreator) Create(ctx context.Context, req PRCreateRequest) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh command is not available: %w", err)
	}

	if err := os.MkdirAll(req.ArtifactDir, 0o755); err != nil {
		return "", err
	}

	bodyPath := filepath.Join(req.ArtifactDir, "pr-body.md")
	if err := os.WriteFile(bodyPath, []byte(req.Body), 0o644); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--repo", req.Repository,
		"--head", req.BranchName,
		"--title", req.Title,
		"--body-file", bodyPath,
	)
	cmd.Dir = req.WorkDir

	raw, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(raw))
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w: %s", err, output)
	}
	if output == "" {
		return "", fmt.Errorf("gh pr create returned empty output")
	}
	if err := writeCommandLog(req.ArtifactDir, "gh-pr-create.log", output); err != nil {
		return "", err
	}
	return output, nil
}

func startPRWorker(ctx context.Context, root string, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger) error {
	pusher, creator := newPRPublisher(cfg.App().Provider)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			if err := runPendingPRCreations(ctx, cfg, orch, pusher, creator, root, logger); err != nil && ctx.Err() == nil {
				logger.Printf("pr worker error: %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

func newPRPublisher(provider string) (BranchPusher, PRCreator) {
	if strings.EqualFold(strings.TrimSpace(provider), "mock") {
		return &MockBranchPusher{}, &MockPRCreator{}
	}
	return &GitBranchPusher{Remote: "origin"}, &GHPRCreator{}
}

func runPendingPRCreations(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, pusher BranchPusher, creator PRCreator, root string, logger *log.Logger) error {
	jobs, err := orch.ListJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if job.Type != domain.JobTypeIssue || job.State != domain.StatePRCreating {
			continue
		}

		req, err := buildPRCreateRequest(cfg, job)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
			continue
		}
		req.WorkDir = root

		if err := pusher.Push(ctx, req); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_push_failed", map[string]any{"error": err.Error()})
			continue
		}

		url, err := creator.Create(ctx, req)
		if err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := writePRCreateArtifact(req.ArtifactDir, url, req); err != nil {
			_ = orch.UpdateJobState(ctx, job.ID, domain.StateFailed, "pr_create_failed", map[string]any{"error": err.Error()})
			continue
		}

		if err := orch.UpdateJobState(ctx, job.ID, domain.StateCompleted, "pr_created", map[string]any{
			"url":   url,
			"title": req.Title,
			"head":  req.BranchName,
		}); err != nil {
			logger.Printf("pr_created state transition failed for %s: %v", job.ID, err)
			continue
		}
	}

	return nil
}

func buildPRCreateRequest(cfg *config.Service, job domain.Job) (PRCreateRequest, error) {
	artifactDir := filepath.Join(cfg.Root(), cfg.App().ArtifactsDir, "changes", job.ID)
	summaryPath := filepath.Join(artifactDir, "summary.md")
	summaryRaw, err := os.ReadFile(summaryPath)
	if err != nil {
		return PRCreateRequest{}, err
	}

	title := fmt.Sprintf("%s (#%d)", job.Title, job.GitHubNumber)
	body := buildPRBody(job, string(summaryRaw))

	return PRCreateRequest{
		Repository:  job.Repository,
		BranchName:  job.BranchName,
		Title:       title,
		Body:        body,
		ArtifactDir: artifactDir,
	}, nil
}

func buildPRBody(job domain.Job, summary string) string {
	return fmt.Sprintf("## Summary\n\n%s\n\n## Source\n\n- Repository: `%s`\n- Issue: #%d\n- Job: `%s`\n", strings.TrimSpace(summary), job.Repository, job.GitHubNumber, job.ID)
}

func writePRCreateArtifact(artifactDir string, url string, req PRCreateRequest) error {
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(map[string]any{
		"url":        url,
		"repository": req.Repository,
		"branchName": req.BranchName,
		"title":      req.Title,
		"pushed":     true,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(artifactDir, "pr-create.json"), raw, 0o644)
}

func writeCommandLog(artifactDir string, name string, content string) error {
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(artifactDir, name), []byte(content), 0o644)
}
