package naming

import (
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestRenderPRTitleUsesTemplate(t *testing.T) {
	t.Parallel()

	job := domain.Job{Repository: "owner/repo", GitHubNumber: 12, Title: "Implement feature"}
	got := RenderPRTitle("PR {{issue_number}}: {{issue_title}}", job)
	if got != "PR 12: Implement feature" {
		t.Fatalf("unexpected title %q", got)
	}
}

func TestRenderPRTitleFallsBackToDefault(t *testing.T) {
	t.Parallel()

	job := domain.Job{Repository: "owner/repo", GitHubNumber: 12, Title: "Implement feature"}
	got := RenderPRTitle("", job)
	if got != "[#12]Implement feature" {
		t.Fatalf("unexpected title %q", got)
	}
}

func TestRenderBranchNameUsesTemplate(t *testing.T) {
	t.Parallel()

	item := domain.RepositoryItem{Repository: "owner/repo", Number: 12, Title: "Implement feature"}
	got := RenderBranchName("feature_{{issue_number}}", item)
	if got != "feature_12" {
		t.Fatalf("unexpected branch %q", got)
	}
}

func TestRenderBranchNameFallsBackToDefault(t *testing.T) {
	t.Parallel()

	item := domain.RepositoryItem{Repository: "owner/repo", Number: 12, Title: "Implement feature"}
	got := RenderBranchName("", item)
	if got != "issue_12" {
		t.Fatalf("unexpected branch %q", got)
	}
}
