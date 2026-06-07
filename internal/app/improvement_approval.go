package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"gopkg.in/yaml.v3"
)

const defaultImprovementBranch = "develop"

func publishApprovedImprovement(ctx context.Context, cfg *config.Service, repository string, issueNumber int, title string, draft string, relatedJobID string, logger *log.Logger) error {
	repoConfig, ok := findRepositoryConfig(cfg.App(), repository)
	if !ok || !repoConfig.ImprovementEnabled {
		return fmt.Errorf("improvement is disabled for repository %q", repository)
	}

	workDir, err := prepareRepositoryWorkspace(ctx, cfg, repository, repoConfig.WorkDir)
	if err != nil {
		return err
	}

	artifactDir := filepath.Join(artifacts.RepositoryWorkerJobDir(cfg.Root(), cfg.App().ArtifactsDir, repository, issueNumber), "improvement")
	branch := strings.TrimSpace(repoConfig.ImprovementBranch)
	if branch == "" {
		branch = defaultImprovementBranch
	}

	if logger != nil {
		logger.Printf("publishing approved improvement repository=%s issue=%d branch=%s work_dir=%s", repository, issueNumber, branch, workDir)
	}

	if err := checkoutImprovementBranch(ctx, workDir, artifactDir, branch, strings.TrimSpace(repoConfig.Branch)); err != nil {
		return err
	}

	approvedDir := artifacts.RepositoryWorkerImprovementApprovedDir(workDir, repoConfig.ImprovementDir)
	if err := os.MkdirAll(approvedDir, 0o755); err != nil {
		return err
	}
	fileName := artifacts.RepositoryWorkerWorkArtifactFileName(issueNumber, title)
	approvedPath := filepath.Join(approvedDir, fileName)
	improvementCtx := loadImprovementArtifactContext(filepath.Join(artifactDir, "context.json"))
	document, err := buildApprovedImprovementDocument(repository, issueNumber, title, draft, relatedJobID, improvementCtx, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := os.WriteFile(approvedPath, []byte(document), 0o644); err != nil {
		return err
	}

	relPath, err := filepath.Rel(workDir, approvedPath)
	if err != nil {
		return err
	}
	relPath = filepath.ToSlash(relPath)
	if err := writeCommandLog(artifactDir, "git-write.log", relPath); err != nil {
		return err
	}

	if err := gitAddPath(ctx, workDir, artifactDir, relPath); err != nil {
		return err
	}
	changed, err := gitPathHasChanges(ctx, workDir, relPath)
	if err != nil {
		return err
	}
	if changed {
		commitTitle := strings.TrimSpace(title)
		if commitTitle == "" {
			commitTitle = fmt.Sprintf("issue #%d", issueNumber)
		}
		if err := gitCommitImprovement(ctx, workDir, artifactDir, issueNumber, commitTitle); err != nil {
			return err
		}
	} else if err := writeCommandLog(artifactDir, "git-commit.log", "no changes to commit"); err != nil {
		return err
	}

	return gitPushBranch(ctx, workDir, artifactDir, branch)
}

func checkoutImprovementBranch(ctx context.Context, workDir string, artifactDir string, branch string, configuredBaseBranch string) error {
	if output, err := runGitCommand(ctx, workDir, "git", "fetch", "--prune", "origin"); err != nil {
		return err
	} else if err := writeCommandLog(artifactDir, "git-fetch.log", output); err != nil {
		return err
	}

	remoteBranch := "origin/" + branch
	if _, err := runGitCommand(ctx, workDir, "git", "show-ref", "--verify", "--quiet", "refs/remotes/"+remoteBranch); err == nil {
		output, err := runGitCommand(ctx, workDir, "git", "checkout", "-f", "-B", branch, remoteBranch)
		if err != nil {
			return err
		}
		if err := writeCommandLog(artifactDir, "git-checkout.log", output); err != nil {
			return err
		}
		output, err = runGitCommand(ctx, workDir, "git", "reset", "--hard", remoteBranch)
		if err != nil {
			return err
		}
		return writeCommandLog(artifactDir, "git-reset.log", output)
	}

	baseBranch, err := resolveImprovementBaseBranch(ctx, workDir, configuredBaseBranch)
	if err != nil {
		return err
	}
	baseRemote := "origin/" + baseBranch
	output, err := runGitCommand(ctx, workDir, "git", "checkout", "-f", "-B", baseBranch, baseRemote)
	if err != nil {
		return err
	}
	if err := writeCommandLog(artifactDir, "git-checkout.log", output); err != nil {
		return err
	}
	output, err = runGitCommand(ctx, workDir, "git", "reset", "--hard", baseRemote)
	if err != nil {
		return err
	}
	if err := writeCommandLog(artifactDir, "git-reset.log", output); err != nil {
		return err
	}
	output, err = runGitCommand(ctx, workDir, "git", "checkout", "-B", branch)
	if err != nil {
		return err
	}
	return writeCommandLog(artifactDir, "git-branch.log", output)
}

func resolveImprovementBaseBranch(ctx context.Context, workDir string, configuredBaseBranch string) (string, error) {
	candidates := make([]string, 0, 4)
	if trimmed := strings.TrimSpace(configuredBaseBranch); trimmed != "" {
		candidates = append(candidates, trimmed)
	}
	if resolved, err := resolveRepositoryBaseBranch(ctx, workDir, configuredBaseBranch); err == nil {
		candidates = append(candidates, strings.TrimSpace(resolved))
	}
	candidates = append(candidates, "main", "master")
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := runGitCommand(ctx, workDir, "git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+candidate); err == nil {
			return candidate, nil
		}
	}

	output, err := runGitCommand(ctx, workDir, "git", "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "origin/") || line == "origin/HEAD" {
			continue
		}
		return strings.TrimPrefix(line, "origin/"), nil
	}
	return "", fmt.Errorf("resolve improvement base branch: no remote branch found")
}

func gitAddPath(ctx context.Context, workDir string, artifactDir string, relPath string) error {
	output, err := runGitCommand(ctx, workDir, "git", "add", relPath)
	if err != nil {
		return err
	}
	return writeCommandLog(artifactDir, "git-add.log", output)
}

func gitPathHasChanges(ctx context.Context, workDir string, relPath string) (bool, error) {
	output, err := runGitCommand(ctx, workDir, "git", "status", "--porcelain", "--", relPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func gitCommitImprovement(ctx context.Context, workDir string, artifactDir string, issueNumber int, title string) error {
	output, err := runGitCommand(ctx, workDir, "git", "commit", "-m", fmt.Sprintf("improvement: approve issue #%d %s", issueNumber, title))
	if err != nil {
		return err
	}
	return writeCommandLog(artifactDir, "git-commit.log", output)
}

func gitPushBranch(ctx context.Context, workDir string, artifactDir string, branch string) error {
	output, err := runGitCommand(ctx, workDir, "git", "push", "-u", "origin", branch)
	if err != nil {
		return err
	}
	return writeCommandLog(artifactDir, "git-push.log", output)
}

type approvedImprovementFrontMatter struct {
	ID        string                        `yaml:"id"`
	Title     string                        `yaml:"title"`
	Scope     string                        `yaml:"scope"`
	Phases    []string                      `yaml:"phases"`
	Status    string                        `yaml:"status"`
	UpdatedAt string                        `yaml:"updatedAt"`
	Source    approvedImprovementSourceInfo `yaml:"source"`
}

type approvedImprovementSourceInfo struct {
	JobID       string `yaml:"jobId,omitempty"`
	IssueNumber int    `yaml:"issueNumber"`
	Repository  string `yaml:"repository"`
	Event       string `yaml:"event"`
}

type improvementArtifactContext struct {
	JobType string `json:"jobType"`
	Comment string `json:"comment"`
}

func buildApprovedImprovementDocument(repository string, issueNumber int, title string, draft string, relatedJobID string, improvementCtx improvementArtifactContext, updatedAt time.Time) (string, error) {
	body := strings.TrimSpace(stripMarkdownFrontMatter(draft))
	if body == "" {
		body = strings.TrimSpace(draft)
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		trimmedTitle = fmt.Sprintf("改善案 #%d", issueNumber)
	}
	phases := deriveImprovementPhases(body, improvementCtx)
	frontMatter := approvedImprovementFrontMatter{
		ID:        fmt.Sprintf("issue-%d-%s", issueNumber, sanitizeImprovementID(title)),
		Title:     trimmedTitle,
		Scope:     "repository",
		Phases:    phases,
		Status:    "active",
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		Source: approvedImprovementSourceInfo{
			JobID:       strings.TrimSpace(relatedJobID),
			IssueNumber: issueNumber,
			Repository:  repository,
			Event:       "improvement_approved",
		},
	}
	raw, err := yaml.Marshal(frontMatter)
	if err != nil {
		return "", err
	}
	return "---\n" + string(raw) + "---\n\n" + body + "\n", nil
}

func loadImprovementArtifactContext(path string) improvementArtifactContext {
	var out improvementArtifactContext
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func deriveImprovementPhases(body string, improvementCtx improvementArtifactContext) []string {
	found := make(map[string]struct{})
	collectImprovementPhases(found, body)
	if len(found) == 0 {
		collectImprovementPhases(found, improvementCtx.Comment)
	}
	if len(found) == 0 {
		switch strings.TrimSpace(improvementCtx.JobType) {
		case "pr_review":
			found["review"] = struct{}{}
		case "pr_feedback":
			found["implementation"] = struct{}{}
			found["fix"] = struct{}{}
		default:
			found["design"] = struct{}{}
			found["implementation"] = struct{}{}
		}
	}
	if len(found) == 1 {
		if _, ok := found["design"]; ok {
			found["implementation"] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(found))
	for _, phase := range []string{"design", "implementation", "fix", "review"} {
		if _, ok := found[phase]; ok {
			ordered = append(ordered, phase)
		}
	}
	if len(ordered) == 0 {
		ordered = append(ordered, "design", "implementation")
	}
	sort.Strings(ordered)
	sort.SliceStable(ordered, func(i, j int) bool {
		order := map[string]int{"design": 0, "implementation": 1, "fix": 2, "review": 3}
		return order[ordered[i]] < order[ordered[j]]
	})
	return ordered
}

func collectImprovementPhases(found map[string]struct{}, text string) {
	lower := strings.ToLower(text)
	mappings := map[string]string{
		"design":         "design",
		"設計":             "design",
		"implementation": "implementation",
		"実装":             "implementation",
		"fix":            "fix",
		"修正":             "fix",
		"レビュー修正":         "fix",
		"review":         "review",
		"レビュー":           "review",
	}
	for needle, phase := range mappings {
		if strings.Contains(lower, strings.ToLower(needle)) {
			found[phase] = struct{}{}
		}
	}
	if strings.Contains(lower, "ui") || strings.Contains(lower, "レイアウト") || strings.Contains(lower, "画面") || strings.Contains(lower, "文言") {
		found["design"] = struct{}{}
		found["implementation"] = struct{}{}
	}
	if strings.Contains(lower, "prコメント") || strings.Contains(lower, "review comment") || strings.Contains(lower, "helper") {
		found["fix"] = struct{}{}
		found["implementation"] = struct{}{}
	}
}

func stripMarkdownFrontMatter(text string) string {
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	rest := strings.TrimPrefix(text, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return text
	}
	return rest[idx+5:]
}

func sanitizeImprovementID(title string) string {
	trimmed := strings.TrimSpace(strings.ToLower(title))
	if trimmed == "" {
		return "policy"
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "@", "-", "?", "-", "#", "-", ".", "-", ",", "-")
	trimmed = replacer.Replace(trimmed)
	parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return "policy"
	}
	joined := strings.Join(parts, "-")
	if len(joined) > 48 {
		return joined[:48]
	}
	return joined
}
