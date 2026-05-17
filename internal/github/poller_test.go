package github

import (
	"context"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type stubRepositoryLister struct {
	issues        []domain.RepositoryItem
	projectIssues []domain.RepositoryItem
	prs           []domain.RepositoryItem
}

func (s stubRepositoryLister) ListIssues(context.Context, string, time.Time) ([]domain.RepositoryItem, error) {
	return s.issues, nil
}

func (s stubRepositoryLister) ListProjectIssues(context.Context, string, time.Time) ([]domain.RepositoryItem, error) {
	return s.projectIssues, nil
}

func (s stubRepositoryLister) ListPullRequests(context.Context, string, time.Time) ([]domain.RepositoryItem, error) {
	return s.prs, nil
}

func TestPollerPollMatchedIssue(t *testing.T) {
	t.Parallel()

	poller := NewPoller(stubRepositoryLister{
		issues: []domain.RepositoryItem{
			{
				Repository: "owner/repo",
				Number:     10,
				Title:      "Feature",
				Labels:     []string{"ai:design"},
				Target:     domain.TargetIssue,
				UpdatedAt:  time.Now().UTC(),
			},
		},
	}, func() []config.WatchRule {
		return []config.WatchRule{
			{
				ID:           "rule-1",
				Name:         "Issue Rule",
				Enabled:      true,
				Repositories: []string{"owner/repo"},
				Target:       "issue",
				Labels:       []string{"ai:design"},
			},
		}
	}, nil)

	events, err := poller.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestPollerPollMatchedProjectIssue(t *testing.T) {
	t.Parallel()

	poller := NewPoller(stubRepositoryLister{
		projectIssues: []domain.RepositoryItem{
			{
				Repository: "owner/repo",
				Number:     11,
				Title:      "Feature",
				Labels:     []string{"ai:design"},
				Target:     domain.TargetIssueProject,
				ProjectCards: []domain.ProjectCard{
					{
						Project: "Roadmap",
						Fields: []domain.ProjectField{
							{Name: "Status", Value: "Ready"},
						},
					},
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}, func() []config.WatchRule {
		return []config.WatchRule{
			{
				ID:           "rule-1",
				Name:         "Project Issue Rule",
				Enabled:      true,
				Repositories: []string{"owner/repo"},
				Target:       "issue_project",
				ProjectName:  "Roadmap",
				ProjectFilters: []config.ProjectFieldFilter{
					{Field: "Status", Values: []string{"Ready"}},
				},
				Labels: []string{"ai:design"},
			},
		}
	}, nil)

	events, err := poller.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != domain.DomainEventIssueMatched {
		t.Fatalf("expected issue_matched, got %s", events[0].Type)
	}
}
