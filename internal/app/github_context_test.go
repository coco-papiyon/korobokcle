package app

import "testing"

func TestBranchIssueNumber(t *testing.T) {
	tests := []struct {
		branch string
		want   int
	}{
		{branch: "issue_#114", want: 114},
		{branch: "issue-119", want: 119},
		{branch: "feature/issue_120-conflict", want: 120},
		{branch: "main", want: 0},
	}
	for _, tt := range tests {
		if got := branchIssueNumber(tt.branch); got != tt.want {
			t.Errorf("branchIssueNumber(%q) = %d, want %d", tt.branch, got, tt.want)
		}
	}
}
