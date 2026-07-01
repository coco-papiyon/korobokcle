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
		{name: "review fix implementation approved", labels: []string{"state:review_fix_implementation_approved"}, wantK: domain.JobKindIssueImplementation, wantS: domain.StateReviewFixImplementationApproved},
		{name: "review fixed falls back", labels: []string{"state:review_fixed"}, wantK: domain.JobKindIssueDesign, wantS: domain.StateDetected},
	}

	for _, tt := range tests {
		gotK, gotS := classifyIssue(tt.labels)
		if gotK != tt.wantK || gotS != tt.wantS {
			t.Fatalf("%s: classifyIssue() = (%s, %s), want (%s, %s)", tt.name, gotK, gotS, tt.wantK, tt.wantS)
		}
	}
}

func TestClassifyPullRequest(t *testing.T) {
	tests := []struct {
		name             string
		labels           []string
		mergeable        string
		mergeStateStatus string
		wantK            domain.JobKind
		wantS            domain.JobState
	}{
		{name: "default", labels: nil, wantK: domain.JobKindPRReview, wantS: domain.StateReviewRunning},
		{name: "review fixed", labels: []string{"state:review_fixed"}, wantK: domain.JobKindPRReview, wantS: domain.StateReviewRunning},
		{name: "review comment", labels: []string{"state:pr_review_comment"}, wantK: domain.JobKindPRFeedback, wantS: domain.StatePRReviewComment},
		{name: "review fix implementation running", labels: []string{"state:review_fix_implementation_running"}, wantK: domain.JobKindPRFeedback, wantS: domain.StateReviewFixImplementationRunning},
		{name: "review fix implementation ready", labels: []string{"state:review_fix_implementation_ready"}, wantK: domain.JobKindPRFeedback, wantS: domain.StateReviewFixImplementationReady},
		{name: "review fix implementation approved", labels: []string{"state:review_fix_implementation_approved"}, wantK: domain.JobKindPRFeedback, wantS: domain.StateReviewFixImplementationApproved},
		{name: "review fix design approved", labels: []string{"state:review_fix_design_approved"}, wantK: domain.JobKindPRFeedback, wantS: domain.StateReviewFixDesignApproved},
		{name: "implementation label wins", labels: []string{"state:pr_review_comment", "state:review_fix_implementation_ready"}, wantK: domain.JobKindPRFeedback, wantS: domain.StateReviewFixImplementationReady},
		{name: "approved label wins", labels: []string{"state:pr_review_comment", "state:review_fix_design_approved"}, wantK: domain.JobKindPRFeedback, wantS: domain.StateReviewFixDesignApproved},
		{name: "review fixed wins", labels: []string{"state:pr_review_comment", "state:review_fixed"}, wantK: domain.JobKindPRReview, wantS: domain.StateReviewRunning},
		{name: "conflicting", mergeable: "CONFLICTING", wantK: domain.JobKindPRConflict, wantS: domain.StatePRConflict},
		{name: "dirty merge state", mergeStateStatus: "DIRTY", wantK: domain.JobKindPRConflict, wantS: domain.StatePRConflict},
	}

	for _, tt := range tests {
		record := ghPRRecord{Mergeable: tt.mergeable, MergeStateStatus: tt.mergeStateStatus}
		for _, label := range tt.labels {
			record.Labels = append(record.Labels, struct {
				Name string `json:"name"`
			}{Name: label})
		}
		kind, state := classifyPullRequest(record)
		if kind != tt.wantK || state != tt.wantS {
			t.Fatalf("%s: classifyPullRequest() = (%s, %s), want (%s, %s)", tt.name, kind, state, tt.wantK, tt.wantS)
		}
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

func TestJobIDForPRConflict(t *testing.T) {
	conflict := ghPRRecord{Number: 42, Mergeable: "CONFLICTING"}
	if got := jobIDForPR(conflict); got != "pr-conflict-42" {
		t.Fatalf("jobIDForPR() = %q, want pr-conflict-42", got)
	}
	regular := ghPRRecord{Number: 42, Mergeable: "MERGEABLE"}
	if got := jobIDForPR(regular); got != "pr-42" {
		t.Fatalf("jobIDForPR() = %q, want pr-42", got)
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
