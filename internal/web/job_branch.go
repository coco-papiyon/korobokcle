package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobBranchResolver interface {
	Resolve(context.Context, domain.Job, domain.WatchSettings) (string, error)
}

type defaultJobBranchResolver struct{}

func NewDefaultJobBranchResolver() JobBranchResolver {
	return defaultJobBranchResolver{}
}

func (defaultJobBranchResolver) Resolve(ctx context.Context, job domain.Job, settings domain.WatchSettings) (string, error) {
	switch job.Kind {
	case domain.JobKindIssueDesign, domain.JobKindIssueImplementation:
		return renderBranchName(settings.BranchNamePattern, job.Number), nil
	case domain.JobKindPRReview, domain.JobKindPRFeedback:
		return loadPRBranch(ctx, job)
	default:
		return "", nil
	}
}

func loadPRBranch(ctx context.Context, job domain.Job) (string, error) {
	raw, err := runGHJSON(ctx, "pr", "view", "--repo", job.Repository, strconv.Itoa(job.Number), "--json", "headRefName")
	if err != nil {
		return "", err
	}
	var resp struct {
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("decode PR branch: %w", err)
	}
	return strings.TrimSpace(resp.HeadRefName), nil
}

func runGHJSON(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func renderBranchName(pattern string, issueNumber int) string {
	branch := strings.TrimSpace(pattern)
	if branch == "" {
		branch = "issue_#<issue番号>"
	}
	issueNumberText := strconv.Itoa(issueNumber)
	branch = strings.NewReplacer(
		"<issue番号>", issueNumberText,
		"<issueNumber>", issueNumberText,
		"{issue番号}", issueNumberText,
		"{issueNumber}", issueNumberText,
	).Replace(branch)
	return strings.TrimSpace(branch)
}
