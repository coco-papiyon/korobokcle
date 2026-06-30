package app

import (
	"context"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestClassifyIssue(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		wantK  domain.JobKind
		wantS  domain.JobState
	}{
		{name: "default", labels: nil, wantK: domain.JobKindIssueDesign, wantS: domain.StateDetected},
		{name: "design approved", labels: []string{"state:design_approved"}, wantK: domain.JobKindIssueImplementation, wantS: domain.StateDesignApproved},
		{name: "review fix design approved", labels: []string{"state:review_fix_design_approved"}, wantK: domain.JobKindIssueImplementation, wantS: domain.StateReviewFixDesignApproved},
	}

	for _, tt := range tests {
		gotK, gotS := classifyIssue(tt.labels)
		if gotK != tt.wantK || gotS != tt.wantS {
			t.Fatalf("%s: classifyIssue() = (%s, %s), want (%s, %s)", tt.name, gotK, gotS, tt.wantK, tt.wantS)
		}
	}
}

func TestClassifyPullRequestReviewComment(t *testing.T) {
	kind, state := classifyPullRequest([]string{"state:pr_review_comment"})
	if kind != domain.JobKindPRFeedback || state != domain.StatePRReviewComment {
		t.Fatalf("classifyPullRequest() = (%s, %s), want (%s, %s)", kind, state, domain.JobKindPRFeedback, domain.StatePRReviewComment)
	}
}

func TestGitHubSourceEmptyRepository(t *testing.T) {
	src := NewGitHubSource(nil, "", nil)
	jobs, err := src.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs = %d, want 0", len(jobs))
	}
}

func TestSearchConditionMatches(t *testing.T) {
	cond := domain.SearchCondition{
		LabelIncludes: []string{"bug"},
		TitleContains: []string{"fix"},
		Authors:       []string{"alice"},
		Assignees:     []string{"bob"},
	}
	if !cond.Matches("Fix crash", []string{"bug", "state:detected"}, "alice", []string{"bob"}) {
		t.Fatal("expected condition to match")
	}
	if cond.Matches("Fix crash", []string{"enhancement"}, "alice", []string{"bob"}) {
		t.Fatal("expected label include mismatch")
	}
}
