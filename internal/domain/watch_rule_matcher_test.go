package domain

import (
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestEvaluateWatchRuleMatched(t *testing.T) {
	t.Parallel()

	rule := config.WatchRule{
		Enabled:      true,
		Repositories: []string{"owner/repo"},
		Target:       "issue",
		Labels:       []string{"ai:design"},
	}
	item := RepositoryItem{
		Repository: "owner/repo",
		Target:     TargetIssue,
		Labels:     []string{"ai:design", "bug"},
		UpdatedAt:  time.Now(),
	}

	result := EvaluateWatchRule(rule, item)
	if result.Status != MatchStatusMatched {
		t.Fatalf("expected matched, got %s", result.Status)
	}
}

func TestEvaluateWatchRuleDraftExcluded(t *testing.T) {
	t.Parallel()

	rule := config.WatchRule{
		Enabled:        true,
		Repositories:   []string{"owner/repo"},
		Target:         "pull_request",
		ExcludeDraftPR: true,
	}
	item := RepositoryItem{
		Repository: "owner/repo",
		Target:     TargetPullRequest,
		Draft:      true,
	}

	result := EvaluateWatchRule(rule, item)
	if result.Status != MatchStatusIgnored {
		t.Fatalf("expected ignored, got %s", result.Status)
	}
}

func TestEvaluateWatchRuleMatchedWithRepositoryURLAndAssignee(t *testing.T) {
	t.Parallel()

	rule := config.WatchRule{
		Enabled:      true,
		Repositories: []string{"https://github.com/coco-papiyon/korobokcle"},
		Target:       "issue",
		Assignees:    []string{"coco-papiyon"},
	}
	item := RepositoryItem{
		Repository: "coco-papiyon/korobokcle",
		Target:     TargetIssue,
		Assignees:  []string{"coco-papiyon"},
		UpdatedAt:  time.Now(),
	}

	result := EvaluateWatchRule(rule, item)
	if result.Status != MatchStatusMatched {
		t.Fatalf("expected matched, got %s", result.Status)
	}
}
