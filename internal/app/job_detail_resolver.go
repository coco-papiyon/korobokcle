package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobDetailResolver struct {
	settings SettingsStore
}

func NewJobDetailResolver(settings SettingsStore) *JobDetailResolver {
	return &JobDetailResolver{settings: settings}
}

func (r *JobDetailResolver) ResolveJobBranch(ctx context.Context, job domain.Job) (string, error) {
	if branch := strings.TrimSpace(job.Branch); branch != "" {
		return branch, nil
	}
	switch job.Kind {
	case domain.JobKindPRReview, domain.JobKindPRAcceptance, domain.JobKindPRFeedback, domain.JobKindPRConflict:
		return resolvePRBranch(ctx, job)
	default:
		return resolveIssueBranch(ctx, r.settings, job)
	}
}

func resolveIssueBranch(ctx context.Context, settingsStore SettingsStore, job domain.Job) (string, error) {
	if settingsStore == nil {
		return "", nil
	}
	settings, err := settingsStore.Load(ctx)
	if err != nil {
		return "", err
	}
	settings = domain.NormalizeWatchSettings(settings)
	return renderBranchName(settings.BranchNamePattern, job.Number), nil
}

func resolvePRBranch(ctx context.Context, job domain.Job) (string, error) {
	raw, err := runGHJSON(ctx, "pr", "view", "--repo", job.Repository, fmt.Sprintf("%d", job.Number), "--json", "headRefName")
	if err != nil {
		return "", err
	}
	var pr struct {
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(raw, &pr); err != nil {
		return "", fmt.Errorf("decode PR branch: %w", err)
	}
	return strings.TrimSpace(pr.HeadRefName), nil
}
